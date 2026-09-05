package local

import (
	"bufio"
	"bytes"
	"strings"
	"unicode/utf8"
)

const (
	defaultChunkBytes   = 3072
	defaultChunkOverlap = 256
)

type chunk struct {
	Index     int
	LineStart int
	LineEnd   int
	PageStart int // 0 = no form-feed page markers; else 1-based PDF page
	Symbol    string
	Kind      string
	Text      string
}

// ChunkParams controls packing of extracted text into retrieval chunks.
type ChunkParams struct {
	TargetBytes  int // soft max chunk size in bytes (default 3072)
	OverlapBytes int // suffix of prior chunk prepended to the next (default 256)
}

func normalizeChunkParams(p ChunkParams) ChunkParams {
	if p.TargetBytes <= 0 {
		p.TargetBytes = defaultChunkBytes
	}
	if p.OverlapBytes < 0 {
		p.OverlapBytes = 0
	}
	if p.OverlapBytes >= p.TargetBytes {
		p.OverlapBytes = p.TargetBytes / 4
	}
	return p
}

type segment struct {
	lineStart int
	lineEnd   int
	pageStart int // 0 = unknown / no pages; else 1-based
	text      string
	hardStart bool // do not append onto a non-empty buffer (heading / page break)
}

func chunkFile(content []byte, params ChunkParams) ([]chunk, error) {
	params = normalizeChunkParams(params)
	segments, err := splitSegments(content)
	if err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		return nil, nil
	}

	// Expand units larger than the target into hard windows.
	var units []segment
	for _, seg := range segments {
		parts := splitOversized(seg, params.TargetBytes)
		for i := range parts {
			if i > 0 {
				parts[i].hardStart = true
			}
		}
		units = append(units, parts...)
	}

	var chunks []chunk
	var buf strings.Builder
	bufStart, bufEnd, bufPage := 0, 0, 0
	var overlapCarry string

	flush := func() {
		text := strings.TrimSpace(buf.String())
		if text == "" {
			buf.Reset()
			return
		}
		chunks = append(chunks, chunk{
			Index:     len(chunks),
			LineStart: bufStart,
			LineEnd:   bufEnd,
			PageStart: bufPage,
			Text:      text,
		})
		if params.OverlapBytes > 0 {
			overlapCarry = overlapSuffix(text, params.OverlapBytes)
		} else {
			overlapCarry = ""
		}
		buf.Reset()
	}

	for _, u := range units {
		if buf.Len() > 0 && u.hardStart {
			flush()
		}
		if buf.Len() == 0 {
			bufStart = u.lineStart
			bufPage = u.pageStart
			if overlapCarry != "" {
				buf.WriteString(overlapCarry)
				if !strings.HasSuffix(overlapCarry, "\n") {
					buf.WriteByte('\n')
				}
			}
		}

		sep := ""
		if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "\n") {
			sep = "\n\n"
		}
		candidateLen := buf.Len() + len(sep) + len(u.text)
		if buf.Len() > 0 && candidateLen > params.TargetBytes {
			flush()
			bufStart = u.lineStart
			bufPage = u.pageStart
			if overlapCarry != "" {
				buf.WriteString(overlapCarry)
				if !strings.HasSuffix(overlapCarry, "\n") {
					buf.WriteByte('\n')
				}
			}
		}

		if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "\n") {
			buf.WriteString("\n\n")
		}
		buf.WriteString(u.text)
		bufEnd = u.lineEnd

		if buf.Len() >= params.TargetBytes {
			flush()
		}
	}
	flush()

	return chunks, nil
}

func splitSegments(content []byte) ([]segment, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	hasPages := bytes.Contains(content, []byte{'\f'})
	page := 1
	segPage := func() int {
		if hasPages {
			return page
		}
		return 0
	}

	var segments []segment
	lineNo := 0
	var lines []string
	segStart := 0
	segPageStart := 0

	flush := func(endLine int, hardStart bool) {
		if len(lines) == 0 {
			return
		}
		text := strings.TrimRight(strings.Join(lines, "\n"), "\n")
		if strings.TrimSpace(text) == "" {
			lines = nil
			return
		}
		segments = append(segments, segment{
			lineStart: segStart,
			lineEnd:   endLine,
			pageStart: segPageStart,
			text:      text,
			hardStart: hardStart,
		})
		lines = nil
	}

	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()

		// Form-feed (common PDF page break from pdftotext): hard boundary.
		if strings.ContainsRune(raw, '\f') {
			parts := strings.Split(raw, "\f")
			for i, part := range parts {
				if i > 0 {
					flush(lineNo-1, false)
					page++
				}
				part = strings.TrimRight(part, "\r")
				if strings.TrimSpace(part) == "" {
					continue
				}
				if len(lines) == 0 {
					segStart = lineNo
					segPageStart = segPage()
				}
				lines = append(lines, part)
				if i > 0 {
					flush(lineNo, true)
				}
			}
			continue
		}

		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			flush(lineNo-1, false)
			continue
		}

		// Markdown ATX heading starts a new hard segment (heading + following body pack until next hard start).
		if isMarkdownHeading(raw) {
			flush(lineNo-1, false)
			segStart = lineNo
			segPageStart = segPage()
			lines = []string{raw}
			// Keep heading open so following body lines join this segment until blank/heading/page.
			continue
		}

		if len(lines) == 0 {
			segStart = lineNo
			segPageStart = segPage()
		}
		lines = append(lines, raw)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush(lineNo, false)

	// Mark heading-led segments as hard starts (except the first).
	for i := range segments {
		if i == 0 {
			continue
		}
		firstLine, _, _ := strings.Cut(segments[i].text, "\n")
		if isMarkdownHeading(firstLine) {
			segments[i].hardStart = true
		}
	}
	return segments, nil
}

func isMarkdownHeading(line string) bool {
	s := strings.TrimLeft(line, " \t")
	if s == "" || s[0] != '#' {
		return false
	}
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return false
	}
	if n == len(s) {
		return true
	}
	return s[n] == ' ' || s[n] == '\t'
}

func splitOversized(seg segment, target int) []segment {
	if len(seg.text) <= target {
		return []segment{seg}
	}
	var out []segment
	text := seg.text
	first := true
	for len(text) > 0 {
		n := target
		if n > len(text) {
			n = len(text)
		}
		// Prefer breaking at newline within the window.
		if n < len(text) {
			if i := strings.LastIndex(text[:n], "\n"); i > target/4 {
				n = i + 1
			} else {
				// Avoid splitting a UTF-8 rune.
				for n > 0 && !utf8.RuneStart(text[n]) {
					n--
				}
				if n == 0 {
					_, size := utf8.DecodeRuneInString(text)
					n = size
				}
			}
		}
		piece := strings.TrimSpace(text[:n])
		text = text[n:]
		if piece == "" {
			continue
		}
		out = append(out, segment{
			lineStart: seg.lineStart,
			lineEnd:   seg.lineEnd,
			pageStart: seg.pageStart,
			text:      piece,
			hardStart: seg.hardStart && first,
		})
		first = false
	}
	if len(out) == 0 {
		return []segment{seg}
	}
	return out
}

func overlapSuffix(text string, n int) string {
	if n <= 0 || text == "" {
		return ""
	}
	if len(text) <= n {
		return text
	}
	start := len(text) - n
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}
	// Prefer starting at a word/line boundary inside the suffix window.
	suf := text[start:]
	if i := strings.IndexAny(suf, " \n\t"); i >= 0 && i+1 < len(suf) {
		suf = suf[i+1:]
	}
	return strings.TrimSpace(suf)
}

func trimSnippet(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
