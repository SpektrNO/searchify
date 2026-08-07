package local

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/extract"
)

const schemaVersion = 2

type Service struct {
	cfg                 *config.Config
	db                  *sql.DB
	extract             *extract.Registry
	embedMu             sync.Mutex
	embedder            Embedder
	embedErr            error
	embedForTest        Embedder
	spawnEmbedForTest   func(path string) error // optional; tests avoid real exec
	spawnExtractForTest func(path string) (string, []string, error)
}

type IndexReport struct {
	Indexed  int      `json:"indexed"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Errors   int      `json:"errors"`
	Messages []string `json:"messages,omitempty"`
}

func NewService(cfg *config.Config) (*Service, error) {
	if err := os.MkdirAll(cfg.IndexDir, 0o755); err != nil {
		return nil, fmt.Errorf("create index dir: %w", err)
	}

	dbPath := filepath.Join(cfg.IndexDir, "index.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open index db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := configureSQLite(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	svc := &Service{
		cfg: cfg,
		db:  db,
		extract: extract.NewRegistry(extract.Options{
			OCREnabled: cfg.OCREnabled,
			OCRLang:    cfg.OCRLang,
			TextOnly:   cfg.TextOnly,
		}),
	}
	if err := svc.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return svc, nil
}

func configureSQLite(db *sql.DB) error {
	// Cap cache and disable mmap so a large existing index.db cannot pin tens of GB into RSS.
	for _, pragma := range []string{
		`PRAGMA busy_timeout=5000`,
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA temp_store=FILE`,
		`PRAGMA mmap_size=0`,
		`PRAGMA cache_size=-65536`, // kibibytes when negative → ~64 MiB
	} {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("sqlite %s: %w", pragma, err)
		}
	}
	return nil
}

