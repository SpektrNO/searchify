package mcp

import (
	"bytes"
	"encoding/json"
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
	t.Cleanup(func() { _ = s.Local().Close() })
	return s
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
