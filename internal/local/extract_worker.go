package local

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spektr/searchify/internal/extract"
)

// extractWorkerResult is the JSON contract for `searchify extract --file`.
type extractWorkerResult struct {
	OK    bool     `json:"ok"`
	Text  string   `json:"text,omitempty"`
	Warn  []string `json:"warn,omitempty"`
	Error string   `json:"error,omitempty"`
	Skip  bool     `json:"skip,omitempty"`
}

// ExtractFileText runs registry extract in-process (used by CLI extract worker).
func (s *Service) ExtractFileText(ctx context.Context, path string) (string, []string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	return s.extractFileInProcess(ctx, path, info.Size())
}

func (s *Service) extractFile(ctx context.Context, path string, size int64) (string, []string, error) {
	if s.cfg.ExtractInProcess || extract.IsPassthrough(path) {
		return s.extractFileInProcess(ctx, path, size)
	}
	if s.spawnExtractForTest != nil {
		return s.spawnExtractForTest(path)
	}
	return s.spawnExtractFile(ctx, path)
}

func (s *Service) extractFileInProcess(ctx context.Context, path string, size int64) (string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	return s.extract.Extract(ctx, path, f, size)
}

func (s *Service) spawnExtractFile(ctx context.Context, path string) (string, []string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", nil, fmt.Errorf("resolve executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, exe, "extract", "--file", path)
	cmd.Env = environWith(map[string]string{
		// Child must extract in-process; otherwise it would spawn forever.
		"SEARCHIFY_EXTRACT_INPROCESS": "1",
		"SEARCHIFY_SKIP_EMBED":        "1",
		"SEARCHIFY_TEXT_ONLY":         "0",
	})
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			// Parent killed the worker on SEARCHIFY_EXTRACT_TIMEOUT — skip file, keep indexing.
			return "", nil, extract.Skip("extract timed out or cancelled")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		// Prefer JSON body even on non-zero exit.
		if res, parseErr := parseExtractWorkerJSON(stdout.Bytes()); parseErr == nil {
			return decodeExtractWorkerResult(res)
		}
		return "", nil, fmt.Errorf("extract worker: %s", msg)
	}
	res, err := parseExtractWorkerJSON(stdout.Bytes())
	if err != nil {
		tail := strings.TrimSpace(stderr.String())
		if tail != "" {
			return "", nil, fmt.Errorf("extract worker bad JSON: %w (%s)", err, tail)
		}
		return "", nil, fmt.Errorf("extract worker bad JSON: %w", err)
	}
	return decodeExtractWorkerResult(res)
}

func parseExtractWorkerJSON(raw []byte) (extractWorkerResult, error) {
	raw = bytes.TrimSpace(raw)
	var res extractWorkerResult
	if len(raw) == 0 {
		return res, fmt.Errorf("empty stdout")
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return res, err
	}
	return res, nil
}

func decodeExtractWorkerResult(res extractWorkerResult) (string, []string, error) {
	if res.Skip {
		msg := res.Error
		if msg == "" {
			msg = "skipped"
		}
		return "", res.Warn, extract.Skip("%s", msg)
	}
	if !res.OK {
		if res.Error == "" {
			res.Error = "extract failed"
		}
		return "", res.Warn, errors.New(res.Error)
	}
	return res.Text, res.Warn, nil
}

// ExtractTimeoutOrDefault returns configured extract deadline.
func (s *Service) ExtractTimeoutOrDefault() time.Duration {
	if s.cfg.ExtractTimeout > 0 {
		return s.cfg.ExtractTimeout
	}
	return 30 * time.Second
}
