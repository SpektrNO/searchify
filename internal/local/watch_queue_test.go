package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spektr/searchify/internal/config"
)

func testQueueWatcher(t *testing.T) (*IndexWatcher, *fsnotify.Watcher, string) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{
		Roots: []string{root}, WatchPaths: []string{root},
		IndexDir: t.TempDir(), SkipEmbed: true,
		WatchDebounce: time.Millisecond, MaxFileBytes: 1024,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })
	w, err := NewIndexWatcher(cfg, svc)
	if err != nil {
		t.Fatal(err)
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fw.Close() })
	return w, fw, root
}

func TestWatchQueueBoundedAndDebounced(t *testing.T) {
	w, _, root := testQueueWatcher(t)
	for i := range 10000 {
		w.schedule(filepath.Join(root, fmt.Sprintf("%d.md", i)))
	}
	if len(w.pending) != maxWatchPending || w.rescanDue.IsZero() || len(w.wake) != 1 {
		t.Fatalf("pending=%d rescan=%v wake=%d", len(w.pending), w.rescanDue, len(w.wake))
	}
	path := filepath.Join(root, "0.md")
	before := w.pending[path]
	w.schedule(path)
	if len(w.pending) != maxWatchPending || !w.pending[path].After(before) {
		t.Fatal("duplicate should refresh the deadline, not grow the queue")
	}

	now := time.Now()
	w.mu.Lock()
	w.pending = map[string]time.Time{path: now.Add(time.Hour)}
	w.rescanDue = now.Add(-time.Second)
	w.mu.Unlock()
	got, rescan, due := w.next(now)
	if got != "" || !rescan || due.After(now) || !w.rescanDue.IsZero() {
		t.Fatalf("recovery not dequeued: path=%q rescan=%v due=%v", got, rescan, due)
	}
	got, rescan, due = w.next(now)
	if got != path || rescan || !due.After(now) || len(w.pending) != 1 {
		t.Fatal("unsettled file should remain queued")
	}
	w.next(now.Add(2 * time.Hour))
	if len(w.pending) != 0 {
		t.Fatal("ready item was not removed")
	}
}

func TestWatchTriageBeforeQueue(t *testing.T) {
	w, _, root := testQueueWatcher(t)
	w.cfg.ExcludeFiles = []string{"ignored.md"}
	for name, size := range map[string]int{
		"large.md": 1025, "ignored.md": 10, "binary.bin": 10, "keep.md": 10,
	} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
		w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	}
	if len(w.pending) != 1 {
		t.Fatalf("triage queued %d files, want 1", len(w.pending))
	}
	path := filepath.Join(root, ".git", "objects", "test.md")
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Remove})
	w.handleEvent(fsnotify.Event{Name: filepath.Join(root, "noise.md"), Op: fsnotify.Chmod})
	if len(w.pending) != 1 {
		t.Fatal("excluded directory or chmod noise was queued")
	}
	w.cfg.ExcludeDirs = []string{"*cache*"}
	cacheFile := filepath.Join(root, "cache.md")
	if err := os.WriteFile(cacheFile, []byte("keep file despite directory pattern\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w.handleEvent(fsnotify.Event{Name: cacheFile, Op: fsnotify.Create})
	if _, ok := w.pending[cacheFile]; !ok {
		t.Fatal("directory glob incorrectly excluded a file name")
	}
}

func TestWatchRecoveryIndexesOverflowAndPrunesInPages(t *testing.T) {
	w, fw, root := testQueueWatcher(t)
	// More stale rows than one recovery page, including a directory deletion.
	for i := range directoryBatchSize + 3 {
		path := filepath.Join(root, "gone", fmt.Sprintf("%03d.md", i))
		if _, err := w.svc.db.Exec(`INSERT INTO files VALUES (?, 0, 1, 'x', 'now')`, path); err != nil {
			t.Fatal(err)
		}
	}
	for i := range maxWatchPending {
		w.schedule(filepath.Join(root, fmt.Sprintf("missing-%d.md", i)))
	}
	path := filepath.Join(root, "cloned", "nested", "new.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("overflow recovered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Create})
	if _, ok := w.pending[path]; ok || w.rescanDue.IsZero() {
		t.Fatal("expected overflow to be represented by a recovery scan")
	}
	// Make the recovery ready while the per-file entries are still debouncing.
	w.mu.Lock()
	for pending := range w.pending {
		w.pending[pending] = time.Now().Add(time.Hour)
	}
	w.rescanDue = time.Now().Add(-time.Second)
	w.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.work(ctx, fw)
	}()
	defer func() {
		cancel()
		<-done
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		files, err := w.svc.ListIndexedFiles("")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 1 && files[0] == path {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("recovery worker did not finish: files=%v", files)
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	files, err := w.svc.ListIndexedFiles("")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != path {
		t.Fatalf("recovered files=%v", files)
	}
	attached := false
	for _, dir := range fw.WatchList() {
		if dir == filepath.Dir(path) {
			attached = true
		}
	}
	if !attached {
		t.Fatal("recovery did not attach watchers to new directories")
	}
}

func TestWatchDeleteRecreateAndPopulatedDirectory(t *testing.T) {
	w, fw, root := testQueueWatcher(t)
	path := filepath.Join(root, "imported", "nested", "new.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new contents\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w.handleEvent(fsnotify.Event{Name: filepath.Join(root, "imported"), Op: fsnotify.Create})
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Remove})
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Create})
	w.process(context.Background(), fw, filepath.Join(root, "imported"))
	w.process(context.Background(), fw, path)
	n, err := w.svc.DocumentCount()
	if err != nil || n != 1 {
		t.Fatalf("delete/recreate lost new file: count=%d err=%v", n, err)
	}
}

