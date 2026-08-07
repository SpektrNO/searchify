package extract

import (
	"context"
	"fmt"
	"io"
	"net/mail"
	"strings"
)

type rtfExtractor struct{}

func (rtfExtractor) Extensions() []string { return []string{".rtf"} }

func (rtfExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = path
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	text := strings.TrimSpace(stripRTF(string(data)))
	if text == "" {
		return "", nil, fmt.Errorf("rtf: no text found")
	}
	return text, nil, nil
}

// stripRTF is a minimal control-word stripper — good enough for plain legacy docs.
func stripRTF(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		switch s[i] {
		case '\\':
			if i+1 >= len(s) {
				i++
				continue
			}
			next := s[i+1]
			if next == '\\' || next == '{' || next == '}' {
				b.WriteByte(next)
				i += 2
				continue
			}
			if next == '\'' && i+3 < len(s) {
				// hex escaped char \'hh — skip for simplicity
				i += 4
				continue
			}
			// skip control word
			i += 2
			for i < len(s) && ((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z')) {
				i++
			}
			if i < len(s) && (s[i] == '-' || (s[i] >= '0' && s[i] <= '9')) {
				if s[i] == '-' {
					i++
				}
				for i < len(s) && s[i] >= '0' && s[i] <= '9' {
					i++
				}
			}
			if i < len(s) && s[i] == ' ' {
				i++
			}
		case '{', '}':
			i++
		case '\r':
			i++
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return collapseBlankLines(b.String())
}

type emlExtractor struct{}

func (emlExtractor) Extensions() []string { return []string{".eml"} }

func (emlExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = path
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return "", nil, fmt.Errorf("eml parse: %w", err)
	}
	var b strings.Builder
	if subj := msg.Header.Get("Subject"); subj != "" {
		b.WriteString(subj)
		b.WriteByte('\n')
	}
	if from := msg.Header.Get("From"); from != "" {
		b.WriteString("From: ")
		b.WriteString(from)
		b.WriteByte('\n')
	}
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return "", nil, err
	}
	b.Write(body)
	text := strings.TrimSpace(b.String())
	if text == "" {
		return "", nil, fmt.Errorf("eml: empty message")
	}
	return text, nil, nil
}
