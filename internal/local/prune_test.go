package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spektr/searchify/internal/config"
)

func TestPruneIndexRemovesMissingKeepsPresent(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "keep.md")
	gone := filepath.Join(root, "gone.md")
	for _, p := range []string{keep, gone} {
		if err := os.WriteFile(p, []byte("content "+filepath.Base(p)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := testIndexedService(t, root, root)
	defer svc.Close()

	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}

	dry, err := svc.PruneIndex(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if dry.Scanned != 2 || dry.Removed != 1 || dry.Skipped != 1 {
		t.Fatalf("dry_run unexpected: %+v", dry)
	}
	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.DocumentCount != 2 {
		t.Fatalf("dry_run must not mutate index, docs=%d", status.DocumentCount)
	}

	report, err := svc.PruneIndex(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 || report.Skipped != 1 || report.Errors != 0 {
		t.Fatalf("prune unexpected: %+v", report)
	}

	status, err = svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.DocumentCount != 1 {
		t.Fatalf("expected 1 doc after prune, got %d", status.DocumentCount)
	}

	again, err := svc.PruneIndex(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if again.Removed != 0 || again.Scanned != 1 || again.Skipped != 1 {
		t.Fatalf("idempotent prune unexpected: %+v", again)
	}
}

func TestPruneIndexScopedPaths(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	a := filepath.Join(docs, "a.md")
	b := filepath.Join(root, "other.md")
	for _, p := range []string{a, b} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := testIndexedService(t, root, root)
	defer svc.Close()

	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}

	report, err := svc.PruneIndex([]string{docs}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 || report.Scanned != 1 {
		t.Fatalf("scoped prune unexpected: %+v", report)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.DocumentCount != 1 {
		t.Fatalf("other.md should remain until unscoped prune, docs=%d", status.DocumentCount)
	}
}

func TestPruneIndexOutsideRoots(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "in.md")
	if err := os.WriteFile(path, []byte("in\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := testIndexedService(t, root, path)
	defer svc.Close()

	// Simulate roots config change: indexed path no longer under configured roots.
	svc.cfg.Roots = []string{t.TempDir()}

	report, err := svc.PruneIndex(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 {
		t.Fatalf("expected out-of-root orphan removed, got %+v", report)
	}
}

func TestPruneIndexEmptyScopeOK(t *testing.T) {
	cfg := &config.Config{
		Roots:      []string{t.TempDir()},
		IndexDir:   filepath.Join(t.TempDir(), "index"),
		EmbedModel: "stub",
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.embedForTest = &stubEmbedder{}

	report, err := svc.PruneIndex(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 0 || report.Removed != 0 {
		t.Fatalf("empty index prune: %+v", report)
	}
}
