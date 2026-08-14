package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDedupeNestedRootsKeepsOuter(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "nested")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	sib := t.TempDir()

	got := dedupeNestedRoots([]string{root, child, sib})
	if len(got) != 2 {
		t.Fatalf("got %v want 2 outermost roots", got)
	}
	for _, p := range got {
		if p == child {
			t.Fatalf("nested child %q should have been dropped: %v", child, got)
		}
	}
}

func TestParseRootsDropsNested(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "docs")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := parseRoots(root + "," + child)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != filepath.Clean(root) {
		t.Fatalf("got %v want [%q]", got, filepath.Clean(root))
	}
}

func TestResolveSnippetChars(t *testing.T) {
	cfg := &Config{SnippetChars: 400}
	if n := cfg.ResolveSnippetChars(0); n != 400 {
		t.Fatalf("default override 0: got %d want 400", n)
	}
	if n := cfg.ResolveSnippetChars(1200); n != 1200 {
		t.Fatalf("query override: got %d want 1200", n)
	}
	if n := cfg.ResolveSnippetChars(9000); n != maxSnippetChars {
		t.Fatalf("over max: got %d want %d", n, maxSnippetChars)
	}
	if n := (*Config)(nil).ResolveSnippetChars(0); n != defaultSnippetChars {
		t.Fatalf("nil cfg: got %d want %d", n, defaultSnippetChars)
	}
}
