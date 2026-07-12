package local

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/spektr/searchify/internal/config"
)

const schemaVersion = 1

type Service struct {
	cfg *config.Config
	db  *sql.DB
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

	svc := &Service{cfg: cfg, db: db}
	if err := svc.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return svc, nil
}

func (s *Service) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Service) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}

	var version int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)
	if err != nil && !strings.Contains(err.Error(), "no such table") {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version >= schemaVersion {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

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
			return fmt.Errorf("migrate: %w", err)
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
	_, err := s.db.Exec(`DELETE FROM chunks_fts WHERE file_path = ?`, path)
	return err
}

func (s *Service) indexFile(path string, info os.FileInfo, content []byte) error {
	chunks, err := chunkFile(content)
	if err != nil {
		return err
	}
	if err := s.deleteFileChunks(path); err != nil {
		return err
	}

	for _, c := range chunks {
		id := fmt.Sprintf("%s#chunk-%d", path, c.Index)
		_, err := s.db.Exec(
			`INSERT INTO chunks_fts(id, file_path, chunk_index, line_start, line_end, text)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			id, path, c.Index, c.LineStart, c.LineEnd, c.Text,
		)
		if err != nil {
			return err
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
	return err
}
