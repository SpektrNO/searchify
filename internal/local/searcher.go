package local

import (
	"fmt"
	"path/filepath"

	"github.com/spektr/searchify/internal/search"
)

const (
	defaultSearchLimit = 10
	maxSearchLimit     = 50
)

func (s *Service) ChunkCount() (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM chunks_fts`).Scan(&count)
	return count, err
}

func (s *Service) DocumentCount() (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&count)
	return count, err
}

func (s *Service) Status() (search.IndexStatus, error) {
	docCount, err := s.DocumentCount()
	if err != nil {
		return search.IndexStatus{}, err
	}
	chunkCount, err := s.ChunkCount()
	if err != nil {
		return search.IndexStatus{}, err
	}
	indexedAt, err := s.getMeta("indexed_at")
	if err != nil {
		return search.IndexStatus{}, err
	}

	status := search.IndexStatus{
		IndexDir:      s.cfg.IndexDir,
		Roots:         append([]string(nil), s.cfg.Roots...),
		DocumentCount: int(docCount),
		ChunkCount:    int(chunkCount),
		Ready:         chunkCount > 0,
	}
	if indexedAt != "" {
		status.IndexedAt = &indexedAt
	}
	if !status.Ready {
		status.Message = "index not ready; call index_paths first"
	}
	return status, nil
}

func (s *Service) Search(query string, limit int) ([]search.Result, error) {
	chunkCount, err := s.ChunkCount()
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}
	if chunkCount == 0 {
		return nil, fmt.Errorf("index not ready; call index_paths first")
	}

	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil, fmt.Errorf("query has no searchable terms")
	}

	rows, err := s.db.Query(`
		SELECT id, file_path, line_start, text, bm25(chunks_fts) AS score
		FROM chunks_fts
		WHERE chunks_fts MATCH ?
		ORDER BY score DESC
		LIMIT ?`,
		ftsQuery, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search index: %w", err)
	}
	defer rows.Close()

	results := make([]search.Result, 0, limit)
	for rows.Next() {
		var id, filePath, text string
		var lineStart int
		var score float64
		if err := rows.Scan(&id, &filePath, &lineStart, &text, &score); err != nil {
			return nil, err
		}
		base := filepath.Base(filePath)
		results = append(results, search.Result{
			ID:      id,
			Title:   fmt.Sprintf("%s:%d", base, lineStart),
			Path:    filePath,
			Snippet: trimSnippet(text, 300),
			Score:   score,
			Source:  "local",
			Line:    lineStart,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Service) Ready() (bool, error) {
	count, err := s.ChunkCount()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

