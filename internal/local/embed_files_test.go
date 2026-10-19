package local

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEmbedCommandCleansTemp(t *testing.T) {
	for _, mode := range []string{"success", "failure", "start-failure"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(key, root)
			}
			t.Setenv("SEARCHIFY_EMBED_BACKEND", "process")
			t.Setenv("SEARCHIFY_SKIP_EMBED", "1")
			unrelated := filepath.Join(root, "kjarni-unrelated")
			if err := os.Mkdir(unrelated, 0o700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "worker-temp")
			var previous string
			for range 2 {
				cmd := exec.Command(os.Args[0], "-test.run=^TestEmbedWorkerProcess$", "--", mode, marker)
				if mode == "start-failure" {
					cmd = exec.Command(filepath.Join(root, "missing-executable"))
				}
				err := runEmbedCommand(cmd)
				if mode == "success" && err != nil {
					t.Fatal(err)
				}
				if mode != "success" && err == nil {
					t.Fatal("expected worker error")
				}
				if mode == "failure" {
					var exitErr *exec.ExitError
					if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
						t.Fatalf("worker exit error not preserved: %v", err)
					}
				}
				if mode != "start-failure" {
					data, err := os.ReadFile(marker)
					if err != nil {
						t.Fatal(err)
					}
					workerTemp := string(data)
					if filepath.Dir(workerTemp) != root || !strings.HasPrefix(filepath.Base(workerTemp), "searchify-embed-") {
						t.Fatalf("worker temp directory not isolated under %q: %q", root, workerTemp)
					}
					if workerTemp == previous {
						t.Fatalf("worker temp directory reused: %q", workerTemp)
					}
					previous = workerTemp
					if _, err := os.Stat(workerTemp); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("worker temp directory remains: %v", err)
					}
				}
				entries, err := os.ReadDir(root)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 1 || entries[0].Name() != "kjarni-unrelated" {
					t.Fatalf("expected only unrelated directory to remain: %v", entries)
				}
			}
		})
	}
}

func TestRunEmbedCommandTempCreationFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, missing)
	}
	err := runEmbedCommand(exec.Command("must-not-run"))
	if err == nil || !strings.Contains(err.Error(), "create embed worker temp directory") {
		t.Fatalf("expected temp directory creation error, got %v", err)
	}
}

func TestEmbedWorkerProcess(t *testing.T) {
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) != 2 {
		return
	}
	tmp := os.TempDir()
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		if os.Getenv(key) != tmp {
			os.Exit(2)
		}
	}
	if os.Getenv("SEARCHIFY_EMBED_BACKEND") != "onnx" || os.Getenv("SEARCHIFY_SKIP_EMBED") != "0" {
		os.Exit(3)
	}
	libDir, err := os.MkdirTemp("", "kjarni-")
	if err != nil {
		os.Exit(4)
	}
	if err := os.WriteFile(filepath.Join(libDir, "libkjarni_ffi.so"), []byte("test library"), 0o600); err != nil {
		os.Exit(5)
	}
	if err := os.WriteFile(args[1], []byte(tmp), 0o600); err != nil {
		os.Exit(6)
	}
	if args[0] == "failure" {
		os.Exit(7)
	}
	os.Exit(0)
}
