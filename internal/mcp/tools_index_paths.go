package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type indexPathsInput struct {
	Paths []string `json:"paths" jsonschema:"files or directories to index under allowed roots"`
	Force bool     `json:"force,omitempty" jsonschema:"re-index even when file metadata is unchanged"`
}

type indexPathsOutput struct {
	Indexed  int      `json:"indexed"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Errors   int      `json:"errors"`
	Messages []string `json:"messages,omitempty"`
}

func (s *Server) indexPaths(ctx context.Context, req *mcp.CallToolRequest, input indexPathsInput) (*mcp.CallToolResult, indexPathsOutput, error) {
	_ = ctx
	_ = req

	if len(input.Paths) == 0 {
		return toolErrorResult("paths is required"), indexPathsOutput{}, nil
	}

	report, err := s.local.IndexPaths(input.Paths, input.Force)
	if err != nil {
		return toolErrorResult("index failed: %v", err), indexPathsOutput{}, nil
	}

	return nil, indexPathsOutput{
		Indexed:  report.Indexed,
		Updated:  report.Updated,
		Skipped:  report.Skipped,
		Errors:   report.Errors,
		Messages: report.Messages,
	}, nil
}
