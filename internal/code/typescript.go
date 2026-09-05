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

//go:embed codeparse_typescript.mjs
var typescriptASTScript []byte

// TSAnalyzer uses a short-lived Node worker for TS/JS.
type TSAnalyzer struct{}

func (TSAnalyzer) Lang() string { return "typescript" }
func (TSAnalyzer) Exts() []string {
	return []string{".ts", ".tsx", ".js", ".jsx"}
}

func (TSAnalyzer) Analyze(path string, src []byte) (Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runTypeScriptAST(ctx, path, src)
}

func runTypeScriptAST(ctx context.Context, path string, src []byte) (Result, error) {
	node, err := lookPathNode()
	if err != nil {
		return Result{}, err
	}

	dir, err := os.MkdirTemp("", "searchify-codeparse-ts-*")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	scriptPath := filepath.Join(dir, "codeparse_typescript.mjs")
	if err := os.WriteFile(scriptPath, typescriptASTScript, 0o600); err != nil {
		return Result{}, err
	}
	ext := extOf(path)
	if ext == "" {
		ext = ".ts"
	}
	srcPath := filepath.Join(dir, "source"+ext)
	if err := os.WriteFile(srcPath, src, 0o600); err != nil {
		return Result{}, err
	}

	// Pass original path so the worker can walk-up for node_modules/typescript.
	cmd := exec.CommandContext(ctx, node, scriptPath, srcPath, path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("typescript AST timed out")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if res, parseErr := parseWorkerOut(stdout.Bytes()); parseErr == nil && res.err != "" {
			return Result{}, fmt.Errorf("%s", res.err)
		}
		return Result{}, fmt.Errorf("typescript AST worker: %s", msg)
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

func lookPathNode() (string, error) {
	for _, name := range []string{"node", "nodejs"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("node not found on PATH")
}

func init() {
	Register(TSAnalyzer{})
}
