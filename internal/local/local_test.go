package local

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/search"
)

type stubEmbedder struct{}

func (s *stubEmbedder) Encode(text string) ([]float32, error) {
	return encodeStubVector(text), nil
}

func (s *stubEmbedder) EncodeBatch(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = encodeStubVector(text)
	}
	return out, nil
}

func (s *stubEmbedder) Close() error { return nil }

func encodeStubVector(text string) []float32 {
	v := make([]float32, 8)
	lower := strings.ToLower(text)
	if strings.Contains(lower, "shard") {
		v[0] = 1
	}
	if strings.Contains(lower, "partition") {
		v[1] = 1
	}
	if strings.Contains(lower, "realm") {
		v[2] = 1
	}
	norm := float32(0)
	for _, x := range v {
		norm += x * x
	}
	if norm == 0 {
		return v
	}
	norm = float32(math.Sqrt(float64(norm)))
	for i := range v {
		v[i] /= norm
	}
	return v
}

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
		Roots:      []string{root},
		IndexDir:   indexDir,
		EmbedModel: "stub",
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.embedForTest = &stubEmbedder{}

	report, err := svc.IndexPaths([]string{docs}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Indexed != 1 {
		t.Fatalf("expected 1 indexed, got %d", report.Indexed)
	}

	results, err := svc.Search(SearchParams{Query: "shard realm", Mode: search.ModeKeyword, Limit: 5})
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
	if !status.VectorReady {
		t.Fatal("expected vector_ready after indexing")
	}

	report2, err := svc.IndexPaths([]string{docs}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report2.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", report2.Skipped)
	}
}

func TestHybridFindsParaphrase(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "realms.md")
	content := "Each realm is scoped by shard_id for isolation.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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

	if _, err := svc.IndexPaths([]string{path}, false); err != nil {
		t.Fatal(err)
	}

	keyword, err := svc.Search(SearchParams{
		Query: "how are realms partitioned",
		Mode:  search.ModeKeyword,
		Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keyword) > 0 {
		t.Log("keyword unexpectedly matched; continuing")
	}

	vector, err := svc.Search(SearchParams{
		Query: "how are realms partitioned",
		Mode:  search.ModeVector,
		Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(vector) == 0 {
		t.Fatal("expected vector hit for paraphrase query")
	}

	hybrid, err := svc.Search(SearchParams{
		Query: "how are realms partitioned",
		Mode:  search.ModeHybrid,
		Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hybrid) == 0 {
		t.Fatal("expected hybrid hit for paraphrase query")
	}
}

func TestBuildFTSQuery(t *testing.T) {
	got := buildFTSQuery(`shard "realm"`)
	want := `"shard" AND "realm"`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
