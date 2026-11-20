package app

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
	"github.com/fsnotify/fsnotify"

	"pdf-watcher/assets"
	"pdf-watcher/internal/config"
	"pdf-watcher/internal/i18n"
	"pdf-watcher/internal/index"
	"pdf-watcher/internal/logx"
	"pdf-watcher/internal/meta"
	"pdf-watcher/internal/pipeline"
	"pdf-watcher/internal/win"
	"pdf-watcher/internal/zotero"
)

type App struct {
	cfg     *config.Manager
	log     *logx.Logger
	idx     *index.Store
	zc      *zotero.Client
	pipe    *pipeline.Pipeline
	watch   *WatcherHub
	exePath string

	menuMu    sync.Mutex
	trayReady atomic.Bool
}

func Run() error {
	exe, _ := os.Executable()
	exe, _ = filepath.Abs(exe)

	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	log, err := logx.New(cfg.LogsDir())
	if err != nil {
		return err
	}
	idx, err := index.Open(cfg.IndexPath())
	if err != nil {
		return err
	}

	i18n.ApplyConfigOrSystem(cfg.Get().UI.Language, win.SystemUILang())

	zc := zotero.New(cfg.Get().Zotero.LocalAPIURL)
	zc.ServerID = cfg.Get().Zotero.ServerID
	zc.APIKey = cfg.Get().Zotero.APIKey
	zc.OnPersistAuth = func(serverID, apiKey string) {
		_ = cfg.Update(func(c *config.Config) {
			c.Zotero.ServerID = serverID
			c.Zotero.APIKey = apiKey
		})
	}
	zc.OnClearKey = func() {
		_ = cfg.Update(func(c *config.Config) {
			c.Zotero.APIKey = ""
		})
	}

	c0 := cfg.Get()
	res := &meta.Resolver{
		Crossref: meta.NewCrossref(),
		PubMed:   meta.NewPubMed(c0.PubMed.Tool, c0.PubMed.Email, c0.PubMed.TimeoutSec),
		AI:       meta.NewOpenAI(c0.AI.APIURL, c0.AI.APIKey, c0.AI.Model, c0.AI.TimeoutSec),
		AIOn:     c0.AI.Enable,
	}

	a := &App{cfg: cfg, log: log, idx: idx, zc: zc, exePath: exe}
	pipe := pipeline.New(cfg.Get, log, idx, zc, res, win.TryExclusiveOpen)
	pipe.PersistCollectionID = a.persistCollectionID
	a.pipe = pipe
	a.watch = NewWatcherHub(log, pipe)

	cfg.OnChange(func(c config.Config) {
		i18n.ApplyConfigOrSystem(c.UI.Language, win.SystemUILang())
		res.AI = meta.NewOpenAI(c.AI.APIURL, c.AI.APIKey, c.AI.Model, c.AI.TimeoutSec)
		res.AIOn = c.AI.Enable
		res.PubMed = meta.NewPubMed(c.PubMed.Tool, c.PubMed.Email, c.PubMed.TimeoutSec)
		a.watch.Rebuild(c)
		a.rebuildMenu()
	})

	go a.watchConfigFile()
	a.watch.Rebuild(cfg.Get())

	// sync autostart on first run
	if cfg.Get().System.AutoStart {
		_ = win.SetAutoStartReg(config.AppName, exe, true)
	}

	systray.Run(a.onReady, a.onExit)
	return nil
}

func (a *App) onReady() {
	systray.SetIcon(assets.Icon)
	systray.SetTooltip(i18n.T("tray.tooltip"))
	systray.SetOnRClick(func(menu systray.IMenu) { menu.ShowMenu() })
	a.trayReady.Store(true)
	a.rebuildMenu()
	go func() { _ = a.reconcileCollections() }()
}

func (a *App) onExit() {
	a.watch.Close()
	_ = a.idx.Close()
	_ = a.log.Close()
}

