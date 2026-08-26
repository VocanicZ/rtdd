// Package gittest builds real git repositories for tests in other packages.
//
// It lives under internal/gitctx/ deliberately. Git is never mocked in this engine, so
// every git-dependent test needs a real `git init` in t.TempDir(); at the same time
// internal/gitctx is the only part of the tree permitted to invoke git, so the shell-out
// that builds those fixtures belongs here rather than being re-invented per package.
// Nothing outside a test may import this package.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run executes a git subcommand in dir and returns its combined output, failing the test
// if git exits nonzero. The environment is pinned so the result never depends on the
// developer's global git config.
func Run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+dir,
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// Init creates an empty repository with a deterministic identity in t.TempDir() and
// returns its path. The test is skipped when git is not installed.
func Init(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	Run(t, dir, "init", "-q", "-b", "main")
	Run(t, dir, "config", "user.email", "rtdd@example.com")
	Run(t, dir, "config", "user.name", "rtdd test")
	Run(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

// Write creates dir/rel, and any missing parent directories, with the given content.
func Write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Commit stages everything in dir and commits it, returning the new short SHA.
func Commit(t *testing.T, dir, msg string) string {
	t.Helper()
	Run(t, dir, "add", "-A")
	Run(t, dir, "commit", "-q", "-m", msg)
	return HeadShort(t, dir)
}

// HeadShort returns the short SHA of HEAD.
func HeadShort(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(Run(t, dir, "rev-parse", "--short", "HEAD"))
}
