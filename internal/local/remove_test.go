package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/search"
)

func TestRemovePathsFileAndIdempotent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "docs", "a.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("shard realm delete me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := testIndexedService(t, root, path)
	defer svc.Close()

	hits, err := svc.Search(SearchParams{Query: "shard", Mode: search.ModeKeyword, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits.Results) == 0 {
		t.Fatal("expected hit before remove")
	}

	// Delete from disk; index row must still be removable.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	report, err := svc.RemovePaths([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 || report.Errors != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}

	hits, err = svc.Search(SearchParams{Query: "shard", Mode: search.ModeKeyword, Limit: 5})
	if err == nil && len(hits.Results) != 0 {
		t.Fatalf("expected no hits after remove, got %d", len(hits.Results))
	}
	// Empty index may refuse search; document_count is the source of truth.
	_ = hits

	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.DocumentCount != 0 {
		t.Fatalf("expected 0 files, got %d", status.DocumentCount)
	}
	if status.ChunkCount != 0 {
		t.Fatalf("expected 0 chunks, got %d", status.ChunkCount)
	}

	report2, err := svc.RemovePaths([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if report2.Skipped != 1 || report2.Removed != 0 || report2.Errors != 0 {
		t.Fatalf("expected idempotent skip, got %+v", report2)
	}
}

func TestRemovePathsDirectoryPrefix(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	a := filepath.Join(docs, "a.md")
	b := filepath.Join(docs, "sub", "b.md")
	other := filepath.Join(root, "other.md")
	for _, p := range []string{a, b, other} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("content "+filepath.Base(p)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := testIndexedService(t, root, root)
	defer svc.Close()

	report, err := svc.RemovePaths([]string{docs})
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 2 {
		t.Fatalf("expected 2 removed under docs, got %+v", report)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.DocumentCount != 1 {
		t.Fatalf("expected other.md to remain, file_count=%d", status.DocumentCount)
	}
}

func TestRemovePathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Roots:      []string{root},
		IndexDir:   filepath.Join(t.TempDir(), "index"),
		EmbedModel: "stub",
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.embedForTest = &stubEmbedder{}

	report, err := svc.RemovePaths([]string{outside})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors != 1 || report.Removed != 0 {
		t.Fatalf("expected outside-root error, got %+v", report)
	}
}

func TestRemovePathsRelativePathBase(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	path := filepath.Join(proj, "readme.md")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("hello relative remove\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Roots:      []string{root},
		IndexDir:   filepath.Join(t.TempDir(), "index"),
		EmbedModel: "stub",
		PathBase:   proj,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.embedForTest = &stubEmbedder{}

	if _, err := svc.IndexPaths([]string{path}, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	report, err := svc.RemovePaths([]string{"readme.md"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 {
		t.Fatalf("expected relative PATH_BASE remove, got %+v", report)
	}
}

func TestRemovePathsDoesNotMatchSiblingPrefix(t *testing.T) {
	root := t.TempDir()
	foo := filepath.Join(root, "foo.md")
	foobar := filepath.Join(root, "foo-bar.md")
	for _, p := range []string{foo, foobar} {
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := testIndexedService(t, root, root)
	defer svc.Close()

	report, err := svc.RemovePaths([]string{foo})
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 {
		t.Fatalf("expected only foo.md removed, got %+v", report)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.DocumentCount != 1 {
		t.Fatalf("expected foo-bar.md to remain, file_count=%d", status.DocumentCount)
	}
}

func testIndexedService(t *testing.T, root string, indexPath string) *Service {
	t.Helper()
	cfg := &config.Config{
		Roots:      []string{root},
		IndexDir:   filepath.Join(t.TempDir(), "index"),
		EmbedModel: "stub",
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	svc.embedForTest = &stubEmbedder{}
	if _, err := svc.IndexPaths([]string{indexPath}, false); err != nil {
		t.Fatal(err)
	}
	return svc
}
