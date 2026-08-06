package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spektr/searchify/internal/search"
)

const webSearchPath = "/v1/web-search"

// SearchOptions controls a web search request.
type SearchOptions struct {
	Query     string
	Count     int
	Freshness string
	Summary   bool
}

type webSearchRequest struct {
	Query     string `json:"query"`
	Freshness string `json:"freshness,omitempty"`
	Summary   bool   `json:"summary"`
	Count     int    `json:"count"`
}

type webSearchResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		WebPages struct {
			Value []webPageValue `json:"value"`
		} `json:"webPages"`
	} `json:"data"`
}

type webPageValue struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Snippet  string `json:"snippet"`
	Summary  string `json:"summary"`
}

// WebSearch queries LangSearch and maps results to search.Result.
func (c *Client) WebSearch(ctx context.Context, opts SearchOptions) ([]search.Result, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("LANGSEARCH_API_KEY is required for search_web")
	}

	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	count := opts.Count
	if count <= 0 {
		count = 10
	}
	if count > 10 {
		count = 10
	}

	freshness := strings.TrimSpace(opts.Freshness)
	if freshness == "" {
		freshness = "noLimit"
	}
	switch freshness {
	case "oneDay", "oneWeek", "oneMonth", "oneYear", "noLimit":
		// ok
	default:
		return nil, fmt.Errorf("invalid freshness %q (want oneDay|oneWeek|oneMonth|oneYear|noLimit)", freshness)
	}

	cacheKey := fmt.Sprintf("%s|%s|%t|%d", query, freshness, opts.Summary, count)
	if raw, ok := c.cache.get(cacheKey); ok {
		return parseWebSearchResults(raw)
	}

	body, err := json.Marshal(webSearchRequest{
		Query:     query,
		Freshness: freshness,
		Summary:   opts.Summary,
		Count:     count,
	})
	if err != nil {
		return nil, err
	}

	raw, err := c.postJSON(ctx, webSearchPath, body)
	if err != nil {
		return nil, err
	}

	results, err := parseWebSearchResults(raw)
	if err != nil {
		return nil, err
	}
	c.cache.set(cacheKey, raw)
	return results, nil
}

func parseWebSearchResults(raw []byte) ([]search.Result, error) {
	var parsed webSearchResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode web search response: %w", err)
	}
	if parsed.Code != 0 && parsed.Code != 200 {
		if parsed.Msg != "" {
			return nil, fmt.Errorf("web search API error: %s", parsed.Msg)
		}
		return nil, fmt.Errorf("web search API error code %d", parsed.Code)
	}

	pages := parsed.Data.WebPages.Value
	out := make([]search.Result, 0, len(pages))
	for i, p := range pages {
		snippet := strings.TrimSpace(p.Summary)
		if snippet == "" {
			snippet = strings.TrimSpace(p.Snippet)
		}
		id := p.ID
		if id == "" {
			id = p.URL
		}
		out = append(out, search.Result{
			ID:      id,
			Title:   p.Name,
			URL:     p.URL,
			Snippet: snippet,
			Score:   1.0 / float64(i+1),
			Source:  "web",
		})
	}
	return out, nil
}
