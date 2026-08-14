package config

import (
	"fmt"
	"strings"
)

// EmbedEngine selects which stack produces vectors (distinct from EmbedBackend = where).
type EmbedEngine string

const (
	EmbedEngineKjarni EmbedEngine = "kjarni" // default; in-process via kjarni-go
	EmbedEngineOllama EmbedEngine = "ollama" // local Ollama /api/embed
	EmbedEngineHTTP   EmbedEngine = "http"   // generic HTTP embeddings endpoint
)

// Known kjarni-go embedding models (see kjarni NewEmbedder docs).
const (
	EmbedModelMiniLML6V2  = "minilm-l6-v2"    // 384-d, default (fast)
	EmbedModelMPNetBaseV2 = "mpnet-base-v2"   // 768-d, higher quality
	EmbedModelDistilBERT  = "distilbert-base" // 768-d
)

const defaultOllamaURL = "http://127.0.0.1:11434"

// KnownEmbedModels lists selectable SEARCHIFY_EMBED_MODEL values for engine=kjarni.
var KnownEmbedModels = []string{
	EmbedModelMiniLML6V2,
	EmbedModelMPNetBaseV2,
	EmbedModelDistilBERT,
}

// EmbedModelDims is the output dimension for each known kjarni model.
var EmbedModelDims = map[string]int{
	EmbedModelMiniLML6V2:  384,
	EmbedModelMPNetBaseV2: 768,
	EmbedModelDistilBERT:  768,
}

// NormalizeEmbedModel trims and lowercases the model id (kjarni).
func NormalizeEmbedModel(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ParseEmbedEngine validates SEARCHIFY_EMBED_ENGINE.
func ParseEmbedEngine(raw string) (EmbedEngine, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "", string(EmbedEngineKjarni):
		return EmbedEngineKjarni, nil
	case string(EmbedEngineOllama):
		return EmbedEngineOllama, nil
	case string(EmbedEngineHTTP):
		return EmbedEngineHTTP, nil
	default:
		return "", fmt.Errorf(
			"%s=%q is invalid; choose one of: kjarni, ollama, http",
			EnvEmbedEngine, raw,
		)
	}
}

// ValidateEmbedModel rejects unknown kjarni model names.
func ValidateEmbedModel(raw string) (string, error) {
	name := NormalizeEmbedModel(raw)
	if name == "" {
		return defaultEmbedModel, nil
	}
	if _, ok := EmbedModelDims[name]; ok {
		return name, nil
	}
	return "", fmt.Errorf(
		"%s=%q is not a supported kjarni embedding model; choose one of: %s (or set %s=ollama|http)",
		EnvEmbedModel, raw, strings.Join(KnownEmbedModels, ", "), EnvEmbedEngine,
	)
}

// ResolveEmbedSettings validates engine + model + URL together.
func ResolveEmbedSettings(engineRaw, modelRaw, urlRaw string) (engine EmbedEngine, model, url string, err error) {
	engine, err = ParseEmbedEngine(engineRaw)
	if err != nil {
		return "", "", "", err
	}
	url = strings.TrimSpace(urlRaw)

	switch engine {
	case EmbedEngineKjarni:
		model, err = ValidateEmbedModel(modelRaw)
		if err != nil {
			return "", "", "", err
		}
		return engine, model, "", nil
	case EmbedEngineOllama:
		model = strings.TrimSpace(modelRaw)
		if model == "" {
			return "", "", "", fmt.Errorf("%s is required when %s=ollama", EnvEmbedModel, EnvEmbedEngine)
		}
		if url == "" {
			url = defaultOllamaURL
		}
		return engine, model, strings.TrimRight(url, "/"), nil
	case EmbedEngineHTTP:
		model = strings.TrimSpace(modelRaw)
		if model == "" {
			return "", "", "", fmt.Errorf("%s is required when %s=http", EnvEmbedModel, EnvEmbedEngine)
		}
		if url == "" {
			return "", "", "", fmt.Errorf("%s is required when %s=http (full embeddings URL)", EnvEmbedURL, EnvEmbedEngine)
		}
		return engine, model, url, nil
	default:
		return "", "", "", fmt.Errorf("unknown embed engine %q", engine)
	}
}