func (a *App) rebuildMenu() {
	if !a.trayReady.Load() {
		return
	}
	a.menuMu.Lock()
	defer a.menuMu.Unlock()
	systray.ResetMenu()
	systray.SetTooltip(i18n.T("tray.tooltip"))

	cfg := a.cfg.Get()

	mAdd := systray.AddMenuItem(i18n.T("tray.addWatch"), "")
	mAdd.Click(func() { go a.addWatch() })

	mWatched := systray.AddMenuItem(i18n.T("tray.watched"), "")
	anyDir := false
	for i, d := range cfg.WatchDirs {
		if d.Path == "" {
			continue
		}
		anyDir = true
		idx := i
		sub := mWatched.AddSubMenuItem(d.Path, "")
		if d.Enable {
			p := sub.AddSubMenuItem(i18n.T("tray.pauseDir"), "")
			p.Click(func() { a.setDirEnable(idx, false) })
		} else {
			p := sub.AddSubMenuItem(i18n.T("tray.resumeDir"), "")
			p.Click(func() { a.setDirEnable(idx, true) })
		}
		rm := sub.AddSubMenuItem(i18n.T("tray.removeDir"), "")
		rm.Click(func() { a.removeDir(idx) })
	}
	if !anyDir {
		mWatched.AddSubMenuItem(i18n.T("tray.emptyWatch"), "").Disable()
	}

	systray.AddSeparator()
	if a.pipe.Paused() {
		m := systray.AddMenuItem(i18n.T("tray.resumeAll"), "")
		m.Click(func() { a.pipe.SetPaused(false); a.rebuildMenu() })
	} else {
		m := systray.AddMenuItem(i18n.T("tray.pauseAll"), "")
		m.Click(func() { a.pipe.SetPaused(true); a.rebuildMenu() })
	}

	mCfg := systray.AddMenuItem(i18n.T("tray.openConfig"), "")
	mCfg.Click(func() { _ = win.OpenNotepad(a.cfg.Path()) })
	mLogs := systray.AddMenuItem(i18n.T("tray.openLogs"), "")
	mLogs.Click(func() { _ = win.OpenExplorer(a.cfg.LogsDir()) })

	mHelp := systray.AddMenuItem(i18n.T("tray.help"), "")
	mGuideZh := mHelp.AddSubMenuItem(i18n.T("tray.openGuideZh"), "")
	mGuideZh.Click(func() { go a.openGuide("zh") })
	mGuideEn := mHelp.AddSubMenuItem(i18n.T("tray.openGuideEn"), "")
	mGuideEn.Click(func() { go a.openGuide("en") })
	mColl := mHelp.AddSubMenuItem(i18n.T("tray.exportCollections"), "")
	mColl.Click(func() { go a.exportCollectionsCSV() })

	mLang := systray.AddMenuItem(i18n.T("tray.language"), "")
	mZh := mLang.AddSubMenuItemCheckbox(i18n.T("tray.lang.zh"), "", i18n.Get() == i18n.ZhCN)
	mEn := mLang.AddSubMenuItemCheckbox(i18n.T("tray.lang.en"), "", i18n.Get() == i18n.En)
	mZh.Click(func() { a.setLang("zh-CN") })
	mEn.Click(func() { a.setLang("en") })

	autoOn := win.AutoStartEnabledReg(config.AppName)
	mAuto := systray.AddMenuItemCheckbox(i18n.T("tray.autoStart"), "", autoOn)
	mAuto.Click(func() {
		next := !win.AutoStartEnabledReg(config.AppName)
		_ = win.SetAutoStartReg(config.AppName, a.exePath, next)
		_ = a.cfg.Update(func(c *config.Config) { c.System.AutoStart = next })
		a.rebuildMenu()
	})

	systray.AddSeparator()
	mQuit := systray.AddMenuItem(i18n.T("tray.quit"), "")
	mQuit.Click(func() { systray.Quit() })
}

func (a *App) setLang(lang string) {
	_ = a.cfg.Update(func(c *config.Config) { c.UI.Language = lang })
	i18n.Set(lang)
	a.rebuildMenu()
}

// openGuide writes embedded READMEs to APPDATA and opens one in Notepad.
// which: "zh" or "en"
func (a *App) openGuide(which string) {
	zhPath, enPath, err := assets.EnsureGuides(a.cfg.Dir())
	if err != nil {
		a.log.Error("-", "guide", err.Error())
		return
	}
	path := enPath
	if which == "zh" {
		path = zhPath
	}
	if err := win.OpenNotepad(path); err != nil {
		a.log.Error("-", "guide", err.Error())
	}
}

