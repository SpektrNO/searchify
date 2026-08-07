package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spektr/searchify/internal/config"
)

func TestIndexWatcherIndexesAndRemoves(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Roots:         []string{root},
		IndexDir:      filepath.Join(t.TempDir(), "index"),
		EmbedModel:    "stub",
		WatchPaths:    []string{root},
		WatchDebounce: 50 * time.Millisecond,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.embedForTest = &stubEmbedder{}

	w, err := NewIndexWatcher(cfg, svc)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Give watcher time to attach.
	time.Sleep(100 * time.Millisecond)

	path := filepath.Join(root, "watched.md")
	if err := os.WriteFile(path, []byte("hello watch shard\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		n, err := svc.DocumentCount()
		if err != nil {
			t.Fatal(err)
		}
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for watch index")
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(3 * time.Second)
	for {
		n, err := svc.DocumentCount()
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for watch remove")
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}
}
