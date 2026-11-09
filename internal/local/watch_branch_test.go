package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spektr/searchify/internal/config"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=searchify",
		"GIT_AUTHOR_EMAIL=searchify@example.com",
		"GIT_COMMITTER_NAME=searchify",
		"GIT_COMMITTER_EMAIL=searchify@example.com",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func initGitRepo(t *testing.T, branch string) string {
	t.Helper()
	requireGit(t)
	root := t.TempDir()
	runGit(t, root, "init", "-b", branch)
	return root
}

func commitFile(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "--", name)
	runGit(t, root, "commit", "-m", "add "+name)
	return path
}

func seedRemoteHead(t *testing.T, root, remote, branch string) {
	t.Helper()
	runGit(t, root, "remote", "add", remote, "https://example.invalid/"+remote+".git")
	runGit(t, root, "symbolic-ref", "refs/remotes/"+remote+"/HEAD", "refs/remotes/"+remote+"/"+branch)
}

func branchWatcher(t *testing.T, roots ...string) *IndexWatcher {
	t.Helper()
	cfg := &config.Config{
		Roots: roots, WatchPaths: roots,
		IndexDir: t.TempDir(), SkipEmbed: true,
		WatchDebounce: time.Millisecond, MaxFileBytes: 1 << 20,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })
	w, err := NewIndexWatcher(cfg, svc)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func testFS(t *testing.T) *fsnotify.Watcher {
	t.Helper()
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fw.Close() })
	return fw
}

func writeMD(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestWatchIndexesOnDefaultBranch(t *testing.T) {
	root := initGitRepo(t, "main")
	commitFile(t, root, "readme.md", "hello\n")
	seedRemoteHead(t, root, "origin", "main")
	w := branchWatcher(t, root)
	path := filepath.Join(root, "notes.md")
	writeMD(t, path, "notes\n")
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	if _, ok := w.pending[path]; !ok {
		t.Fatalf("on-default write was not queued: %v", w.pending)
	}
}

func TestWatchDropsEventsOffDefaultBranch(t *testing.T) {
	root := initGitRepo(t, "main")
	commitFile(t, root, "readme.md", "hello\n")
	seedRemoteHead(t, root, "origin", "main")
	w := branchWatcher(t, root)
	path := filepath.Join(root, "notes.md")
	writeMD(t, path, "notes\n")
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	if _, ok := w.pending[path]; !ok {
		t.Fatal("expected the default branch to queue the write")
	}
	runGit(t, root, "checkout", "-b", "feature")
	w.handleEvent(fsnotify.Event{Name: filepath.Join(root, ".git", "HEAD"), Op: fsnotify.Write})
	if len(w.pending) != 0 {
		t.Fatalf("pending not dropped after leaving default: %v", w.pending)
	}
	feature := filepath.Join(root, "feature.md")
	writeMD(t, feature, "feature\n")
	w.handleEvent(fsnotify.Event{Name: feature, Op: fsnotify.Create})
	if len(w.pending) != 0 || !w.rescanDue.IsZero() {
		t.Fatalf("off-default event queued pending=%v rescan=%v", w.pending, w.rescanDue)
	}
}

func TestWatchDetachedHeadIsNotIndexed(t *testing.T) {
	root := initGitRepo(t, "main")
	commitFile(t, root, "readme.md", "hello\n")
	seedRemoteHead(t, root, "origin", "main")
	runGit(t, root, "checkout", "--detach")
	w := branchWatcher(t, root)
	path := filepath.Join(root, "detached.md")
	writeMD(t, path, "detached\n")
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	if len(w.pending) != 0 {
		t.Fatalf("detached HEAD queued work: %v", w.pending)
	}
}

func TestWatchRepoWithoutRemoteIsNotGated(t *testing.T) {
	root := initGitRepo(t, "main")
	commitFile(t, root, "readme.md", "hello\n")
	runGit(t, root, "checkout", "-b", "feature")
	w := branchWatcher(t, root)
	path := filepath.Join(root, "local.md")
	writeMD(t, path, "local\n")
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	if _, ok := w.pending[path]; !ok {
		t.Fatal("repo with no remote should keep today's watch behavior")
	}
}

func TestWatchCheckoutClearsOverflowBeforeOtherRoots(t *testing.T) {
	repo := initGitRepo(t, "main")
	commitFile(t, repo, "readme.md", "hello\n")
	seedRemoteHead(t, repo, "origin", "main")
	other := t.TempDir()
	otherFile := filepath.Join(other, "other.md")
	writeMD(t, otherFile, "other\n")
	w := branchWatcher(t, repo, other)

	for i := range maxWatchPending {
		path := filepath.Join(repo, fmt.Sprintf("%d.md", i))
		writeMD(t, path, "queued\n")
		w.schedule(path)
	}
	extra := filepath.Join(repo, "overflow.md")
	writeMD(t, extra, "overflow\n")
	w.schedule(extra)
	if w.rescanDue.IsZero() || !w.rescanFromOverflow || w.overflowRepo != repo {
		t.Fatalf("overflow rescan=%v from=%v repo=%q", w.rescanDue, w.rescanFromOverflow, w.overflowRepo)
	}
	runGit(t, repo, "checkout", "-b", "feature")
	w.handleEvent(fsnotify.Event{Name: filepath.Join(repo, ".git", "HEAD"), Op: fsnotify.Write})
	if len(w.pending) != 0 || !w.rescanDue.IsZero() || w.rescanFromOverflow {
		t.Fatalf("checkout left recovery pending=%d rescan=%v overflow=%v", len(w.pending), w.rescanDue, w.rescanFromOverflow)
	}
	w.runRescan(context.Background(), nil)
	files, err := w.svc.ListIndexedFiles("")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("cancelled recovery indexed %v", files)
	}
}

