package config

import "testing"

func TestSkipDirBuiltins(t *testing.T) {
	var cfg Config
	for _, name := range []string{"site-packages", "Site-Packages", "node_modules", "dist-packages", "foo.egg-info", ".venv", "bin"} {
		if !cfg.SkipDir(name) {
			t.Fatalf("expected skip %q", name)
		}
	}
	if cfg.SkipDir("src") || cfg.SkipDir("python3.12") {
		t.Fatal("project dirs must not be skipped")
	}
	if !cfg.SkipPath(`/home/dev/proj/cmd/bin`) {
		t.Fatal("every bin directory is skipped")
	}
	path := `/home/dev/proj/lib/python3.12/site-packages/pkg.py`
	if !cfg.PathHasSkipDir(path) {
		t.Fatal("expected site-packages path to be excluded")
	}
	if cfg.PathHasSkipDir(`/home/dev/proj/src/main.py`) {
		t.Fatal("source path should be kept")
	}
}

func TestExcludePatterns(t *testing.T) {
	got, err := parseExcludeDirs(` generated , *cache* , python*/site-packages , **/third_party `)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %#v", got)
	}
	cfg := &Config{ExcludeDirs: got}
	if !cfg.SkipPath(`/proj/generated`) || !cfg.SkipDir("Generated") {
		t.Fatal("exact name should skip every matching directory")
	}
	if !cfg.SkipPath(`/proj/build/.cache`) {
		t.Fatal("name glob *cache* should match .cache")
	}
	if !cfg.SkipPath(`/opt/lib/python3.12/site-packages`) {
		t.Fatal("path pattern should match python*/site-packages")
	}
	if !cfg.SkipPath(`/repo/vendor/third_party`) {
		t.Fatal("** /third_party should match")
	}
	if cfg.SkipPath(`/proj/src/app`) {
		t.Fatal("unrelated directory must be kept")
	}
	if !cfg.PathHasSkipDir(`/opt/lib/python3.12/site-packages/pkg.py`) {
		t.Fatal("file under a path pattern must be excluded")
	}
	for _, bad := range []string{"*", "**", "..", ""} {
		if bad == "" {
			continue
		}
		if _, err := parseExcludeDirs(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestExcludeFilePatterns(t *testing.T) {
	got, err := parseExcludeFiles(` go.sum , *.min.js , Go.Mod `)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	cfg := &Config{ExcludeFiles: got}
	if !cfg.SkipFile(`/repo/go.sum`) || !cfg.SkipFile(`/repo/GO.SUM`) {
		t.Fatal("exact file name should match case-insensitively")
	}
	if !cfg.SkipFile(`/web/app.min.js`) || !cfg.SkipFile(`/web/vendor/lib.MIN.JS`) {
		t.Fatal("glob should match the base name only")
	}
	if !cfg.SkipFile(`/mod/go.mod`) {
		t.Fatal("Go.Mod should match go.mod")
	}
	if cfg.SkipFile(`/repo/main.go`) || cfg.SkipFile(`/repo/go.sum/notes.md`) {
		t.Fatal("unrelated files and names that are directories must be kept")
	}
	for _, bad := range []string{"*", "**", "vendor/go.sum", ".."} {
		if _, err := parseExcludeFiles(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}
