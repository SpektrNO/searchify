package code

import (
	"regexp"
	"strings"
)

var (
	csUsingRe = regexp.MustCompile(`(?m)^\s*using\s+(?:static\s+)?([\w.]+)\s*;`)
	csTypeRe  = regexp.MustCompile(`(?m)(?:^|[\n;{}])\s*((?:(?:public|private|protected|internal|static|abstract|sealed|partial|file)\s+)*)(?:(class|struct|interface|enum|record(?:\s+class|\s+struct)?)\s+)([A-Za-z_][\w]*)`)
	csMethodRe = regexp.MustCompile(`(?m)(?:^|[\n{;])\s*(?:(?:public|private|protected|internal|static|virtual|override|abstract|async|sealed|partial|new|extern|unsafe)\s+)+[A-Za-z_~][\w.<>,\[\]\s\?]*\s+([A-Za-z_][\w]*)\s*(?:<[^>]*>)?\s*\(`)
	csCtorRe   = regexp.MustCompile(`(?m)(?:^|[\n{;])\s*(?:(?:public|private|protected|internal|static)\s+)+([A-Za-z_][\w]*)\s*\(`)
	csCallRe   = regexp.MustCompile(`\b([A-Za-z_][\w]*)\s*(?:\.\s*([A-Za-z_][\w]*))?\s*\(`)
	csTopFnRe  = regexp.MustCompile(`(?m)^(?:\s*(?:public|private|protected|internal|static|async)\s+)+[A-Za-z_][\w.<>,\[\]\?\s]*\s+([A-Za-z_][\w]*)\s*(?:<[^>]*>)?\s*\(`)
)

