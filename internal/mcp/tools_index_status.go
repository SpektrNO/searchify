package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/search"
)

type indexStatusInput struct{}

type indexStatusOutput struct {
	Status search.IndexStatus `json:"status"`
}

func (s *Server) indexStatus(ctx context.Context, req *mcp.CallToolRequest, _ indexStatusInput) (*mcp.CallToolResult, indexStatusOutput, error) {
	_ = ctx
	_ = req

	return nil, indexStatusOutput{Status: search.IndexStatus{
		IndexDir:      s.cfg.IndexDir,
		Roots:         append([]string(nil), s.cfg.Roots...),
		DocumentCount: 0,
		ChunkCount:    0,
		IndexedAt:     nil,
		Ready:         false,
		Message:       "index not built yet; use index_paths in a future release",
	}}, nil
}
