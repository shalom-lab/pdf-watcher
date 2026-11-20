package app

import (
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"

	"pdf-watcher/internal/config"
	"pdf-watcher/internal/filter"
	"pdf-watcher/internal/logx"
	"pdf-watcher/internal/pipeline"
	"pdf-watcher/internal/win"
)

type WatcherHub struct {
	log  *logx.Logger
	pipe *pipeline.Pipeline
	mu   sync.Mutex
	w    *fsnotify.Watcher
	dirs map[string]config.WatchDir
}

func NewWatcherHub(log *logx.Logger, pipe *pipeline.Pipeline) *WatcherHub {
	return &WatcherHub{log: log, pipe: pipe, dirs: map[string]config.WatchDir{}}
}

func (h *WatcherHub) Rebuild(cfg config.Config) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.w != nil {
		_ = h.w.Close()
		h.w = nil
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		h.log.Error("-", "watch", err.Error())
		return
	}
	h.w = w
	h.dirs = map[string]config.WatchDir{}
	for _, d := range cfg.WatchDirs {
		if d.Path == "" || !d.Enable {
			continue
		}
		abs, err := filepath.Abs(d.Path)
		if err != nil {
			continue
		}
		if err := w.Add(abs); err != nil {
			h.log.Warn(abs, "watch", err.Error())
			continue
		}
		h.dirs[abs] = d
		h.log.Info(abs, "watch", "added")
	}
	go h.loop(cfg)
}

func (h *WatcherHub) loop(cfg config.Config) {
	w := h.w
	if w == nil {
		return
	}
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
				continue
			}
			path := ev.Name
			if win.IsHiddenOrSystem(path) {
				continue
			}
			h.mu.Lock()
			watchAbs, dirCfg, ok := h.matchDir(path)
			h.mu.Unlock()
			if !ok {
				continue
			}
			if !filter.ShouldProcess(path, dirCfg.IgnoreNames) {
				continue
			}
			h.pipe.Enqueue(pipeline.Job{
				Path:         path,
				WatchDir:     watchAbs,
				CollectionID: dirCfg.CollectionID,
				IgnoreNames:  dirCfg.IgnoreNames,
			})
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			h.log.Warn("-", "watch", err.Error())
		}
	}
}

func (h *WatcherHub) matchDir(path string) (watchAbs string, d config.WatchDir, ok bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", config.WatchDir{}, false
	}
	dir := filepath.Dir(abs)
	d, ok = h.dirs[dir]
	return dir, d, ok
}

func (h *WatcherHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.w != nil {
		_ = h.w.Close()
		h.w = nil
	}
}
