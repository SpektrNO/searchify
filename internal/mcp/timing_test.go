package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spektr/searchify/internal/local"
)

func TestSearchFileIncludesDuration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.md")
	if err := os.WriteFile(path, []byte("hello SEARCHIFY_ROOTS world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := testServer(t, "secret")
	s.cfg.Roots = []string{root}

	_, out, err := s.searchFile(context.Background(), nil, searchFileInput{
		Path:  path,
		Query: "SEARCHIFY_ROOTS",
		Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.DurationMs < 0 {
		t.Fatalf("duration_ms=%d", out.DurationMs)
	}
	if out.Count < 1 {
		t.Fatal("expected hits")
	}
}

func TestSearchLocalAndIndexIncludeDuration(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(docs, "a.md")
	if err := os.WriteFile(path, []byte("# Shard Realm\n\nThe shard_id partitions realms.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := testServer(t, "secret")
	s.cfg.Roots = []string{root}
	s.Local().SetEmbedderForTest(&stubEmbedder{})

	_, indexOut, err := s.indexPaths(context.Background(), nil, indexPathsInput{
		Paths: []string{docs},
	})
	if err != nil {
		t.Fatal(err)
	}
	if indexOut.DurationMs < 0 {
		t.Fatalf("index duration_ms=%d", indexOut.DurationMs)
	}
	if indexOut.Indexed < 1 {
		t.Fatalf("expected indexed file, got %+v", indexOut)
	}

	_, searchOut, err := s.searchLocal(context.Background(), nil, searchLocalInput{
		Query: "shard realm",
		Mode:  "keyword",
		Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if searchOut.DurationMs < 0 {
		t.Fatalf("search duration_ms=%d", searchOut.DurationMs)
	}
	if searchOut.Count < 1 {
		t.Fatal("expected search hits")
	}
	if searchOut.Timing == nil || searchOut.Timing.KeywordMs == nil {
		t.Fatalf("expected keyword timing, got %+v", searchOut.Timing)
	}
}

// stubEmbedder mirrors local test stub without importing unexported local helpers.
type stubEmbedder struct{}

func (s *stubEmbedder) Encode(text string) ([]float32, error) {
	return []float32{1, 0, 0, 0, 0, 0, 0, 0}, nil
}

func (s *stubEmbedder) EncodeBatch(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 0, 0, 0, 0, 0, 0, 0}
	}
	return out, nil
}

func (s *stubEmbedder) Close() error { return nil }

// Ensure local.Embedder is satisfied at compile time in tests that set it.
var _ local.Embedder = (*stubEmbedder)(nil)
