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
	gate     *branchGate
	fs       *fsnotify.Watcher

	mu                 sync.Mutex
	pending            map[string]time.Time
	rescanDue          time.Time
	rescanAll          bool
	rescanRepos        map[string]struct{}
	rescanFromOverflow bool
	overflowRepo       string
	wake               chan struct{}
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
		gate:    newBranchGate(nil),
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
	w.fs = watcher
	defer func() { w.fs = nil }()

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
	w.trackRepo(watcher, root)
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
			w.trackRepo(watcher, path)
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
	// HEAD is inside skipped .git. Read it, and do not index it.
	if w.gate.isHeadEvent(path) {
		w.applyBranch(w.gate.consider(path))
		// git replaces HEAD by rename, which drops an inode watch.
		if w.fs != nil {
			if err := w.fs.Add(path); err != nil {
				slog.Warn("watch git HEAD failed", "path", path, "err", err)
			}
		}
		return
	}
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
	decision := w.gate.consider(path)
	w.applyBranch(decision)
	if !decision.open {
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
	w.rescanAll = true
	w.rescanFromOverflow = false
	w.overflowRepo = ""
	w.mu.Unlock()
	w.notify()
}

func (w *IndexWatcher) requestRepoRescan(repo string) {
	w.mu.Lock()
	if w.rescanRepos == nil {
		w.rescanRepos = make(map[string]struct{})
	}
	w.rescanRepos[repo] = struct{}{}
	if w.rescanDue.IsZero() {
		w.rescanDue = time.Now().Add(w.debounce)
	}
	w.mu.Unlock()
	w.notify()
}

func (w *IndexWatcher) schedule(path string) {
	decision := w.gate.consider(path)
	w.applyBranch(decision)
	if !decision.open {
		return
	}
	w.mu.Lock()
	_, exists := w.pending[path]
	if exists || len(w.pending) < maxWatchPending {
		w.pending[path] = time.Now().Add(w.debounce)
	} else if decision.repo != "" && w.rescanFromOverflow && w.overflowRepo == decision.repo && !w.rescanAll {
		// A checkout keeps rewriting the tree after the queue fills. Wait until
		// that flood is quiet, then re-read HEAD before scanning other roots.
		w.rescanDue = time.Now().Add(w.debounce)
	} else if w.rescanDue.IsZero() {
		// One recovery marker replaces arbitrarily many overflow events.
		w.rescanDue = time.Now().Add(w.debounce)
		w.rescanFromOverflow = true
		w.overflowRepo = decision.repo
		slog.Warn("watch queue full; scheduling recovery scan", "queue_limit", maxWatchPending)
	}
	w.mu.Unlock()
	w.notify()
}

func (w *IndexWatcher) applyBranch(d branchDecision) {
	if d.drop && d.repo != "" {
		w.dropRepo(d.repo)
	}
	if d.rescan && d.repo != "" {
		w.requestRepoRescan(d.repo)
	}
}

func (w *IndexWatcher) dropRepo(repo string) {
	w.mu.Lock()
	for path := range w.pending {
		if pathWithin(path, repo) {
			delete(w.pending, path)
		}
	}
	delete(w.rescanRepos, repo)
	cleared := false
	if w.rescanFromOverflow && w.overflowRepo == repo && !w.rescanAll {
		w.rescanFromOverflow = false
		w.overflowRepo = ""
		if len(w.rescanRepos) == 0 {
			w.rescanDue = time.Time{}
			cleared = true
		}
	}
	w.mu.Unlock()
	if cleared {
		w.notify()
	}
}

func (w *IndexWatcher) trackRepo(watcher *fsnotify.Watcher, path string) {
	if watcher == nil {
		return
	}
	head, already := w.gate.watchHead(path)
	if head == "" || already {
		return
	}
	if err := watcher.Add(head); err != nil {
		slog.Warn("watch git HEAD failed", "path", head, "err", err)
	}
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
	if !w.branchOpen(path) {
		return
	}
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
	report, err := w.svc.IndexPathsOpts(paths, IndexPathsOptions{Context: ctx, SkipDir: w.skipBlockedDir})
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
	w.mu.Lock()
	all := w.rescanAll
	repos := w.rescanRepos
	fromOverflow := w.rescanFromOverflow
	overflowRepo := w.overflowRepo
	w.rescanAll = false
	w.rescanRepos = nil
	w.rescanFromOverflow = false
	w.overflowRepo = ""
	w.mu.Unlock()

	// A checkout that filled the queue must not recovery-scan every watch root
	// once HEAD has left the default branch. If that cancellation already
	// cleared the overflow flags, there is nothing left to scan.
	if fromOverflow && overflowRepo != "" && !w.branchOpen(overflowRepo) {
		return
	}
	if !all && len(repos) == 0 {
		if !fromOverflow {
			return
		}
		all = true
	}
	roots := w.recoveryRoots(all, repos)
	if len(roots) == 0 {
		return
	}
	for _, root := range roots {
		if err := w.addRecursive(ctx, watcher, root); err != nil && ctx.Err() == nil {
			slog.Warn("watch rescan add failed", "path", root, "err", err)
		}
	}
	w.removeMissing(ctx, roots)
	if ctx.Err() == nil {
		w.index(ctx, roots)
	}
}

func (w *IndexWatcher) recoveryRoots(all bool, repos map[string]struct{}) []string {
	seen := make(map[string]struct{})
	var roots []string
	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		if !w.branchOpen(path) {
			return
		}
		seen[path] = struct{}{}
		roots = append(roots, path)
	}
	if all {
		for _, root := range w.cfg.WatchPaths {
			add(root)
		}
		return roots
	}
	for _, watchPath := range w.cfg.WatchPaths {
		for repo := range repos {
			switch {
			case pathWithin(watchPath, repo):
				add(watchPath)
			case pathWithin(repo, watchPath):
				add(repo)
			}
		}
	}
	return roots
}

func (w *IndexWatcher) branchOpen(path string) bool {
	decision := w.gate.consider(path)
	w.applyBranch(decision)
	return decision.open
}

func (w *IndexWatcher) skipBlockedDir(dir string) bool {
	return !w.branchOpen(dir)
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
				if !pathWithin(path, root) || w.skipBlockedDir(path) {
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
