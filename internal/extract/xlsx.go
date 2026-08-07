package extract

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

type xlsxExtractor struct{}

func (xlsxExtractor) Extensions() []string { return []string{".xlsx"} }

func (xlsxExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = path
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", nil, fmt.Errorf("xlsx open: %w", err)
	}
	defer func() { _ = f.Close() }()

	var parts []string
	for _, sheet := range f.GetSheetList() {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		var sb strings.Builder
		sb.WriteString(sheet)
		sb.WriteByte('\n')
		for _, row := range rows {
			sb.WriteString(strings.Join(row, "\t"))
			sb.WriteByte('\n')
		}
		parts = append(parts, sb.String())
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		return "", nil, fmt.Errorf("xlsx: no cell text found")
	}
	return text, nil, nil
}

type csvExtractor struct{}

func (csvExtractor) Extensions() []string { return []string{".csv"} }

func (csvExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = path
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	// Prefer newline-preserving UTF-8 text; lightly normalize via csv for robustness.
	cr := csv.NewReader(bytes.NewReader(data))
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	var sb strings.Builder
	for {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Fall back to raw bytes (still searchable).
			return string(data), []string{"csv parse soft-fail; indexed as raw text"}, nil
		}
		sb.WriteString(strings.Join(rec, "\t"))
		sb.WriteByte('\n')
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return string(data), nil, nil
	}
	return text, nil, nil
}
