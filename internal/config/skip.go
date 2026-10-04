package config

import (
	"path"
	"path/filepath"
	"strings"
)

// defaultSkipDirNames are dependency and VCS directories skipped anywhere in a walk.
// Match is case-insensitive. A plain name such as "bin" skips every directory with that name.
var defaultSkipDirNames = []string{
	".git", ".cursor", "node_modules", "vendor", "bin", ".searchify",
	"venv", ".venv", "__pycache__", ".tox", ".mypy_cache",
	"site-packages", "dist-packages", "__pypackages__",
	"bower_components", "jspm_packages", ".yarn", ".pnpm-store",
}

// SkipDir reports whether a single path component is a built-in or exact extra skip.
func (c *Config) SkipDir(name string) bool {
	if name == "" || name == "." || name == string(filepath.Separator) {
		return false
	}
	if hasSkipName(defaultSkipDirNames, name) || strings.HasSuffix(strings.ToLower(name), ".egg-info") {
		return true
	}
	if c == nil {
		return false
	}
	for _, pattern := range c.ExcludeDirs {
		if patternKind(pattern) == patternExact && strings.EqualFold(pattern, name) {
			return true
		}
	}
	return false
}

// SkipPath reports whether a directory path should be left out of the walk.
// Extra SEARCHIFY_EXCLUDE_DIRS entries may be exact names, name globs (* and ?),
// or slash-separated path patterns (** matches any number of segments).
func (c *Config) SkipPath(dirPath string) bool {
	return c.segmentsSkipped(cleanSegs(dirPath))
}

// PathHasSkipDir reports whether path lies under a skipped directory.
func (c *Config) PathHasSkipDir(filePath string) bool {
	segs := cleanSegs(filePath)
	if len(segs) == 0 {
		return false
	}
	if c.segmentsSkipped(segs[:len(segs)-1]) {
		return true
	}
	return c.SkipDir(segs[len(segs)-1])
}

func (c *Config) segmentsSkipped(segs []string) bool {
	if len(segs) == 0 {
		return false
	}
	for _, seg := range segs {
		if c.SkipDir(seg) || c.nameGlobMatches(seg) {
			return true
		}
	}
	if c == nil {
		return false
	}
	for _, pattern := range c.ExcludeDirs {
		if patternKind(pattern) == patternPath && suffixMatch(pattern, segs) {
			return true
		}
	}
	return false
}

func (c *Config) nameGlobMatches(name string) bool {
	if c == nil {
		return false
	}
	lower := strings.ToLower(name)
	for _, pattern := range c.ExcludeDirs {
		if patternKind(pattern) != patternGlob {
			continue
		}
		ok, err := path.Match(strings.ToLower(pattern), lower)
		if err == nil && ok {
			return true
		}
	}
	return false
}

type excludeKind int

const (
	patternExact excludeKind = iota
	patternGlob
	patternPath
)

func patternKind(pattern string) excludeKind {
	if strings.Contains(pattern, "/") {
		return patternPath
	}
	if strings.ContainsAny(pattern, "*?[") {
		return patternGlob
	}
	return patternExact
}

func hasSkipName(names []string, name string) bool {
	for _, n := range names {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

func suffixMatch(pattern string, segs []string) bool {
	pat := splitPattern(strings.ToLower(pattern))
	lower := make([]string, len(segs))
	for i, s := range segs {
		lower[i] = strings.ToLower(s)
	}
	for i := 0; i < len(lower); i++ {
		if matchSeq(pat, lower[i:]) {
			return true
		}
	}
	return false
}

func matchSeq(pat, segs []string) bool {
	return matchAt(pat, segs, 0, 0)
}

func matchAt(pat, segs []string, pi, si int) bool {
	if pi == len(pat) {
		return si == len(segs)
	}
	if pat[pi] == "**" {
		if pi == len(pat)-1 {
			return true
		}
		for k := si; k <= len(segs); k++ {
			if matchAt(pat, segs, pi+1, k) {
				return true
			}
		}
		return false
	}
	if si >= len(segs) {
		return false
	}
	ok, err := path.Match(pat[pi], segs[si])
	if err != nil || !ok {
		return false
	}
	return matchAt(pat, segs, pi+1, si+1)
}

func cleanSegs(p string) []string {
	p = filepath.ToSlash(filepath.Clean(p))
	p = strings.Trim(p, "/")
	if p == "" || p == "." {
		return nil
	}
	var segs []string
	for _, s := range strings.Split(p, "/") {
		if s != "" && s != "." {
			segs = append(segs, s)
		}
	}
	return segs
}

func splitPattern(pattern string) []string {
	var segs []string
	for _, s := range strings.Split(pattern, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

func parseExcludeDirs(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		pattern := strings.TrimSpace(part)
		if pattern == "" {
			continue
		}
		pattern = strings.ReplaceAll(pattern, `\`, `/`)
		pattern = strings.Trim(pattern, "/")
		if !validExcludePattern(pattern) {
			return nil, errExcludeDir(part)
		}
		key := strings.ToLower(pattern)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, pattern)
	}
	return out, nil
}

func validExcludePattern(pattern string) bool {
	if pattern == "" || pattern == "." || pattern == ".." || pattern == "*" || pattern == "**" {
		return false
	}
	segs := splitPattern(pattern)
	if len(segs) == 0 {
		return false
	}
	onlyStars := true
	for _, seg := range segs {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
		if seg != "**" && seg != "*" {
			onlyStars = false
		}
		if seg == "**" {
			continue
		}
		if _, err := path.Match(seg, "x"); err != nil {
			return false
		}
	}
	return !onlyStars
}

func errExcludeDir(part string) error {
	return &excludeDirError{part: strings.TrimSpace(part)}
}

type excludeDirError struct{ part string }

func (e *excludeDirError) Error() string {
	return EnvExcludeDirs + " entry " + strconvQuote(e.part) + " is not a valid directory pattern"
}

func strconvQuote(s string) string {
	return `"` + s + `"`
}
