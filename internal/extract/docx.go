package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type docxExtractor struct{}

func (docxExtractor) Extensions() []string { return []string{".docx"} }

func (docxExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = path
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	text, err := extractDocxXML(ctx, data, "word/document.xml")
	if err != nil {
		return "", nil, err
	}
	return text, nil, nil
}

type pptxExtractor struct{}

func (pptxExtractor) Extensions() []string { return []string{".pptx"} }

func (pptxExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = path
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, fmt.Errorf("pptx zip: %w", err)
	}

	type slide struct {
		n    int
		file *zip.File
	}
	var slides []slide
	for _, f := range zr.File {
		n, ok := pptxSlideNum(f.Name)
		if !ok {
			continue
		}
		slides = append(slides, slide{n: n, file: f})
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].n < slides[j].n })

	parts := make([]string, 0, len(slides))
	for _, s := range slides {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		rc, err := s.file.Open()
		if err != nil {
			continue
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}
		parts = append(parts, strings.TrimSpace(stripOOXMLText(body)))
	}

	text := strings.Join(parts, "\f")
	if strings.TrimSpace(strings.ReplaceAll(text, "\f", "")) == "" {
		return "", nil, fmt.Errorf("pptx: no slide text found")
	}
	return text, nil, nil
}

func pptxSlideNum(name string) (int, bool) {
	const prefix = "ppt/slides/slide"
	const suffix = ".xml"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return 0, false
	}
	mid := name[len(prefix) : len(name)-len(suffix)]
	if mid == "" {
		return 0, false
	}
	n, err := strconv.Atoi(mid)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

type odfExtractor struct{}

func (odfExtractor) Extensions() []string { return []string{".odt", ".ods", ".odp"} }

func (odfExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".odp") {
		text, err := extractODP(ctx, data)
		if err != nil {
			return "", nil, fmt.Errorf("odp: %w", err)
		}
		return text, nil, nil
	}
	text, err := extractDocxXML(ctx, data, "content.xml")
	if err != nil {
		return "", nil, fmt.Errorf("odf: %w", err)
	}
	return text, nil, nil
}

func extractODP(ctx context.Context, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("zip: %w", err)
	}
	var raw []byte
	for _, f := range zr.File {
		if f.Name == "content.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			raw, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}
	if raw == nil {
		return "", fmt.Errorf("missing content.xml")
	}

	parts, err := splitODPPages(raw)
	if err != nil {
		return "", err
	}
	text := strings.Join(parts, "\f")
	if strings.TrimSpace(strings.ReplaceAll(text, "\f", "")) == "" {
		return "", fmt.Errorf("no slide text found")
	}
	return text, nil
}

// splitODPPages returns one text part per draw:page (presentation slide).
func splitODPPages(data []byte) ([]string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var parts []string
	var b strings.Builder
	inPage := false
	pageDepth := 0

	flush := func() {
		if !inPage {
			return
		}
		parts = append(parts, strings.TrimSpace(collapseBlankLines(b.String())))
		b.Reset()
	}

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if isODFDrawPage(t.Name) {
				if inPage {
					flush()
				}
				inPage = true
				pageDepth = 1
				continue
			}
			if inPage {
				pageDepth++
				switch t.Name.Local {
				case "p", "h", "br", "cr", "tab", "line-break":
					b.WriteByte('\n')
				}
			}
		case xml.EndElement:
			if !inPage {
				continue
			}
			pageDepth--
			if isODFDrawPage(t.Name) || pageDepth <= 0 {
				flush()
				inPage = false
				pageDepth = 0
			}
		case xml.CharData:
			if inPage {
				b.Write(t)
			}
		}
	}
	flush()
	if len(parts) == 0 {
		// Fallback: flat text (malformed / unexpected ODP layout).
		t := strings.TrimSpace(stripOOXMLText(data))
		if t == "" {
			return nil, fmt.Errorf("no draw:page content")
		}
		return []string{t}, nil
	}
	return parts, nil
}

func isODFDrawPage(name xml.Name) bool {
	if name.Local != "page" {
		return false
	}
	space := strings.ToLower(name.Space)
	return strings.Contains(space, "drawing") || strings.HasSuffix(space, "/draw") || space == "draw"
}

func extractDocxXML(ctx context.Context, data []byte, entry string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("office zip: %w", err)
	}
	var raw []byte
	for _, f := range zr.File {
		if f.Name == entry {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			raw, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}
	if raw == nil {
		return "", fmt.Errorf("missing %s", entry)
	}
	text := strings.TrimSpace(stripOOXMLText(raw))
	if text == "" {
		return "", fmt.Errorf("no text in %s", entry)
	}
	return text, nil
}

// stripOOXMLText collects text nodes and treats paragraph/break-ish elements as newlines.
func stripOOXMLText(data []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var b strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := t.Name.Local
			switch local {
			case "p", "h", "br", "cr", "tab":
				b.WriteByte('\n')
			}
		case xml.CharData:
			b.Write(t)
		}
	}
	return collapseBlankLines(b.String())
}

func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
