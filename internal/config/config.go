package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	EnvRoots      = "SEARCHIFY_ROOTS"
	EnvIndexDir   = "SEARCHIFY_INDEX_DIR"
	EnvLangSearch = "LANGSEARCH_API_KEY"
	EnvHTTPToken  = "SEARCHIFY_HTTP_TOKEN"
	EnvHTTPAddr   = "SEARCHIFY_HTTP_ADDR"
	EnvEmbedModel = "SEARCHIFY_EMBED_MODEL"

	defaultEmbedModel = "minilm-l6-v2"
	defaultHTTPAddr   = ":8080"
)

type Config struct {
	Roots      []string
	IndexDir   string
	LangSearch string
	HTTPToken  string
	HTTPAddr   string
	EmbedModel string
}

func Load() (*Config, error) {
	roots, err := parseRoots(os.Getenv(EnvRoots))
	if err != nil {
		return nil, err
	}

	indexDir, err := defaultIndexDir(os.Getenv(EnvIndexDir))
	if err != nil {
		return nil, err
	}

	return &Config{
		Roots:      roots,
		IndexDir:   indexDir,
		LangSearch: os.Getenv(EnvLangSearch),
		HTTPToken:  os.Getenv(EnvHTTPToken),
		HTTPAddr:   defaultString(os.Getenv(EnvHTTPAddr), defaultHTTPAddr),
		EmbedModel: defaultString(os.Getenv(EnvEmbedModel), defaultEmbedModel),
	}, nil
}

func parseRoots(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("%s is required (comma-separated absolute paths)", EnvRoots)
	}

	parts := strings.Split(raw, ",")
	roots := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		root := strings.TrimSpace(part)
		if root == "" {
			continue
		}

		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("resolve root %q: %w", root, err)
		}

		abs = filepath.Clean(abs)
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}

	if len(roots) == 0 {
		return nil, fmt.Errorf("%s must contain at least one path", EnvRoots)
	}

	return roots, nil
}

func defaultIndexDir(raw string) (string, error) {
	if strings.TrimSpace(raw) != "" {
		return filepath.Abs(filepath.Clean(raw))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, ".searchify", "index"), nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (c *Config) AllowedPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	abs = filepath.Clean(abs)

	for _, root := range c.Roots {
		if pathWithinRoot(abs, root) {
			return abs, nil
		}
	}

	return "", fmt.Errorf("path %q is outside allowed roots", abs)
}

func pathWithinRoot(path, root string) bool {
	root = filepath.Clean(root)
	if path == root {
		return true
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
