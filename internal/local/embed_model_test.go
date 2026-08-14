package local

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/search"
)

func TestEmbedModelChangeClearsVectors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.md")
	if err := os.WriteFile(path, []byte("model switch re-embed test content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Roots:            []string{root},
		IndexDir:         filepath.Join(t.TempDir(), "index"),
		EmbedModel:       "stub",
		EmbedBackend:     config.EmbedBackendONNX,
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

	if _, err := svc.IndexPaths([]string{path}, false); err != nil {
		t.Fatal(err)
	}
	vc, err := svc.VectorCount()
	if err != nil || vc < 1 {
		t.Fatalf("vectors=%d err=%v", vc, err)
	}
	if err := svc.setMeta("embed_model", "minilm-l6-v2"); err != nil {
		t.Fatal(err)
	}

	// Simulate config switch to a different model id (still stub embedder for tests).
	cfg.EmbedModel = "mpnet-base-v2"
	report, err := svc.EmbedFiles(nil, EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Embedded < 1 {
		t.Fatalf("expected re-embed after model change, got %+v", report)
	}
	stored, err := svc.getMeta("embed_model")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "mpnet-base-v2" {
		t.Fatalf("meta embed_model=%q", stored)
	}
}

func TestEmbedEngineChangeClearsVectors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "c.md")
	if err := os.WriteFile(path, []byte("engine switch re-embed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Roots:            []string{root},
		IndexDir:         filepath.Join(t.TempDir(), "index"),
		EmbedModel:       "stub",
		EmbedEngine:      config.EmbedEngineKjarni,
		EmbedBackend:     config.EmbedBackendONNX,
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

	if _, err := svc.IndexPaths([]string{path}, false); err != nil {
		t.Fatal(err)
	}
	cfg.EmbedEngine = config.EmbedEngineOllama
	cfg.EmbedModel = "nomic-embed-text"
	cfg.EmbedURL = "http://127.0.0.1:9"
	// Keep stub so we do not call real Ollama; reconcile should still clear.
	report, err := svc.EmbedFiles(nil, EmbedOptions{Force: false})
	if err != nil {
		t.Fatal(err)
	}
	if report.Embedded < 1 {
		t.Fatalf("expected re-embed after engine change, got %+v", report)
	}
	eng, err := svc.getMeta("embed_engine")
	if err != nil {
		t.Fatal(err)
	}
	if eng != string(config.EmbedEngineOllama) {
		t.Fatalf("embed_engine=%q", eng)
	}
}

func TestVectorSearchRejectsEmbedModelMismatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "b.md")
	if err := os.WriteFile(path, []byte("vector search model mismatch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Roots:            []string{root},
		IndexDir:         filepath.Join(t.TempDir(), "index"),
		EmbedModel:       "stub",
		EmbedBackend:     config.EmbedBackendONNX,
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

	if _, err := svc.IndexPaths([]string{path}, false); err != nil {
		t.Fatal(err)
	}
	cfg.EmbedModel = "mpnet-base-v2"

	_, err = svc.Search(SearchParams{Query: "mismatch", Mode: search.ModeVector, Limit: 5})
	if err == nil || !strings.Contains(err.Error(), "embed_model") {
		t.Fatalf("want embed_model mismatch error, got %v", err)
	}

	kw, err := svc.Search(SearchParams{Query: "mismatch", Mode: search.ModeKeyword, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(kw.Results) == 0 {
		t.Fatal("keyword search should still work during model mismatch")
	}
}
