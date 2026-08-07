package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/file"
	"github.com/spektr/searchify/internal/search"
)

type searchFileInput struct {
	Path          string `json:"path" jsonschema:"file path; absolute under SEARCHIFY_ROOTS, or relative resolved under roots / SEARCHIFY_PATH_BASE"`
	Query         string `json:"query" jsonschema:"text to search for"`
	Limit         int    `json:"limit,omitempty" jsonschema:"maximum number of matches to return (default 10)"`
	CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema:"match case exactly when true"`
}

type searchFileOutput struct {
	Count      int             `json:"count"`
	Results    []search.Result `json:"results"`
	DurationMs int             `json:"duration_ms"`
}

func (s *Server) searchFile(ctx context.Context, req *mcp.CallToolRequest, input searchFileInput) (*mcp.CallToolResult, searchFileOutput, error) {
	_ = ctx
	_ = req
	start := time.Now()

	allowed, err := s.cfg.AllowedPath(input.Path)
	if err != nil {
		return toolErrorResult("access denied: %v", err), searchFileOutput{DurationMs: elapsedMs(start)}, nil
	}

	results, err := file.Search(allowed, file.SearchOptions{
		Query:         input.Query,
		Limit:         input.Limit,
		CaseSensitive: input.CaseSensitive,
	})
	if err != nil {
		return toolErrorResult("search failed: %v", err), searchFileOutput{DurationMs: elapsedMs(start)}, nil
	}

	duration := elapsedMs(start)
	logToolTiming("search_file", duration)
	return nil, searchFileOutput{
		Count:      len(results),
		Results:    results,
		DurationMs: duration,
	}, nil
}
