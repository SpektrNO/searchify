package extract

import (
	"context"
	"io"
	"strings"
	"unicode/utf8"
)

var passthroughExts = map[string]struct{}{
	".md": {}, ".txt": {}, ".go": {}, ".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {},
	".cs": {},
	".json": {}, ".yaml": {}, ".yml": {}, ".sql": {}, ".sh": {}, ".py": {}, ".rs": {},
	// Cheap P1 text expands
	".xml": {}, ".toml": {}, ".ini": {}, ".log": {}, ".rst": {}, ".adoc": {}, ".markdown": {},
}

type passthroughExtractor struct{}

func (passthroughExtractor) Extensions() []string {
	exts := make([]string, 0, len(passthroughExts))
	for e := range passthroughExts {
		exts = append(exts, e)
	}
	return exts
}

func (passthroughExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = path
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	var warn []string
	if !utf8.Valid(data) {
		warn = append(warn, "invalid UTF-8 replaced")
		data = []byte(strings.ToValidUTF8(string(data), "\uFFFD"))
	}
	return string(data), warn, nil
}
