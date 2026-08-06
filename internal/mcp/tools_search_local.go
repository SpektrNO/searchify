package mcp

import (
	"context"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/local"
	"github.com/spektr/searchify/internal/rank"
	"github.com/spektr/searchify/internal/search"
	"github.com/spektr/searchify/internal/web"
)

type searchLocalInput struct {
	Query  string `json:"query" jsonschema:"search query"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 10, max 50)"`
	Mode   string `json:"mode,omitempty" jsonschema:"search mode: keyword, vector, or hybrid (default hybrid when vectors exist)"`
	Rerank bool   `json:"rerank,omitempty" jsonschema:"rerank results with LangSearch (requires LANGSEARCH_API_KEY)"`
}

type searchLocalOutput struct {
	Count   int             `json:"count"`
	Results []search.Result `json:"results"`
}

func (s *Server) searchLocal(ctx context.Context, req *mcp.CallToolRequest, input searchLocalInput) (*mcp.CallToolResult, searchLocalOutput, error) {
	_ = req

	mode, err := parseSearchMode(input.Mode)
	if err != nil {
		return toolErrorResult("%v", err), searchLocalOutput{}, nil
	}

	if input.Rerank && s.cfg.LangSearch == "" {
		return toolErrorResult("LANGSEARCH_API_KEY is required when rerank=true"), searchLocalOutput{}, nil
	}

	limit := input.Limit
	results, err := s.local.Search(local.SearchParams{
		Query: input.Query,
		Limit: limit,
		Mode:  mode,
	})
	if err != nil {
		return toolErrorResult("search failed: %v", err), searchLocalOutput{}, nil
	}

	if input.Rerank && len(results) > 0 {
		results, err = rerankResults(ctx, s.web, input.Query, results, limit)
		if err != nil {
			return toolErrorResult("rerank failed: %v", err), searchLocalOutput{}, nil
		}
	}

	return nil, searchLocalOutput{
		Count:   len(results),
		Results: results,
	}, nil
}

func parseSearchMode(raw string) (search.Mode, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "":
		return "", nil
	case "keyword":
		return search.ModeKeyword, nil
	case "vector":
		return search.ModeVector, nil
	case "hybrid":
		return search.ModeHybrid, nil
	default:
		return "", errUnknownMode{mode: raw}
	}
}

type errUnknownMode struct{ mode string }

func (e errUnknownMode) Error() string {
	return "unknown mode " + strconv.Quote(e.mode)
}

func rerankResults(ctx context.Context, client *web.Client, query string, results []search.Result, limit int) ([]search.Result, error) {
	docs := make([]string, len(results))
	for i, r := range results {
		docs[i] = r.Snippet
	}

	topN := limit
	if topN <= 0 {
		topN = len(results)
	}
	if topN > len(results) {
		topN = len(results)
	}

	ranked, err := rank.RerankWithClient(ctx, client, query, docs, topN)
	if err != nil {
		return nil, err
	}

	out := make([]search.Result, 0, len(ranked))
	for _, item := range ranked {
		idx, err := strconv.Atoi(item.ID)
		if err != nil || idx < 0 || idx >= len(results) {
			continue
		}
		r := results[idx]
		r.Score = item.Score
		out = append(out, r)
	}
	return out, nil
}
