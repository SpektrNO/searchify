package local

import (
	"fmt"
	"os"
	"strings"
)

// PruneReport summarizes index_prune / CLI prune results.
type PruneReport struct {
	Scanned  int      `json:"scanned"`
	Removed  int      `json:"removed"`
	Skipped  int      `json:"skipped"`
	Errors   int      `json:"errors"`
	DryRun   bool     `json:"dry_run,omitempty"`
	Messages []string `json:"messages,omitempty"`
}

// PruneIndex drops indexed files that are missing on disk or outside current roots.
// If paths is empty, all indexed files are considered. dryRun counts orphans without deleting.
func (s *Service) PruneIndex(paths []string, dryRun bool) (PruneReport, error) {
	report := PruneReport{DryRun: dryRun}

	candidates, err := s.pruneCandidates(paths, &report)
	if err != nil {
		return report, err
	}
	report.Scanned = len(candidates)

	var orphans []string
	for _, p := range candidates {
		orphan, err := s.isOrphanIndexedPath(p)
		if err != nil {
			report.Errors++
			report.addPruneMessage(fmt.Sprintf("%s: %v", p, err))
			continue
		}
		if orphan {
			orphans = append(orphans, p)
			continue
		}
		report.Skipped++
	}

	if dryRun {
		report.Removed = len(orphans)
		return report, nil
	}

	for _, p := range orphans {
		if err := s.removeIndexedFile(p); err != nil {
			report.Errors++
			report.addPruneMessage(fmt.Sprintf("%s: %v", p, err))
			continue
		}
		report.Removed++
	}
	return report, nil
}

func (r *PruneReport) addPruneMessage(msg string) {
	if len(r.Messages) >= maxIndexMessages {
		return
	}
	r.Messages = append(r.Messages, msg)
}

func (s *Service) pruneCandidates(paths []string, report *PruneReport) ([]string, error) {
	if len(paths) == 0 {
		return s.listAllIndexedPaths()
	}

	seen := make(map[string]struct{})
	var out []string
	for _, raw := range paths {
		raw = strings.TrimSpace(raw)
		if raw == "" || raw == "." {
			report.Errors++
			report.addPruneMessage("empty path")
			continue
		}

		candidates, err := s.cfg.AllowlistedCandidates(raw)
		if err != nil {
			report.Errors++
			report.addPruneMessage(err.Error())
			continue
		}

		target, err := s.pickRemoveTarget(candidates)
		if err != nil {
			report.Errors++
			report.addPruneMessage(err.Error())
			continue
		}
		if target == "" {
			// No indexed hit yet — still use unique allowlisted join as scope.
			if len(candidates) == 1 {
				target = candidates[0]
			} else {
				report.Errors++
				report.addPruneMessage(fmt.Sprintf("relative path %q is ambiguous and not in index; use an absolute path", raw))
				continue
			}
		}

		matched, err := s.listIndexedUnder(target)
		if err != nil {
			report.Errors++
			report.addPruneMessage(fmt.Sprintf("%s: %v", raw, err))
			continue
		}
		for _, p := range matched {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Service) listAllIndexedPaths() ([]string, error) {
	rows, err := s.db.Query(`SELECT path FROM files ORDER BY path`)
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

func (s *Service) isOrphanIndexedPath(p string) (bool, error) {
	if !s.cfg.UnderAnyRoot(p) {
		return true, nil
	}
	_, err := os.Stat(p)
	if err == nil {
		return false, nil
	}
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, err
}
