package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listIndexedFilesInput struct {
	Prefix string `json:"prefix,omitempty" jsonschema:"optional path prefix; list that file and descendants under the index"`
}

type listIndexedFilesOutput struct {
	Count      int      `json:"count"`
	Paths      []string `json:"paths"`
	DurationMs int      `json:"duration_ms"`
}

func (s *Server) listIndexedFiles(ctx context.Context, req *mcp.CallToolRequest, input listIndexedFilesInput) (*mcp.CallToolResult, listIndexedFilesOutput, error) {
	_ = ctx
	_ = req
	start := time.Now()

	paths, err := s.local.ListIndexedFiles(input.Prefix)
	if err != nil {
		return toolErrorResult("list indexed files failed: %v", err), listIndexedFilesOutput{DurationMs: elapsedMs(start)}, nil
	}
	if paths == nil {
		paths = []string{}
	}

	duration := elapsedMs(start)
	logToolTiming("list_indexed_files", duration)
	return nil, listIndexedFilesOutput{
		Count:      len(paths),
		Paths:      paths,
		DurationMs: duration,
	}, nil
}
