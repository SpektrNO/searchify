package local

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// EmbedReport summarizes searchify embed / worker results.
type EmbedReport struct {
	Files    int      `json:"files"`
	Embedded int      `json:"embedded"` // chunk vectors written
	Skipped  int      `json:"skipped"`  // files with nothing to do
	Errors   int      `json:"errors"`
	Messages []string `json:"messages,omitempty"`
}

// EmbedProgress is emitted during EmbedFiles.
type EmbedProgress struct {
	Current int
	Total   int
	Path    string
	Status  string // scan|start|ok|skip|error
}

// EmbedOptions configures EmbedFiles.
type EmbedOptions struct {
	Force    bool // re-embed even when vectors exist
	Progress func(EmbedProgress)
}

// EmbedFiles writes chunk_vectors for indexed files (in-process ONNX).
// Prefer invoking via CLI `searchify embed` so the process can exit and free native RSS.
func (s *Service) EmbedFiles(paths []string, opts EmbedOptions) (EmbedReport, error) {
	report := EmbedReport{}
	cleared, err := s.reconcileEmbedModelForWrite()
	if err != nil {
		return report, err
	}
	force := opts.Force || cleared
	files, err := s.filesForEmbed(paths)
	if err != nil {
		return report, err
	}
	report.Files = len(files)
	s.emitEmbedProgress(opts.Progress, EmbedProgress{Total: len(files), Status: "scan"})

	for i, path := range files {
		n := i + 1
		s.emitEmbedProgress(opts.Progress, EmbedProgress{Current: n, Total: len(files), Path: path, Status: "start"})
		nEmb, err := s.embedFileChunks(path, force)
		if err != nil {
			report.Errors++
			report.addEmbedMessage(fmt.Sprintf("%s: %v", path, err))
			s.emitEmbedProgress(opts.Progress, EmbedProgress{Current: n, Total: len(files), Path: path, Status: "error"})
			continue
		}
		if nEmb == 0 {
			report.Skipped++
			s.emitEmbedProgress(opts.Progress, EmbedProgress{Current: n, Total: len(files), Path: path, Status: "skip"})
		} else {
			report.Embedded += nEmb
			s.emitEmbedProgress(opts.Progress, EmbedProgress{Current: n, Total: len(files), Path: path, Status: "ok"})
		}
		if s.cfg.EmbedReload {
			s.dropEmbedder()
		}
		runtime.GC()
		debug.FreeOSMemory()
	}

	if report.Embedded > 0 {
		if err := s.setMeta("embed_model", s.cfg.EmbedModel); err != nil {
			return report, err
		}
		if err := s.setMeta("indexed_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
			return report, err
		}
	}
	s.dropEmbedder()
	runtime.GC()
	debug.FreeOSMemory()
	return report, nil
}

func (s *Service) filesForEmbed(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return s.listAllIndexedPaths()
	}
	seen := make(map[string]struct{})
	var out []string
	for _, raw := range paths {
		allowed, err := s.cfg.AllowedPath(raw)
		if err != nil {
			candidates, cErr := s.cfg.AllowlistedCandidates(raw)
			if cErr != nil || len(candidates) == 0 {
				return nil, err
			}
			allowed = candidates[0]
		}
		info, err := os.Stat(allowed)
		if err == nil && info.IsDir() {
			children, err := s.listIndexedUnder(allowed)
			if err != nil {
				return nil, err
			}
			for _, p := range children {
				if _, ok := seen[p]; ok {
					continue
				}
				seen[p] = struct{}{}
				out = append(out, p)
			}
			continue
		}
		clean := filepath.Clean(allowed)
		if _, ok := seen[clean]; ok {
			continue
		}
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM files WHERE path = ?`, clean).Scan(&n); err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out, nil
}

func (s *Service) embedFileChunks(path string, force bool) (int, error) {
	type row struct {
		id   string
		text string
	}
	var q string
	if force {
		q = `SELECT id, text FROM chunks_fts WHERE file_path = ? ORDER BY chunk_index`
	} else {
		q = `SELECT c.id, c.text FROM chunks_fts c
			LEFT JOIN chunk_vectors v ON v.chunk_id = c.id
			WHERE c.file_path = ? AND v.chunk_id IS NULL
			ORDER BY c.chunk_index`
	}
	rows, err := s.db.Query(q, path)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.text); err != nil {
			return 0, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	ids := make([]string, len(items))
	texts := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.id
		texts[i] = it.text
	}
	if err := s.embedAndStore(ids, texts); err != nil {
		return 0, err
	}
	return len(items), nil
}

// SpawnEmbedFile runs a short-lived `searchify embed --file` child (ONNX in child only).
func (s *Service) SpawnEmbedFile(path string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.Command(exe, "embed", "--file", path)
	// Override (do not append): Unix getenv returns the first matching key.
	cmd.Env = environWith(map[string]string{
		"SEARCHIFY_EMBED_BACKEND": "onnx",
		"SEARCHIFY_SKIP_EMBED":    "0",
	})
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("embed worker: %w", err)
	}
	return nil
}

func environWith(overrides map[string]string) []string {
	base := os.Environ()
	out := make([]string, 0, len(base)+len(overrides))
	for _, e := range base {
		key, _, ok := strings.Cut(e, "=")
		if !ok {
			out = append(out, e)
			continue
		}
		if _, hit := overrides[key]; hit {
			continue
		}
		out = append(out, e)
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

func (s *Service) emitEmbedProgress(fn func(EmbedProgress), p EmbedProgress) {
	if fn != nil {
		fn(p)
	}
}

func (r *EmbedReport) addEmbedMessage(msg string) {
	if len(r.Messages) >= maxIndexMessages {
		return
	}
	r.Messages = append(r.Messages, msg)
}
