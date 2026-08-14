package local

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spektr/searchify/internal/rank"
	"github.com/spektr/searchify/internal/search"
)

const (
	defaultSearchLimit = 10
	maxSearchLimit     = 50
	candidatePool      = 50
)

type SearchParams struct {
	Query      string
	Limit      int
	Mode       search.Mode
	SnippetMax int // 0 = use SEARCHIFY_SNIPPET_CHARS / default 300
}

// LegTiming is best-effort per-leg timing for search_local breakdowns.
type LegTiming struct {
	KeywordMs int
	VectorMs  int
	RRFMs     int
}

// SearchOutcome is search hits plus resolved mode and leg timings.
type SearchOutcome struct {
	Results []search.Result
	Mode    search.Mode
	Timing  LegTiming
}

func (s *Service) DefaultMode() (search.Mode, error) {
	count, err := s.VectorCount()
	if err != nil {
		return "", err
	}
	if count > 0 {
		return search.ModeHybrid, nil
	}
	return search.ModeKeyword, nil
}

func (s *Service) Search(params SearchParams) (SearchOutcome, error) {
	limit := normalizeLimit(params.Limit)
	snippetMax := s.cfg.ResolveSnippetChars(params.SnippetMax)
	mode := params.Mode
	if mode == "" {
		var err error
		mode, err = s.DefaultMode()
		if err != nil {
			return SearchOutcome{}, err
		}
	}

	switch mode {
	case search.ModeKeyword:
		start := time.Now()
		results, err := s.searchKeyword(params.Query, limit, snippetMax)
		if err != nil {
			return SearchOutcome{}, err
		}
		return SearchOutcome{
			Results: results,
			Mode:    mode,
			Timing:  LegTiming{KeywordMs: elapsedMs(start)},
		}, nil
	case search.ModeVector:
		start := time.Now()
		results, err := s.searchVector(params.Query, limit, snippetMax)
		if err != nil {
			return SearchOutcome{}, err
		}
		return SearchOutcome{
			Results: results,
			Mode:    mode,
			Timing:  LegTiming{VectorMs: elapsedMs(start)},
		}, nil
	case search.ModeHybrid:
		results, timing, err := s.searchHybridTimed(params.Query, limit, snippetMax)
		if err != nil {
			return SearchOutcome{}, err
		}
		return SearchOutcome{Results: results, Mode: mode, Timing: timing}, nil
	default:
		return SearchOutcome{}, fmt.Errorf("unknown search mode %q", mode)
	}
}

func elapsedMs(start time.Time) int {
	ms := int(time.Since(start) / time.Millisecond)
	if ms < 0 {
		return 0
	}
	return ms
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchLimit
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

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
	vectorCount, err := s.VectorCount()
	if err != nil {
		return search.IndexStatus{}, err
	}
	indexedAt, err := s.getMeta("indexed_at")
	if err != nil {
		return search.IndexStatus{}, err
	}
	embedModel, err := s.getMeta("embed_model")
	if err != nil {
		return search.IndexStatus{}, err
	}
	if embedModel == "" {
		embedModel = s.cfg.EmbedModel
	}
	embedEngine, err := s.getMeta("embed_engine")
	if err != nil {
		return search.IndexStatus{}, err
	}
	if embedEngine == "" {
		embedEngine = s.wantEmbedEngine()
	}

	vectorReady := chunkCount > 0 && vectorCount == chunkCount
	status := search.IndexStatus{
		IndexDir:         s.cfg.IndexDir,
		Roots:            append([]string(nil), s.cfg.Roots...),
		DocumentCount:    int(docCount),
		ChunkCount:       int(chunkCount),
		VectorChunkCount: int(vectorCount),
		EmbedEngine:      embedEngine,
		EmbedModel:       embedModel,
		VectorReady:      vectorReady,
		OCREnabled:       s.cfg.OCREnabled,
		IndexExtensions:  s.extract.Extensions(),
		Ready:            chunkCount > 0,
	}
	if indexedAt != "" {
		status.IndexedAt = &indexedAt
	}
	if !status.Ready {
		status.Message = "index not ready; call index_paths first"
	} else if !vectorReady {
		status.Message = "vectors incomplete; run index_paths --force to embed all chunks"
	}
	return status, nil
}

func (s *Service) searchKeyword(query string, limit, snippetMax int) ([]search.Result, error) {
	if err := s.requireIndex(); err != nil {
		return nil, err
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

	return scanSearchResults(rows, limit, snippetMax)
}

func (s *Service) searchVector(query string, limit, snippetMax int) ([]search.Result, error) {
	if err := s.requireIndex(); err != nil {
		return nil, err
	}
	if err := s.requireMatchingEmbedModel(); err != nil {
		return nil, err
	}

	vectorCount, err := s.VectorCount()
	if err != nil {
		return nil, err
	}
	if vectorCount == 0 {
		return nil, fmt.Errorf("no vectors in index; run index_paths --force")
	}

	embedder, err := s.getEmbedder()
	if err != nil {
		return nil, err
	}
	queryVec, err := embedder.Encode(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	vectors, err := s.loadAllChunkVectors()
	if err != nil {
		return nil, err
	}

	hits := topKByCosine(queryVec, vectors, limit)
	scores := make(map[string]float64, len(hits))
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.chunkID)
		scores[hit.chunkID] = float64(hit.score)
	}
	return s.resultsByChunkIDs(ids, scores, snippetMax)
}

