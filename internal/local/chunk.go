package local

import (
	"bufio"
	"bytes"
	"strings"
)

const targetChunkBytes = 3072

type chunk struct {
	Index     int
	LineStart int
	LineEnd   int
	Text      string
}

func chunkFile(content []byte) ([]chunk, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var paragraphs []struct {
		lineStart int
		lineEnd   int
		text      string
	}

	lineNo := 0
	var paraLines []string
	paraStart := 0

	flushPara := func(endLine int) {
		if len(paraLines) == 0 {
			return
		}
		paragraphs = append(paragraphs, struct {
			lineStart int
			lineEnd   int
			text      string
		}{
			lineStart: paraStart,
			lineEnd:   endLine,
			text:      strings.Join(paraLines, "\n"),
		})
		paraLines = nil
	}

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			flushPara(lineNo - 1)
			continue
		}
		if len(paraLines) == 0 {
			paraStart = lineNo
		}
		paraLines = append(paraLines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flushPara(lineNo)

	if len(paragraphs) == 0 {
		return nil, nil
	}

	var chunks []chunk
	var buf strings.Builder
	bufStart := paragraphs[0].lineStart
	bufEnd := paragraphs[0].lineEnd

	flushChunk := func() {
		text := strings.TrimSpace(buf.String())
		if text == "" {
			buf.Reset()
			return
		}
		chunks = append(chunks, chunk{
			Index:     len(chunks),
			LineStart: bufStart,
			LineEnd:   bufEnd,
			Text:      text,
		})
		buf.Reset()
	}

	for i, p := range paragraphs {
		if buf.Len() == 0 {
			bufStart = p.lineStart
		}

		sep := ""
		if buf.Len() > 0 {
			sep = "\n\n"
		}
		candidate := buf.String() + sep + p.text
		if buf.Len() > 0 && len(candidate) > targetChunkBytes {
			bufEnd = paragraphs[i-1].lineEnd
			flushChunk()
			bufStart = p.lineStart
			buf.WriteString(p.text)
			bufEnd = p.lineEnd
			continue
		}

		if buf.Len() > 0 {
			buf.WriteString(sep)
		}
		buf.WriteString(p.text)
		bufEnd = p.lineEnd

		if buf.Len() >= targetChunkBytes {
			flushChunk()
		}
	}
	flushChunk()

	return chunks, nil
}

func trimSnippet(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
