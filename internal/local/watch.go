package local

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spektr/searchify/internal/config"
)

const maxWatchPending = 1024

// IndexWatcher triages events into a bounded queue consumed by one worker.
type IndexWatcher struct {
	cfg      *config.Config
	svc      *Service
	debounce time.Duration
	rescan   time.Duration

	mu        sync.Mutex
	pending   map[string]time.Time
	rescanDue time.Time
	wake      chan struct{}
}

func NewIndexWatcher(cfg *config.Config, svc *Service) (*IndexWatcher, error) {
	if len(cfg.WatchPaths) == 0 {
		return nil, fmt.Errorf("no watch paths configured")
	}
	debounce := cfg.WatchDebounce
	if debounce <= 0 {
		debounce = time.Second
	}
	return &IndexWatcher{
		cfg: cfg, svc: svc, debounce: debounce, rescan: cfg.WatchRescan,
		pending: make(map[string]time.Time),
		wake:    make(chan struct{}, 1),
	}, nil
}

// Run blocks until cancelled, including waiting for its worker to stop.
func (w *IndexWatcher) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer watcher.Close()

	for _, p := range w.cfg.WatchPaths {
		if err := w.addRecursive(ctx, watcher, p); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("watch add failed", "path", p, "err", err)
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.work(ctx, watcher)
	}()
	defer func() {
		cancel()
		<-done
	}()
	w.requestRescan()

	slog.Info("index watch started", "paths", len(w.cfg.WatchPaths),
		"debounce", w.debounce.String(), "rescan", w.rescan.String(),
		"queue_limit", maxWatchPending, "workers", 1)

	var rescanCh <-chan time.Time
	if w.rescan > 0 {
		ticker := time.NewTicker(w.rescan)
		defer ticker.Stop()
		rescanCh = ticker.C
	}
	for {
		select {
		case <-ctx.Done():
			slog.Info("index watch stopped")
			return ctx.Err()
		case <-rescanCh:
			w.requestRescan()
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			w.handleEvent(ev)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("watch error", "err", err)
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				w.requestRescan()
			}
		}
	}
}

func (w *IndexWatcher) addRecursive(ctx context.Context, watcher *fsnotify.Watcher, root string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return watcher.Add(filepath.Dir(root))
	}
	if w.cfg.SkipPath(root) {
		return nil
	}
	if err := watcher.Add(root); err != nil {
		return err
	}
	return walkDirectories(ctx, root, func(path string, d os.DirEntry) error {
		if d.IsDir() && !w.cfg.SkipPath(path) {
			if err := w.addRecursive(ctx, watcher, path); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				slog.Warn("watch add dir failed", "path", path, "err", err)
			}
		}
		return nil
	})
}

func (w *IndexWatcher) handleEvent(ev fsnotify.Event) {
	path := filepath.Clean(ev.Name)
	if !w.cfg.UnderAnyRoot(path) || w.cfg.PathHasSkipDir(path) {
		return
	}
	// Watching a single file attaches to its parent; ignore its siblings.
	inScope := false
	for _, root := range w.cfg.WatchPaths {
		if pathWithin(path, root) {
			inScope = true
			break
		}
	}
	if !inScope || !ev.Has(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) {
		return
	}
	if ev.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if w.cfg.SkipPath(path) {
				return
			}
			// Attach and crawl in the worker, not in the event-draining loop.
			w.schedule(path)
			return
		}
	}
	if ev.Has(fsnotify.Remove | fsnotify.Rename) {
		// Directory removals must reconcile descendants, even without an extension.
		w.schedule(path)
		return
	}
	if w.cfg.SkipFile(path) || !w.svc.extract.HasExtension(path) {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("watch triage failed", "path", path, "err", err)
		}
		return
	}
	if info.Mode().IsRegular() && info.Size() <= maxIndexFileBytes(w.cfg) {
		w.schedule(path)
	}
}

func (w *IndexWatcher) notify() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *IndexWatcher) requestRescan() {
	w.mu.Lock()
	if w.rescanDue.IsZero() {
		w.rescanDue = time.Now().Add(w.debounce)
	}
	w.mu.Unlock()
	w.notify()
}

func (w *IndexWatcher) schedule(path string) {
	w.mu.Lock()
	_, exists := w.pending[path]
	if exists || len(w.pending) < maxWatchPending {
		w.pending[path] = time.Now().Add(w.debounce)
	} else if w.rescanDue.IsZero() {
		// One recovery marker replaces arbitrarily many overflow events.
		w.rescanDue = time.Now().Add(w.debounce)
		slog.Warn("watch queue full; scheduling recovery scan", "queue_limit", maxWatchPending)
	}
	w.mu.Unlock()
	w.notify()
}

