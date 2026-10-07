package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// whichRepo is a committed Python project: total calls add, report calls total, and each
// of add, total and report has a test; unused has none.
func whichRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n\n\ndef unused():\n    return 0\n")
	gittest.Write(t, dir, "src/report.py", "from src.calc import total\n\n\ndef report(xs):\n    return str(total(xs))\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add, total\n\n\ndef test_add():\n    assert add(1, 2) == 3\n\n\ndef test_total():\n    assert total([1, 2]) == 3\n")
	gittest.Write(t, dir, "tests/test_report.py", "from src.report import report\n\n\ndef test_report():\n    assert report([1, 2]) == \"3\"\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// editTotalAndUnused changes the body of total (line 6) and of unused (line 10).
func editTotalAndUnused(t *testing.T, dir string) {
	t.Helper()
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1]) + 0\n\n\ndef unused():\n    return 1\n")
}

// tripwirePATH replaces PATH with a directory holding git and a tripwire for every test
// runner and toolchain rtdd v0.2 used to invoke. It returns the file a tripwire writes
// when anything runs one: `rtdd which` must leave it absent (spec §1: rtdd never
// executes a test and has no per-language code path).
func tripwirePATH(t *testing.T) string {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not on PATH")
	}
	bin := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "tripped")
	for _, tool := range []string{"python", "python3", "pytest", "go", "node", "npm", "npx", "cargo",
		"mvn", "gradle", "dotnet", "php", "phpunit", "ruby", "bundle", "rspec", "graphify"} {
		script := "#!/bin/sh\necho " + tool + " >> " + marker + "\nexit 1\n"
		if err := os.WriteFile(filepath.Join(bin, tool), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	return marker
}

// PRD #410 AC2: changed nodes, Rounds 1-3, untested and the graph source, in text.
func TestWhichPrintsChangedNodesRoundsUntestedAndGraphSource(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	code, out, errOut := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	want := "graph: scanner, built at " + gittest.HeadShort(t, dir) + ", 0 stale files\n" +
		"changed nodes:\n" +
		"  src/calc.py::total  (lines 5-6)\n" +
		"  src/calc.py::unused  (lines 9-10)\n" +
		"Round 1 — run these first:\n" +
		"  tests/test_calc.py::test_total\n" +
		"Round 2 — then these:\n" +
		"  tests/test_calc.py::test_add\n" +
		"  tests/test_report.py::test_report\n" +
		"Round 3 — the full suite, once, at the end\n" +
		"untested:\n" +
		"  src/calc.py::unused\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}
}

// PRD #410 AC2: empty Rounds 1 and 2 say "no linked test", never anything that reads as
// a pass, and exit 0 (AC6).
func TestWhichWithEmptyRoundsSaysNoLinkedTestAndExits0(t *testing.T) {
	dir := whichRepo(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n\n\ndef unused():\n    return 2\n")
	code, out, errOut := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	if got := strings.Count(out, "  no linked test\n"); got != 2 {
		t.Errorf("want Round 1 and Round 2 each to say %q, got %d in:\n%s", "no linked test", got, out)
	}
	for _, banned := range []string{"pass", "PASS", "green", "nothing to run"} {
		if strings.Contains(out, banned) {
			t.Errorf("output reads as a result (%q):\n%s", banned, out)
		}
	}
}

// PRD #410 AC2, AC6: --base selects changes since that ref; a bad ref and an unknown
// flag are usage errors (2); outside a git repository is an environment error (3).
func TestWhichBaseAndExitCodes(t *testing.T) {
	dir := whichRepo(t)
	first := gittest.HeadShort(t, dir)
	editTotalAndUnused(t, dir)
	gittest.Commit(t, dir, "edit")
	if _, out, _ := rtdd(t, dir, "which"); !strings.Contains(out, "changed nodes:\n  none\n") {
		t.Errorf("a clean tree against HEAD should change nothing:\n%s", out)
	}
	if _, out, _ := rtdd(t, dir, "which", "--base", first); !strings.Contains(out, "  src/calc.py::total  (lines 5-6)\n") {
		t.Errorf("--base %s should see the committed edit:\n%s", first, out)
	}
	for _, c := range []struct {
		name string
		dir  string
		args []string
		want int
	}{
		{"unknown ref", dir, []string{"which", "--base", "no-such-ref"}, 2},
		{"unknown flag", dir, []string{"which", "--bogus"}, 2},
		{"stray argument", dir, []string{"which", "src/calc.py"}, 2},
		{"outside a git repository", t.TempDir(), []string{"which"}, 3},
	} {
		if code, _, _ := rtdd(t, c.dir, c.args...); code != c.want {
			t.Errorf("%s: exit %d, want %d", c.name, code, c.want)
		}
	}
}

// PRD #410 AC2: `rtdd which` runs nothing but git.
func TestWhichSpawnsNoProcessButGit(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	marker := tripwirePATH(t)
	if code, _, errOut := rtdd(t, dir, "which"); code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	if b, err := os.ReadFile(marker); err == nil {
		t.Errorf("rtdd which ran %s", b)
	}
}

// A deleted file has no line left to own a node, so it selects nothing — and says so on
// stderr instead of staying silent (docs/plans/10-rounds-cutover.md, Review Focus).
func TestWhichWarnsAboutADeletedFileAndSelectsNothingForIt(t *testing.T) {
	dir := whichRepo(t)
	if err := os.Remove(filepath.Join(dir, "src/report.py")); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	if !strings.Contains(errOut, "rtdd which: warning: src/report.py was deleted") {
		t.Errorf("stderr does not warn about the deleted file:\n%s", errOut)
	}
	if !strings.Contains(out, "changed nodes:\n  none\n") {
		t.Errorf("a deleted file should own no changed node:\n%s", out)
	}
}
