package local

import (
	"context"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/extract"
)

const directoryBatchSize = 128

// walkDirectories does not sort or retain a whole directory's entries.
func walkDirectories(ctx context.Context, root string, visit func(string, os.DirEntry) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, err := os.Open(root)
	if err != nil {
		return err
	}
	defer dir.Close()
	for {
		entries, readErr := dir.ReadDir(directoryBatchSize)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := visit(filepath.Join(root, entry.Name()), entry); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func indexablePaths(ctx context.Context, cfg *config.Config, reg *extract.Registry, roots []string, message func(string)) iter.Seq[string] {
	return func(yield func(string) bool) {
		maxBytes := maxIndexFileBytes(cfg)
		visitFile := func(path string, info os.FileInfo) bool {
			if !info.Mode().IsRegular() || cfg.PathHasSkipDir(path) {
				return true
			}
			if cfg.SkipFile(path) {
				message(path + ": skipped (SEARCHIFY_EXCLUDE_FILES)")
				return true
			}
			if !reg.HasExtension(path) {
				return true
			}
			if info.Size() > maxBytes {
				message(path + ": skipped (file larger than " + formatByteLimit(maxBytes) + ")")
				return true
			}
			return yield(path)
		}
		stopped := false
		var walk func(string)
		walk = func(dir string) {
			err := walkDirectories(ctx, dir, func(path string, d os.DirEntry) error {
				if stopped {
					return context.Canceled
				}
				if d.IsDir() {
					if !cfg.SkipPath(path) {
						walk(path)
					}
					return nil
				}
				if cfg.SkipFile(path) || !reg.HasExtension(path) || !d.Type().IsRegular() {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					message(err.Error())
				} else if !visitFile(path, info) {
					stopped = true
					return context.Canceled
				}
				return nil
			})
			if err != nil && !stopped && ctx.Err() == nil {
				message(err.Error())
			}
		}
		for _, root := range roots {
			if stopped || ctx.Err() != nil {
				return
			}
			allowed, err := cfg.AllowedPath(root)
			if err != nil {
				message(err.Error())
				continue
			}
			info, err := os.Stat(allowed)
			if err != nil {
				message(err.Error())
				continue
			}
			if info.IsDir() {
				if !cfg.SkipPath(allowed) {
					walk(allowed)
				}
			} else if !visitFile(allowed, info) {
				return
			}
		}
	}
}

func pathWithin(path, root string) bool {
	return path == root || strings.HasPrefix(path, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator))
}

func maxIndexFileBytes(cfg *config.Config) int64 {
	if cfg.MaxFileBytes > 0 {
		return cfg.MaxFileBytes
	}
	return 32 * 1024 * 1024
}

func formatByteLimit(n int64) string {
	const miB = 1024 * 1024
	if n%(miB) == 0 && n >= miB {
		return fmt.Sprintf("%dMB", n/miB)
	}
	return fmt.Sprintf("%d bytes", n)
}
