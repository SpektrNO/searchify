package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultEmbedHTTPTimeout = 60 * time.Second

type ollamaEmbedder struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func newOllamaEmbedder(baseURL, model string) (Embedder, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("ollama embed URL is empty")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("ollama embed model is empty")
	}
	return &ollamaEmbedder{
		baseURL: base,
		model:   model,
		httpClient: &http.Client{
			Timeout: defaultEmbedHTTPTimeout,
		},
	}, nil
}

func (e *ollamaEmbedder) Encode(text string) ([]float32, error) {
	vecs, err := e.EncodeBatch([]string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) != 1 {
		return nil, fmt.Errorf("ollama embed: expected 1 vector, got %d", len(vecs))
	}
	return vecs[0], nil
}

func (e *ollamaEmbedder) EncodeBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{
		"model": e.model,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}
	raw, err := e.post(e.baseURL+"/api/embed", body)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Embeddings [][]float64 `json:"embeddings"`
		Embedding  []float64   `json:"embedding"`
		Error      string      `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode ollama embed response: %w", err)
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("ollama embed: %s", parsed.Error)
	}
	if len(parsed.Embeddings) > 0 {
		return float64MatrixTo32(parsed.Embeddings), nil
	}
	// Some builds return a single embedding field for one input.
	if len(texts) == 1 && len(parsed.Embedding) > 0 {
		return [][]float32{float64SliceTo32(parsed.Embedding)}, nil
	}
	return nil, fmt.Errorf("ollama embed: empty embeddings in response")
}

func (e *ollamaEmbedder) Close() error { return nil }

func (e *ollamaEmbedder) post(url string, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultEmbedHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, trimEmbedBody(raw))
	}
	return raw, nil
}

type httpEmbedder struct {
	url        string
	model      string
	httpClient *http.Client
}

func newHTTPEmbedder(url, model string) (Embedder, error) {
	u := strings.TrimSpace(url)
	if u == "" {
		return nil, fmt.Errorf("http embed URL is empty")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("http embed model is empty")
	}
	return &httpEmbedder{
		url:   u,
		model: model,
		httpClient: &http.Client{
			Timeout: defaultEmbedHTTPTimeout,
		},
	}, nil
}

func (e *httpEmbedder) Encode(text string) ([]float32, error) {
	body, err := json.Marshal(map[string]any{
		"model": e.model,
		"input": text,
	})
	if err != nil {
		return nil, err
	}
	raw, err := e.post(body)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Embedding  []float64   `json:"embedding"`
		Embeddings [][]float64 `json:"embeddings"`
		Error      string      `json:"error"`
		Message    string      `json:"message"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode http embed response: %w", err)
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("http embed: %s", parsed.Error)
	}
	if parsed.Message != "" && len(parsed.Embedding) == 0 && len(parsed.Embeddings) == 0 {
		return nil, fmt.Errorf("http embed: %s", parsed.Message)
	}
	if len(parsed.Embedding) > 0 {
		return float64SliceTo32(parsed.Embedding), nil
	}
	if len(parsed.Embeddings) == 1 {
		return float64SliceTo32(parsed.Embeddings[0]), nil
	}
	return nil, fmt.Errorf("http embed: missing embedding in response")
}

func (e *httpEmbedder) EncodeBatch(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := e.Encode(t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (e *httpEmbedder) Close() error { return nil }

func (e *httpEmbedder) post(body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultEmbedHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http embed request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http embed HTTP %d: %s", resp.StatusCode, trimEmbedBody(raw))
	}
	return raw, nil
}

func float64SliceTo32(in []float64) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v)
	}
	return out
}

func float64MatrixTo32(in [][]float64) [][]float32 {
	out := make([][]float32, len(in))
	for i, row := range in {
		out[i] = float64SliceTo32(row)
	}
	return out
}

func trimEmbedBody(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 240 {
		return s[:240] + "…"
	}
	return s
}
