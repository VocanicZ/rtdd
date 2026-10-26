package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// fileNodesRepo commits internal/scan/testdata/filenodes: a test that reads prompts/x.md
// and conf/app.yaml, and misc/orphan.toml, which nothing names.
func fileNodesRepo(t *testing.T) string {
	t.Helper()
	fixture, err := filepath.Abs(filepath.Join("..", "..", "internal", "scan", "testdata", "filenodes"))
	if err != nil {
		t.Fatal(err)
	}
	dir := gittest.Init(t)
	copyTree(t, fixture, dir)
	gittest.Commit(t, dir, "fixture")
	return dir
}

// #504 acceptance: changing a file that defines nothing — a prompt, a config — selects the
// test that reads it in Round 1.
func TestWhichSelectsTheTestThatReadsAChangedFileWithNoDefinition(t *testing.T) {
	for _, file := range []string{"prompts/x.md", "conf/app.yaml"} {
		t.Run(file, func(t *testing.T) {
			dir := fileNodesRepo(t)
			b, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil {
				t.Fatal(err)
			}
			gittest.Write(t, dir, file, string(b)+"one more line\n")
			w, _ := runE2EWhich(t, dir)
			assertIDs(t, "changed_nodes", w.changed(), []string{file})
			assertIDs(t, "Round 1", w.round(1), []string{"tests/test_load.py::test_load"})
			assertIDs(t, "untested", w.Untested, []string{})
		})
	}
}

// #504 acceptance: a changed file nothing names is listed under untested — in the text
// too — and the exit code is unchanged (runE2EWhich requires 0).
func TestWhichListsAChangedFileNothingNamesAsUntested(t *testing.T) {
	dir := fileNodesRepo(t)
	gittest.Write(t, dir, "misc/orphan.toml", "answer = 43\n")
	w, text := runE2EWhich(t, dir)
	assertIDs(t, "changed_nodes", w.changed(), []string{"misc/orphan.toml"})
	assertIDs(t, "Round 1", w.round(1), []string{})
	assertIDs(t, "untested", w.Untested, []string{"misc/orphan.toml"})
	if !strings.Contains(text, "untested:\n  misc/orphan.toml\n") {
		t.Errorf("text does not list misc/orphan.toml under untested:\n%s", text)
	}
}

// #504: explain works for every file node: a file that defines nothing shows the tests
// that name it.
func TestExplainAFileWithNoDefinition(t *testing.T) {
	dir := fileNodesRepo(t)
	code, out, errOut := rtdd(t, dir, "explain", "conf/app.yaml")
	if code != 0 {
		t.Fatalf("rtdd explain conf/app.yaml = %d, stderr %q", code, errOut)
	}
	want := "conf/app.yaml  (file, lines 1-2)\n" +
		"  tests:\n" +
		"    tests/test_load.py::test_load  (tests/test_load.py:6, references)\n" +
		"  callers:\n" +
		"    prompts/x.md  (prompts/x.md:1, references)\n" +
		"  callees:\n" +
		"    none\n"
	if out != want {
		t.Errorf("rtdd explain conf/app.yaml:\n%s\nwant:\n%s", out, want)
	}
}
