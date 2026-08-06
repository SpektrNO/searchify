package rank

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spektr/searchify/internal/web"
)

// Rerank reorders documents using the LangSearch semantic rerank API.
// Prefer injecting a shared web.Client via RerankWithClient when available.
func Rerank(ctx context.Context, apiKey, query string, documents []string, topN int) ([]RankedItem, error) {
	return RerankWithClient(ctx, web.NewClient(apiKey), query, documents, topN)
}

// RerankWithClient uses an existing LangSearch client.
func RerankWithClient(ctx context.Context, client *web.Client, query string, documents []string, topN int) ([]RankedItem, error) {
	if client == nil {
		return nil, fmt.Errorf("LANGSEARCH_API_KEY is required for rerank")
	}
	ranked, err := client.Rerank(ctx, query, documents, topN)
	if err != nil {
		return nil, err
	}
	out := make([]RankedItem, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, RankedItem{
			ID:    strconv.Itoa(r.Index),
			Score: r.Score,
		})
	}
	return out, nil
}
