package local

import (
	"fmt"
	"os"
	"time"
)

const maxIndexMessages = 20

func (s *Service) IndexPaths(paths []string, force bool) (IndexReport, error) {
	report := IndexReport{}
	files, walkMessages := collectIndexablePaths(s.cfg, paths)
	report.Messages = append(report.Messages, walkMessages...)

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

		content, err := os.ReadFile(path)
		if err != nil {
			report.Errors++
			report.addMessage(err.Error())
			continue
		}

		if err := s.indexFile(path, info, content); err != nil {
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

func (r *IndexReport) addMessage(msg string) {
	if len(r.Messages) >= maxIndexMessages {
		return
	}
	r.Messages = append(r.Messages, msg)
}
