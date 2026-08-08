package config

import (
	"fmt"
	"strings"
)

// Known kjarni-go embedding models (see kjarni NewEmbedder docs).
const (
	EmbedModelMiniLML6V2   = "minilm-l6-v2"   // 384-d, default (fast)
	EmbedModelMPNetBaseV2  = "mpnet-base-v2"  // 768-d, higher quality
	EmbedModelDistilBERT   = "distilbert-base" // 768-d
)

// KnownEmbedModels lists selectable SEARCHIFY_EMBED_MODEL values.
var KnownEmbedModels = []string{
	EmbedModelMiniLML6V2,
	EmbedModelMPNetBaseV2,
	EmbedModelDistilBERT,
}

// EmbedModelDims is the output dimension for each known model.
var EmbedModelDims = map[string]int{
	EmbedModelMiniLML6V2:  384,
	EmbedModelMPNetBaseV2: 768,
	EmbedModelDistilBERT:  768,
}

// NormalizeEmbedModel trims and lowercases the model id.
func NormalizeEmbedModel(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ValidateEmbedModel rejects unknown model names with a helpful error.
func ValidateEmbedModel(raw string) (string, error) {
	name := NormalizeEmbedModel(raw)
	if name == "" {
		return defaultEmbedModel, nil
	}
	if _, ok := EmbedModelDims[name]; ok {
		return name, nil
	}
	return "", fmt.Errorf(
		"%s=%q is not a supported embedding model; choose one of: %s",
		EnvEmbedModel, raw, strings.Join(KnownEmbedModels, ", "),
	)
}
