package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
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
	var parts []string
	for _, f := range zr.File {
		name := f.Name
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}
		t := stripOOXMLText(body)
		if strings.TrimSpace(t) != "" {
			parts = append(parts, t)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if text == "" {
		return "", nil, fmt.Errorf("pptx: no slide text found")
	}
	return text, nil, nil
}

type odfExtractor struct{}

func (odfExtractor) Extensions() []string { return []string{".odt", ".ods", ".odp"} }

func (odfExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = path
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	text, err := extractDocxXML(ctx, data, "content.xml")
	if err != nil {
		return "", nil, fmt.Errorf("odf: %w", err)
	}
	return text, nil, nil
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
