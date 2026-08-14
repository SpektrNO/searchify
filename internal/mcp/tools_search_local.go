package mcp

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spektr/searchify/internal/rank"
	"github.com/spektr/searchify/internal/search"
	"github.com/spektr/searchify/internal/web"
)

type searchLocalInput struct {
	Query      string `json:"query" jsonschema:"search query"`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 10, max 50)"`
	Mode       string `json:"mode,omitempty" jsonschema:"search mode: keyword, vector, or hybrid (default hybrid when vectors exist)"`
	Rerank     bool   `json:"rerank,omitempty" jsonschema:"rerank results with LangSearch (requires LANGSEARCH_API_KEY)"`
	SnippetMax int    `json:"snippet_max,omitempty" jsonschema:"max snippet characters (default SEARCHIFY_SNIPPET_CHARS or 300, max 8000)"`
}

type searchLocalTiming struct {
	KeywordMs *int `json:"keyword_ms,omitempty"`
	VectorMs  *int `json:"vector_ms,omitempty"`
	RRFMs     *int `json:"rrf_ms,omitempty"`
	RerankMs  *int `json:"rerank_ms,omitempty"`
}

type searchLocalOutput struct {
	Count      int                `json:"count"`
	Results    []search.Result    `json:"results"`
	DurationMs int                `json:"duration_ms"`
	Timing     *searchLocalTiming `json:"timing,omitempty"`
}

func intPtr(v int) *int { return &v }

func (s *Server) searchLocal(ctx context.Context, req *mcp.CallToolRequest, input searchLocalInput) (*mcp.CallToolResult, searchLocalOutput, error) {
	_ = req
	start := time.Now()

	out, mode, err := s.executeSearchLocal(ctx, input)
	out.DurationMs = elapsedMs(start)
	if err != nil {
		return toolErrorResult("%v", err), out, nil
	}

	logToolTiming("search_local", out.DurationMs, "mode", string(mode))
	return nil, out, nil
}

func parseSearchMode(raw string) (search.Mode, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "":
		return "", nil
	case "keyword":
		return search.ModeKeyword, nil
	case "vector":
		return search.ModeVector, nil
	case "hybrid":
		return search.ModeHybrid, nil
	default:
		return "", errUnknownMode{mode: raw}
	}
}

type errUnknownMode struct{ mode string }

func (e errUnknownMode) Error() string {
	return "unknown mode " + strconv.Quote(e.mode)
}

func rerankResults(ctx context.Context, client *web.Client, query string, results []search.Result, limit int) ([]search.Result, error) {
	docs := make([]string, len(results))
	for i, r := range results {
		docs[i] = r.Snippet
	}

	topN := limit
	if topN <= 0 {
		topN = len(results)
	}
	if topN > len(results) {
		topN = len(results)
	}

	ranked, err := rank.RerankWithClient(ctx, client, query, docs, topN)
	if err != nil {
		return nil, err
	}

	out := make([]search.Result, 0, len(ranked))
	for _, item := range ranked {
		idx, err := strconv.Atoi(item.ID)
		if err != nil || idx < 0 || idx >= len(results) {
			continue
		}
		r := results[idx]
		r.Score = item.Score
		out = append(out, r)
	}
	return out, nil
}
