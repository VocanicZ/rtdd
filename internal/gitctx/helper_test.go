package gitctx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a real git repository in t.TempDir() with a deterministic identity.
// Git is never mocked in this package: the bugs this code exists to prevent live in
// git's actual output.
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.email", "rtdd@example.com")
	gitRun(t, dir, "config", "user.name", "rtdd test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

// run executes a git subcommand in dir against a real repository and returns its
// combined output. It fails the test if git exits nonzero.
func run(t *testing.T, dir string, args ...string) string {
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

// gitRun is the name every other git test in this milestone calls; it is run.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return run(t, dir, args...)
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func remove(t *testing.T, dir, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

// commit stages everything and commits, returning the short SHA.
func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", msg)
	return strings.TrimSpace(gitRun(t, dir, "rev-parse", "--short", "HEAD"))
}
