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

//go:embed codeparse_csharp/codeparse_csharp.csproj
var csharpCsproj []byte

//go:embed codeparse_csharp/Program.cs
var csharpProgram []byte

// CSharpAnalyzer extracts units/symbols/refs for .cs files.
// Prefers a short-lived Roslyn worker when `dotnet` is on PATH; otherwise
// uses an in-process brace-aware heuristic (no SDK required).
type CSharpAnalyzer struct{}

func (CSharpAnalyzer) Lang() string   { return "csharp" }
func (CSharpAnalyzer) Exts() []string { return []string{".cs"} }

func (CSharpAnalyzer) Analyze(path string, src []byte) (Result, error) {
	if _, err := lookPathDotnet(); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if res, err := runCSharpRoslyn(ctx, path, src); err == nil && (len(res.Units) > 0 || len(res.Symbols) > 0) {
			return res, nil
		}
	}
	return analyzeCSharpHeuristic(path, src)
}

func lookPathDotnet() (string, error) {
	if p, err := exec.LookPath("dotnet"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("dotnet not found on PATH")
}

func runCSharpRoslyn(ctx context.Context, path string, src []byte) (Result, error) {
	dotnet, err := lookPathDotnet()
	if err != nil {
		return Result{}, err
	}

	dir, err := os.MkdirTemp("", "searchify-codeparse-cs-*")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	projDir := filepath.Join(dir, "worker")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(projDir, "codeparse_csharp.csproj"), csharpCsproj, 0o600); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(projDir, "Program.cs"), csharpProgram, 0o600); err != nil {
		return Result{}, err
	}
	srcPath := filepath.Join(dir, "source.cs")
	if err := os.WriteFile(srcPath, src, 0o600); err != nil {
		return Result{}, err
	}

	cmd := exec.CommandContext(ctx, dotnet, "run", "--project", projDir, "-c", "Release", "--nologo", "--", srcPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "DOTNET_CLI_TELEMETRY_OPTOUT=1", "DOTNET_NOLOGO=1")
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("csharp Roslyn timed out")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if res, parseErr := parseWorkerOut(stdout.Bytes()); parseErr == nil && res.err != "" {
			return Result{}, fmt.Errorf("%s", res.err)
		}
		return Result{}, fmt.Errorf("csharp Roslyn worker: %s", msg)
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

func init() {
	Register(CSharpAnalyzer{})
}
