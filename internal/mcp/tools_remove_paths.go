package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type removePathsInput struct {
	Paths []string `json:"paths" jsonschema:"files or directories to remove from the index (under allowed roots; need not exist on disk)"`
}

type removePathsOutput struct {
	Removed    int      `json:"removed"`
	Skipped    int      `json:"skipped"`
	Errors     int      `json:"errors"`
	Messages   []string `json:"messages,omitempty"`
	DurationMs int      `json:"duration_ms"`
}

func (s *Server) removePaths(ctx context.Context, req *mcp.CallToolRequest, input removePathsInput) (*mcp.CallToolResult, removePathsOutput, error) {
	_ = ctx
	_ = req
	start := time.Now()

	if len(input.Paths) == 0 {
		return toolErrorResult("paths is required"), removePathsOutput{DurationMs: elapsedMs(start)}, nil
	}

	report, err := s.local.RemovePaths(input.Paths)
	if err != nil {
		return toolErrorResult("remove failed: %v", err), removePathsOutput{DurationMs: elapsedMs(start)}, nil
	}

	duration := elapsedMs(start)
	logToolTiming("remove_paths", duration, "paths", len(input.Paths), "removed", report.Removed)
	return nil, removePathsOutput{
		Removed:    report.Removed,
		Skipped:    report.Skipped,
		Errors:     report.Errors,
		Messages:   report.Messages,
		DurationMs: duration,
	}, nil
}
