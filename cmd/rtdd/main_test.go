package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// writeFile writes a working-tree file without committing it. The file is left
// untracked or dirty on purpose: that is the state an agent's editor leaves behind.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	gittest.Write(t, dir, rel, content)
}

// gitRun runs git in dir and returns its stdout.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return gittest.Run(t, dir, args...)
}

// newTestRepo builds a real git repository holding a small python source tree and its
// tests. Git is never mocked.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)

	gittest.Write(t, dir, "src/auth.py", "def login():\n    return 1\n")
	gittest.Write(t, dir, "src/db.py", "def query():\n    return 2\n")
	gittest.Write(t, dir, "src/render.py", "def page():\n    return 3\n")
	gittest.Write(t, dir, "templates/page.html", "<p>hi</p>\n")
	gittest.Write(t, dir, "tests/test_auth.py", "def test_logout():\n    pass\n")
	gittest.Write(t, dir, "tests/test_login.py", "def test_login():\n    pass\n")
	gittest.Write(t, dir, "tests/test_db.py", "def test_query():\n    pass\n")
	gittest.Write(t, dir, "tests/test_render.py", "def test_page():\n    pass\n")
	// The test repo ignores .rtdd/ so rtdd's own config and graph cache never show up in
	// the changed set and every assertion is about the code under test.
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	return dir
}

func headShort(t *testing.T, dir string) string {
	t.Helper()
	return gittest.HeadShort(t, dir)
}

// rtdd runs the CLI with dir as the working directory.
func rtdd(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Chdir(dir)
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestRunWithNoArgsIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	code, _, stderr := rtdd(t, dir)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage") {
		t.Errorf("stderr = %q, want usage text", stderr)
	}
}

func TestRunWithAnUnknownCommandIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	code, _, stderr := rtdd(t, dir, "frobnicate")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "frobnicate") {
		t.Errorf("stderr = %q, want it to name the unknown command", stderr)
	}
}

// An unknown flag is a usage error, not a crash and not a silent success.
func TestStatusWithAnUnknownFlagIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)

	code, _, stderr := rtdd(t, dir, "status", "--frobnicate")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("stderr is empty; a usage error must say what was wrong")
	}
}

// Exit code 1 meant "a test failed". rtdd runs no test, so no invocation of the CLI may
// produce it — a 1 here would be an unrelated failure wearing the costume of a red test
// suite (spec §8: exit codes are 0, 2 and 3 only).
func TestNothingExitsOne(t *testing.T) {
	repo := newTestRepo(t)
	writeFile(t, repo, "src/auth.py", "def login():\n    return 42\n")
	broken := newTestRepo(t)
	writeFile(t, broken, ".rtdd/config.yaml", ": : not: yaml: [\n")
	nonRepo := t.TempDir()

	cases := []struct {
		name string
		dir  string
		args []string
	}{
		{"no args", repo, nil},
		{"help", repo, []string{"--help"}},
		{"unknown command", repo, []string{"frobnicate"}},
		{"removed command", repo, []string{"status"}},
		{"which", repo, []string{"which"}},
		{"which --json", repo, []string{"which", "--json"}},
		{"which unknown flag", repo, []string{"which", "--frobnicate"}},
		{"which malformed config", broken, []string{"which"}},
		{"which outside a repo", nonRepo, []string{"which"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := rtdd(t, tc.dir, tc.args...)
			if code == 1 {
				t.Errorf("%v exited 1; rtdd runs no test", tc.args)
			}
		})
	}
}

func TestWhichRejectsAnUnknownFlag(t *testing.T) {
	dir := newTestRepo(t)

	code, _, stderr := rtdd(t, dir, "which", "--nope")
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr is empty")
	}
}
