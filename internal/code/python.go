package code

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed codeparse_python.py
var pythonASTScript []byte

// PythonAnalyzer uses a short-lived python3 AST worker.
type PythonAnalyzer struct{}

func (PythonAnalyzer) Lang() string     { return "python" }
func (PythonAnalyzer) Exts() []string   { return []string{".py"} }

func (PythonAnalyzer) Analyze(path string, src []byte) (Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runPythonAST(ctx, path, src)
}

func runPythonAST(ctx context.Context, path string, src []byte) (Result, error) {
	py, err := lookPathPython()
	if err != nil {
		return Result{}, err
	}

	dir, err := os.MkdirTemp("", "searchify-codeparse-*")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	scriptPath := filepath.Join(dir, "codeparse_python.py")
	if err := os.WriteFile(scriptPath, pythonASTScript, 0o600); err != nil {
		return Result{}, err
	}
	srcPath := filepath.Join(dir, "source.py")
	if err := os.WriteFile(srcPath, src, 0o600); err != nil {
		return Result{}, err
	}

	cmd := exec.CommandContext(ctx, py, scriptPath, srcPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("python AST timed out")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		// Prefer JSON error body on stdout.
		if res, parseErr := parseWorkerOut(stdout.Bytes()); parseErr == nil && res.err != "" {
			return Result{}, fmt.Errorf("%s", res.err)
		}
		return Result{}, fmt.Errorf("python AST worker: %s", msg)
	}
	out, err := parseWorkerOut(stdout.Bytes())
	if err != nil {
		return Result{}, err
	}
	if out.err != "" {
		return Result{}, fmt.Errorf("%s", out.err)
	}
	return out.result, nil
}

func lookPathPython() (string, error) {
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("python3 not found on PATH")
}
