package local

import (
	"path/filepath"
)

// IndexStats is a compact inventory summary of the local index.
type IndexStats struct {
	FileCount        int     `json:"file_count"`
	FolderCount      int     `json:"folder_count"`
	VectorChunkCount int     `json:"vector_chunk_count"`
	TotalBytes       int64   `json:"total_bytes"`
	LastIndexChange  *string `json:"last_index_change,omitempty"` // RFC3339 UTC when content last changed
}

// Stats returns aggregate counts for indexed files (no last_scan tracking).
func (s *Service) Stats() (IndexStats, error) {
	var out IndexStats

	fileCount, err := s.DocumentCount()
	if err != nil {
		return out, err
	}
	out.FileCount = int(fileCount)

	vectorCount, err := s.VectorCount()
	if err != nil {
		return out, err
	}
	out.VectorChunkCount = int(vectorCount)

	err = s.db.QueryRow(`SELECT COALESCE(SUM(size), 0) FROM files`).Scan(&out.TotalBytes)
	if err != nil {
		return out, err
	}

	folders, err := s.uniqueIndexedFolderCount()
	if err != nil {
		return out, err
	}
	out.FolderCount = folders

	indexedAt, err := s.getMeta("indexed_at")
	if err != nil {
		return out, err
	}
	if indexedAt != "" {
		out.LastIndexChange = &indexedAt
	}
	return out, nil
}

func (s *Service) uniqueIndexedFolderCount() (int, error) {
	rows, err := s.db.Query(`SELECT path FROM files`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return 0, err
		}
		dir := filepath.Clean(filepath.Dir(path))
		if dir == "" || dir == "." {
			continue
		}
		seen[dir] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return len(seen), nil
}