func TestWatchWorkerAndAPIIndexingAreSerialized(t *testing.T) {
	w, fw, root := testQueueWatcher(t)
	var active, peak atomic.Int32
	w.svc.spawnExtractForTest = func(string) (string, []string, error) {
		n := active.Add(1)
		for prev := peak.Load(); n > prev && !peak.CompareAndSwap(prev, n); prev = peak.Load() {
		}
		defer active.Add(-1)
		time.Sleep(10 * time.Millisecond)
		return "bounded extraction\n", nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.work(ctx, fw)
	}()
	defer func() {
		cancel()
		<-done
	}()
	var wg sync.WaitGroup
	for i := range 8 {
		path := filepath.Join(root, fmt.Sprintf("%d.docx", i))
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		w.schedule(path)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := w.svc.IndexPaths([]string{path}, true); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	cancel()
	<-done
	if peak.Load() != 1 {
		t.Fatalf("concurrent extractors=%d, want 1", peak.Load())
	}
}

func TestWatchWorkerCancellationDoesNotFlushBacklog(t *testing.T) {
	w, fw, root := testQueueWatcher(t)
	w.debounce = time.Hour
	path := filepath.Join(root, "pending.md")
	if err := os.WriteFile(path, []byte("must remain pending\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w.schedule(path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w.work(ctx, fw)
	n, err := w.svc.DocumentCount()
	if err != nil || n != 0 {
		t.Fatalf("cancelled worker indexed backlog: count=%d err=%v", n, err)
	}
}

func TestWatchRunWaitsForActiveWorkerOnShutdown(t *testing.T) {
	w, _, root := testQueueWatcher(t)
	path := filepath.Join(root, "active.docx")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	w.svc.spawnExtractForTest = func(string) (string, []string, error) {
		close(entered)
		<-release
		return "cancelled extraction\n", nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh, done := make(chan error, 1), make(chan struct{})
	go func() {
		defer close(done)
		errCh <- w.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		releaseOnce.Do(func() { close(release) })
		<-done
	})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("startup recovery did not begin extraction")
	}
	cancel()
	select {
	case err := <-errCh:
		t.Fatalf("Run returned before worker stopped: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error=%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not join its worker")
	}
}

func TestWatchSingleFileIgnoresSiblingEvents(t *testing.T) {
	w, _, root := testQueueWatcher(t)
	target := filepath.Join(root, "target.md")
	sibling := filepath.Join(root, "sibling.md")
	w.cfg.WatchPaths = []string{target}
	for _, path := range []string{target, sibling} {
		if err := os.WriteFile(path, []byte("content\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	}
	if len(w.pending) != 1 {
		t.Fatalf("pending=%v", w.pending)
	}
	if _, ok := w.pending[target]; !ok {
		t.Fatal("target was not queued")
	}
}

func TestIndexCancellationWhileWaitingForWorker(t *testing.T) {
	w, _, root := testQueueWatcher(t)
	w.svc.indexGate <- struct{}{}
	defer func() { <-w.svc.indexGate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := w.svc.IndexPathsOpts([]string{root}, IndexPathsOptions{Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