func analyzeCSharpHeuristic(_ string, src []byte) (Result, error) {
	s := string(src)
	starts := lineStartsCS(s)
	masked := maskCSharpNoise(s)

	var units []Unit
	var symbols []Symbol
	var refs []Ref

	addSym := func(kind, name, qn string, start, end int) {
		a := lineColCS(starts, start)
		b := lineColCS(starts, max(start, end-1))
		if qn == "" {
			qn = name
		}
		symbols = append(symbols, Symbol{
			Kind: kind, Name: name, QualName: qn,
			Line: a.line, EndLine: b.line, Col: a.col,
		})
	}
	addUnit := func(kind, name, qn string, start, end int) {
		a := lineColCS(starts, start)
		b := lineColCS(starts, max(start, end-1))
		if qn == "" {
			qn = name
		}
		units = append(units, Unit{
			Kind: kind, Name: name, QualName: qn,
			LineStart: a.line, LineEnd: b.line,
			ByteStart: byteAtCS(s, start), ByteEnd: byteAtCS(s, end),
		})
		addSym(kind, name, qn, start, end)
	}

	type decl struct {
		kind, name string
		start, end int
	}
	var types []decl

	for _, m := range csTypeRe.FindAllStringSubmatchIndex(masked, -1) {
		// groups: 0 full, 1 mods, 2 typekw, 3 name
		if len(m) < 8 {
			continue
		}
		nameStart, nameEnd := m[6], m[7]
		if nameStart < 0 {
			continue
		}
		name := masked[nameStart:nameEnd]
		kwStart, kwEnd := m[4], m[5]
		kw := strings.TrimSpace(masked[kwStart:kwEnd])
		kind := "type"
		if strings.HasPrefix(kw, "class") {
			kind = "class"
		}
		d := topLevelDepthCS(masked, nameStart)
		if d > 1 {
			continue
		}
		start := lineStartCS(masked, nameStart)
		brace := strings.IndexByte(masked[nameEnd:], '{')
		end := nameEnd
		if brace >= 0 {
			end = matchBalancedCS(masked, nameEnd+brace, '{', '}')
		}
		types = append(types, decl{kind: kind, name: name, start: start, end: end})
	}

	first := len(s)
	for _, t := range types {
		if t.start < first {
			first = t.start
		}
	}
	// Top-level functions (C# 9+)
	var topFns []decl
	for _, m := range csTopFnRe.FindAllStringSubmatchIndex(masked, -1) {
		if len(m) < 4 {
			continue
		}
		ns, ne := m[2], m[3]
		if topLevelDepthCS(masked, ns) != 0 {
			continue
		}
		// skip if this is inside a type we already found as a method
		insideType := false
		for _, t := range types {
			if ns >= t.start && ns < t.end {
				insideType = true
				break
			}
		}
		if insideType {
			continue
		}
		name := masked[ns:ne]
		if isCSKeyword(name) {
			continue
		}
		start := lineStartCS(masked, ns)
		end := functionEndCS(masked, ns)
		topFns = append(topFns, decl{kind: "function", name: name, start: start, end: end})
		if start < first {
			first = start
		}
	}

	if first > 0 && first < len(s) && strings.TrimSpace(s[:first]) != "" {
		le := lineColCS(starts, first)
		units = append(units, Unit{
			Kind: "module", Name: "", QualName: "",
			LineStart: 1, LineEnd: le.line,
			ByteStart: 0, ByteEnd: byteAtCS(s, first),
		})
	}

	for _, t := range types {
		addUnit(t.kind, t.name, t.name, t.start, t.end)
		open := strings.IndexByte(masked[t.start:t.end], '{')
		if open < 0 {
			continue
		}
		open += t.start
		seen := map[string]bool{}
		// methods
		body := masked[open:t.end]
		for _, m := range csMethodRe.FindAllStringSubmatchIndex(body, -1) {
			if len(m) < 4 {
				continue
			}
			ns, ne := m[2], m[3]
			abs := open + ns
			if depthIn(masked, open, abs) != 1 {
				continue
			}
			name := masked[abs : open+ne]
			if isCSKeyword(name) || name == t.name {
				continue
			}
			key := t.name + "." + name
			if seen[key] {
				continue
			}
			seen[key] = true
			addSym("method", name, key, abs, open+ne)
		}
		// constructors
		for _, m := range csCtorRe.FindAllStringSubmatchIndex(body, -1) {
			if len(m) < 4 {
				continue
			}
			ns, ne := m[2], m[3]
			abs := open + ns
			if depthIn(masked, open, abs) != 1 {
				continue
			}
			name := masked[abs : open+ne]
			if name != t.name {
				continue
			}
			key := t.name + ".constructor"
			if seen[key] {
				continue
			}
			seen[key] = true
			addSym("method", "constructor", key, abs, open+ne)
		}
	}

	for _, f := range topFns {
		kind := f.kind
		line := masked[f.start:min(f.end, f.start+80)]
		if strings.Contains(line, "async") {
			kind = "async_function"
		}
		addUnit(kind, f.name, f.name, f.start, f.end)
	}

	for _, m := range csUsingRe.FindAllStringSubmatchIndex(s, -1) {
		if len(m) < 4 {
			continue
		}
		qn := s[m[2]:m[3]]
		name := qn
		if i := strings.LastIndex(qn, "."); i >= 0 {
			name = qn[i+1:]
		}
		pos := lineColCS(starts, m[0])
		refs = append(refs, Ref{Kind: "import", Name: name, QualName: qn, Line: pos.line, Col: pos.col})
	}

	keywords := map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"typeof": true, "sizeof": true, "new": true, "return": true, "nameof": true,
	}
	for _, m := range csCallRe.FindAllStringSubmatchIndex(masked, -1) {
		if len(m) < 4 {
			continue
		}
		if m[4] >= 0 {
			recv := masked[m[2]:m[3]]
			name := masked[m[4]:m[5]]
			if keywords[recv] {
				continue
			}
			pos := lineColCS(starts, m[0])
			refs = append(refs, Ref{Kind: "call", Name: name, QualName: recv + "." + name, Line: pos.line, Col: pos.col})
		} else {
			name := masked[m[2]:m[3]]
			if keywords[name] || isCSKeyword(name) {
				continue
			}
			pos := lineColCS(starts, m[0])
			refs = append(refs, Ref{Kind: "call", Name: name, QualName: name, Line: pos.line, Col: pos.col})
		}
	}

	return Result{Units: units, Symbols: symbols, Refs: refs}, nil
}

