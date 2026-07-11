package file

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/spektr/searchify/internal/search"
)

const defaultLimit = 10

type SearchOptions struct {
	Query         string
	Limit         int
	CaseSensitive bool
}

func Search(path string, opts SearchOptions) ([]search.Result, error) {
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	terms := tokenize(query, opts.CaseSensitive)
	if len(terms) == 0 {
		return nil, fmt.Errorf("query has no searchable terms")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	type hit struct {
		line  int
		text  string
		score float64
		terms int
	}

	hits := make([]hit, 0, limit*2)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		score, matched := scoreLine(line, terms, opts.CaseSensitive)
		if matched == 0 {
			continue
		}

		hits = append(hits, hit{
			line:  lineNo,
			text:  strings.TrimSpace(line),
			score: score,
			terms: matched,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		if hits[i].terms != hits[j].terms {
			return hits[i].terms > hits[j].terms
		}
		return hits[i].line < hits[j].line
	})

	if len(hits) > limit {
		hits = hits[:limit]
	}

	results := make([]search.Result, len(hits))
	base := filepath.Base(path)
	for i, h := range hits {
		results[i] = search.Result{
			ID:      fmt.Sprintf("%s:%d", path, h.line),
			Title:   fmt.Sprintf("%s:%d", base, h.line),
			Path:    path,
			Snippet: h.text,
			Score:   h.score,
			Source:  "file",
			Line:    h.line,
		}
	}

	return results, nil
}

func scoreLine(line string, terms []string, caseSensitive bool) (float64, int) {
	compareLine := line
	if !caseSensitive {
		compareLine = strings.ToLower(line)
	}

	var score float64
	matched := 0

	for _, term := range terms {
		compareTerm := term
		if !caseSensitive {
			compareTerm = strings.ToLower(term)
		}

		count := strings.Count(compareLine, compareTerm)
		if count == 0 {
			continue
		}

		matched++
		score += float64(count)
	}

	if matched == len(terms) {
		score += 2
	}

	return score, matched
}

func tokenize(query string, caseSensitive bool) []string {
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return unicode.IsSpace(r)
	})

	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, "\"'`")
		if field == "" {
			continue
		}
		if !caseSensitive {
			field = strings.ToLower(field)
		}
		terms = append(terms, field)
	}

	return terms
}
