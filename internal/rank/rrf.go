package rank

import "sort"

const DefaultRRFK = 60

// RankedItem is one retrieval result with a stable document ID.
type RankedItem struct {
	ID    string
	Score float64
}

// RRF merges multiple ranked lists using reciprocal rank fusion.
func RRF(lists [][]RankedItem, k int, limit int) []RankedItem {
	if k <= 0 {
		k = DefaultRRFK
	}
	if limit <= 0 {
		limit = 10
	}

	scores := make(map[string]float64)
	for _, list := range lists {
		for rank, item := range list {
			scores[item.ID] += 1.0 / (float64(k) + float64(rank+1))
		}
	}

	merged := make([]RankedItem, 0, len(scores))
	for id, score := range scores {
		merged = append(merged, RankedItem{ID: id, Score: score})
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Score == merged[j].Score {
			return merged[i].ID < merged[j].ID
		}
		return merged[i].Score > merged[j].Score
	})

	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}