// next returns the oldest ready item, or the deadline of the next item.
func (w *IndexWatcher) next(now time.Time) (path string, rescan bool, due time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	due = w.rescanDue
	rescan = !due.IsZero()
	for p, deadline := range w.pending {
		if due.IsZero() || deadline.Before(due) {
			path, rescan, due = p, false, deadline
		}
	}
	if !due.IsZero() && !due.After(now) {
		if rescan {
			w.rescanDue = time.Time{}
		} else {
			delete(w.pending, path)
		}
	}
	return
}

func (w *IndexWatcher) work(ctx context.Context, watcher *fsnotify.Watcher) {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for ctx.Err() == nil {
		now := time.Now()
		path, rescan, due := w.next(now)
		if !due.IsZero() && !due.After(now) {
			if rescan {
				w.runRescan(ctx, watcher)
			} else {
				w.process(ctx, watcher, path)
			}
			continue
		}
		var timerCh <-chan time.Time
		if !due.IsZero() {
			timer.Reset(time.Until(due))
			timerCh = timer.C
		}
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-timerCh:
		}
	}
}

func (w *IndexWatcher) process(ctx context.Context, watcher *fsnotify.Watcher, path string) {
	// Reconcile current disk state, so delete/recreate bursts index the new file.
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		w.removeMissing(ctx, []string{path})
		return
	}
	if err != nil {
		slog.Warn("watch stat failed", "path", path, "err", err)
		return
	}
	if info.IsDir() {
		if err := w.addRecursive(ctx, watcher, path); err != nil && ctx.Err() == nil {
			slog.Warn("watch add failed", "path", path, "err", err)
		}
	}
	w.index(ctx, []string{path})
}

func (w *IndexWatcher) index(ctx context.Context, paths []string) {
	report, err := w.svc.IndexPathsOpts(paths, IndexPathsOptions{Context: ctx})
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("watch index failed", "paths", paths, "err", err)
		}
		return
	}
	slog.Info("watch indexed", "paths", paths, "indexed", report.Indexed,
		"updated", report.Updated, "skipped", report.Skipped,
		"errors", report.Errors, "messages", report.Messages)
}

func (w *IndexWatcher) runRescan(ctx context.Context, watcher *fsnotify.Watcher) {
	for _, root := range w.cfg.WatchPaths {
		if err := w.addRecursive(ctx, watcher, root); err != nil && ctx.Err() == nil {
			slog.Warn("watch rescan add failed", "path", root, "err", err)
		}
	}
	w.removeMissing(ctx, w.cfg.WatchPaths)
	if ctx.Err() == nil {
		w.index(ctx, w.cfg.WatchPaths)
	}
}

// Page through indexed paths so recovery of a huge deleted tree stays bounded too.
func (w *IndexWatcher) removeMissing(ctx context.Context, roots []string) {
	select {
	case w.svc.indexGate <- struct{}{}:
		defer func() { <-w.svc.indexGate }()
	case <-ctx.Done():
		return
	}
	for _, root := range roots {
		last := ""
		for ctx.Err() == nil {
			rows, err := w.svc.db.QueryContext(ctx,
				`SELECT path FROM files WHERE path > ? AND (path = ? OR path LIKE ? ESCAPE '\')
				 ORDER BY path LIMIT ?`,
				last, root, escapeLike(strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator))+"%", directoryBatchSize)
			if err != nil {
				if ctx.Err() == nil {
					slog.Warn("watch reconcile failed", "path", root, "err", err)
				}
				return
			}
			var paths []string
			for rows.Next() {
				var path string
				if err = rows.Scan(&path); err != nil {
					break
				}
				paths = append(paths, path)
			}
			err = errors.Join(err, rows.Err(), rows.Close())
			if err != nil {
				slog.Warn("watch reconcile read failed", "path", root, "err", err)
				return
			}
			if len(paths) == 0 {
				break
			}
			for _, path := range paths {
				if ctx.Err() != nil {
					return
				}
				if !pathWithin(path, root) {
					continue
				}
				orphan, err := w.svc.isOrphanIndexedPath(path)
				if err == nil && orphan {
					err = w.svc.removeIndexedFile(path)
				}
				if err != nil {
					slog.Warn("watch remove failed", "path", path, "err", err)
				}
			}
			last = paths[len(paths)-1]
		}
	}
}
