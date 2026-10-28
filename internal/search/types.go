package search

type Mode string

const (
	ModeKeyword Mode = "keyword"
	ModeVector  Mode = "vector"
	ModeHybrid  Mode = "hybrid"
)

type Result struct {
	ID      string  `json:"id"`
	Title   string  `json:"title,omitempty"`
	Path    string  `json:"path,omitempty"`
	URL     string  `json:"url,omitempty"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
	// KeywordScore is FTS5 bm25() when the keyword leg contributed this hit.
	KeywordScore *float64 `json:"keyword_score,omitempty"`
	// VectorScore is cosine similarity when the vector leg contributed this hit.
	VectorScore *float64 `json:"vector_score,omitempty"`
	// KeywordRank is 1-based rank in the keyword leg candidate list.
	KeywordRank *int `json:"keyword_rank,omitempty"`
	// VectorRank is 1-based rank in the vector leg candidate list.
	VectorRank *int `json:"vector_rank,omitempty"`
	Source     string `json:"source"`
	Line       int    `json:"line,omitempty"`
	Page       int    `json:"page,omitempty"` // 1-based PDF page when known
	Symbol     string `json:"symbol,omitempty"`
	Kind       string `json:"symbol_kind,omitempty"`
}

type IndexStatus struct {
	IndexDir         string   `json:"index_dir"`
	Roots            []string `json:"roots"`
	DocumentCount    int      `json:"document_count"`
	ChunkCount       int      `json:"chunk_count"`
	VectorChunkCount int      `json:"vector_chunk_count"`
	EmbedEngine      string   `json:"embed_engine,omitempty"`
	EmbedModel       string   `json:"embed_model,omitempty"`
	VectorReady      bool     `json:"vector_ready"`
	OCREnabled       bool     `json:"ocr_enabled"`
	IndexExtensions  []string `json:"index_extensions,omitempty"`
	IndexedAt        *string  `json:"indexed_at"`
	Ready            bool     `json:"ready"`
	Message          string   `json:"message,omitempty"`
}
