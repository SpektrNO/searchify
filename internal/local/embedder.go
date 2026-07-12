package local

import (
	"fmt"

	kjarni "github.com/olafurjohannsson/kjarni-go"
)

// Embedder encodes text into dense vectors for semantic search.
type Embedder interface {
	Encode(text string) ([]float32, error)
	EncodeBatch(texts []string) ([][]float32, error)
	Close() error
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