func (s *Service) searchHybridTimed(query string, limit, snippetMax int) ([]search.Result, LegTiming, error) {
	if err := s.requireIndex(); err != nil {
		return nil, LegTiming{}, err
	}

	kwStart := time.Now()
	keywordResults, keywordErr := s.searchKeywordCandidates(query, candidatePool)
	keywordMs := elapsedMs(kwStart)

	vecStart := time.Now()
	vectorResults, vectorErr := s.searchVectorCandidates(query, candidatePool)
	vectorMs := elapsedMs(vecStart)

	if len(keywordResults) == 0 && len(vectorResults) == 0 {
		if keywordErr != nil && vectorErr != nil {
			return nil, LegTiming{}, fmt.Errorf("hybrid search failed: keyword: %v; vector: %v", keywordErr, vectorErr)
		}
		return nil, LegTiming{}, fmt.Errorf("no results")
	}

	lists := make([][]rank.RankedItem, 0, 2)
	if len(keywordResults) > 0 {
		lists = append(lists, keywordResults)
	}
	if len(vectorResults) > 0 {
		lists = append(lists, vectorResults)
	}

	rrfStart := time.Now()
	merged := rank.RRF(lists, rank.DefaultRRFK, limit)
	rrfMs := elapsedMs(rrfStart)

	scores := make(map[string]float64, len(merged))
	ids := make([]string, 0, len(merged))
	for _, item := range merged {
		ids = append(ids, item.ID)
		scores[item.ID] = item.Score
	}
	results, err := s.resultsByChunkIDs(ids, scores, snippetMax)
	if err != nil {
		return nil, LegTiming{}, err
	}
	return results, LegTiming{
		KeywordMs: keywordMs,
		VectorMs:  vectorMs,
		RRFMs:     rrfMs,
	}, nil
}

func (s *Service) searchKeywordCandidates(query string, limit int) ([]rank.RankedItem, error) {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil, fmt.Errorf("query has no searchable terms")
	}

	rows, err := s.db.Query(`
		SELECT id, bm25(chunks_fts) AS score
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

	items := make([]rank.RankedItem, 0, limit)
	for rows.Next() {
		var id string
		var score float64
		if err := rows.Scan(&id, &score); err != nil {
			return nil, err
		}
		items = append(items, rank.RankedItem{ID: id, Score: score})
	}
	return items, rows.Err()
}

func (s *Service) searchVectorCandidates(query string, limit int) ([]rank.RankedItem, error) {
	if err := s.requireMatchingEmbedModel(); err != nil {
		return nil, err
	}
	vectorCount, err := s.VectorCount()
	if err != nil {
		return nil, err
	}
	if vectorCount == 0 {
		return nil, nil
	}

	embedder, err := s.getEmbedder()
	if err != nil {
		return nil, err
	}
	queryVec, err := embedder.Encode(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	vectors, err := s.loadAllChunkVectors()
	if err != nil {
		return nil, err
	}

	hits := topKByCosine(queryVec, vectors, limit)
	items := make([]rank.RankedItem, 0, len(hits))
	for _, hit := range hits {
		items = append(items, rank.RankedItem{ID: hit.chunkID, Score: float64(hit.score)})
	}
	return items, nil
}

func (s *Service) resultsByChunkIDs(ids []string, scores map[string]float64, snippetMax int) ([]search.Result, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, file_path, line_start, text
		FROM chunks_fts
		WHERE id IN (%s)`, joinPlaceholders(placeholders))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[string]search.Result, len(ids))
	for rows.Next() {
		var id, filePath, text string
		var lineStart int
		if err := rows.Scan(&id, &filePath, &lineStart, &text); err != nil {
			return nil, err
		}
		base := filepath.Base(filePath)
		byID[id] = search.Result{
			ID:      id,
			Title:   fmt.Sprintf("%s:%d", base, lineStart),
			Path:    filePath,
			Snippet: trimSnippet(text, snippetMax),
			Score:   scores[id],
			Source:  "local",
			Line:    lineStart,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	results := make([]search.Result, 0, len(ids))
	for _, id := range ids {
		if r, ok := byID[id]; ok {
			results = append(results, r)
		}
	}
	return results, nil
}

func scanSearchResults(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}, limit, snippetMax int) ([]search.Result, error) {
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
			Snippet: trimSnippet(text, snippetMax),
			Score:   score,
			Source:  "local",
			Line:    lineStart,
		})
	}
	return results, rows.Err()
}

func (s *Service) requireIndex() error {
	count, err := s.ChunkCount()
	if err != nil {
		return fmt.Errorf("read index: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("index not ready; call index_paths first")
	}
	return nil
}

func (s *Service) Ready() (bool, error) {
	count, err := s.ChunkCount()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func joinPlaceholders(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
