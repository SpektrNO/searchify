package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type indexPruneInput struct {
	Paths  []string `json:"paths,omitempty" jsonschema:"optional files or directories to limit prune scope (under allowed roots)"`
	DryRun bool     `json:"dry_run,omitempty" jsonschema:"if true, report orphans without deleting"`
}

type indexPruneOutput struct {
	Scanned    int      `json:"scanned"`
	Removed    int      `json:"removed"`
	Skipped    int      `json:"skipped"`
	Errors     int      `json:"errors"`
	DryRun     bool     `json:"dry_run,omitempty"`
	Messages   []string `json:"messages,omitempty"`
	DurationMs int      `json:"duration_ms"`
}

func (s *Server) indexPrune(ctx context.Context, req *mcp.CallToolRequest, input indexPruneInput) (*mcp.CallToolResult, indexPruneOutput, error) {
	_ = ctx
	_ = req
	start := time.Now()

	report, err := s.local.PruneIndex(input.Paths, input.DryRun)
	if err != nil {
		return toolErrorResult("prune failed: %v", err), indexPruneOutput{DurationMs: elapsedMs(start)}, nil
	}

	duration := elapsedMs(start)
	logToolTiming("index_prune", duration, "removed", report.Removed, "dry_run", report.DryRun)
	return nil, indexPruneOutput{
		Scanned:    report.Scanned,
		Removed:    report.Removed,
		Skipped:    report.Skipped,
		Errors:     report.Errors,
		DryRun:     report.DryRun,
		Messages:   report.Messages,
		DurationMs: duration,
	}, nil
}