func isCSKeyword(s string) bool {
	switch s {
	case "if", "for", "while", "switch", "catch", "using", "namespace", "class", "struct",
		"interface", "enum", "record", "return", "new", "typeof", "sizeof", "nameof",
		"get", "set", "init", "value", "async", "await", "where", "select", "from":
		return true
	default:
		return false
	}
}

func maskCSharpNoise(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	i := 0
	for i < len(src) {
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			b.WriteString("  ")
			i += 2
			for i < len(src) && src[i] != '\n' {
				b.WriteByte(' ')
				i++
			}
			continue
		}
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			b.WriteString("  ")
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				if src[i] == '\n' {
					b.WriteByte('\n')
				} else {
					b.WriteByte(' ')
				}
				i++
			}
			if i+1 < len(src) {
				b.WriteString("  ")
				i += 2
			}
			continue
		}
		q := src[i]
		if q == '"' || q == '\'' {
			b.WriteByte(' ')
			i++
			for i < len(src) {
				if src[i] == '\\' {
					b.WriteString("  ")
					i += 2
					continue
				}
				if src[i] == q {
					b.WriteByte(' ')
					i++
					break
				}
				if src[i] == '\n' {
					b.WriteByte('\n')
				} else {
					b.WriteByte(' ')
				}
				i++
			}
			continue
		}
		// verbatim @"..."
		if q == '@' && i+1 < len(src) && src[i+1] == '"' {
			b.WriteString("  ")
			i += 2
			for i < len(src) {
				if src[i] == '"' && i+1 < len(src) && src[i+1] == '"' {
					b.WriteString("  ")
					i += 2
					continue
				}
				if src[i] == '"' {
					b.WriteByte(' ')
					i++
					break
				}
				if src[i] == '\n' {
					b.WriteByte('\n')
				} else {
					b.WriteByte(' ')
				}
				i++
			}
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}

func lineStartsCS(src string) []int {
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

type posCS struct{ line, col int }

func lineColCS(starts []int, offset int) posCS {
	lo, hi := 0, len(starts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if starts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return posCS{line: lo + 1, col: offset - starts[lo] + 1}
}

func byteAtCS(src string, charIndex int) int {
	if charIndex <= 0 {
		return 0
	}
	if charIndex >= len(src) {
		return len([]byte(src))
	}
	// src is a Go string of source bytes; offsets from masking are byte indices.
	return charIndex
}

func topLevelDepthCS(masked string, idx int) int {
	d := 0
	for i := 0; i < idx && i < len(masked); i++ {
		switch masked[i] {
		case '{':
			d++
		case '}':
			d--
		}
	}
	return d
}

func depthIn(masked string, from, at int) int {
	d := 0
	for i := from; i < at && i < len(masked); i++ {
		switch masked[i] {
		case '{':
			d++
		case '}':
			d--
		}
	}
	return d
}

func lineStartCS(masked string, idx int) int {
	for i := idx - 1; i >= 0; i-- {
		if masked[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

func matchBalancedCS(masked string, from int, open, close byte) int {
	if from >= len(masked) || masked[from] != open {
		return from
	}
	d := 0
	for i := from; i < len(masked); i++ {
		switch masked[i] {
		case open:
			d++
		case close:
			d--
			if d == 0 {
				return i + 1
			}
		}
	}
	return len(masked)
}

func functionEndCS(masked string, nameStart int) int {
	paren := strings.IndexByte(masked[nameStart:], '(')
	if paren < 0 {
		return nameStart
	}
	p := nameStart + paren
	p = matchBalancedCS(masked, p, '(', ')')
	i := p
	for i < len(masked) && masked[i] != '{' && masked[i] != ';' {
		i++
	}
	if i < len(masked) && masked[i] == '{' {
		return matchBalancedCS(masked, i, '{', '}')
	}
	if i < len(masked) && masked[i] == ';' {
		return i + 1
	}
	return i
}
