package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type indexStatsInput struct{}

type indexStatsOutput struct {
	FileCount        int     `json:"file_count"`
	FolderCount      int     `json:"folder_count"`
	VectorChunkCount int     `json:"vector_chunk_count"`
	TotalBytes       int64   `json:"total_bytes"`
	LastIndexChange  *string `json:"last_index_change,omitempty"`
	DurationMs       int     `json:"duration_ms"`
}

func (s *Server) indexStats(ctx context.Context, req *mcp.CallToolRequest, _ indexStatsInput) (*mcp.CallToolResult, indexStatsOutput, error) {
	_ = ctx
	_ = req
	start := time.Now()

	stats, err := s.local.Stats()
	if err != nil {
		return toolErrorResult("index stats failed: %v", err), indexStatsOutput{DurationMs: elapsedMs(start)}, nil
	}

	duration := elapsedMs(start)
	logToolTiming("index_stats", duration)
	return nil, indexStatsOutput{
		FileCount:        stats.FileCount,
		FolderCount:      stats.FolderCount,
		VectorChunkCount: stats.VectorChunkCount,
		TotalBytes:       stats.TotalBytes,
		LastIndexChange:  stats.LastIndexChange,
		DurationMs:       duration,
	}, nil
}
