package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/local"
)

type lookupSymbolInput struct {
	Query      string `json:"query" jsonschema:"symbol name or qual_name (exact or prefix)"`
	Kind       string `json:"kind,omitempty" jsonschema:"optional kind filter: function, class, method, …"`
	PathPrefix string `json:"path_prefix,omitempty" jsonschema:"optional path or directory prefix"`
	Limit      int    `json:"limit,omitempty" jsonschema:"max results (default 20, max 100)"`
}

type lookupSymbolOutput struct {
	Count      int                `json:"count"`
	Symbols    []local.SymbolHit  `json:"symbols"`
	DurationMs int                `json:"duration_ms"`
}

type findReferencesInput struct {
	Symbol     string `json:"symbol" jsonschema:"symbol name or qual_name"`
	PathPrefix string `json:"path_prefix,omitempty" jsonschema:"optional path or directory prefix"`
	Limit      int    `json:"limit,omitempty" jsonschema:"max results (default 20, max 100)"`
}

type findReferencesOutput struct {
	Count      int             `json:"count"`
	Refs       []local.RefHit  `json:"refs"`
	DurationMs int             `json:"duration_ms"`
}

func (s *Server) lookupSymbol(ctx context.Context, req *mcp.CallToolRequest, input lookupSymbolInput) (*mcp.CallToolResult, lookupSymbolOutput, error) {
	_ = ctx
	_ = req
	start := time.Now()
	hits, err := s.local.LookupSymbol(local.LookupSymbolParams{
		Query:      input.Query,
		Kind:       input.Kind,
		PathPrefix: input.PathPrefix,
		Limit:      input.Limit,
	})
	out := lookupSymbolOutput{Symbols: hits, Count: len(hits), DurationMs: elapsedMs(start)}
	if hits == nil {
		out.Symbols = []local.SymbolHit{}
	}
	if err != nil {
		return toolErrorResult("%v", err), out, nil
	}
	logToolTiming("lookup_symbol", out.DurationMs)
	return nil, out, nil
}

func (s *Server) findReferences(ctx context.Context, req *mcp.CallToolRequest, input findReferencesInput) (*mcp.CallToolResult, findReferencesOutput, error) {
	_ = ctx
	_ = req
	start := time.Now()
	hits, err := s.local.FindReferences(local.FindReferencesParams{
		Symbol:     input.Symbol,
		PathPrefix: input.PathPrefix,
		Limit:      input.Limit,
	})
	out := findReferencesOutput{Refs: hits, Count: len(hits), DurationMs: elapsedMs(start)}
	if hits == nil {
		out.Refs = []local.RefHit{}
	}
	if err != nil {
		return toolErrorResult("%v", err), out, nil
	}
	logToolTiming("find_references", out.DurationMs)
	return nil, out, nil
}
