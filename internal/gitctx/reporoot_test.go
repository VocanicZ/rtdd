package gitctx_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// A repository made by real `git init` resolves to the same root `git rev-parse
// --show-toplevel` reports. Agreeing with git is the whole contract; asserting it
// against git itself is the only way to keep the claim honest.
func TestFindRepoRootAgreesWithGitOnARealRepository(t *testing.T) {
	repo := gittest.Init(t)
	deep := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}

	got, err := gitctx.FindRepoRoot(deep)
	if err != nil {
		t.Fatalf("FindRepoRoot: %v", err)
	}

	out, err := exec.Command("git", "-C", deep, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	want := strings.TrimSpace(string(out))

	gotEval, _ := filepath.EvalSymlinks(got)
	wantEval, _ := filepath.EvalSymlinks(want)
	if gotEval != wantEval {
		t.Fatalf("FindRepoRoot = %q, git says %q", gotEval, wantEval)
	}
}

// Issue #369: a stray `.git` above a real repository must not capture the walk, and a
// stray with no repository anywhere above it must be an error rather than a wrong root.
func TestFindRepoRootIgnoresAStrayGitAncestor(t *testing.T) {
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir stray .git: %v", err)
	}
	inner := filepath.Join(outer, "work")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatalf("mkdir work: %v", err)
	}

	if got, err := gitctx.FindRepoRoot(inner); err == nil {
		t.Fatalf("FindRepoRoot under a stray .git = %q, want an error", got)
	}
}

// IsRepoRoot is the single rule both commands share; these are the shapes measured
// against git at triage.
func TestIsRepoRoot(t *testing.T) {
	cases := []struct {
		name  string
		build func(t *testing.T, dir string)
		want  bool
	}{
		{"no .git at all", func(*testing.T, string) {}, false},
		{"empty .git directory", func(t *testing.T, dir string) {
			mkdirAll(t, filepath.Join(dir, ".git"))
		}, false},
		{".git directory with only info/", func(t *testing.T, dir string) {
			mkdirAll(t, filepath.Join(dir, ".git", "info"))
		}, false},
		{".git directory with HEAD", func(t *testing.T, dir string) {
			mkdirAll(t, filepath.Join(dir, ".git"))
			writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")
		}, true},
		{".git file with a dead gitdir", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".git"), "gitdir: /nonexistent-rtdd-369\n")
		}, false},
		{".git file whose gitdir has no HEAD", func(t *testing.T, dir string) {
			mkdirAll(t, filepath.Join(dir, "gitdir"))
			writeFile(t, filepath.Join(dir, ".git"), "gitdir: gitdir\n")
		}, false},
		{".git file with a resolving relative gitdir", func(t *testing.T, dir string) {
			mkdirAll(t, filepath.Join(dir, "gitdir"))
			writeFile(t, filepath.Join(dir, "gitdir", "HEAD"), "ref: refs/heads/main\n")
			writeFile(t, filepath.Join(dir, ".git"), "gitdir: gitdir\n")
		}, true},
		{".git file with no gitdir line", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".git"), "not a pointer\n")
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.build(t, dir)
			if got := gitctx.IsRepoRoot(dir); got != tc.want {
				t.Errorf("IsRepoRoot = %v, want %v", got, tc.want)
			}
		})
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
