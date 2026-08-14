package local

import (
	"fmt"

	"github.com/spektr/searchify/internal/config"

	kjarni "github.com/olafurjohannsson/kjarni-go"
)

// Embedder encodes text into dense vectors for semantic search.
type Embedder interface {
	Encode(text string) ([]float32, error)
	EncodeBatch(texts []string) ([][]float32, error)
	Close() error
}

func newEmbedder(cfg *config.Config) (Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	switch cfg.EffectiveEmbedEngine() {
	case config.EmbedEngineKjarni:
		return newKjarniEmbedder(cfg.EmbedModel)
	case config.EmbedEngineOllama:
		return newOllamaEmbedder(cfg.EmbedURL, cfg.EmbedModel)
	case config.EmbedEngineHTTP:
		return newHTTPEmbedder(cfg.EmbedURL, cfg.EmbedModel)
	default:
		return nil, fmt.Errorf("unsupported embed engine %q", cfg.EmbedEngine)
	}
}

type kjarniEmbedder struct {
	inner *kjarni.Embedder
}

func newKjarniEmbedder(model string) (Embedder, error) {
	e, err := kjarni.NewEmbedder(model, kjarni.WithQuiet(true))
	if err != nil {
		return nil, fmt.Errorf("create embedder %q: %w", model, err)
	}
	return &kjarniEmbedder{inner: e}, nil
}

func (e *kjarniEmbedder) Encode(text string) ([]float32, error) {
	return e.inner.Encode(text)
}

func (e *kjarniEmbedder) EncodeBatch(texts []string) ([][]float32, error) {
	return e.inner.EncodeBatch(texts)
}

func (e *kjarniEmbedder) Close() error {
	return e.inner.Close()
}
