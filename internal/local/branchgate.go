package local

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultBranchResolveTimeout = 20 * time.Second

// gitExec runs git in dir and returns trimmed stdout.
type gitExec func(ctx context.Context, dir string, args ...string) (string, error)

// branchDecision is the watcher's view of one path after reading HEAD.
// open is false only when the path sits in a git work tree whose default
// branch is known and not currently checked out (including detached HEAD).
type branchDecision struct {
	repo   string
	open   bool
	drop   bool
	rescan bool
}

// branchGate caches each repo's remote default branch for the process lifetime.
// It reads HEAD from the work tree and does not watch the rest of .git.
type branchGate struct {
	mu       sync.Mutex
	repos    map[string]*gitRepo
	byHead   map[string]*gitRepo
	dirCache map[string]string
	run      gitExec
}

type gitRepo struct {
	root          string
	headPath      string
	resolved      bool
	gated         bool
	remote        string
	defaultBranch string
	seen          bool
	onDefault     bool
	headWatched   bool
}

func newBranchGate(run gitExec) *branchGate {
	if run == nil {
		run = defaultGitExec
	}
	return &branchGate{
		repos:    make(map[string]*gitRepo),
		byHead:   make(map[string]*gitRepo),
		dirCache: make(map[string]string),
		run:      run,
	}
}

func defaultGitExec(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, msg)
	}
	return strings.TrimSpace(string(out)), nil
}

// consider reports whether path may be indexed or recovery-scanned.
// The first observation of an off-default repo sets drop. A later move
// onto the default branch sets rescan. The default branch name is resolved
// at most once per repo.
func (g *branchGate) consider(path string) branchDecision {
	g.mu.Lock()
	defer g.mu.Unlock()
	repo, ok := g.repoForLocked(path)
	if !ok {
		return branchDecision{open: true}
	}
	if !repo.resolved {
		g.resolveLocked(repo)
	}
	d := branchDecision{repo: repo.root, open: true}
	if !repo.gated {
		return d
	}
	on, branch, detached, err := readCheckedOut(repo.headPath, repo.defaultBranch)
	if err != nil {
		// A missing HEAD file should not hide edits. Stay on today's behavior.
		return d
	}
	label := branch
	if detached {
		label = "detached"
	}
	if !repo.seen {
		repo.seen = true
		repo.onDefault = on
		if !on {
			d.open = false
			d.drop = true
			slog.Info("watch skipping git repo off default branch", "repo", repo.root, "branch", label)
		}
		return d
	}
	if repo.onDefault && !on {
		repo.onDefault = false
		d.open = false
		d.drop = true
		slog.Info("watch skipping git repo off default branch", "repo", repo.root, "branch", label)
		return d
	}
	if !repo.onDefault && on {
		repo.onDefault = true
		d.rescan = true
		slog.Info("watch rescan git repo on default branch", "repo", repo.root, "branch", label)
		return d
	}
	if !on {
		d.open = false
	}
	return d
}

// watchHead returns the repo HEAD path to pass to fsnotify, once per repo.
func (g *branchGate) watchHead(path string) (head string, already bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	repo, ok := g.repoForLocked(path)
	if !ok {
		return "", false
	}
	if !repo.resolved {
		g.resolveLocked(repo)
	}
	if repo.headWatched {
		return repo.headPath, true
	}
	repo.headWatched = true
	return repo.headPath, false
}

func (g *branchGate) isHeadEvent(path string) bool {
	path = filepath.Clean(path)
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.byHead[path]; ok {
		return true
	}
	return filepath.Base(path) == "HEAD" && filepath.Base(filepath.Dir(path)) == ".git"
}

func (g *branchGate) repoForLocked(path string) (*gitRepo, bool) {
	path = filepath.Clean(path)
	if repo := g.byHead[path]; repo != nil {
		return repo, true
	}
	dir := path
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		dir = filepath.Dir(path)
	}
	var walked []string
	for {
		if cached, ok := g.dirCache[dir]; ok {
			for _, w := range walked {
				g.dirCache[w] = cached
			}
			if cached == "" {
				return nil, false
			}
			return g.repos[cached], true
		}
		walked = append(walked, dir)
		if root, head, ok := worktreeAt(dir); ok {
			repo := g.repos[root]
			if repo == nil {
				repo = &gitRepo{root: root, headPath: head}
				g.repos[root] = repo
				g.byHead[head] = repo
			}
			for _, w := range walked {
				g.dirCache[w] = root
			}
			return repo, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			for _, w := range walked {
				g.dirCache[w] = ""
			}
			return nil, false
		}
		dir = parent
	}
}