func TestWatchReturnToDefaultRescansThatRepoOnly(t *testing.T) {
	repo := initGitRepo(t, "main")
	commitFile(t, repo, "readme.md", "hello\n")
	seedRemoteHead(t, repo, "origin", "main")
	runGit(t, repo, "checkout", "-b", "feature")
	repoFile := filepath.Join(repo, "back.md")
	writeMD(t, repoFile, "back on main\n")
	other := t.TempDir()
	otherFile := filepath.Join(other, "other.md")
	writeMD(t, otherFile, "leave me\n")
	w := branchWatcher(t, repo, other)
	w.handleEvent(fsnotify.Event{Name: repoFile, Op: fsnotify.Write})
	if len(w.pending) != 0 || !w.rescanDue.IsZero() {
		t.Fatal("feature branch should not schedule work")
	}
	runGit(t, repo, "checkout", "main")
	w.handleEvent(fsnotify.Event{Name: filepath.Join(repo, ".git", "HEAD"), Op: fsnotify.Write})
	if w.rescanAll || w.rescanFromOverflow {
		t.Fatal("returning to the default branch scheduled a full recovery")
	}
	if _, ok := w.rescanRepos[repo]; !ok || w.rescanDue.IsZero() {
		t.Fatalf("repo rescan repos=%v due=%v", w.rescanRepos, w.rescanDue)
	}
	w.mu.Lock()
	w.rescanDue = time.Now().Add(-time.Second)
	w.mu.Unlock()
	fw := testFS(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.work(ctx, fw)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		files, err := w.svc.ListIndexedFiles("")
		if err != nil {
			t.Fatal(err)
		}
		var sawRepo, sawOther bool
		for _, file := range files {
			if file == repoFile {
				sawRepo = true
			}
			if file == otherFile {
				sawOther = true
			}
		}
		if sawRepo && !sawOther {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("repo-only rescan files=%v", files)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestWatchUnresolvedDefaultBranchIsNotGated(t *testing.T) {
	root := initGitRepo(t, "main")
	commitFile(t, root, "readme.md", "hello\n")
	runGit(t, root, "remote", "add", "origin", "https://example.invalid/repo.git")
	runGit(t, root, "checkout", "-b", "feature")
	w := branchWatcher(t, root)
	var setHeads int
	real := w.gate.run
	w.gate.run = func(ctx context.Context, dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "remote" && args[1] == "set-head" {
			setHeads++
			return "", errors.New("unreachable")
		}
		return real(ctx, dir, args...)
	}
	path := filepath.Join(root, "feature.md")
	writeMD(t, path, "still indexed\n")
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	if _, ok := w.pending[path]; !ok {
		t.Fatal("unresolved default branch should leave the repo ungated")
	}
	if setHeads != 1 {
		t.Fatalf("set-head calls=%d, want 1", setHeads)
	}
}

func TestWatchUsesUpstreamRemoteNotOrigin(t *testing.T) {
	root := initGitRepo(t, "main")
	commitFile(t, root, "readme.md", "hello\n")
	seedRemoteHead(t, root, "origin", "main")
	seedRemoteHead(t, root, "upstream", "develop")
	runGit(t, root, "update-ref", "refs/remotes/upstream/main", "HEAD")
	runGit(t, root, "branch", "--set-upstream-to", "upstream/main")
	w := branchWatcher(t, root)
	path := filepath.Join(root, "notes.md")
	writeMD(t, path, "notes\n")
	// upstream HEAD is develop, so main is not the default branch.
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	if len(w.pending) != 0 {
		t.Fatal("origin/HEAD was used instead of the upstream remote")
	}
	decision := w.gate.consider(root)
	repo := w.gate.repos[decision.repo]
	if repo == nil || repo.remote != "upstream" || repo.defaultBranch != "develop" {
		t.Fatalf("remote=%v branch=%v", repo, decision)
	}
}

func TestWatchSoleRemoteIsDefaultWhenNotOrigin(t *testing.T) {
	root := initGitRepo(t, "trunk")
	commitFile(t, root, "readme.md", "hello\n")
	seedRemoteHead(t, root, "upstream", "trunk")
	w := branchWatcher(t, root)
	path := filepath.Join(root, "notes.md")
	writeMD(t, path, "notes\n")
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	if _, ok := w.pending[path]; !ok {
		t.Fatal("sole remote default branch was not indexed")
	}
}

func TestWatchSetHeadOnceThenCaches(t *testing.T) {
	root := initGitRepo(t, "main")
	commitFile(t, root, "readme.md", "hello\n")
	runGit(t, root, "remote", "add", "origin", "https://example.invalid/repo.git")
	w := branchWatcher(t, root)
	var setHeads int
	real := w.gate.run
	w.gate.run = func(ctx context.Context, dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "remote" && args[1] == "set-head" {
			setHeads++
			if err := exec.Command("git", "-C", dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main").Run(); err != nil {
				return "", err
			}
			return "", nil
		}
		return real(ctx, dir, args...)
	}
	path := filepath.Join(root, "notes.md")
	writeMD(t, path, "notes\n")
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
	runGit(t, root, "checkout", "-b", "feature")
	w.handleEvent(fsnotify.Event{Name: filepath.Join(root, ".git", "HEAD"), Op: fsnotify.Write})
	if setHeads != 1 {
		t.Fatalf("set-head calls=%d, want 1", setHeads)
	}
	if _, ok := w.pending[path]; ok {
		t.Fatal("cached default branch was not applied after checkout")
	}
}

func TestWatchNestedOffBranchRepoSkippedOnParentRescan(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	requireGit(t)
	runGit(t, repo, "init", "-b", "main")
	commitFile(t, repo, "readme.md", "hello\n")
	seedRemoteHead(t, repo, "origin", "main")
	runGit(t, repo, "checkout", "-b", "feature")
	repoFile := filepath.Join(repo, "secret.md")
	writeMD(t, repoFile, "do not index\n")
	notes := filepath.Join(parent, "notes.md")
	writeMD(t, notes, "index me\n")
	w := branchWatcher(t, parent)
	w.requestRescan()
	w.mu.Lock()
	w.rescanDue = time.Now().Add(-time.Second)
	w.mu.Unlock()
	w.runRescan(context.Background(), testFS(t))
	files, err := w.svc.ListIndexedFiles("")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != notes {
		t.Fatalf("files=%v", files)
	}
}

func TestManualIndexIgnoresDefaultBranchGate(t *testing.T) {
	root := initGitRepo(t, "main")
	commitFile(t, root, "readme.md", "hello\n")
	seedRemoteHead(t, root, "origin", "main")
	runGit(t, root, "checkout", "-b", "feature")
	path := filepath.Join(root, "manual.md")
	writeMD(t, path, "manual index\n")
	w := branchWatcher(t, root)
	if _, err := w.svc.IndexPaths([]string{path}, false); err != nil {
		t.Fatal(err)
	}
	n, err := w.svc.DocumentCount()
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func TestWorktreeGitdirFile(t *testing.T) {
	gitdir := t.TempDir()
	head := filepath.Join(gitdir, "HEAD")
	if err := os.WriteFile(head, []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gotRoot, gotHead, ok := worktreeAt(root)
	if !ok || gotRoot != root || gotHead != head {
		t.Fatalf("root=%q head=%q ok=%v", gotRoot, gotHead, ok)
	}
	on, branch, detached, err := readCheckedOut(head, "main")
	if err != nil || !on || branch != "main" || detached {
		t.Fatalf("on=%v branch=%q detached=%v err=%v", on, branch, detached, err)
	}
}
