package rank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const langSearchRerankURL = "https://api.langsearch.com/v1/rerank"

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

// Rerank reorders documents using the LangSearch semantic rerank API.
func Rerank(ctx context.Context, apiKey, query string, documents []string, topN int) ([]RankedItem, error) {
	if apiKey == "" {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, langSearchRerankURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rerank HTTP %d: %s", resp.StatusCode, trimBody(raw))
	}

	var parsed rerankResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode rerank response: %w", err)
	}
	if parsed.Code != 0 && parsed.Code != http.StatusOK {
		if parsed.Msg != "" {
			return nil, fmt.Errorf("rerank API error: %s", parsed.Msg)
		}
		return nil, fmt.Errorf("rerank API error code %d", parsed.Code)
	}

	out := make([]RankedItem, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.Index < 0 || r.Index >= len(documents) {
			continue
		}
		out = append(out, RankedItem{
			ID:    fmt.Sprintf("%d", r.Index),
			Score: r.RelevanceScore,
		})
	}
	return out, nil
}

func trimBody(b []byte) string {
	const max = 200
	s := string(bytes.TrimSpace(b))
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
