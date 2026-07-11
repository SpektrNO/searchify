package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/config"
)

const (
	serverName    = "searchify"
	serverVersion = "0.1.0"
)

type Server struct {
	cfg *config.Config
	mcp *mcp.Server
}

func NewServer(cfg *config.Config) *Server {
	s := &Server{cfg: cfg}
	s.mcp = mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	s.registerTools()
	return s
}

func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) registerTools() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "search_file",
		Description: "Search for text within a single local file under configured roots.",
	}, s.searchFile)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "index_status",
		Description: "Report local index status, configured roots, and readiness.",
	}, s.indexStatus)
}

func toolErrorResult(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(format, args...)},
		},
		IsError: true,
	}
}
