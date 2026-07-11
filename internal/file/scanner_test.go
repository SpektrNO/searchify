package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	content := "alpha line\nbeta alpha line\nnothing here\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Search(path, SearchOptions{Query: "alpha beta", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Line != 2 {
		t.Fatalf("expected best match on line 2, got line %d", results[0].Line)
	}
}
