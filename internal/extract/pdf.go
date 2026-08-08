package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

	var warn []string

	// Prefer Poppler when available: killable via CommandContext, avoids
	// ledongthuc hangs/OOM on pathological PDFs (e.g. some corporate decks).
	if path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			text, w, err := extractPDFViaPdftotext(ctx, path)
			if err == nil {
				warn = append(warn, w...)
				text = strings.TrimSpace(text)
				if text != "" {
					return text, warn, nil
				}
				warn = append(warn, "pdftotext returned empty")
				if e.opts.OCREnabled {
					data, rerr := io.ReadAll(r)
					if rerr != nil {
						return "", warn, rerr
					}
					return e.ocrFallback(ctx, path, data, warn, fmt.Errorf("pdftotext empty"))
				}
				return "", warn, Skip("pdf has no extractable text (scanned?); set SEARCHIFY_OCR=1 for OCR fallback")
			}
			if !isMissingExec(err) {
				warn = append(warn, fmt.Sprintf("pdftotext failed: %v", err))
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return "", warn, Skip("pdf extract timed out (pdftotext)")
				}
				if e.opts.OCREnabled {
					data, rerr := io.ReadAll(r)
					if rerr != nil {
						return "", warn, rerr
					}
					return e.ocrFallback(ctx, path, data, warn, err)
				}
				// Do not fall through to ledongthuc after a real pdftotext failure —
				// that library is what hangs/OOMs on the same files.
				return "", warn, fmt.Errorf("pdftotext: %w", err)
			}
			// pdftotext not installed → Go fallback below.
		}
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return "", warn, err
	}
	if err := ctx.Err(); err != nil {
		return "", warn, err
	}

	text, err := plainTextFromPDF(ctx, data)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) ||
			strings.Contains(err.Error(), "cancelled") {
			return "", warn, Skip("pdf extract timed out (go parser)")
		}
		if e.opts.OCREnabled {
			return e.ocrFallback(ctx, path, data, warn, err)
		}
		return "", warn, fmt.Errorf("pdf extract: %w", err)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		if e.opts.OCREnabled {
			return e.ocrFallback(ctx, path, data, warn, fmt.Errorf("no extractable text"))
		}
		return "", warn, Skip("pdf has no extractable text (scanned?); set SEARCHIFY_OCR=1 for OCR fallback")
	}
	return text, warn, nil
}

func (e pdfExtractor) ocrFallback(ctx context.Context, path string, data []byte, warn []string, cause error) (string, []string, error) {
	ocrText, ocrWarn, ocrErr := ocrPDFViaPoppler(ctx, path, data, e.opts.OCRLang)
	warn = append(warn, fmt.Sprintf("pdf text extract failed (%v); trying OCR", cause))
	warn = append(warn, ocrWarn...)
	if ocrErr != nil {
		return "", warn, fmt.Errorf("pdf extract failed: %v; OCR failed: %w", cause, ocrErr)
	}
	if strings.TrimSpace(ocrText) == "" {
		return "", warn, fmt.Errorf("pdf extract failed: %v; OCR returned empty", cause)
	}
	return ocrText, warn, nil
}

func extractPDFViaPdftotext(ctx context.Context, path string) (string, []string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", nil, fmt.Errorf("pdftotext not found: %w", err)
	}
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", path, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", nil, ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", nil, fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil, nil
}

func isMissingExec(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "not found in %path%") ||
		strings.Contains(msg, "pdftotext not found")
}

// plainTextFromPDF extracts text page-by-page so ctx can cancel between pages.
// A single pathological page can still block until the extract worker is killed.
func plainTextFromPDF(ctx context.Context, data []byte) (text string, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			text = ""
			err = fmt.Errorf("pdf library panic: %v", rec)
		}
	}()

	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pdf open: %w", err)
	}

	pages := reader.NumPage()
	if pages <= 0 {
		return "", nil
	}

	fonts := make(map[string]*pdf.Font)
	var buf strings.Builder
	for i := 1; i <= pages; i++ {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("pdf extract cancelled at page %d/%d: %w", i, pages, err)
		}
		p := reader.Page(i)
		if p.V.IsNull() {
			continue
		}
		for _, name := range p.Fonts() {
			if _, ok := fonts[name]; !ok {
				f := p.Font(name)
				fonts[name] = &f
			}
		}
		pageText, err := p.GetPlainText(fonts)
		if err != nil {
			return "", fmt.Errorf("pdf page %d: %w", i, err)
		}
		buf.WriteString(pageText)
		if i < pages {
			buf.WriteByte('\n')
		}
	}
	return buf.String(), nil
}
