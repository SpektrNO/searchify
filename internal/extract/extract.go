// Package extract turns source files into UTF-8 plain text for indexing and search_file.
package extract

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
)

// Extractor converts a file into searchable plain text.
type Extractor interface {
	// Extensions returns lowercase extensions including the leading dot, e.g. ".pdf".
	Extensions() []string
	// Extract returns UTF-8 plain text suitable for chunking.
	// warn is non-fatal (partial OCR, truncated pages). err means skip this file.
	Extract(ctx context.Context, path string, r io.Reader, size int64) (text string, warn []string, err error)
}

// Options control OCR and related extract behavior.
type Options struct {
	OCREnabled bool
	OCRLang    string // e.g. "eng"; used when OCREnabled
}

// Registry maps extension → Extractor. First registered wins.
type Registry struct {
	mu      sync.RWMutex
	byExt   map[string]Extractor
	order   []string
	opts    Options
}

// NewRegistry builds the default extractors for the given options.
func NewRegistry(opts Options) *Registry {
	if opts.OCRLang == "" {
		opts.OCRLang = "eng"
	}
	reg := &Registry{
		byExt: make(map[string]Extractor),
		opts:  opts,
	}
	reg.MustRegister(passthroughExtractor{})
	reg.MustRegister(htmlExtractor{})
	reg.MustRegister(pdfExtractor{opts: opts})
	reg.MustRegister(docxExtractor{})
	reg.MustRegister(xlsxExtractor{})
	reg.MustRegister(csvExtractor{})
	reg.MustRegister(imageExtractor{opts: opts})
	// Stretch formats (pure Go / ZIP+XML).
	reg.MustRegister(pptxExtractor{})
	reg.MustRegister(odfExtractor{})
	reg.MustRegister(rtfExtractor{})
	reg.MustRegister(emlExtractor{})
	return reg
}

// MustRegister panics on duplicate extension (programming error).
func (r *Registry) MustRegister(e Extractor) {
	if err := r.Register(e); err != nil {
		panic(err)
	}
}

// Register associates an extractor with its extensions. First wins per ext.
func (r *Registry) Register(e Extractor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byExt == nil {
		r.byExt = make(map[string]Extractor)
	}
	for _, ext := range e.Extensions() {
		ext = strings.ToLower(ext)
		if !strings.HasPrefix(ext, ".") {
			return fmt.Errorf("extract: extension %q must start with '.'", ext)
		}
		if _, exists := r.byExt[ext]; exists {
			continue // first registered wins
		}
		r.byExt[ext] = e
		r.order = append(r.order, ext)
	}
	return nil
}

// Lookup returns the extractor for path's extension, if any.
func (r *Registry) Lookup(path string) (Extractor, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byExt[ext]
	return e, ok
}

// HasExtension reports whether the path extension is indexable.
func (r *Registry) HasExtension(path string) bool {
	_, ok := r.Lookup(path)
	return ok
}

// Extensions returns all registered extensions (sorted by registration order).
func (r *Registry) Extensions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// IsPassthrough reports whether the extension is handled as UTF-8 text (no format parser).
func IsPassthrough(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := passthroughExts[ext]
	return ok
}

// Extract opens extraction for path using the registry.
func (r *Registry) Extract(ctx context.Context, path string, reader io.Reader, size int64) (string, []string, error) {
	e, ok := r.Lookup(path)
	if !ok {
		return "", nil, fmt.Errorf("unsupported extension %q", filepath.Ext(path))
	}
	return e.Extract(ctx, path, reader, size)
}
