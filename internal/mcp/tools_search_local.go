package mcp

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/search"
)

type searchLocalInput struct {
	Query string `json:"query" jsonschema:"search query"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 10, max 50)"`
	Mode  string `json:"mode,omitempty" jsonschema:"search mode; phase 2 supports keyword only"`
}

type searchLocalOutput struct {
	Count   int             `json:"count"`
	Results []search.Result `json:"results"`
}

func (s *Server) searchLocal(ctx context.Context, req *mcp.CallToolRequest, input searchLocalInput) (*mcp.CallToolResult, searchLocalOutput, error) {
	_ = ctx
	_ = req

	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	switch mode {
	case "", "keyword":
		// ok
	case "vector", "hybrid":
		return toolErrorResult("mode %q not available until phase 3", input.Mode), searchLocalOutput{}, nil
	default:
		return toolErrorResult("unknown mode %q", input.Mode), searchLocalOutput{}, nil
	}

	results, err := s.local.Search(input.Query, input.Limit)
	if err != nil {
		return toolErrorResult("search failed: %v", err), searchLocalOutput{}, nil
	}

	return nil, searchLocalOutput{
		Count:   len(results),
		Results: results,
	}, nil
}
