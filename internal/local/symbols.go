package local

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SymbolHit is one row from the symbols table.
type SymbolHit struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Lang     string `json:"lang"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	QualName string `json:"qual_name"`
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line"`
	Col      int    `json:"col"`
	ChunkID  string `json:"chunk_id,omitempty"`
}

// RefHit is one row from symbol_refs.
type RefHit struct {
	Path     string `json:"path"`
	Lang     string `json:"lang"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	QualName string `json:"qual_name"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
}

// LookupSymbolParams filters symbol definitions.
type LookupSymbolParams struct {
	Query      string
	Kind       string
	PathPrefix string
	Limit      int
}

// FindReferencesParams filters symbol references.
type FindReferencesParams struct {
	Symbol     string
	PathPrefix string
	Limit      int
}

func normalizeSymbolLimit(n int) int {
	if n <= 0 {
		return 20
	}
	if n > 100 {
		return 100
	}
	return n
}

// LookupSymbol finds definitions by name or qual_name (exact or prefix).
func (s *Service) LookupSymbol(p LookupSymbolParams) ([]SymbolHit, error) {
	q := strings.TrimSpace(p.Query)
	if q == "" {
		return nil, fmt.Errorf("query is required")
	}
	limit := normalizeSymbolLimit(p.Limit)
	like := escapeLike(q) + "%"

	args := []any{q, q, like, like}
	sql := `
		SELECT id, path, lang, kind, name, qual_name, line, end_line, col, COALESCE(chunk_id, '')
		FROM symbols
		WHERE (name = ? OR qual_name = ? OR name LIKE ? ESCAPE '\' OR qual_name LIKE ? ESCAPE '\')`
	if k := strings.TrimSpace(p.Kind); k != "" {
		sql += ` AND kind = ?`
		args = append(args, k)
	}
	if pref := strings.TrimSpace(p.PathPrefix); pref != "" {
		sql += ` AND (path = ? OR path LIKE ? ESCAPE '\')`
		args = append(args, pref, escapeLike(pref)+string(filepath.Separator)+"%")
	}
	sql += ` ORDER BY qual_name, path, line LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SymbolHit
	for rows.Next() {
		var h SymbolHit
		if err := rows.Scan(&h.ID, &h.Path, &h.Lang, &h.Kind, &h.Name, &h.QualName, &h.Line, &h.EndLine, &h.Col, &h.ChunkID); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// FindReferences finds best-effort refs matching symbol name or qual_name.
func (s *Service) FindReferences(p FindReferencesParams) ([]RefHit, error) {
	sym := strings.TrimSpace(p.Symbol)
	if sym == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	limit := normalizeSymbolLimit(p.Limit)
	simple := sym
	if i := strings.LastIndex(sym, "."); i >= 0 && i+1 < len(sym) {
		simple = sym[i+1:]
	}

	args := []any{sym, sym, simple, simple}
	sql := `
		SELECT path, lang, kind, name, qual_name, line, col
		FROM symbol_refs
		WHERE (qual_name = ? OR name = ? OR qual_name = ? OR name = ?)`
	if pref := strings.TrimSpace(p.PathPrefix); pref != "" {
		sql += ` AND (path = ? OR path LIKE ? ESCAPE '\')`
		args = append(args, pref, escapeLike(pref)+string(filepath.Separator)+"%")
	}
	sql += ` ORDER BY path, line LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RefHit
	for rows.Next() {
		var h RefHit
		if err := rows.Scan(&h.Path, &h.Lang, &h.Kind, &h.Name, &h.QualName, &h.Line, &h.Col); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