func (g *branchGate) resolveLocked(repo *gitRepo) {
	repo.resolved = true
	remotes := g.gitLines(repo.root, "remote")
	if len(remotes) == 0 {
		return
	}
	remote := ""
	if upstream, err := g.git(repo.root, "rev-parse", "--symbolic-full-name", "@{upstream}"); err == nil {
		remote = remoteFromUpstream(upstream)
	}
	if remote == "" {
		switch {
		case len(remotes) == 1:
			remote = remotes[0]
		case containsRemote(remotes, "origin"):
			remote = "origin"
		default:
			return
		}
	}
	repo.remote = remote
	if branch := g.readRemoteHead(repo.root, remote); branch != "" {
		repo.defaultBranch = branch
		repo.gated = true
		return
	}
	if _, err := g.git(repo.root, "remote", "set-head", remote, "--auto"); err != nil {
		slog.Warn("watch default branch unresolved; not gating repo", "repo", repo.root, "remote", remote, "err", err)
		return
	}
	if branch := g.readRemoteHead(repo.root, remote); branch != "" {
		repo.defaultBranch = branch
		repo.gated = true
		return
	}
	slog.Warn("watch default branch unresolved; not gating repo", "repo", repo.root, "remote", remote)
}

func (g *branchGate) readRemoteHead(root, remote string) string {
	target, err := g.git(root, "symbolic-ref", "--quiet", "refs/remotes/"+remote+"/HEAD")
	if err != nil {
		return ""
	}
	return branchFromRemoteHead(remote, target)
}

func (g *branchGate) git(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultBranchResolveTimeout)
	defer cancel()
	return g.run(ctx, dir, args...)
}

func (g *branchGate) gitLines(dir string, args ...string) []string {
	out, err := g.git(dir, args...)
	if err != nil || out == "" {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func remoteFromUpstream(full string) string {
	full = strings.TrimSpace(full)
	if rest, ok := strings.CutPrefix(full, "refs/remotes/"); ok {
		full = rest
	}
	remote, _, ok := strings.Cut(full, "/")
	if !ok || remote == "" || remote == "refs" {
		return ""
	}
	return remote
}

func branchFromRemoteHead(remote, target string) string {
	prefix := "refs/remotes/" + remote + "/"
	branch, ok := strings.CutPrefix(strings.TrimSpace(target), prefix)
	if !ok || branch == "" || branch == "HEAD" {
		return ""
	}
	return branch
}

func containsRemote(remotes []string, name string) bool {
	for _, remote := range remotes {
		if remote == name {
			return true
		}
	}
	return false
}

// worktreeAt reports a git work tree rooted exactly at dir.
func worktreeAt(dir string) (root, head string, ok bool) {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		return "", "", false
	}
	if info.IsDir() {
		head = filepath.Join(gitPath, "HEAD")
		if _, err := os.Stat(head); err != nil {
			return "", "", false
		}
		return dir, head, true
	}
	if !info.Mode().IsRegular() {
		return "", "", false
	}
	head, ok = headFromGitFile(gitPath)
	if !ok {
		return "", "", false
	}
	return dir, head, true
}

func headFromGitFile(gitFile string) (string, bool) {
	b, err := os.ReadFile(gitFile)
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(string(b))
	const prefix = "gitdir: "
	gitdir, ok := strings.CutPrefix(line, prefix)
	if !ok || gitdir == "" {
		return "", false
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(filepath.Dir(gitFile), gitdir)
	}
	head := filepath.Join(gitdir, "HEAD")
	if _, err := os.Stat(head); err != nil {
		return "", false
	}
	return head, true
}

// readCheckedOut reports whether HEAD names defaultBranch.
func readCheckedOut(headPath, defaultBranch string) (on bool, branch string, detached bool, err error) {
	b, err := os.ReadFile(headPath)
	if err != nil {
		return false, "", false, err
	}
	text := strings.TrimSpace(string(b))
	const prefix = "ref: refs/heads/"
	branch, ok := strings.CutPrefix(text, prefix)
	if !ok || branch == "" || strings.Contains(branch, "..") {
		return false, "", true, nil
	}
	return branch == defaultBranch, branch, false, nil
}
