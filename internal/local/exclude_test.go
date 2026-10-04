package local

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/extract"
)

func TestCollectSkipsDependencyDirs(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "src", "main.py")
	site := filepath.Join(root, "lib", "python3.12", "site-packages", "pkg.py")
	mods := filepath.Join(root, "web", "node_modules", "leftpad", "index.js")
	extra := filepath.Join(root, "build", "wheels", "pkg.py")
	for _, p := range []string{keep, site, mods, extra} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{Roots: []string{root}, ExcludeDirs: []string{"wheels"}}
	reg := extract.NewRegistry(extract.Options{})
	files, _ := collectIndexablePaths(cfg, reg, []string{root})
	if len(files) != 1 || files[0] != filepath.Clean(keep) {
		t.Fatalf("files=%v", files)
	}
}

func TestPruneRemovesExcludedDirs(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "keep.md")
	if err := os.WriteFile(keep, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	site := filepath.Join(root, "lib", "python3.12", "site-packages", "pkg.py")
	if err := os.MkdirAll(filepath.Dir(site), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(site, []byte("lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := testIndexedService(t, root, keep)
	defer svc.Close()
	if _, err := svc.db.Exec(
		`INSERT INTO files(path, mtime_ns, size, content_hash, indexed_at) VALUES (?, 0, 1, 'x', ?)`,
		site, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		t.Fatal(err)
	}

	report, err := svc.PruneIndex(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 {
		t.Fatalf("removed=%d report=%+v", report.Removed, report)
	}
	left, err := svc.ListIndexedFiles("")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0] != keep {
		t.Fatalf("left=%v", left)
	}
}

func TestCollectSkipsExcludedFiles(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "src", "main.go")
	lock := filepath.Join(root, "web", "package-lock.json")
	min := filepath.Join(root, "web", "app.min.js")
	gen := filepath.Join(root, "src", "bindata_gen.go")
	for _, p := range []string{keep, lock, min, gen} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Roots:        []string{root},
		ExcludeFiles: []string{"package-lock.json", "*.min.js", "*_gen.go"},
	}
	reg := extract.NewRegistry(extract.Options{})
	files, _ := collectIndexablePaths(cfg, reg, []string{root})
	if len(files) != 1 || files[0] != filepath.Clean(keep) {
		t.Fatalf("files=%v", files)
	}

	msgs := collectMessages(cfg, reg, lock)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "SEARCHIFY_EXCLUDE_FILES") {
		t.Fatalf("messages=%v", msgs)
	}
}

func collectMessages(cfg *config.Config, reg *extract.Registry, path string) []string {
	_, messages := collectIndexablePaths(cfg, reg, []string{path})
	return messages
}

func TestPruneRemovesExcludedFiles(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "keep.md")
	if err := os.WriteFile(keep, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(root, "package-lock.json")
	if err := os.WriteFile(lock, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := testIndexedService(t, root, keep)
	defer svc.Close()
	svc.cfg.ExcludeFiles = []string{"package-lock.json"}
	if _, err := svc.db.Exec(
		`INSERT INTO files(path, mtime_ns, size, content_hash, indexed_at) VALUES (?, 0, 1, 'x', ?)`,
		lock, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		t.Fatal(err)
	}

	report, err := svc.PruneIndex(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 {
		t.Fatalf("removed=%d report=%+v", report.Removed, report)
	}
	left, err := svc.ListIndexedFiles("")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0] != keep {
		t.Fatalf("left=%v", left)
	}
}
