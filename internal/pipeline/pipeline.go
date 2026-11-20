package pipeline

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"pdf-watcher/internal/config"
	"pdf-watcher/internal/extract"
	"pdf-watcher/internal/index"
	"pdf-watcher/internal/logx"
	"pdf-watcher/internal/meta"
	"pdf-watcher/internal/stable"
	"pdf-watcher/internal/zotero"
)

type Job struct {
	Path         string
	WatchDir     string // monitored folder; used for pdf-watcher/<folderName> collection
	CollectionID string // explicit override; empty => auto folder collection
	IgnoreNames  []string
}

type Pipeline struct {
	Log       *logx.Logger
	Index     *index.Store
	Zotero    *zotero.Client
	Resolver  *meta.Resolver
	Exclusive stable.ExclusiveOpener

	getCfg func() config.Config
	// PersistCollectionID writes Zotero collection key back into watchDirs config.
	PersistCollectionID func(watchDir, collectionName, collectionID string)
	// OnZoteroReachable fires when Local API reachability flips (true=up).
	OnZoteroReachable func(ok bool)

	sem      chan struct{}
	queue    chan Job
	inflight sync.Map // path -> struct{}
	keyLocks sync.Map // assoc key -> *sync.Mutex

	pausedMu sync.RWMutex
	paused   bool

	zoteroOK atomic.Bool
	backoff  time.Duration
}

func New(getCfg func() config.Config, log *logx.Logger, idx *index.Store, zc *zotero.Client, res *meta.Resolver, excl stable.ExclusiveOpener) *Pipeline {
	p := &Pipeline{
		Log:       log,
		Index:     idx,
		Zotero:    zc,
		Resolver:  res,
		Exclusive: excl,
		getCfg:    getCfg,
		sem:       make(chan struct{}, 2),
		queue:     make(chan Job, 256),
		backoff:   5 * time.Second,
	}
	go p.loop()
	go p.probeLoop()
	return p
}

func (p *Pipeline) SetPaused(v bool) {
	p.pausedMu.Lock()
	p.paused = v
	p.pausedMu.Unlock()
}

func (p *Pipeline) Paused() bool {
	p.pausedMu.RLock()
	defer p.pausedMu.RUnlock()
	return p.paused
}

func (p *Pipeline) ZoteroOK() bool { return p.zoteroOK.Load() }

func (p *Pipeline) setZoteroOK(ok bool) {
	prev := p.zoteroOK.Swap(ok)
	if prev == ok {
		return
	}
	if p.OnZoteroReachable != nil {
		p.OnZoteroReachable(ok)
	}
}

func (p *Pipeline) Enqueue(job Job) {
	if p.Paused() {
		p.Log.Info(job.Path, "queue", "skipped: global pause")
		return
	}
	cfg := p.getCfg()
	if len(p.queue) >= cfg.System.MaxQueue {
		p.Log.Error(job.Path, "queue", "full; drop")
		return
	}
	if _, loaded := p.inflight.LoadOrStore(job.Path, struct{}{}); loaded {
		return
	}
	select {
	case p.queue <- job:
	default:
		p.inflight.Delete(job.Path)
		p.Log.Error(job.Path, "queue", "full; drop")
	}
}

func (p *Pipeline) loop() {
	for job := range p.queue {
		p.sem <- struct{}{}
		go func(j Job) {
			defer func() {
				<-p.sem
				p.inflight.Delete(j.Path)
			}()
			p.process(j)
		}(job)
	}
}

func (p *Pipeline) probeLoop() {
	var lastErr string
	for {
		_, err := p.Zotero.Probe()
		if err != nil {
			p.setZoteroOK(false)
			msg := err.Error()
			if msg != lastErr {
				p.Log.Warn("-", "zotero", "probe: "+msg)
				lastErr = msg
			}
			time.Sleep(p.backoff)
			if p.backoff < 60*time.Second {
				p.backoff *= 2
				if p.backoff > 60*time.Second {
					p.backoff = 60 * time.Second
				}
			}
			continue
		}
		wasDown := lastErr != ""
		p.setZoteroOK(true)
		if wasDown {
			p.Log.Info("-", "zotero", "probe ok; resumed")
			lastErr = ""
		}
		p.backoff = 5 * time.Second
		time.Sleep(15 * time.Second)
	}
}

