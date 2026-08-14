package extract

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type imageExtractor struct {
	opts Options
}

func (imageExtractor) Extensions() []string {
	return []string{".png", ".jpg", ".jpeg", ".webp", ".tif", ".tiff", ".gif"}
}

func (e imageExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = size
	if !e.opts.OCREnabled {
		return "", nil, Skip("OCR disabled; set SEARCHIFY_OCR=1 (requires tesseract on PATH)")
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	// Prefer path on disk when available (indexing opens the real file).
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			text, warn, err := runTesseract(ctx, path, e.opts.OCRLang)
			return text, warn, err
		}
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	tmp, err := os.CreateTemp("", "searchify-ocr-*"+filepath.Ext(path))
	if err != nil {
		return "", nil, err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		return "", nil, err
	}
	return runTesseract(ctx, tmpPath, e.opts.OCRLang)
}

func runTesseract(ctx context.Context, imagePath, lang string) (string, []string, error) {
	if lang == "" {
		lang = "eng"
	}
	if _, err := exec.LookPath("tesseract"); err != nil {
		return "", []string{"tesseract not found on PATH"}, fmt.Errorf("tesseract not available: %w", err)
	}
	cmd := exec.CommandContext(ctx, "tesseract", imagePath, "stdout", "-l", lang)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", nil, fmt.Errorf("tesseract: %s", msg)
	}
	text := strings.TrimSpace(stdout.String())
	if text == "" {
		return "", nil, fmt.Errorf("tesseract returned empty text")
	}
	return text, nil, nil
}

// ocrPDFViaPoppler renders PDF pages with pdftoppm then runs tesseract.
func ocrPDFViaPoppler(ctx context.Context, path string, data []byte, lang string) (string, []string, error) {
	var warn []string
	if _, err := exec.LookPath("tesseract"); err != nil {
		return "", []string{"tesseract not found on PATH"}, fmt.Errorf("tesseract not available: %w", err)
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return "", []string{"pdftoppm not found on PATH (poppler-utils)"}, fmt.Errorf("pdftoppm not available for PDF OCR: %w", err)
	}

	dir, err := os.MkdirTemp("", "searchify-pdf-ocr-*")
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	pdfPath := path
	if pdfPath == "" {
		pdfPath = filepath.Join(dir, "in.pdf")
		if err := os.WriteFile(pdfPath, data, 0o600); err != nil {
			return "", nil, err
		}
	} else if _, err := os.Stat(path); err != nil {
		pdfPath = filepath.Join(dir, "in.pdf")
		if err := os.WriteFile(pdfPath, data, 0o600); err != nil {
			return "", nil, err
		}
	}

	prefix := filepath.Join(dir, "page")
	cmd := exec.CommandContext(ctx, "pdftoppm", "-png", "-r", "150", pdfPath, prefix)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", warn, fmt.Errorf("pdftoppm: %s", msg)
	}

	entries, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return "", warn, err
	}
	if len(entries) == 0 {
		return "", warn, fmt.Errorf("pdftoppm produced no pages")
	}

	var parts []string
	for _, img := range entries {
		if err := ctx.Err(); err != nil {
			return "", warn, err
		}
		text, w, err := runTesseract(ctx, img, lang)
		warn = append(warn, w...)
		if err != nil {
			warn = append(warn, fmt.Sprintf("%s: %v", filepath.Base(img), err))
			continue
		}
		parts = append(parts, text)
	}
	joined := strings.TrimSpace(strings.Join(parts, "\f"))
	if joined == "" {
		return "", warn, fmt.Errorf("PDF OCR produced no text")
	}
	return joined, warn, nil
}
