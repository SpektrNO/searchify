package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebSearchSuccessAndCache(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/v1/web-search" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{
				"webPages": map[string]any{
					"value": []map[string]any{
						{
							"id":      "https://api.langsearch.com/v1/web-search#1",
							"name":    "Example",
							"url":     "https://example.com",
							"snippet": "short",
							"summary": "long summary about the topic",
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient("test-key",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithCacheTTL(time.Minute),
	)

	opts := SearchOptions{Query: "go mcp", Count: 5, Summary: true}
	first, err := client.WebSearch(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 result, got %d", len(first))
	}
	if first[0].Source != "web" || first[0].URL != "https://example.com" {
		t.Fatalf("unexpected result: %+v", first[0])
	}
	if first[0].Snippet != "long summary about the topic" {
		t.Fatalf("expected summary in snippet, got %q", first[0].Snippet)
	}

	second, err := client.WebSearch(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 {
		t.Fatal("expected cached result")
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 network hit, got %d", hits.Load())
	}
}

func TestWebSearchRequiresAPIKey(t *testing.T) {
	client := NewClient("")
	_, err := client.WebSearch(context.Background(), SearchOptions{Query: "x"})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestWebSearchRetries429(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"msg":"rate limit"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{
				"webPages": map[string]any{
					"value": []map[string]any{
						{"id": "1", "name": "Ok", "url": "https://ok.example", "snippet": "s"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient("test-key",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithSleep(func(time.Duration) {}),
	)

	results, err := client.WebSearch(context.Background(), SearchOptions{Query: "retry"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if hits.Load() != 2 {
		t.Fatalf("expected 2 hits, got %d", hits.Load())
	}
}

func TestWebSearchExhausted429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer srv.Close()

	client := NewClient("test-key",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithSleep(func(time.Duration) {}),
	)

	_, err := client.WebSearch(context.Background(), SearchOptions{Query: "fail"})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
}
