package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	EnvRoots           = "SEARCHIFY_ROOTS"
	EnvIndexDir        = "SEARCHIFY_INDEX_DIR"
	EnvLangSearch      = "LANGSEARCH_API_KEY"
	EnvHTTPToken       = "SEARCHIFY_HTTP_TOKEN"
	EnvHTTPAddr        = "SEARCHIFY_HTTP_ADDR"
	EnvEmbedModel      = "SEARCHIFY_EMBED_MODEL"
	EnvPathBase        = "SEARCHIFY_PATH_BASE"
	EnvWatchPaths      = "SEARCHIFY_WATCH_PATHS"
	EnvWatchDebounce   = "SEARCHIFY_WATCH_DEBOUNCE"
	EnvWatchRescan     = "SEARCHIFY_WATCH_RESCAN"
	EnvOCR             = "SEARCHIFY_OCR"
	EnvOCRLang         = "SEARCHIFY_OCR_LANG"
	EnvMaxFileBytes    = "SEARCHIFY_MAX_FILE_BYTES"
	EnvExtractTimeout  = "SEARCHIFY_EXTRACT_TIMEOUT"
	EnvEmbedBatch      = "SEARCHIFY_EMBED_BATCH"
	EnvMaxChunksFile   = "SEARCHIFY_MAX_CHUNKS_PER_FILE"
	EnvMaxExtractBytes = "SEARCHIFY_MAX_EXTRACT_BYTES"

	defaultEmbedModel      = "minilm-l6-v2"
	defaultHTTPAddr        = ":8080"
	defaultWatchDebounce   = time.Second
	defaultMaxFileBytes    = int64(8 * 1024 * 1024) // safer default; raise via env for large PDFs
	defaultExtractTimeout  = 30 * time.Second
	defaultOCRLang         = "eng"
	defaultEmbedBatch      = 16
	defaultMaxChunksFile   = 256
	defaultMaxExtractBytes = int64(2 * 1024 * 1024)
)

type Config struct {
	Roots            []string
	IndexDir         string
	LangSearch       string
	HTTPToken        string
	HTTPAddr         string
	EmbedModel       string
	PathBase         string        // optional; relative paths tried here first
	WatchPaths       []string      // optional; empty disables auto-index watch
	WatchDebounce    time.Duration // coalesce fs events (default 1s)
	WatchRescan      time.Duration // optional periodic IndexPaths; 0 disables
	OCREnabled       bool
	OCRLang          string
	MaxFileBytes     int64
	ExtractTimeout   time.Duration
	EmbedBatch       int   // ONNX EncodeBatch size (default 16)
	MaxChunksPerFile int   // truncate indexing after N chunks (default 256)
	MaxExtractBytes  int64 // truncate extracted text before chunking (default 2MiB)
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

	watchPaths, err := parseWatchPaths(os.Getenv(EnvWatchPaths), roots)
	if err != nil {
		return nil, err
	}

	debounce, err := parseDurationEnv(EnvWatchDebounce, os.Getenv(EnvWatchDebounce), defaultWatchDebounce)
	if err != nil {
		return nil, err
	}
	rescan, err := parseDurationEnv(EnvWatchRescan, os.Getenv(EnvWatchRescan), 0)
	if err != nil {
		return nil, err
	}

	maxBytes, err := parseInt64Env(EnvMaxFileBytes, os.Getenv(EnvMaxFileBytes), defaultMaxFileBytes)
	if err != nil {
		return nil, err
	}
	extractTimeout, err := parseDurationEnv(EnvExtractTimeout, os.Getenv(EnvExtractTimeout), defaultExtractTimeout)
	if err != nil {
		return nil, err
	}
	embedBatch, err := parseIntEnv(EnvEmbedBatch, os.Getenv(EnvEmbedBatch), defaultEmbedBatch)
	if err != nil {
		return nil, err
	}
	maxChunks, err := parseIntEnv(EnvMaxChunksFile, os.Getenv(EnvMaxChunksFile), defaultMaxChunksFile)
	if err != nil {
		return nil, err
	}
	maxExtract, err := parseInt64Env(EnvMaxExtractBytes, os.Getenv(EnvMaxExtractBytes), defaultMaxExtractBytes)
	if err != nil {
		return nil, err
	}

	return &Config{
		Roots:            roots,
		IndexDir:         indexDir,
		LangSearch:       os.Getenv(EnvLangSearch),
		HTTPToken:        os.Getenv(EnvHTTPToken),
		HTTPAddr:         defaultString(os.Getenv(EnvHTTPAddr), defaultHTTPAddr),
		EmbedModel:       defaultString(os.Getenv(EnvEmbedModel), defaultEmbedModel),
		PathBase:         pathBase,
		WatchPaths:       watchPaths,
		WatchDebounce:    debounce,
		WatchRescan:      rescan,
		OCREnabled:       parseBoolEnv(os.Getenv(EnvOCR)),
		OCRLang:          defaultString(os.Getenv(EnvOCRLang), defaultOCRLang),
		MaxFileBytes:     maxBytes,
		ExtractTimeout:   extractTimeout,
		EmbedBatch:       embedBatch,
		MaxChunksPerFile: maxChunks,
		MaxExtractBytes:  maxExtract,
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

func parseWatchPaths(raw string, roots []string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		abs, err := filepath.Abs(filepath.Clean(part))
		if err != nil {
			return nil, fmt.Errorf("resolve %s entry %q: %w", EnvWatchPaths, part, err)
		}
		under := false
		for _, root := range roots {
			if pathWithinRoot(abs, root) {
				under = true
				break
			}
		}
		if !under {
			return nil, fmt.Errorf("%s entry %q must be under a SEARCHIFY_ROOTS entry", EnvWatchPaths, abs)
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
	}
	return out, nil
}

func parseDurationEnv(name, raw string, fallback time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return d, nil
}

func parseBoolEnv(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseInt64Env(name, raw string, fallback int64) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be > 0", name)
	}
	return n, nil
}

func parseIntEnv(name, raw string, fallback int) (int, error) {
	n, err := parseInt64Env(name, raw, int64(fallback))
	if err != nil {
		return 0, err
	}
	if n > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("%s too large", name)
	}
	return int(n), nil
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

// UnderAnyRoot reports whether abs lies under at least one configured root.
func (c *Config) UnderAnyRoot(abs string) bool {
	abs = filepath.Clean(abs)
	for _, root := range c.Roots {
		if pathWithinRoot(abs, root) {
			return true
		}
	}
	return false
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
