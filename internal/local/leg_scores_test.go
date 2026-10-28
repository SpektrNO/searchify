package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spektr/searchify/internal/search"
)

func TestLegScoresKeywordOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "kw.md")
	if err := os.WriteFile(path, []byte("LEG_SCORE_TOKEN alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestSearchService(t, root)
	if _, err := svc.IndexPaths([]string{root}, false); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Search(SearchParams{Query: "LEG_SCORE_TOKEN", Mode: search.ModeKeyword, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("expected hits")
	}
	r := out.Results[0]
	if r.KeywordScore == nil {
		t.Fatal("expected keyword_score")
	}
	if r.KeywordRank == nil || *r.KeywordRank != 1 {
		t.Fatalf("expected keyword_rank=1, got %v", r.KeywordRank)
	}
	if r.VectorScore != nil || r.VectorRank != nil {
		t.Fatalf("vector leg must be omitted in keyword mode: %+v", r)
	}
	if r.Score != *r.KeywordScore {
		t.Fatalf("keyword mode score should equal keyword_score: score=%v kw=%v", r.Score, *r.KeywordScore)
	}
}

func TestLegScoresVectorOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "vec.md")
	if err := os.WriteFile(path, []byte("LEG_VEC_TOKEN beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestSearchService(t, root)
	if _, err := svc.IndexPaths([]string{root}, false); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Search(SearchParams{Query: "LEG_VEC_TOKEN", Mode: search.ModeVector, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("expected hits")
	}
	r := out.Results[0]
	if r.VectorScore == nil {
		t.Fatal("expected vector_score")
	}
	if r.VectorRank == nil || *r.VectorRank != 1 {
		t.Fatalf("expected vector_rank=1, got %v", r.VectorRank)
	}
	if r.KeywordScore != nil || r.KeywordRank != nil {
		t.Fatalf("keyword leg must be omitted in vector mode: %+v", r)
	}
	if r.Score != *r.VectorScore {
		t.Fatalf("vector mode score should equal vector_score: score=%v vec=%v", r.Score, *r.VectorScore)
	}
}

func TestLegScoresHybridBothLegs(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hy.md")
	if err := os.WriteFile(path, []byte("LEG_HYBRID_TOKEN gamma delta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestSearchService(t, root)
	if _, err := svc.IndexPaths([]string{root}, false); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Search(SearchParams{Query: "LEG_HYBRID_TOKEN", Mode: search.ModeHybrid, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("expected hits")
	}
	r := out.Results[0]
	if r.KeywordScore == nil || r.VectorScore == nil {
		t.Fatalf("hybrid hit on both legs needs both scores: kw=%v vec=%v", r.KeywordScore, r.VectorScore)
	}
	if r.KeywordRank == nil || r.VectorRank == nil {
		t.Fatalf("expected both ranks: kw=%v vec=%v", r.KeywordRank, r.VectorRank)
	}
	if *r.KeywordRank < 1 || *r.VectorRank < 1 {
		t.Fatalf("ranks must be 1-based: kw=%d vec=%d", *r.KeywordRank, *r.VectorRank)
	}
}
