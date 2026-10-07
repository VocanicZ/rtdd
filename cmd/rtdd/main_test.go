package main

import (
	"bytes"
	"os"
	"strconv"
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

// newTestRepo builds a real git repository containing the source tree the fixture map
// describes. Git is never mocked.
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
	// The test repo ignores .rtdd/ so the fixture map and adapter never show up in the
	// changed set and every assertion is about the code under test. A real host repo
	// commits .rtdd/map.jsonl instead; either way it selects nothing.
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	return dir
}

func headShort(t *testing.T, dir string) string {
	t.Helper()
	return gittest.HeadShort(t, dir)
}

// installRTDD copies the fixture map, meta, and adapter into dir/.rtdd/, substituting
// seedSHA for the SEEDSHA token in the map and the meta.
func installRTDD(t *testing.T, dir, seedSHA string, cycles int) {
	t.Helper()
	raw, err := os.ReadFile("testdata/map.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	gittest.Write(t, dir, ".rtdd/map.jsonl", strings.ReplaceAll(string(raw), "SEEDSHA", seedSHA))

	ad, err := os.ReadFile("testdata/adapter.yaml")
	if err != nil {
		t.Fatal(err)
	}
	gittest.Write(t, dir, ".rtdd/adapter.yaml", string(ad))

	meta := `{"v":2,"adapter":"python","seeded_at":"` + seedSHA + `","cycles":` +
		strconv.Itoa(cycles) + "}\n"
	gittest.Write(t, dir, ".rtdd/meta.json", meta)
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
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "status", "--frobnicate")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("stderr is empty; a usage error must say what was wrong")
	}
}

// Exit code 1 means "a test failed". No M1a command executes a test, so no invocation of
// the CLI in this milestone may produce it — a 1 here would be an unrelated failure
// wearing the costume of a red test suite.
func TestNothingInM1aExitsOne(t *testing.T) {
	seeded := newTestRepo(t)
	installRTDD(t, seeded, headShort(t, seeded), 7)
	bare := newTestRepo(t)
	broken := newTestRepo(t)
	gittest.Write(t, broken, ".rtdd/map.jsonl", "this is not json\n")
	gittest.Write(t, broken, ".rtdd/adapter.yaml", ": : not: yaml: [\n")
	nonRepo := t.TempDir()

	cases := []struct {
		name string
		dir  string
		args []string
	}{
		{"no args", seeded, nil},
		{"help", seeded, []string{"--help"}},
		{"unknown command", seeded, []string{"frobnicate"}},
		{"status seeded", seeded, []string{"status"}},
		{"status unseeded", bare, []string{"status"}},
		{"status unknown flag", seeded, []string{"status", "--frobnicate"}},
		{"status malformed map", broken, []string{"status"}},
		{"status outside a repo", nonRepo, []string{"status"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := rtdd(t, tc.dir, tc.args...)
			if code == 1 {
				t.Errorf("%v exited 1; nothing in M1a runs a test", tc.args)
			}
		})
	}
}

func TestWhichRejectsAnUnknownFlag(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "which", "--nope")
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr is empty")
	}
}
