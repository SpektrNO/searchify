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
	EnvPathBase   = "SEARCHIFY_PATH_BASE"

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
	PathBase   string // optional; relative paths tried here first
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

	pathBase, err := parsePathBase(os.Getenv(EnvPathBase), roots)
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
		PathBase:   pathBase,
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

func parsePathBase(raw string, roots []string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", EnvPathBase, err)
	}
	abs = filepath.Clean(abs)

	for _, root := range roots {
		if pathWithinRoot(abs, root) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("%s %q must be under a SEARCHIFY_ROOTS entry", EnvPathBase, abs)
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

// AllowedPath resolves path against allowlisted roots.
// Absolute paths must lie under a root.
// Relative paths are joined with SEARCHIFY_PATH_BASE (if set) then each root;
// they must exist on disk and not escape the root via "..".
func (c *Config) AllowedPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if path == "." {
		return "", fmt.Errorf("path %q is not allowed; use an absolute path or a relative path under SEARCHIFY_ROOTS", path)
	}

	if filepath.IsAbs(path) {
		return c.allowAbsolute(filepath.Clean(path))
	}

	matches, err := c.relativeCandidates(filepath.Clean(path), true)
	if err != nil {
		return "", err
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("relative path %q not found under SEARCHIFY_ROOTS (%d root(s)); use an absolute path or set SEARCHIFY_PATH_BASE", path, len(c.Roots))
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("relative path %q is ambiguous; matches: %s", path, strings.Join(matches, ", "))
	}
}

// AllowlistedCandidates returns allowlisted absolute path candidates without requiring
// the path to exist on disk. Absolute inputs yield at most one candidate.
// Relative inputs may yield multiple join candidates (caller disambiguates, e.g. via index).
func (c *Config) AllowlistedCandidates(path string) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if path == "." {
		return nil, fmt.Errorf("path %q is not allowed; use an absolute path or a relative path under SEARCHIFY_ROOTS", path)
	}

	if filepath.IsAbs(path) {
		abs, err := c.allowAbsolute(filepath.Clean(path))
		if err != nil {
			return nil, err
		}
		return []string{abs}, nil
	}

	matches, err := c.relativeCandidates(filepath.Clean(path), false)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("relative path %q is outside allowed roots or escapes via ..", path)
	}
	return matches, nil
}

func (c *Config) allowAbsolute(abs string) (string, error) {
	for _, root := range c.Roots {
		if pathWithinRoot(abs, root) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("path %q is outside allowed roots", abs)
}

func (c *Config) relativeCandidates(rel string, requireExist bool) ([]string, error) {
	var matches []string
	seen := make(map[string]struct{})

	try := func(base string) {
		candidate := filepath.Clean(filepath.Join(base, rel))
		if !pathWithinRoot(candidate, base) {
			// Joining with ".." escaped this base; skip.
			return
		}
		// Candidate must also lie under some configured root (base may be PathBase).
		underRoot := false
		for _, root := range c.Roots {
			if pathWithinRoot(candidate, root) {
				underRoot = true
				break
			}
		}
		if !underRoot {
			return
		}
		if requireExist {
			if _, err := os.Stat(candidate); err != nil {
				return
			}
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		matches = append(matches, candidate)
	}

	if c.PathBase != "" {
		try(c.PathBase)
	}
	for _, root := range c.Roots {
		try(root)
	}
	return matches, nil
}

func pathWithinRoot(path, root string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return true
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