func (p *Pipeline) process(job Job) {
	cfg := p.getCfg()
	path := job.Path
	p.Log.Info(path, "start", "begin")

	if !p.ZoteroOK() {
		if _, err := p.Zotero.Probe(); err != nil {
			p.setZoteroOK(false)
			p.Log.Warn(path, "zotero", "unavailable; requeue until Zotero is up: "+err.Error())
			time.AfterFunc(p.backoff, func() { p.Enqueue(job) })
			return
		}
		p.setZoteroOK(true)
	}

	err := stable.WaitReady(path, stable.Options{
		Wait:      time.Duration(cfg.System.StableWaitMs) * time.Millisecond,
		Poll:      time.Duration(cfg.System.StablePollMs) * time.Millisecond,
		Timeout:   time.Duration(cfg.System.StableTimeoutMs) * time.Millisecond,
		Exclusive: p.Exclusive,
	})
	if err != nil {
		p.Log.Error(path, "stable", err.Error())
		return
	}

	if err := extract.CheckMagic(path); err != nil {
		p.Log.Warn(path, "magic", err.Error())
		return
	}

	hash, err := zotero.FileMD5(path)
	if err != nil {
		p.Log.Error(path, "hash", err.Error())
		return
	}

	serverID := p.Zotero.ServerID
	if serverID == "" {
		if sid, err := p.Zotero.Probe(); err == nil {
			serverID = sid
			p.setZoteroOK(true)
		} else {
			p.setZoteroOK(false)
			p.Log.Warn(path, "zotero", "unavailable; requeue: "+err.Error())
			time.AfterFunc(p.backoff, func() { p.Enqueue(job) })
			return
		}
	}

	if existing, _ := p.Index.FindByHash(serverID, hash); existing != nil {
		p.Log.Info(path, "skip", "same file hash")
		return
	}

	ex := extract.FromFile(path)
	if ex.Reason != "" {
		p.Log.Warn(path, "extract", ex.Reason)
	} else {
		p.Log.Infof(path, "extract", "chars=%d", len(ex.Text))
	}

	ids := meta.MergeIDs(meta.ExtractFrontIDs(ex.Text), meta.ExtractIDsFromPath(path))
	p.Log.Infof(path, "ids", "doi=%s pmid=%s pmcid=%s", ids.DOI, ids.PMID, ids.PMCID)
	assocKey := assocLockKey(ids, "")
	unlock := p.lockKey(assocKey)
	defer unlock()

	if hit := p.findByIDs(serverID, ids); hit != nil {
		p.attach(path, hash, hit.ItemKey, meta.Record{DOI: ids.DOI, PMID: ids.PMID, PMCID: ids.PMCID, Source: "index"}, job, serverID)
		return
	}

	rec := p.Resolver.ResolveFull(meta.ResolveInput{Path: path, Text: ex.Text})
	if rec.DOI == "" {
		rec.DOI = ids.DOI
	}
	if rec.PMID == "" {
		rec.PMID = ids.PMID
	}
	if rec.PMCID == "" {
		rec.PMCID = ids.PMCID
	}
	p.Log.Infof(path, "meta", "source=%s title=%q doi=%s pmid=%s", rec.Source, rec.Title, rec.DOI, rec.PMID)
	if rec.Warn != "" {
		p.Log.Warn(path, "meta", rec.Warn)
	}

	titleNorm := ""
	if meta.CanMatchByTitle(rec.Title, rec.Source) {
		titleNorm = meta.NormalizeTitle(rec.Title)
		if hit, _ := p.Index.FindByTitleNorm(serverID, titleNorm); hit != nil {
			p.attach(path, hash, hit.ItemKey, rec, job, serverID)
			return
		}
	}

	// create new
	coll, err := p.resolveCollection(job, cfg)
	if err != nil {
		p.handleZErr(path, "collection", err, job)
		return
	}
	parentKey, err := p.Zotero.CreateParent(rec, coll)
	if err != nil {
		p.handleZErr(path, "create_parent", err, job)
		return
	}
	attKey, err := p.Zotero.CreateAttachment(parentKey, path)
	if err != nil {
		p.handleZErr(path, "create_att", err, job)
		return
	}
	if err := p.Zotero.UploadFile(attKey, path); err != nil {
		_ = p.Zotero.DeleteItem(attKey) // avoid empty attachment shell in Zotero
		p.handleZErr(path, "upload", err, job)
		return
	}
	_ = p.Index.Insert(index.Record{
		ServerID: serverID, FileHash: hash, Path: path,
		DOI: rec.DOI, PMID: rec.PMID, PMCID: rec.PMCID,
		TitleNorm: titleNorm, ItemKey: parentKey, AttachmentKey: attKey, Source: rec.Source,
	})
	p.Log.Infof(path, "done", "action=create item=%s att=%s source=%s coll=%s", parentKey, attKey, rec.Source, coll)
}

