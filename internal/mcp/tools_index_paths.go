package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/local"
)

type indexPathsInput struct {
	Paths []string `json:"paths" jsonschema:"files or directories to index under allowed roots"`
	Force bool     `json:"force,omitempty" jsonschema:"re-index even when file metadata is unchanged"`
}

type indexPathsOutput struct {
	Indexed    int      `json:"indexed"`
	Updated    int      `json:"updated"`
	Skipped    int      `json:"skipped"`
	Errors     int      `json:"errors"`
	Messages   []string `json:"messages,omitempty"`
	DurationMs int      `json:"duration_ms"`
}

func (s *Server) indexPaths(ctx context.Context, req *mcp.CallToolRequest, input indexPathsInput) (*mcp.CallToolResult, indexPathsOutput, error) {
	_ = req
	start := time.Now()

	if len(input.Paths) == 0 {
		return toolErrorResult("paths is required"), indexPathsOutput{DurationMs: elapsedMs(start)}, nil
	}

	report, err := s.local.IndexPathsOpts(input.Paths, local.IndexPathsOptions{Context: ctx, Force: input.Force})
	if err != nil {
		return toolErrorResult("index failed: %v", err), indexPathsOutput{DurationMs: elapsedMs(start)}, nil
	}

	duration := elapsedMs(start)
	logToolTiming("index_paths", duration)
	return nil, indexPathsOutput{
		Indexed:    report.Indexed,
		Updated:    report.Updated,
		Skipped:    report.Skipped,
		Errors:     report.Errors,
		Messages:   report.Messages,
		DurationMs: duration,
	}, nil
}
