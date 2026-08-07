package local

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RemoveReport summarizes remove_paths / CLI remove results.
type RemoveReport struct {
	Removed  int      `json:"removed"`
	Skipped  int      `json:"skipped"`
	Errors   int      `json:"errors"`
	Messages []string `json:"messages,omitempty"`
}

// RemovePaths drops indexed files under the given paths (exact match or children).
// Paths need not exist on disk. Relative paths are resolved without Stat; when
// multiple allowlisted joins exist, the index disambiguates.
func (s *Service) RemovePaths(paths []string) (RemoveReport, error) {
	report := RemoveReport{}
	if len(paths) == 0 {
		return report, fmt.Errorf("paths is required")
	}

	for _, raw := range paths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			report.Errors++
			report.addMessage("empty path")
			continue
		}

		candidates, err := s.cfg.AllowlistedCandidates(raw)
		if err != nil {
			report.Errors++
			report.addMessage(err.Error())
			continue
		}

		target, err := s.pickRemoveTarget(candidates)
		if err != nil {
			report.Errors++
			report.addMessage(err.Error())
			continue
		}
		if target == "" {
			report.Skipped++
			report.addMessage(fmt.Sprintf("%s: not in index", raw))
			continue
		}

		matched, err := s.listIndexedUnder(target)
		if err != nil {
			report.Errors++
			report.addMessage(fmt.Sprintf("%s: %v", raw, err))
			continue
		}
		if len(matched) == 0 {
			report.Skipped++
			report.addMessage(fmt.Sprintf("%s: not in index", raw))
			continue
		}

		for _, filePath := range matched {
			if err := s.removeIndexedFile(filePath); err != nil {
				report.Errors++
				report.addMessage(fmt.Sprintf("%s: %v", filePath, err))
				continue
			}
			report.Removed++
		}
	}

	return report, nil
}

func (r *RemoveReport) addMessage(msg string) {
	if len(r.Messages) >= maxIndexMessages {
		return
	}
	r.Messages = append(r.Messages, msg)
}

// pickRemoveTarget chooses a single allowlisted path to remove under.
// Empty string means no indexed match (caller should skip).
func (s *Service) pickRemoveTarget(candidates []string) (string, error) {
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	var hits []string
	for _, c := range candidates {
		ok, err := s.pathTouchesIndex(c)
		if err != nil {
			return "", err
		}
		if ok {
			hits = append(hits, c)
		}
	}
	switch len(hits) {
	case 0:
		return "", nil
	case 1:
		return hits[0], nil
	default:
		return "", fmt.Errorf("relative path is ambiguous; matches: %s", strings.Join(hits, ", "))
	}
}

func (s *Service) pathTouchesIndex(p string) (bool, error) {
	var n int
	like := escapeLike(p) + string(filepath.Separator) + "%"
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM files WHERE path = ? OR path LIKE ? ESCAPE '\'`,
		p, like,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Service) listIndexedUnder(p string) ([]string, error) {
	like := escapeLike(p) + string(filepath.Separator) + "%"
	rows, err := s.db.Query(
		`SELECT path FROM files WHERE path = ? OR path LIKE ? ESCAPE '\' ORDER BY path`,
		p, like,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, rows.Err()
}

func (s *Service) removeIndexedFile(path string) error {
	if err := s.deleteFileChunks(path); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM files WHERE path = ?`, path)
	return err
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
