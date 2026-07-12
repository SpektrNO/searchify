package local

import (
	"strings"
	"unicode"
)

func buildFTSQuery(raw string) string {
	fields := strings.FieldsFunc(strings.TrimSpace(raw), func(r rune) bool {
		return unicode.IsSpace(r)
	})

	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, "\"'`")
		if field == "" {
			continue
		}
		field = strings.ReplaceAll(field, `"`, `""`)
		terms = append(terms, `"`+field+`"`)
	}

	return strings.Join(terms, " AND ")
}
