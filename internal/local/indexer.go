package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spektr/searchify/internal/extract"
)

const maxIndexMessages = 20

// IndexProgress reports live progress during IndexPaths (CLI / operators).
type IndexProgress struct {
	Current int    // 1-based file index in the walk list
	Total   int    // total indexable files discovered
	Path    string // current file (empty for scan summary)
	Status  string // "scan" | "start" | "skip" | "indexed" | "updated" | "error" | "empty"
}

// IndexPathsOptions configures IndexPathsOpts.
type IndexPathsOptions struct {
	Force    bool
	Progress func(IndexProgress) // optional; called on stderr-friendly milestones
}

// IndexPaths builds or refreshes the local index for paths under allowed roots.
func (s *Service) IndexPaths(paths []string, force bool) (IndexReport, error) {
	return s.IndexPathsOpts(paths, IndexPathsOptions{Force: force})
}

// IndexPathsOpts is IndexPaths with optional progress reporting.
func (s *Service) IndexPathsOpts(paths []string, opts IndexPathsOptions) (IndexReport, error) {
	report := IndexReport{}
	files, walkMessages := collectIndexablePaths(s.cfg, s.extract, paths)
	report.Messages = append(report.Messages, walkMessages...)

	total := len(files)
	s.emitProgress(opts.Progress, IndexProgress{Total: total, Status: "scan"})

	timeout := s.cfg.ExtractTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	for i, path := range files {
		n := i + 1
		info, err := os.Stat(path)
		if err != nil {
			report.Errors++
			report.addMessage(err.Error())
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "error"})
			continue
		}

		prevMtime, prevSize, exists, err := s.fileRecord(path)
		if err != nil {
			report.Errors++
			report.addMessage(err.Error())
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "error"})
			continue
		}

		if !opts.Force && exists && prevMtime == info.ModTime().UnixNano() && prevSize == info.Size() {
			report.Skipped++
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "skip"})
			continue
		}

		s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "start"})

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		text, warn, err := s.extractFile(ctx, path, info.Size())
		cancel()
		for _, w := range warn {
			report.addMessage(fmt.Sprintf("%s: %s", path, w))
		}
		if err != nil {
			var skip *extract.SkipError
			if errors.As(err, &skip) {
				report.addMessage(fmt.Sprintf("%s: %s", path, skip.Error()))
				s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "skip"})
				continue
			}
			report.Errors++
			report.addMessage(fmt.Sprintf("%s: %v", path, err))
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "error"})
			continue
		}
		if utf8.RuneCountInString(strings.TrimSpace(text)) == 0 {
			report.addMessage(fmt.Sprintf("%s: skipped (empty extract)", path))
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "empty"})
			continue
		}

		maxExtract := s.cfg.MaxExtractBytes
		if maxExtract <= 0 {
			maxExtract = 512 * 1024
		}
		if int64(len(text)) > maxExtract {
			report.addMessage(fmt.Sprintf("%s: extracted text truncated to %d bytes (SEARCHIFY_MAX_EXTRACT_BYTES)", path, maxExtract))
			text = text[:maxExtract]
		}

		warnMsg, err := s.indexFile(path, info, []byte(text))
		if warnMsg != "" {
			report.addMessage(fmt.Sprintf("%s: %s", path, warnMsg))
		}
		if err != nil {
			report.Errors++
			report.addMessage(fmt.Sprintf("%s: %v", path, err))
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "error"})
			continue
		}

		status := "indexed"
		if !exists {
			report.Indexed++
		} else {
			report.Updated++
			status = "updated"
		}
		s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: status})

		// Drop native ONNX RSS (not managed by Go GC) and return pages to the OS.
		if s.cfg.EmbedReload && s.cfg.UseInProcessEmbed() {
			s.dropEmbedder()
		}
		runtime.GC()
		debug.FreeOSMemory()
	}

	if report.Indexed > 0 || report.Updated > 0 {
		if err := s.setMeta("indexed_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
			return report, fmt.Errorf("update indexed_at: %w", err)
		}
	}
	runtime.GC()
	debug.FreeOSMemory()

	return report, nil
}

func (s *Service) emitProgress(fn func(IndexProgress), p IndexProgress) {
	if fn == nil {
		return
	}
	fn(p)
}

func (r *IndexReport) addMessage(msg string) {
	if len(r.Messages) >= maxIndexMessages {
		return
	}
	r.Messages = append(r.Messages, msg)
}
