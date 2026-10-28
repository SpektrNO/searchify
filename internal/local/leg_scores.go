package local

import (
	"github.com/spektr/searchify/internal/rank"
	"github.com/spektr/searchify/internal/search"
)

func float64Ptr(v float64) *float64 { return &v }
func intPtr(v int) *int             { return &v }

func attachKeywordLeg(r *search.Result, score float64, rank int) {
	r.KeywordScore = float64Ptr(score)
	r.KeywordRank = intPtr(rank)
}

func attachVectorLeg(r *search.Result, score float64, rank int) {
	r.VectorScore = float64Ptr(score)
	r.VectorRank = intPtr(rank)
}

// attachHybridLegScores copies per-leg BM25/cosine and 1-based ranks from the
// candidate lists onto fused hits. Missing leg → fields left nil (omitted in JSON).
func attachHybridLegScores(results []search.Result, keyword, vector []rank.RankedItem) {
	type legMeta struct {
		score float64
		rank  int
	}
	kw := make(map[string]legMeta, len(keyword))
	for i, item := range keyword {
		kw[item.ID] = legMeta{score: item.Score, rank: i + 1}
	}
	vec := make(map[string]legMeta, len(vector))
	for i, item := range vector {
		vec[item.ID] = legMeta{score: item.Score, rank: i + 1}
	}
	for i := range results {
		id := results[i].ID
		if m, ok := kw[id]; ok {
			attachKeywordLeg(&results[i], m.score, m.rank)
		}
		if m, ok := vec[id]; ok {
			attachVectorLeg(&results[i], m.score, m.rank)
		}
	}
}
