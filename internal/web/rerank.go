package web

import (
	"context"
	"encoding/json"
	"fmt"
)

const rerankPath = "/v1/rerank"

type rerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int      `json:"top_n,omitempty"`
	ReturnDocuments bool     `json:"return_documents"`
}

type rerankResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

// RerankResult is one reranked document referenced by original index.
type RerankResult struct {
	Index int
	Score float64
}

// Rerank reorders documents using the LangSearch semantic rerank API.
func (c *Client) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("LANGSEARCH_API_KEY is required for rerank")
	}
	if len(documents) == 0 {
		return nil, nil
	}
	if topN <= 0 || topN > len(documents) {
		topN = len(documents)
	}

	body, err := json.Marshal(rerankRequest{
		Model:           "langsearch-reranker-v1",
		Query:           query,
		Documents:       documents,
		TopN:            topN,
		ReturnDocuments: false,
	})
	if err != nil {
		return nil, err
	}

	raw, err := c.postJSON(ctx, rerankPath, body)
	if err != nil {
		return nil, err
	}

	var parsed rerankResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode rerank response: %w", err)
	}
	if parsed.Code != 0 && parsed.Code != 200 {
		if parsed.Msg != "" {
			return nil, fmt.Errorf("rerank API error: %s", parsed.Msg)
		}
		return nil, fmt.Errorf("rerank API error code %d", parsed.Code)
	}

	out := make([]RerankResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.Index < 0 || r.Index >= len(documents) {
			continue
		}
		out = append(out, RerankResult{
			Index: r.Index,
			Score: r.RelevanceScore,
		})
	}
	return out, nil
}
