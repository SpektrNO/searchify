package local

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spektr/searchify/internal/config"
)

const maxFileBytes = 2 * 1024 * 1024

var indexExtensions = map[string]struct{}{
	".md": {}, ".txt": {}, ".go": {}, ".ts": {}, ".tsx": {}, ".js": {},
	".json": {}, ".yaml": {}, ".yml": {}, ".sql": {}, ".sh": {}, ".py": {}, ".rs": {},
}

var skipDirNames = map[string]struct{}{
	".git": {}, ".cursor": {}, "node_modules": {}, "vendor": {}, "bin": {}, ".searchify": {},
}

func collectIndexablePaths(cfg *config.Config, roots []string) ([]string, []string) {
	var files []string
	var messages []string

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
			if shouldIndexFile(allowed) {
				files = append(files, allowed)
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
			if !shouldIndexFile(path) {
				return nil
			}
			stat, err := d.Info()
			if err != nil {
				messages = append(messages, err.Error())
				return nil
			}
			if stat.Size() > maxFileBytes {
				messages = append(messages, filepath.Clean(path)+": skipped (file larger than 2MB)")
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

func shouldIndexFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := indexExtensions[ext]
	return ok
}
