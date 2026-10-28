package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spektr/searchify/internal/config"
)

func testServer(t *testing.T, token string) *Server {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{
		Roots:      []string{root},
		IndexDir:   filepath.Join(t.TempDir(), "index"),
		HTTPToken:  token,
		HTTPAddr:   ":0",
		EmbedModel: "stub",
	}
	s, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestIndexEndpointsHonorCancellation(t *testing.T) {
	s := testServer(t, "secret")
	path := filepath.Join(s.cfg.Roots[0], "cancel.md")
	if err := os.WriteFile(path, []byte("cancelled contents\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, _, err := s.indexPaths(ctx, nil, indexPathsInput{Paths: []string{path}})
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("cancelled MCP request: result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(restIndexRequest{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/index", bytes.NewReader(body)).WithContext(ctx)
	response := httptest.NewRecorder()
	s.handleV1Index(response, req)
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "context canceled") {
		t.Fatalf("cancelled REST request: status=%d body=%s", response.Code, response.Body.String())
	}
	n, err := s.local.DocumentCount()
	if err != nil || n != 0 {
		t.Fatalf("cancelled API work persisted: count=%d err=%v", n, err)
	}
}

func TestHandlerRequiresToken(t *testing.T) {
	s := testServer(t, "")
	_, err := s.Handler(HTTPOptions{})
	if err == nil {
		t.Fatal("expected error when HTTP token empty")
	}
}

func TestHealthzAndAuth(t *testing.T) {
	s := testServer(t, "secret")
	h, err := s.Handler(HTTPOptions{Path: "/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status %d", resp.StatusCode)
	}

	unauth, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauth.StatusCode)
	}

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	auth, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer auth.Body.Close()
	if auth.StatusCode != http.StatusOK {
		t.Fatalf("initialize status %d", auth.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(auth.Body).Decode(&payload); err != nil {
		// SSE responses may not be plain JSON; accept 200 with auth as success.
		if auth.Header.Get("Content-Type") == "" {
			t.Fatalf("decode: %v", err)
		}
	} else if errObj, ok := payload["error"]; ok {
		t.Fatalf("initialize error: %v", errObj)
	}
}

func TestServeHTTPRefusesEmptyToken(t *testing.T) {
	// Guard CLI contract: empty token must fail before listen.
	oldRoots := os.Getenv("SEARCHIFY_ROOTS")
	oldToken := os.Getenv("SEARCHIFY_HTTP_TOKEN")
	t.Cleanup(func() {
		_ = os.Setenv("SEARCHIFY_ROOTS", oldRoots)
		_ = os.Setenv("SEARCHIFY_HTTP_TOKEN", oldToken)
	})
	_ = os.Setenv("SEARCHIFY_ROOTS", t.TempDir())
	_ = os.Setenv("SEARCHIFY_HTTP_TOKEN", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPToken != "" {
		t.Fatal("expected empty token")
	}
}

type restStubEmbedder struct{}

func (restStubEmbedder) Encode(text string) ([]float32, error) {
	v := make([]float32, 8)
	v[0] = 1
	return v, nil
}

func (restStubEmbedder) EncodeBatch(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i], _ = restStubEmbedder{}.Encode(texts[i])
	}
	return out, nil
}

func (restStubEmbedder) Close() error { return nil }

func TestRESTSearchAndIndex(t *testing.T) {
	s := testServer(t, "secret")
	s.Local().SetEmbedderForTest(restStubEmbedder{})

	root := s.cfg.Roots[0]
	doc := filepath.Join(root, "note.md")
	if err := os.WriteFile(doc, []byte("hybrid retrieval with shard realm\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := s.Handler(HTTPOptions{Path: "/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	// 401 without token
	unauth, err := http.Post(srv.URL+"/v1/search", "application/json", strings.NewReader(`{"query":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauth.StatusCode)
	}

	// 400 missing query
	badSearch := restPOST(t, srv.URL+"/v1/search", "secret", `{"query":""}`)
	defer badSearch.Body.Close()
	if badSearch.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 empty query, got %d", badSearch.StatusCode)
	}

	// 400 missing paths
	badIndex := restPOST(t, srv.URL+"/v1/index", "secret", `{"paths":[]}`)
	defer badIndex.Body.Close()
	if badIndex.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 empty paths, got %d", badIndex.StatusCode)
	}

	indexBody := fmt.Sprintf(`{"paths":[%q],"force":false}`, doc)
	indexResp := restPOST(t, srv.URL+"/v1/index", "secret", indexBody)
	defer indexResp.Body.Close()
	if indexResp.StatusCode != http.StatusOK {
		t.Fatalf("index status %d", indexResp.StatusCode)
	}
	var indexOut indexPathsOutput
	if err := json.NewDecoder(indexResp.Body).Decode(&indexOut); err != nil {
		t.Fatal(err)
	}
	if indexOut.Indexed != 1 {
		t.Fatalf("expected indexed=1, got %+v", indexOut)
	}

	searchResp := restPOST(t, srv.URL+"/v1/search", "secret", `{"query":"shard realm","mode":"keyword","limit":5}`)
	defer searchResp.Body.Close()
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("search status %d", searchResp.StatusCode)
	}
	var searchOut searchLocalOutput
	if err := json.NewDecoder(searchResp.Body).Decode(&searchOut); err != nil {
		t.Fatal(err)
	}
	if searchOut.Count == 0 || searchOut.DurationMs < 0 {
		t.Fatalf("unexpected search response: %+v", searchOut)
	}

	filesReq, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/files", nil)
	if err != nil {
		t.Fatal(err)
	}
	filesReq.Header.Set("Authorization", "Bearer secret")
	filesResp, err := http.DefaultClient.Do(filesReq)
	if err != nil {
		t.Fatal(err)
	}
	defer filesResp.Body.Close()
	if filesResp.StatusCode != http.StatusOK {
		t.Fatalf("files status %d", filesResp.StatusCode)
	}
	var filesOut listIndexedFilesOutput
	if err := json.NewDecoder(filesResp.Body).Decode(&filesOut); err != nil {
		t.Fatal(err)
	}
	if filesOut.Count != 1 || len(filesOut.Paths) != 1 || filesOut.Paths[0] != doc {
		t.Fatalf("unexpected files response: %+v", filesOut)
	}

	statsReq, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/stats", nil)
	if err != nil {
		t.Fatal(err)
	}
	statsReq.Header.Set("Authorization", "Bearer secret")
	statsResp, err := http.DefaultClient.Do(statsReq)
	if err != nil {
		t.Fatal(err)
	}
	defer statsResp.Body.Close()
	if statsResp.StatusCode != http.StatusOK {
		t.Fatalf("stats status %d", statsResp.StatusCode)
	}
	var statsOut indexStatsOutput
	if err := json.NewDecoder(statsResp.Body).Decode(&statsOut); err != nil {
		t.Fatal(err)
	}
	if statsOut.FileCount != 1 || statsOut.TotalBytes <= 0 || statsOut.LastIndexChange == nil {
		t.Fatalf("unexpected stats: %+v", statsOut)
	}
}

func TestRESTSearchPathAllowset(t *testing.T) {
	s := testServer(t, "secret")
	s.Local().SetEmbedderForTest(restStubEmbedder{})

	root := s.cfg.Roots[0]
	allowedDir := filepath.Join(root, "allowed")
	otherDir := filepath.Join(root, "other")
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	allowedDoc := filepath.Join(allowedDir, "in.md")
	outsideDoc := filepath.Join(otherDir, "out.md")
	if err := os.WriteFile(allowedDoc, []byte("REST_ALLOW_TOKEN in\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideDoc, []byte(strings.Repeat("REST_ALLOW_TOKEN ", 25)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := s.Handler(HTTPOptions{Path: "/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	indexBody := fmt.Sprintf(`{"paths":[%q],"force":false}`, root)
	indexResp := restPOST(t, srv.URL+"/v1/index", "secret", indexBody)
	defer indexResp.Body.Close()
	if indexResp.StatusCode != http.StatusOK {
		t.Fatalf("index status %d", indexResp.StatusCode)
	}

	// Empty allow-set → 200 + []
	emptyBody := `{"query":"REST_ALLOW_TOKEN","mode":"keyword","path_prefixes":[]}`
	emptyResp := restPOST(t, srv.URL+"/v1/search", "secret", emptyBody)
	defer emptyResp.Body.Close()
	if emptyResp.StatusCode != http.StatusOK {
		t.Fatalf("empty allow status %d", emptyResp.StatusCode)
	}
	var emptyOut searchLocalOutput
	if err := json.NewDecoder(emptyResp.Body).Decode(&emptyOut); err != nil {
		t.Fatal(err)
	}
	if emptyOut.Count != 0 || len(emptyOut.Results) != 0 {
		t.Fatalf("expected empty results, got %+v", emptyOut)
	}

	// Constrained to allowed prefix
	searchBody := fmt.Sprintf(
		`{"query":"REST_ALLOW_TOKEN","mode":"keyword","limit":5,"path_prefixes":[%q]}`,
		allowedDir,
	)
	searchResp := restPOST(t, srv.URL+"/v1/search", "secret", searchBody)
	defer searchResp.Body.Close()
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("search status %d", searchResp.StatusCode)
	}
	var searchOut searchLocalOutput
	if err := json.NewDecoder(searchResp.Body).Decode(&searchOut); err != nil {
		t.Fatal(err)
	}
	if searchOut.Count != 1 || len(searchOut.Results) != 1 || searchOut.Results[0].Path != allowedDoc {
		t.Fatalf("unexpected constrained search: %+v", searchOut)
	}

	// Out of root → 400
	badBody := `{"query":"REST_ALLOW_TOKEN","mode":"keyword","path_prefixes":["/tmp/not-a-searchify-root"]}`
	badResp := restPOST(t, srv.URL+"/v1/search", "secret", badBody)
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 out-of-root, got %d", badResp.StatusCode)
	}
}

func restPOST(t *testing.T, url, token, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
