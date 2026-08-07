package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spektr/searchify/internal/local"
	"github.com/spektr/searchify/internal/search"
)

type restErrorBody struct {
	Error string `json:"error"`
}

type restSearchRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Rerank bool   `json:"rerank,omitempty"`
}

type restIndexRequest struct {
	Paths []string `json:"paths"`
	Force bool     `json:"force,omitempty"`
}

func (s *Server) handleV1Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req restSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeJSONError(w, http.StatusBadRequest, "query is required")
		return
	}

	start := time.Now()
	out, _, err := s.executeSearchLocal(r.Context(), searchLocalInput{
		Query:  req.Query,
		Limit:  req.Limit,
		Mode:   req.Mode,
		Rerank: req.Rerank,
	})
	out.DurationMs = elapsedMs(start)
	if err != nil {
		status := http.StatusInternalServerError
		if isClientSearchError(err) {
			status = http.StatusBadRequest
		}
		writeJSONError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleV1Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req restIndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Paths) == 0 {
		writeJSONError(w, http.StatusBadRequest, "paths is required")
		return
	}

	start := time.Now()
	report, err := s.local.IndexPaths(req.Paths, req.Force)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "index failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, indexPathsOutput{
		Indexed:    report.Indexed,
		Updated:    report.Updated,
		Skipped:    report.Skipped,
		Errors:     report.Errors,
		Messages:   report.Messages,
		DurationMs: elapsedMs(start),
	})
}

func (s *Server) executeSearchLocal(ctx context.Context, input searchLocalInput) (searchLocalOutput, search.Mode, error) {
	mode, err := parseSearchMode(input.Mode)
	if err != nil {
		return searchLocalOutput{}, "", err
	}

	if input.Rerank && s.cfg.LangSearch == "" {
		return searchLocalOutput{}, "", fmt.Errorf("LANGSEARCH_API_KEY is required when rerank=true")
	}

	outcome, err := s.local.Search(local.SearchParams{
		Query: input.Query,
		Limit: input.Limit,
		Mode:  mode,
	})
	if err != nil {
		return searchLocalOutput{}, "", fmt.Errorf("search failed: %w", err)
	}

	results := outcome.Results
	timing := &searchLocalTiming{}
	switch outcome.Mode {
	case search.ModeKeyword:
		timing.KeywordMs = intPtr(outcome.Timing.KeywordMs)
	case search.ModeVector:
		timing.VectorMs = intPtr(outcome.Timing.VectorMs)
	case search.ModeHybrid:
		timing.KeywordMs = intPtr(outcome.Timing.KeywordMs)
		timing.VectorMs = intPtr(outcome.Timing.VectorMs)
		timing.RRFMs = intPtr(outcome.Timing.RRFMs)
	}

	if input.Rerank && len(results) > 0 {
		rerankStart := time.Now()
		results, err = rerankResults(ctx, s.web, input.Query, results, input.Limit)
		timing.RerankMs = intPtr(elapsedMs(rerankStart))
		if err != nil {
			return searchLocalOutput{Timing: timing}, outcome.Mode, fmt.Errorf("rerank failed: %w", err)
		}
	}

	return searchLocalOutput{
		Count:   len(results),
		Results: results,
		Timing:  timing,
	}, outcome.Mode, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, restErrorBody{Error: msg})
}

func isClientSearchError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.HasPrefix(msg, "unknown mode") ||
		strings.Contains(msg, "LANGSEARCH_API_KEY is required")
}
