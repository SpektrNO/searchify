package mcp

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/search"
	"github.com/spektr/searchify/internal/web"
)

type searchWebInput struct {
	Query     string `json:"query" jsonschema:"search query"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 10, max 10)"`
	Freshness string `json:"freshness,omitempty" jsonschema:"time filter: oneDay, oneWeek, oneMonth, oneYear, or noLimit"`
	// Summary defaults to true when omitted; use pointer so false can be set explicitly.
	Summary *bool `json:"summary,omitempty" jsonschema:"include long LLM-friendly summaries (default true)"`
}

type searchWebOutput struct {
	Count   int             `json:"count"`
	Results []search.Result `json:"results"`
}

func (s *Server) searchWeb(ctx context.Context, req *mcp.CallToolRequest, input searchWebInput) (*mcp.CallToolResult, searchWebOutput, error) {
	_ = req

	if s.web == nil || !s.web.Configured() {
		return toolErrorResult("LANGSEARCH_API_KEY is required for search_web"), searchWebOutput{}, nil
	}

	summary := true
	if input.Summary != nil {
		summary = *input.Summary
	}

	results, err := s.web.WebSearch(ctx, web.SearchOptions{
		Query:     strings.TrimSpace(input.Query),
		Count:     input.Limit,
		Freshness: input.Freshness,
		Summary:   summary,
	})
	if err != nil {
		return toolErrorResult("web search failed: %v", err), searchWebOutput{}, nil
	}

	return nil, searchWebOutput{
		Count:   len(results),
		Results: results,
	}, nil
}
