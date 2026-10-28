package local

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/search"
)

func TestPathAllowKeywordExcludesStrongerOutsideHit(t *testing.T) {
	root := t.TempDir()
	allowedDir := filepath.Join(root, "allowed")
	otherDir := filepath.Join(root, "other")
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}

	allowedPath := filepath.Join(allowedDir, "weak.md")
	outsidePath := filepath.Join(otherDir, "strong.md")
	// Outside doc repeats the query token many times so BM25 prefers it globally.
	if err := os.WriteFile(allowedPath, []byte("UNIQUE_ALLOW_TOKEN once.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	strong := strings.Repeat("UNIQUE_ALLOW_TOKEN ", 40) + "\n"
	if err := os.WriteFile(outsidePath, []byte(strong), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestSearchService(t, root)
	if _, err := svc.IndexPaths([]string{root}, false); err != nil {
		t.Fatal(err)
	}

	unscoped, err := svc.Search(SearchParams{
		Query: "UNIQUE_ALLOW_TOKEN",
		Mode:  search.ModeKeyword,
		Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unscoped.Results) < 2 {
		t.Fatalf("expected both docs unscoped, got %+v", unscoped.Results)
	}
	seenOutside := false
	for _, r := range unscoped.Results {
		if r.Path == outsidePath {
			seenOutside = true
		}
	}
	if !seenOutside {
		t.Fatalf("expected outside doc in unscoped results: %+v", unscoped.Results)
	}

	scoped, err := svc.Search(SearchParams{
		Query: "UNIQUE_ALLOW_TOKEN",
		Mode:  search.ModeKeyword,
		Limit: 5,
		Allow: &PathAllow{Prefixes: []string{allowedDir}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped.Results) != 1 {
		t.Fatalf("expected 1 allowed hit, got %+v", scoped.Results)
	}
	if scoped.Results[0].Path != allowedPath {
		t.Fatalf("got %s want %s", scoped.Results[0].Path, allowedPath)
	}
	for _, r := range scoped.Results {
		if r.Path == outsidePath {
			t.Fatal("stronger outside hit must not appear under path_prefixes")
		}
	}
}

func TestPathAllowFillsLimitFromAllowedSet(t *testing.T) {
	root := t.TempDir()
	allowedDir := filepath.Join(root, "team")
	noiseDir := filepath.Join(root, "noise")
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(noiseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		p := filepath.Join(allowedDir, strings.Repeat("a", i+1)+".md")
		if err := os.WriteFile(p, []byte("FILL_LIMIT_TOKEN doc\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 10; i++ {
		p := filepath.Join(noiseDir, strings.Repeat("n", i+1)+".md")
		body := strings.Repeat("FILL_LIMIT_TOKEN ", 20) + "\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := newTestSearchService(t, root)
	if _, err := svc.IndexPaths([]string{root}, false); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Search(SearchParams{
		Query: "FILL_LIMIT_TOKEN",
		Mode:  search.ModeKeyword,
		Limit: 3,
		Allow: &PathAllow{Prefixes: []string{allowedDir}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 3 {
		t.Fatalf("expected limit filled from allow-set, got %d: %+v", len(out.Results), out.Results)
	}
	for _, r := range out.Results {
		if !strings.HasPrefix(r.Path, allowedDir+string(filepath.Separator)) && r.Path != allowedDir {
			t.Fatalf("hit outside allow-set: %s", r.Path)
		}
	}
}

func TestPathAllowEmptyReturnsNoHits(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(docs, "a.md")
	if err := os.WriteFile(path, []byte("EMPTY_ALLOW_TOKEN\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestSearchService(t, root)
	if _, err := svc.IndexPaths([]string{docs}, false); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Search(SearchParams{
		Query: "EMPTY_ALLOW_TOKEN",
		Mode:  search.ModeKeyword,
		Limit: 5,
		Allow: &PathAllow{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 0 {
		t.Fatalf("expected empty results, got %+v", out.Results)
	}
}

func TestPathAllowHybridRespectsPrefixes(t *testing.T) {
	root := t.TempDir()
	allowedDir := filepath.Join(root, "ok")
	otherDir := filepath.Join(root, "nope")
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}

	allowedPath := filepath.Join(allowedDir, "hit.md")
	outsidePath := filepath.Join(otherDir, "miss.md")
	if err := os.WriteFile(allowedPath, []byte("HYBRID_ALLOW_TOKEN alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsidePath, []byte(strings.Repeat("HYBRID_ALLOW_TOKEN ", 30)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestSearchService(t, root)
	svc.embedForTest = &stubEmbedder{}
	if _, err := svc.IndexPaths([]string{root}, false); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Search(SearchParams{
		Query: "HYBRID_ALLOW_TOKEN",
		Mode:  search.ModeHybrid,
		Limit: 5,
		Allow: &PathAllow{Prefixes: []string{allowedDir}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 1 || out.Results[0].Path != allowedPath {
		t.Fatalf("expected only allowed hybrid hit, got %+v", out.Results)
	}
}

func TestPathWithinPrefixDoesNotMatchSiblingPrefix(t *testing.T) {
	if pathWithinPrefix("/docs-private/a.md", "/docs") {
		t.Fatal("/docs must not match /docs-private")
	}
	if !pathWithinPrefix("/docs/a.md", "/docs") {
		t.Fatal("/docs should match /docs/a.md")
	}
	if !config.PathWithin("/docs/a.md", "/docs") {
		t.Fatal("config.PathWithin mismatch")
	}
}

func newTestSearchService(t *testing.T, root string) *Service {
	t.Helper()
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
	t.Cleanup(func() { _ = svc.Close() })
	svc.embedForTest = &stubEmbedder{}
	return svc
}
