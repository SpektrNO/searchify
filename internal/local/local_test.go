package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spektr/searchify/internal/config"
)

func TestIndexAndSearch(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(docs, "sample.md")
	content := "# Shard Realm\n\nThe shard_id partitions realms.\n\n## Other\n\nNothing here.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	indexDir := filepath.Join(t.TempDir(), "index")
	cfg := &config.Config{
		Roots:    []string{root},
		IndexDir: indexDir,
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	report, err := svc.IndexPaths([]string{docs}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Indexed != 1 {
		t.Fatalf("expected 1 indexed, got %d", report.Indexed)
	}

	results, err := svc.Search("shard realm", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search hits")
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Ready {
		t.Fatal("expected ready index")
	}

	report2, err := svc.IndexPaths([]string{docs}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report2.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", report2.Skipped)
	}
}

func TestBuildFTSQuery(t *testing.T) {
	got := buildFTSQuery(`shard "realm"`)
	want := `"shard" AND "realm"`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
