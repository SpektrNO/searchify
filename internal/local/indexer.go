package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spektr/searchify/internal/extract"
)

const maxIndexMessages = 20

func (s *Service) IndexPaths(paths []string, force bool) (IndexReport, error) {
	report := IndexReport{}
	files, walkMessages := collectIndexablePaths(s.cfg, s.extract, paths)
	report.Messages = append(report.Messages, walkMessages...)

	timeout := s.cfg.ExtractTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			report.Errors++
			report.addMessage(err.Error())
			continue
		}

		prevMtime, prevSize, exists, err := s.fileRecord(path)
		if err != nil {
			report.Errors++
			report.addMessage(err.Error())
			continue
		}

		if !force && exists && prevMtime == info.ModTime().UnixNano() && prevSize == info.Size() {
			report.Skipped++
			continue
		}

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
				continue
			}
			report.Errors++
			report.addMessage(fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if utf8.RuneCountInString(strings.TrimSpace(text)) == 0 {
			report.addMessage(fmt.Sprintf("%s: skipped (empty extract)", path))
			continue
		}

		if err := s.indexFile(path, info, []byte(text)); err != nil {
			report.Errors++
			report.addMessage(fmt.Sprintf("%s: %v", path, err))
			continue
		}

		if !exists {
			report.Indexed++
		} else {
			report.Updated++
		}
	}

	if report.Indexed > 0 || report.Updated > 0 {
		if err := s.setMeta("indexed_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
			return report, fmt.Errorf("update indexed_at: %w", err)
		}
	}

	return report, nil
}

func (s *Service) extractFile(ctx context.Context, path string, size int64) (string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	return s.extract.Extract(ctx, path, f, size)
}

func (r *IndexReport) addMessage(msg string) {
	if len(r.Messages) >= maxIndexMessages {
		return
	}
	r.Messages = append(r.Messages, msg)
}
