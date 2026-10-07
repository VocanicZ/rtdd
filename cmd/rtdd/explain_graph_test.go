package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// explainRepo is a committed Python project: total calls add, report calls total, and
// add, total and report each have a test.
func explainRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n\n\ndef unused():\n    return 0\n")
	gittest.Write(t, dir, "src/report.py", "from src.calc import total\n\n\ndef report(xs):\n    return str(total(xs))\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add, total\n\n\ndef test_add():\n    assert add(1, 2) == 3\n\n\ndef test_total():\n    assert total([1, 2]) == 3\n")
	gittest.Write(t, dir, "tests/test_report.py", "from src.report import report\n\n\ndef test_report():\n    assert report([1, 2]) == \"3\"\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// totalExplained is `rtdd explain` on src/calc.py::total in explainRepo.
const totalExplained = "src/calc.py::total  (func, lines 5-6)\n" +
	"  tests:\n" +
	"    tests/test_calc.py::test_total  (tests/test_calc.py:8, calls)\n" +
	"  callers:\n" +
	"    src/report.py::report  (src/report.py:4, calls)\n" +
	"  callees:\n" +
	"    src/calc.py::add  (src/calc.py:1, calls)\n"

// PRD #410 AC4: explain prints a node's tests, callers and callees, for each of the
// three argument forms.
func TestExplainPrintsTestsCallersAndCallees(t *testing.T) {
	dir := explainRepo(t)
	for _, c := range []struct {
		arg, want string
	}{
		{"src/calc.py:6", totalExplained},
		{"total", totalExplained},
		{"src/calc.py::total", totalExplained},
		{"src/report.py", "src/report.py::report  (func, lines 4-5)\n" +
			"  tests:\n" +
			"    tests/test_report.py::test_report  (tests/test_report.py:4, calls)\n" +
			"  callers:\n" +
			"    none\n" +
			"  callees:\n" +
			"    src/calc.py::total  (src/calc.py:5, calls)\n"},
	} {
		code, out, errOut := rtdd(t, dir, "explain", c.arg)
		if code != 0 {
			t.Errorf("rtdd explain %s = %d, stderr %q", c.arg, code, errOut)
			continue
		}
		if out != c.want {
			t.Errorf("rtdd explain %s:\n%s\nwant:\n%s", c.arg, out, c.want)
		}
	}
}

// A file argument is relative to the caller's directory, like any path on a command line.
func TestExplainResolvesAFileRelativeToTheWorkingDirectory(t *testing.T) {
	dir := explainRepo(t)
	code, out, errOut := rtdd(t, dir+"/src", "explain", "calc.py:6")
	if code != 0 || out != totalExplained {
		t.Errorf("rtdd explain calc.py:6 from src/ = %d %q\n%s", code, errOut, out)
	}
}

// Spec §12 (#417): a name defined in several files lists every definition.
func TestExplainANameListsEveryDefinition(t *testing.T) {
	dir := explainRepo(t)
	gittest.Write(t, dir, "src/other.py", "def total(xs):\n    return 0\n")
	gittest.Commit(t, dir, "second total")
	code, out, _ := rtdd(t, dir, "explain", "total")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"src/calc.py::total  (func", "src/other.py::total  (func"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain total does not list %q:\n%s", want, out)
		}
	}
}

// An argument naming nothing, or no argument, is a usage error (2); outside a git
// repository is an environment error (3).
func TestExplainExitCodes(t *testing.T) {
	dir := explainRepo(t)
	for _, c := range []struct {
		dir  string
		args []string
		want int
		msg  string
	}{
		{dir, []string{"explain", "no_such_name"}, 2, `no node matches "no_such_name"`},
		{dir, []string{"explain", "src/calc.py:3"}, 2, `no node matches "src/calc.py:3"`},
		{dir, []string{"explain"}, 2, "usage: rtdd explain <file[:line]|name>"},
		{t.TempDir(), []string{"explain", "total"}, 3, "not inside a git work tree"},
	} {
		code, _, errOut := rtdd(t, c.dir, c.args...)
		if code != c.want || !strings.Contains(errOut, c.msg) {
			t.Errorf("rtdd %v = %d, stderr %q; want %d and %q", c.args, code, errOut, c.want, c.msg)
		}
	}
}

// Issue #449: explain reads the graph only — none of the coverage pipeline's packages.
func TestExplainImportsNoCoveragePipelinePackage(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "explain.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		for _, banned := range []string{"internal/mapstore", "internal/selector", "internal/uncovered", "internal/adapter"} {
			if strings.HasSuffix(strings.Trim(imp.Path.Value, `"`), banned) {
				t.Errorf("explain.go imports %s", imp.Path.Value)
			}
		}
	}
}