func (a *App) persistCollectionID(watchDir, collectionName, collectionID string) {
	_ = a.cfg.Update(func(c *config.Config) {
		for i := range c.WatchDirs {
			if _, ok := config.FindWatchDir([]config.WatchDir{c.WatchDirs[i]}, watchDir); ok {
				c.WatchDirs[i].CollectionID = collectionID
				if collectionName != "" {
					c.WatchDirs[i].CollectionName = collectionName
				}
				return
			}
		}
	})
}

func (a *App) reconcileCollections() error {
	var renames []config.CollRename
	err := a.cfg.Update(func(c *config.Config) {
		var r []config.CollRename
		c.WatchDirs, r = config.ReconcileWatchCollections(c.WatchDirs)
		renames = r
	})
	if err != nil {
		return err
	}
	for _, r := range renames {
		if r.CollectionID == "" || r.NewName == "" {
			continue
		}
		if err := a.zc.RenameCollection(r.CollectionID, r.NewName); err != nil {
			a.log.Warn("-", "collection_rename", err.Error())
		} else {
			a.log.Infof("-", "collection_rename", "id=%s name=%s", r.CollectionID, r.NewName)
		}
	}
	return nil
}

func (a *App) addWatch() {
	dir, err := win.PickFolder(i18n.T("tray.addWatch"))
	if err != nil || dir == "" {
		return
	}
	_ = a.cfg.Update(func(c *config.Config) {
		for i := range c.WatchDirs {
			if c.WatchDirs[i].Path == "" {
				c.WatchDirs[i] = config.WatchDir{Path: dir, Enable: true}
				return
			}
		}
		c.WatchDirs = append(c.WatchDirs, config.WatchDir{Path: dir, Enable: true})
	})
	_ = a.reconcileCollections()
}

func (a *App) setDirEnable(i int, enable bool) {
	_ = a.cfg.Update(func(c *config.Config) {
		if i >= 0 && i < len(c.WatchDirs) {
			c.WatchDirs[i].Enable = enable
		}
	})
	_ = a.reconcileCollections()
}

func (a *App) removeDir(i int) {
	_ = a.cfg.Update(func(c *config.Config) {
		if i >= 0 && i < len(c.WatchDirs) {
			c.WatchDirs = append(c.WatchDirs[:i], c.WatchDirs[i+1:]...)
		}
	})
	_ = a.reconcileCollections()
}

func (a *App) exportCollectionsCSV() {
	if _, err := a.zc.Probe(); err != nil {
		a.log.Error("-", "collections", err.Error())
		return
	}
	cols, err := a.zc.ListCollectionsParsed()
	if err != nil {
		a.log.Error("-", "collections", err.Error())
		return
	}
	byKey := make(map[string]zotero.Collection, len(cols))
	for _, c := range cols {
		byKey[c.Key] = c
	}
	pathFn := func(c zotero.Collection) string {
		parts := []string{c.Name}
		seen := map[string]bool{c.Key: true}
		p := c.Parent
		for p != "" && !seen[p] {
			seen[p] = true
			parent, ok := byKey[p]
			if !ok {
				break
			}
			parts = append([]string{parent.Name}, parts...)
			p = parent.Parent
		}
		return strings.Join(parts, "/")
	}

	outPath := filepath.Join(a.cfg.Dir(), "zotero-collections.csv")
	f, err := os.Create(outPath)
	if err != nil {
		a.log.Error("-", "collections", err.Error())
		return
	}
	// UTF-8 BOM so Excel on Chinese Windows shows Chinese correctly.
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		_ = f.Close()
		a.log.Error("-", "collections", err.Error())
		return
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"path", "name", "key", "parentKey"})
	for _, c := range cols {
		_ = w.Write([]string{pathFn(c), c.Name, c.Key, c.Parent})
	}
	w.Flush()
	cerr := w.Error()
	_ = f.Close()
	if cerr != nil {
		a.log.Error("-", "collections", cerr.Error())
		return
	}
	a.log.Infof("-", "collections", "exported=%s count=%d", outPath, len(cols))
	if err := win.OpenFile(outPath); err != nil {
		a.log.Error("-", "collections", err.Error())
	}
}

func (a *App) watchConfigFile() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	_ = w.Add(filepath.Dir(a.cfg.Path()))
	deb := config.Debounce(400*time.Millisecond, func() {
		_ = a.cfg.ReloadFromDisk()
	})
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if filepath.Base(ev.Name) == "config.yaml" {
				deb()
			}
		case <-w.Errors:
		}
	}
}
