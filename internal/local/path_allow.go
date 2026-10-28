package local

import (
	"path/filepath"
	"strings"
)

// PathAllow is a resolved caller-supplied allow-set.
// A non-nil value with empty Prefixes and Exact means "constrain to nothing"
// (return no hits). Nil means unscoped search.
type PathAllow struct {
	Prefixes []string // absolute path prefixes (segment match)
	Exact    []string // absolute exact paths
}

func (a *PathAllow) emptyConstraint() bool {
	return a != nil && len(a.Prefixes) == 0 && len(a.Exact) == 0
}

// pathAllowSQL returns an AND (…) clause against file_path (or alias.file_path).
func pathAllowSQL(tableAlias string, a *PathAllow) (clause string, args []any) {
	if a == nil {
		return "", nil
	}
	col := "file_path"
	if tableAlias != "" {
		col = tableAlias + ".file_path"
	}
	if a.emptyConstraint() {
		return " AND 1=0", nil
	}

	parts := make([]string, 0, len(a.Exact)+len(a.Prefixes))
	for _, p := range a.Exact {
		parts = append(parts, col+" = ?")
		args = append(args, p)
	}
	sep := string(filepath.Separator)
	for _, p := range a.Prefixes {
		parts = append(parts, "("+col+" = ? OR "+col+" LIKE ? ESCAPE '\\')")
		args = append(args, p, escapeLike(p)+sep+"%")
	}
	return " AND (" + strings.Join(parts, " OR ") + ")", args
}

func pathWithinPrefix(path, prefix string) bool {
	prefix = filepath.Clean(prefix)
	path = filepath.Clean(path)
	if path == prefix {
		return true
	}
	rel, err := filepath.Rel(prefix, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
