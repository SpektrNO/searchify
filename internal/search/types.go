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
	Source  string  `json:"source"`
	Line    int     `json:"line,omitempty"`
}

type IndexStatus struct {
	IndexDir         string   `json:"index_dir"`
	Roots            []string `json:"roots"`
	DocumentCount    int      `json:"document_count"`
	ChunkCount       int      `json:"chunk_count"`
	VectorChunkCount int      `json:"vector_chunk_count"`
	EmbedModel       string   `json:"embed_model,omitempty"`
	VectorReady      bool     `json:"vector_ready"`
	IndexedAt        *string  `json:"indexed_at"`
	Ready            bool     `json:"ready"`
	Message          string   `json:"message,omitempty"`
}
