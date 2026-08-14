package local

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPEmbedderEncode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
			w.WriteHeader(400)
			return
		}
		if req.Model != "demo" || req.Input == "" {
			w.WriteHeader(400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding": []float64{0.1, 0.2, 0.3},
		})
	}))
	defer srv.Close()

	e, err := newHTTPEmbedder(srv.URL, "demo")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	v, err := e.Encode("hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || v[0] != 0.1 {
		t.Fatalf("vec=%v", v)
	}
}

func TestOllamaEmbedderEncodeBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path=%s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
			w.WriteHeader(400)
			return
		}
		embs := make([][]float64, len(req.Input))
		for i := range req.Input {
			embs[i] = []float64{float64(i), 1, 2}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": embs})
	}))
	defer srv.Close()

	e, err := newOllamaEmbedder(srv.URL, "nomic-embed-text")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	vecs, err := e.EncodeBatch([]string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 || vecs[1][0] != 1 {
		t.Fatalf("vecs=%v", vecs)
	}
}