func (s *Service) Close() error {
	s.dropEmbedder()
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Service) currentSchemaVersion() (int, error) {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return 0, err
	}
	var version int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)
	if err != nil && !strings.Contains(err.Error(), "no such table") {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

func (s *Service) migrate() error {
	version, err := s.currentSchemaVersion()
	if err != nil {
		return err
	}
	if version >= schemaVersion {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if version < 1 {
		if err := s.migrateToV1(tx); err != nil {
			return err
		}
		version = 1
	}
	if version < 2 {
		if err := s.migrateToV2(tx); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM schema_version`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_version(version) VALUES (?)`, schemaVersion); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) migrateToV1(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS files (
			path TEXT PRIMARY KEY,
			mtime_ns INTEGER NOT NULL,
			size INTEGER NOT NULL,
			content_hash TEXT NOT NULL,
			indexed_at TEXT NOT NULL
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
			id UNINDEXED,
			file_path UNINDEXED,
			chunk_index UNINDEXED,
			line_start UNINDEXED,
			line_end UNINDEXED,
			text,
			tokenize='unicode61'
		)`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrate v1: %w", err)
		}
	}
	return nil
}

func (s *Service) migrateToV2(tx *sql.Tx) error {
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS chunk_vectors (
		chunk_id TEXT PRIMARY KEY,
		embedding BLOB NOT NULL
	)`); err != nil {
		return fmt.Errorf("migrate v2: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO meta(key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		"embed_model", s.cfg.EmbedModel,
	); err != nil {
		return fmt.Errorf("migrate v2 meta: %w", err)
	}
	return nil
}

func (s *Service) getEmbedder() (Embedder, error) {
	if s.embedForTest != nil {
		return s.embedForTest, nil
	}
	s.embedMu.Lock()
	defer s.embedMu.Unlock()
	if s.embedder != nil {
		return s.embedder, nil
	}
	if s.embedErr != nil {
		return nil, s.embedErr
	}
	s.embedder, s.embedErr = newKjarniEmbedder(s.cfg.EmbedModel)
	return s.embedder, s.embedErr
}

// dropEmbedder closes the native ONNX embedder so process RSS can shrink.
func (s *Service) dropEmbedder() {
	if s.embedForTest != nil {
		return
	}
	s.embedMu.Lock()
	defer s.embedMu.Unlock()
	if s.embedder != nil {
		_ = s.embedder.Close()
		s.embedder = nil
	}
	s.embedErr = nil
}

// SetEmbedderForTest injects a fake embedder (tests only).
func (s *Service) SetEmbedderForTest(e Embedder) {
	s.embedForTest = e
}

func (s *Service) setMeta(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO meta(key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

func (s *Service) getMeta(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func fileHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func (s *Service) fileRecord(path string) (mtimeNS int64, size int64, exists bool, err error) {
	err = s.db.QueryRow(`SELECT mtime_ns, size FROM files WHERE path = ?`, path).Scan(&mtimeNS, &size)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return mtimeNS, size, true, nil
}

func (s *Service) deleteFileChunks(path string) error {
	if _, err := s.db.Exec(
		`DELETE FROM chunk_vectors WHERE chunk_id IN (SELECT id FROM chunks_fts WHERE file_path = ?)`,
		path,
	); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM chunks_fts WHERE file_path = ?`, path)
	return err
}

func (s *Service) indexFile(path string, info os.FileInfo, content []byte) (string, error) {
	chunks, err := chunkFile(content)
	if err != nil {
		return "", err
	}
	maxChunks := s.cfg.MaxChunksPerFile
	if maxChunks <= 0 {
		maxChunks = 64
	}
	var warn string
	if len(chunks) > maxChunks {
		warn = fmt.Sprintf("truncated to %d chunks (SEARCHIFY_MAX_CHUNKS_PER_FILE)", maxChunks)
		chunks = chunks[:maxChunks]
	}
	if err := s.deleteFileChunks(path); err != nil {
		return warn, err
	}

	chunkIDs := make([]string, 0, len(chunks))
	texts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		id := fmt.Sprintf("%s#chunk-%d", path, c.Index)
		chunkIDs = append(chunkIDs, id)
		texts = append(texts, c.Text)
		_, err := s.db.Exec(
			`INSERT INTO chunks_fts(id, file_path, chunk_index, line_start, line_end, text)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			id, path, c.Index, c.LineStart, c.LineEnd, c.Text,
		)
		if err != nil {
			return warn, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	hash := fileHash(content)
	_, err = s.db.Exec(
		`INSERT INTO files(path, mtime_ns, size, content_hash, indexed_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET
		   mtime_ns = excluded.mtime_ns,
		   size = excluded.size,
		   content_hash = excluded.content_hash,
		   indexed_at = excluded.indexed_at`,
		path, info.ModTime().UnixNano(), info.Size(), hash, now,
	)
	if err != nil {
		return warn, err
	}

	if len(texts) == 0 || !s.cfg.WantVectors() {
		if !s.cfg.WantVectors() {
			if warn != "" {
				warn += "; "
			}
			warn += "keyword-only (SEARCHIFY_SKIP_EMBED / SEARCHIFY_EMBED_BACKEND=none); vectors not written"
		}
		return warn, nil
	}

	switch {
	case s.cfg.UseProcessEmbed():
		if err := s.runEmbedWorker(path); err != nil {
			return warn, err
		}
	case s.cfg.UseInProcessEmbed():
		if err := s.embedAndStore(chunkIDs, texts); err != nil {
			return warn, err
		}
		if err := s.setMeta("embed_model", s.cfg.EmbedModel); err != nil {
			return warn, err
		}
	}
	return warn, nil
}

func (s *Service) runEmbedWorker(path string) error {
	if s.spawnEmbedForTest != nil {
		return s.spawnEmbedForTest(path)
	}
	return s.SpawnEmbedFile(path)
}

func (s *Service) embedAndStore(chunkIDs, texts []string) error {
	embedder, err := s.getEmbedder()
	if err != nil {
		return fmt.Errorf("embedder: %w", err)
	}
	batch := s.cfg.EmbedBatch
	if batch <= 0 {
		batch = 1
	}
	for start := 0; start < len(texts); start += batch {
		end := start + batch
		if end > len(texts) {
			end = len(texts)
		}
		ids := chunkIDs[start:end]
		batchTexts := texts[start:end]

		var vectors [][]float32
		if batch == 1 {
			v, err := embedder.Encode(batchTexts[0])
			if err != nil {
				return fmt.Errorf("embed chunks: %w", err)
			}
			vectors = [][]float32{v}
		} else {
			vectors, err = embedder.EncodeBatch(batchTexts)
			if err != nil {
				return fmt.Errorf("embed chunks: %w", err)
			}
		}
		if len(vectors) != len(ids) {
			return fmt.Errorf("embedder returned %d vectors for %d chunks", len(vectors), len(ids))
		}
		for i, id := range ids {
			if err := s.upsertChunkVector(id, vectors[i]); err != nil {
				return err
			}
			vectors[i] = nil
		}
	}
	return nil
}
