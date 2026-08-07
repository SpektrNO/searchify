package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/local"
	"github.com/spektr/searchify/internal/web"
)

const (
	serverName    = "searchify"
	serverVersion = "0.7.0"
)

type Server struct {
	cfg         *config.Config
	local       *local.Service
	web         *web.Client
	mcp         *mcp.Server
	watchCancel context.CancelFunc
}

func NewServer(cfg *config.Config) (*Server, error) {
	localSvc, err := local.NewService(cfg)
	if err != nil {
		return nil, fmt.Errorf("local index: %w", err)
	}

	s := &Server{
		cfg:   cfg,
		local: localSvc,
		web:   web.NewClient(cfg.LangSearch),
	}
	s.mcp = mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	s.registerTools()
	s.startWatch()
	return s, nil
}

func (s *Server) startWatch() {
	if len(s.cfg.WatchPaths) == 0 {
		return
	}
	w, err := local.NewIndexWatcher(s.cfg, s.local)
	if err != nil {
		slog.Warn("index watch disabled", "err", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.watchCancel = cancel
	go func() {
		if err := w.Run(ctx); err != nil && err != context.Canceled {
			slog.Warn("index watch exited", "err", err)
		}
	}()
}

func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) Local() *local.Service {
	return s.local
}

func (s *Server) Close() error {
	if s.watchCancel != nil {
		s.watchCancel()
		s.watchCancel = nil
	}
	if s.local == nil {
		return nil
	}
	return s.local.Close()
}

func (s *Server) registerTools() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "search_file",
		Description: "Search for text within a single local file under configured roots.",
	}, s.searchFile)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "search_local",
		Description: "Hybrid local search (keyword, vector, hybrid) over the persisted index.",
	}, s.searchLocal)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "search_web",
		Description: "Search the internet via LangSearch. Requires LANGSEARCH_API_KEY.",
	}, s.searchWeb)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "index_paths",
		Description: "Build or incrementally update the local keyword index for paths under allowed roots.",
	}, s.indexPaths)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "remove_paths",
		Description: "Remove files or directories from the local index (FTS + vectors). Paths need not exist on disk.",
	}, s.removePaths)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "index_prune",
		Description: "Drop index rows for files missing on disk or outside SEARCHIFY_ROOTS. Optional dry_run.",
	}, s.indexPrune)

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
