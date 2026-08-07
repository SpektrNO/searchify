package local

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spektr/searchify/internal/config"
)

// IndexWatcher watches SEARCHIFY_WATCH_PATHS and updates the index on changes.
type IndexWatcher struct {
	cfg      *config.Config
	svc      *Service
	debounce time.Duration
	rescan   time.Duration

	mu     sync.Mutex
	timers map[string]*time.Timer
	ops    map[string]watchOp
}

type watchOp int

const (
	opIndex watchOp = iota
	opRemove
)

// NewIndexWatcher returns a watcher when cfg.WatchPaths is non-empty.
func NewIndexWatcher(cfg *config.Config, svc *Service) (*IndexWatcher, error) {
	if len(cfg.WatchPaths) == 0 {
		return nil, fmt.Errorf("no watch paths configured")
	}
	debounce := cfg.WatchDebounce
	if debounce <= 0 {
		debounce = time.Second
	}
	return &IndexWatcher{
		cfg:      cfg,
		svc:      svc,
		debounce: debounce,
		rescan:   cfg.WatchRescan,
		timers:   make(map[string]*time.Timer),
		ops:      make(map[string]watchOp),
	}, nil
}

// Run blocks until ctx is cancelled.
func (w *IndexWatcher) Run(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer watcher.Close()

	for _, p := range w.cfg.WatchPaths {
		if err := w.addRecursive(watcher, p); err != nil {
			slog.Warn("watch add failed", "path", p, "err", err)
		}
	}

	slog.Info("index watch started",
		"paths", len(w.cfg.WatchPaths),
		"debounce", w.debounce.String(),
		"rescan", w.rescan.String(),
	)

	var rescanCh <-chan time.Time
	var rescanTicker *time.Ticker
	if w.rescan > 0 {
		rescanTicker = time.NewTicker(w.rescan)
		defer rescanTicker.Stop()
		rescanCh = rescanTicker.C
	}

	for {
		select {
		case <-ctx.Done():
			w.flushPending()
			slog.Info("index watch stopped")
			return ctx.Err()
		case <-rescanCh:
			w.runRescan()
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			w.handleEvent(watcher, ev)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("watch error", "err", err)
		}
	}
}

func (w *IndexWatcher) addRecursive(watcher *fsnotify.Watcher, root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return watcher.Add(filepath.Dir(root))
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		if err := watcher.Add(path); err != nil {
			slog.Warn("watch add dir failed", "path", path, "err", err)
		}
		return nil
	})
}

func (w *IndexWatcher) handleEvent(watcher *fsnotify.Watcher, ev fsnotify.Event) {
	path := filepath.Clean(ev.Name)
	if !w.cfg.UnderAnyRoot(path) {
		return
	}
	base := filepath.Base(path)
	if shouldSkipDir(base) {
		return
	}

	switch {
	case ev.Has(fsnotify.Create):
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			_ = w.addRecursive(watcher, path)
			return
		}
		if shouldIndexFile(path) {
			w.schedule(path, opIndex)
		}
	case ev.Has(fsnotify.Write) || ev.Has(fsnotify.Chmod):
		// Ignore chmod-only noise if also not a write; still debounce writes.
		if ev.Has(fsnotify.Write) && shouldIndexFile(path) {
			w.schedule(path, opIndex)
		}
	case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
		w.schedule(path, opRemove)
	}
}

func (w *IndexWatcher) schedule(path string, op watchOp) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if existing, ok := w.ops[path]; ok {
		if existing == opRemove || op == opRemove {
			w.ops[path] = opRemove
		} else {
			w.ops[path] = opIndex
		}
	} else {
		w.ops[path] = op
	}

	if t, ok := w.timers[path]; ok {
		t.Stop()
	}
	pathCopy := path
	w.timers[path] = time.AfterFunc(w.debounce, func() {
		w.fire(pathCopy)
	})
}

func (w *IndexWatcher) fire(path string) {
	w.mu.Lock()
	op := w.ops[path]
	delete(w.ops, path)
	delete(w.timers, path)
	w.mu.Unlock()

	switch op {
	case opRemove:
		report, err := w.svc.RemovePaths([]string{path})
		if err != nil {
			slog.Warn("watch remove failed", "path", path, "err", err)
			return
		}
		if report.Removed > 0 {
			slog.Info("watch removed", "path", path, "removed", report.Removed)
		}
	case opIndex:
		if _, err := os.Stat(path); err != nil {
			return
		}
		if !shouldIndexFile(path) {
			return
		}
		report, err := w.svc.IndexPaths([]string{path}, false)
		if err != nil {
			slog.Warn("watch index failed", "path", path, "err", err)
			return
		}
		slog.Info("watch indexed",
			"path", path,
			"indexed", report.Indexed,
			"updated", report.Updated,
			"skipped", report.Skipped,
		)
	}
}

func (w *IndexWatcher) flushPending() {
	w.mu.Lock()
	paths := make([]string, 0, len(w.timers))
	for p, t := range w.timers {
		t.Stop()
		paths = append(paths, p)
	}
	w.timers = make(map[string]*time.Timer)
	w.mu.Unlock()
	for _, p := range paths {
		w.fire(p)
	}
}

func (w *IndexWatcher) runRescan() {
	report, err := w.svc.IndexPaths(w.cfg.WatchPaths, false)
	if err != nil {
		slog.Warn("watch rescan failed", "err", err)
		return
	}
	slog.Info("watch rescan",
		"indexed", report.Indexed,
		"updated", report.Updated,
		"skipped", report.Skipped,
		"errors", report.Errors,
	)
}
