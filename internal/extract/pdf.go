package extract

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

type pdfExtractor struct {
	opts Options
}

func (pdfExtractor) Extensions() []string { return []string{".pdf"} }

func (e pdfExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}

	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, fmt.Errorf("pdf open: %w", err)
	}

	plain, err := reader.GetPlainText()
	if err != nil {
		return "", nil, fmt.Errorf("pdf text: %w", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(plain); err != nil {
		return "", nil, fmt.Errorf("pdf read: %w", err)
	}
	text := strings.TrimSpace(buf.String())
	var warn []string

	if text == "" {
		if e.opts.OCREnabled {
			ocrText, ocrWarn, ocrErr := ocrPDFViaPoppler(ctx, path, data, e.opts.OCRLang)
			warn = append(warn, ocrWarn...)
			if ocrErr != nil {
				return "", warn, fmt.Errorf("pdf has no extractable text; OCR failed: %w (install tesseract + poppler-utils, or disable SEARCHIFY_OCR)", ocrErr)
			}
			if strings.TrimSpace(ocrText) == "" {
				return "", warn, fmt.Errorf("pdf has no extractable text (scanned?); OCR returned empty — check SEARCHIFY_OCR_LANG / tesseract")
			}
			return ocrText, warn, nil
		}
		return "", nil, Skip("pdf has no extractable text (scanned?); set SEARCHIFY_OCR=1 for OCR fallback")
	}
	return text, warn, nil
}
