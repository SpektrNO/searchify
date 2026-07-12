package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/file"
	"github.com/spektr/searchify/internal/search"
)

type searchFileInput struct {
	Path          string `json:"path" jsonschema:"absolute or relative path to the file"`
	Query         string `json:"query" jsonschema:"text to search for"`
	Limit         int    `json:"limit,omitempty" jsonschema:"maximum number of matches to return (default 10)"`
	CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema:"match case exactly when true"`
}

type searchFileOutput struct {
	Count   int             `json:"count"`
	Results []search.Result `json:"results"`
}

func (s *Server) searchFile(ctx context.Context, req *mcp.CallToolRequest, input searchFileInput) (*mcp.CallToolResult, searchFileOutput, error) {
	_ = ctx
	_ = req

	allowed, err := s.cfg.AllowedPath(input.Path)
	if err != nil {
		return toolErrorResult("access denied: %v", err), searchFileOutput{}, nil
	}

	results, err := file.Search(allowed, file.SearchOptions{
		Query:         input.Query,
		Limit:         input.Limit,
		CaseSensitive: input.CaseSensitive,
	})
	if err != nil {
		return toolErrorResult("search failed: %v", err), searchFileOutput{}, nil
	}

	return nil, searchFileOutput{
		Count:   len(results),
		Results: results,
	}, nil
}
