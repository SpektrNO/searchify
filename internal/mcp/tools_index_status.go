package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type indexStatusInput struct{}

type indexStatusOutput struct {
	Status indexStatusPayload `json:"status"`
}

type indexStatusPayload struct {
	IndexDir             string   `json:"index_dir"`
	Roots                []string `json:"roots"`
	DocumentCount        int      `json:"document_count"`
	ChunkCount           int      `json:"chunk_count"`
	VectorChunkCount     int      `json:"vector_chunk_count"`
	EmbedModel           string   `json:"embed_model,omitempty"`
	VectorReady          bool     `json:"vector_ready"`
	LangSearchConfigured bool     `json:"langsearch_configured"`
	WatchEnabled         bool     `json:"watch_enabled"`
	WatchPaths           []string `json:"watch_paths,omitempty"`
	OCREnabled           bool     `json:"ocr_enabled"`
	IndexExtensions      []string `json:"index_extensions,omitempty"`
	IndexedAt            *string  `json:"indexed_at"`
	Ready                bool     `json:"ready"`
	Message              string   `json:"message,omitempty"`
}

func (s *Server) indexStatus(ctx context.Context, req *mcp.CallToolRequest, _ indexStatusInput) (*mcp.CallToolResult, indexStatusOutput, error) {
	_ = ctx
	_ = req

	status, err := s.local.Status()
	if err != nil {
		return nil, indexStatusOutput{}, fmt.Errorf("index status: %w", err)
	}

	langConfigured := s.web != nil && s.web.Configured()
	msg := status.Message
	if !langConfigured && msg == "" {
		msg = "LANGSEARCH_API_KEY not set; search_web and rerank unavailable"
	}

	return nil, indexStatusOutput{Status: indexStatusPayload{
		IndexDir:             status.IndexDir,
		Roots:                status.Roots,
		DocumentCount:        status.DocumentCount,
		ChunkCount:           status.ChunkCount,
		VectorChunkCount:     status.VectorChunkCount,
		EmbedModel:           status.EmbedModel,
		VectorReady:          status.VectorReady,
		LangSearchConfigured: langConfigured,
		WatchEnabled:         len(s.cfg.WatchPaths) > 0,
		WatchPaths:           append([]string(nil), s.cfg.WatchPaths...),
		OCREnabled:           status.OCREnabled,
		IndexExtensions:      append([]string(nil), status.IndexExtensions...),
		IndexedAt:            status.IndexedAt,
		Ready:                status.Ready,
		Message:              msg,
	}}, nil
}
