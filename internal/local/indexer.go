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
	Current int    // 1-based file index in the streaming walk
	Total   int    // indexable files in the initial count pass; disk changes can alter this
	Path    string // current file (empty for scan summary)
	Status  string // "scan" | "start" | "skip" | "indexed" | "updated" | "error" | "empty"
	Error   string // failure detail when Status is "error"
}

// IndexPathsOptions configures IndexPathsOpts.
type IndexPathsOptions struct {
	Context  context.Context // optional; cancels discovery, queued work, and extraction
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
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case s.indexGate <- struct{}{}:
		defer func() { <-s.indexGate }()
	case <-ctx.Done():
		return report, ctx.Err()
	}

	total := 0
	for range indexablePaths(ctx, s.cfg, s.extract, paths, report.addMessage) {
		total++
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	s.emitProgress(opts.Progress, IndexProgress{Total: total, Status: "scan"})

	timeout := s.cfg.ExtractTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	n := 0
	for path := range indexablePaths(ctx, s.cfg, s.extract, paths, report.addMessage) {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		n++
		info, err := os.Stat(path)
		if err != nil {
			report.Errors++
			report.addMessage(err.Error())
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "error", Error: err.Error()})
			continue
		}
		if !info.Mode().IsRegular() || info.Size() > maxIndexFileBytes(s.cfg) {
			report.Skipped++
			report.addMessage(fmt.Sprintf("%s: skipped (not a regular file or larger than %s)", path, formatByteLimit(maxIndexFileBytes(s.cfg))))
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "skip"})
			continue
		}

		prevMtime, prevSize, exists, err := s.fileRecord(path)
		if err != nil {
			report.Errors++
			report.addMessage(err.Error())
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "error", Error: err.Error()})
			continue
		}

		if !opts.Force && exists && prevMtime == info.ModTime().UnixNano() && prevSize == info.Size() {
			report.Skipped++
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "skip"})
			continue
		}

		s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "start"})

		extractCtx, cancel := context.WithTimeout(ctx, timeout)
		text, warn, err := s.extractFile(extractCtx, path, info.Size())
		cancel()
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
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
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "error", Error: err.Error()})
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

		warnMsg, err := s.indexFile(ctx, path, info, []byte(text))
		if warnMsg != "" {
			report.addMessage(fmt.Sprintf("%s: %s", path, warnMsg))
		}
		if err != nil {
			if ctx.Err() != nil {
				// Do not leave an unchanged file record masking interrupted embedding.
				if cleanupErr := s.removeIndexedFile(path); cleanupErr != nil {
					return report, errors.Join(ctx.Err(), fmt.Errorf("remove interrupted index for %s: %w", path, cleanupErr))
				}
				return report, ctx.Err()
			}
			report.Errors++
			report.addMessage(fmt.Sprintf("%s: %v", path, err))
			s.emitProgress(opts.Progress, IndexProgress{Current: n, Total: total, Path: path, Status: "error", Error: err.Error()})
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
	if err := ctx.Err(); err != nil {
		return report, err
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
