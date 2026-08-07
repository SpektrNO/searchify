package local

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/extract"
)

var skipDirNames = map[string]struct{}{
	".git": {}, ".cursor": {}, "node_modules": {}, "vendor": {}, "bin": {}, ".searchify": {},
}

func collectIndexablePaths(cfg *config.Config, reg *extract.Registry, roots []string) ([]string, []string) {
	var files []string
	var messages []string
	maxBytes := cfg.MaxFileBytes
	if maxBytes <= 0 {
		maxBytes = 32 * 1024 * 1024
	}

	for _, root := range roots {
		allowed, err := cfg.AllowedPath(root)
		if err != nil {
			messages = append(messages, err.Error())
			continue
		}

		info, err := os.Stat(allowed)
		if err != nil {
			messages = append(messages, err.Error())
			continue
		}

		if !info.IsDir() {
			if reg.HasExtension(allowed) {
				if info.Size() > maxBytes {
					messages = append(messages, filepath.Clean(allowed)+": skipped (file larger than "+formatByteLimit(maxBytes)+")")
				} else {
					files = append(files, filepath.Clean(allowed))
				}
			}
			continue
		}

		err = filepath.WalkDir(allowed, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				messages = append(messages, walkErr.Error())
				return nil
			}
			if d.IsDir() {
				if path != allowed && shouldSkipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !reg.HasExtension(path) {
				return nil
			}
			stat, err := d.Info()
			if err != nil {
				messages = append(messages, err.Error())
				return nil
			}
			if stat.Size() > maxBytes {
				messages = append(messages, filepath.Clean(path)+": skipped (file larger than "+formatByteLimit(maxBytes)+")")
				return nil
			}
			files = append(files, filepath.Clean(path))
			return nil
		})
		if err != nil {
			messages = append(messages, err.Error())
		}
	}

	return files, messages
}

func shouldSkipDir(name string) bool {
	_, ok := skipDirNames[name]
	return ok
}

func formatByteLimit(n int64) string {
	const miB = 1024 * 1024
	if n%(miB) == 0 && n >= miB {
		return fmt.Sprintf("%dMB", n/miB)
	}
	return fmt.Sprintf("%d bytes", n)
}
