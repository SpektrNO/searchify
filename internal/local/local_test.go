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
	if len(results.Results) == 0 {
		t.Fatal("expected search hits")
	}
	if results.Timing.KeywordMs < 0 {
		t.Fatal("expected non-negative keyword timing")
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

func TestListIndexedFiles(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.md")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	b := filepath.Join(sub, "b.md")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("hello\n"), 0o644); err != nil {
			t.Fatal(err)
		}
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
	if _, err := svc.IndexPaths([]string{root}, false); err != nil {
		t.Fatal(err)
	}

	all, err := svc.ListIndexedFiles("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 paths, got %v", all)
	}
	under, err := svc.ListIndexedFiles(sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(under) != 1 || under[0] != b {
		t.Fatalf("expected only %s, got %v", b, under)
	}

	stats, err := svc.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.FileCount != 2 {
		t.Fatalf("file_count=%d", stats.FileCount)
	}
	if stats.FolderCount < 2 {
		t.Fatalf("folder_count=%d want >=2 (root + sub)", stats.FolderCount)
	}
	if stats.TotalBytes <= 0 {
		t.Fatalf("total_bytes=%d", stats.TotalBytes)
	}
	if stats.LastIndexChange == nil || *stats.LastIndexChange == "" {
		t.Fatal("expected last_index_change")
	}
	if stats.VectorChunkCount <= 0 {
		t.Fatalf("vector_chunk_count=%d", stats.VectorChunkCount)
	}
}

func TestSkipEmbedIndexesWithoutVectors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.md")
	if err := os.WriteFile(path, []byte("hello skip embed world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Roots:            []string{root},
		IndexDir:         filepath.Join(t.TempDir(), "index"),
		EmbedModel:       "stub",
		SkipEmbed:        true,
		MaxFileBytes:     1024 * 1024,
		MaxExtractBytes:  1024 * 1024,
		MaxChunksPerFile: 64,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	// No embedder injected — SkipEmbed must not call getEmbedder.

	report, err := svc.IndexPaths([]string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Indexed != 1 {
		t.Fatalf("indexed=%d msgs=%v", report.Indexed, report.Messages)
	}
	vc, err := svc.VectorCount()
	if err != nil {
		t.Fatal(err)
	}
	if vc != 0 {
		t.Fatalf("expected 0 vectors, got %d", vc)
	}
	res, err := svc.Search(SearchParams{Query: "skip embed", Mode: search.ModeKeyword, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) == 0 {
		t.Fatal("expected keyword hits after skip-embed index")
	}
}

func TestEmbedFilesBackfill(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.md")
	if err := os.WriteFile(path, []byte("backfill embed vectors please\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Roots:            []string{root},
		IndexDir:         filepath.Join(t.TempDir(), "index"),
		EmbedModel:       "stub",
		SkipEmbed:        true,
		MaxFileBytes:     1024 * 1024,
		MaxExtractBytes:  1024 * 1024,
		MaxChunksPerFile: 64,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.embedForTest = &stubEmbedder{}

	if _, err := svc.IndexPaths([]string{root}, false); err != nil {
		t.Fatal(err)
	}
	vc, err := svc.VectorCount()
	if err != nil {
		t.Fatal(err)
	}
	if vc != 0 {
		t.Fatalf("expected 0 vectors after skip-embed, got %d", vc)
	}

	cfg.SkipEmbed = false
	cfg.EmbedBackend = config.EmbedBackendONNX
	report, err := svc.EmbedFiles([]string{path}, EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Embedded < 1 {
		t.Fatalf("embedded=%d msgs=%v", report.Embedded, report.Messages)
	}
	vc, err = svc.VectorCount()
	if err != nil {
		t.Fatal(err)
	}
	if vc < 1 {
		t.Fatalf("expected vectors after embed, got %d", vc)
	}
}

func TestProcessEmbedBackendSpawnsWorker(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "b.md")
	if err := os.WriteFile(path, []byte("process backend spawn test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Roots:            []string{root},
		IndexDir:         filepath.Join(t.TempDir(), "index"),
		EmbedModel:       "stub",
		EmbedBackend:     config.EmbedBackendProcess,
		MaxFileBytes:     1024 * 1024,
		MaxExtractBytes:  1024 * 1024,
		MaxChunksPerFile: 64,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.embedForTest = &stubEmbedder{}

	var spawned string
	svc.spawnEmbedForTest = func(p string) error {
		spawned = p
		_, err := svc.EmbedFiles([]string{p}, EmbedOptions{})
		return err
	}

	report, err := svc.IndexPaths([]string{path}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Indexed != 1 {
		t.Fatalf("indexed=%d msgs=%v", report.Indexed, report.Messages)
	}
	if spawned != path {
		t.Fatalf("spawn path=%q want %q", spawned, path)
	}
	vc, err := svc.VectorCount()
	if err != nil {
		t.Fatal(err)
	}
	if vc < 1 {
		t.Fatalf("expected vectors via spawn worker, got %d", vc)
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
	if len(keyword.Results) > 0 {
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
	if len(vector.Results) == 0 {
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
	if len(hybrid.Results) == 0 {
		t.Fatal("expected hybrid hit for paraphrase query")
	}
	if hybrid.Timing.KeywordMs < 0 || hybrid.Timing.VectorMs < 0 {
		t.Fatal("expected non-negative hybrid leg timings")
	}
}

func TestBuildFTSQuery(t *testing.T) {
	got := buildFTSQuery(`shard "realm"`)
	want := `"shard" AND "realm"`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
