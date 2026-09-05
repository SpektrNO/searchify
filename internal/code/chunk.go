package code

import (
	"strings"
	"unicode/utf8"
)

// ChunkFromUnits builds retrieval chunks from analyzer units + source bytes.
// Oversized units are hard-split. Empty symbol for module preamble.
func ChunkFromUnits(src []byte, units []Unit, targetBytes, overlapBytes int) []CodeChunk {
	if targetBytes <= 0 {
		targetBytes = 3072
	}
	if overlapBytes < 0 {
		overlapBytes = 0
	}
	if overlapBytes >= targetBytes {
		overlapBytes = targetBytes / 4
	}
	if len(units) == 0 {
		return nil
	}

	var out []CodeChunk
	for _, u := range units {
		bs, be := u.ByteStart, u.ByteEnd
		if bs < 0 {
			bs = 0
		}
		if be > len(src) {
			be = len(src)
		}
		if be <= bs {
			continue
		}
		text := strings.TrimSpace(string(src[bs:be]))
		if text == "" {
			continue
		}
		sym := u.QualName
		if sym == "" {
			sym = u.Name
		}
		parts := splitTextWindows(text, targetBytes)
		for i, p := range parts {
			ls, le := u.LineStart, u.LineEnd
			if i > 0 {
				ls = u.LineStart // approximate; keep start for title
			}
			out = append(out, CodeChunk{
				Text:      p,
				LineStart: ls,
				LineEnd:   le,
				Symbol:    sym,
				Kind:      u.Kind,
			})
		}
	}
	_ = overlapBytes // reserved; code units are hard boundaries without overlap for v1
	return out
}

// CodeChunk is a retrieval unit produced from code analysis.
type CodeChunk struct {
	Text      string
	LineStart int
	LineEnd   int
	Symbol    string
	Kind      string
}

func splitTextWindows(text string, target int) []string {
	if len(text) <= target {
		return []string{text}
	}
	var out []string
	for len(text) > 0 {
		n := target
		if n > len(text) {
			n = len(text)
		}
		if n < len(text) {
			if i := strings.LastIndex(text[:n], "\n"); i > target/4 {
				n = i + 1
			} else {
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
		if piece != "" {
			out = append(out, piece)
		}
	}
	return out
}