func (p *Pipeline) resolveCollection(job Job, cfg config.Config) (string, error) {
	if job.CollectionID != "" {
		return job.CollectionID, nil
	}
	if wd, ok := config.FindWatchDir(cfg.WatchDirs, job.WatchDir); ok {
		if wd.CollectionID != "" {
			return wd.CollectionID, nil
		}
		name := wd.CollectionName
		if name == "" {
			name = "inbox"
		}
		key, err := p.Zotero.EnsureChildCollection(name)
		if err != nil {
			return "", err
		}
		if p.PersistCollectionID != nil {
			p.PersistCollectionID(job.WatchDir, name, key)
		}
		return key, nil
	}
	if job.WatchDir != "" {
		key, err := p.Zotero.EnsureChildCollection(filepath.Base(job.WatchDir))
		if err != nil {
			return "", err
		}
		return key, nil
	}
	return cfg.Zotero.DefaultCollectionID, nil
}

func (p *Pipeline) attach(path, hash, parentKey string, rec meta.Record, job Job, serverID string) {
	if coll, err := p.resolveCollection(job, p.getCfg()); err == nil && coll != "" {
		_ = p.Zotero.AddItemToCollection(parentKey, coll)
	}
	attKey, err := p.Zotero.CreateAttachment(parentKey, path)
	if err != nil {
		var we *zotero.WriteError
		if errors.As(err, &we) && we.Status == 404 {
			_ = p.Index.DeleteByItemKey(serverID, parentKey)
			p.Log.Warn(path, "attach", "parent 404; index cleared — drop file again to recreate")
			return
		}
		p.Log.Error(path, "attach", err.Error())
		return
	}
	if err := p.Zotero.UploadFile(attKey, path); err != nil {
		_ = p.Zotero.DeleteItem(attKey)
		p.Log.Error(path, "upload", err.Error())
		return
	}
	titleNorm := ""
	if meta.CanMatchByTitle(rec.Title, rec.Source) {
		titleNorm = meta.NormalizeTitle(rec.Title)
	}
	_ = p.Index.Insert(index.Record{
		ServerID: serverID, FileHash: hash, Path: path,
		DOI: rec.DOI, PMID: rec.PMID, PMCID: rec.PMCID,
		TitleNorm: titleNorm, ItemKey: parentKey, AttachmentKey: attKey, Source: rec.Source,
	})
	p.Log.Infof(path, "done", "action=attach item=%s att=%s source=%s", parentKey, attKey, rec.Source)
}

func (p *Pipeline) findByIDs(serverID string, ids meta.IDs) *index.Record {
	if ids.DOI != "" {
		if r, _ := p.Index.FindByDOI(serverID, ids.DOI); r != nil {
			return r
		}
	}
	if ids.PMID != "" {
		if r, _ := p.Index.FindByPMID(serverID, ids.PMID); r != nil {
			return r
		}
	}
	if ids.PMCID != "" {
		if r, _ := p.Index.FindByPMCID(serverID, ids.PMCID); r != nil {
			return r
		}
	}
	return nil
}

func (p *Pipeline) handleZErr(path, stage string, err error, job Job) {
	var we *zotero.WriteError
	if errors.As(err, &we) {
		if we.Denied {
			p.Log.Error(path, stage, "denied")
			return
		}
		if we.Status == 429 {
			p.Log.Warn(path, stage, "429; requeue")
			time.AfterFunc(30*time.Second, func() { p.Enqueue(job) })
			return
		}
	}
	if err.Error() == "one-shot key; need Always Allow" {
		p.Log.Error(path, stage, "need Always Allow")
		return
	}
	// connectivity
	if _, perr := p.Zotero.Probe(); perr != nil {
		p.setZoteroOK(false)
		p.Log.Warn(path, stage, fmt.Sprintf("%v; zotero unavailable, requeue: %v", err, perr))
		time.AfterFunc(p.backoff, func() { p.Enqueue(job) })
		return
	}
	p.Log.Error(path, stage, err.Error())
}

func assocLockKey(ids meta.IDs, titleNorm string) string {
	if ids.DOI != "" {
		return "doi:" + ids.DOI
	}
	if ids.PMID != "" {
		return "pmid:" + ids.PMID
	}
	if ids.PMCID != "" {
		return "pmcid:" + ids.PMCID
	}
	if titleNorm != "" {
		return "title:" + titleNorm
	}
	return "path"
}

func (p *Pipeline) lockKey(key string) func() {
	v, _ := p.keyLocks.LoadOrStore(key, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

func ExtOK(path string) bool {
	ext := filepath.Ext(path)
	return ext != ""
}
