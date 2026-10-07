package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// explainGraphifyRepo is #472's reproduction: total spans lines 10-14 of src/calc.py,
// calls add, is called by report and tested by test_total, and a graphify graph built
// at HEAD is fresh against a clean tree, so it knows every node only by its start line.
func explainGraphifyRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", baseCalcPy)
	gittest.Write(t, dir, "src/report.py", "from src.calc import total\n\n\ndef report(xs):\n    return str(total(xs))\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import total\n\n\n"+
		"def test_total():\n    assert total([1, 2]) == 3\n")
	gittest.Commit(t, dir, "init")
	head := strings.TrimSpace(gittest.Run(t, dir, "rev-parse", "HEAD"))
	gittest.Write(t, dir, "graphify-out/graph.json", `{"nodes":[`+
		`{"id":"add","label":"add()","file_type":"code","source_file":"src/calc.py","source_location":"L1"},`+
		`{"id":"scale","label":"scale()","file_type":"code","source_file":"src/calc.py","source_location":"L5"},`+
		`{"id":"total","label":"total()","file_type":"code","source_file":"src/calc.py","source_location":"L10"},`+
		`{"id":"report","label":"report()","file_type":"code","source_file":"src/report.py","source_location":"L4"},`+
		`{"id":"tt","label":"test_total()","file_type":"code","source_file":"tests/test_calc.py","source_location":"L4"}],`+
		`"links":[{"source":"total","target":"add","relation":"calls"},`+
		`{"source":"report","target":"total","relation":"calls"},`+
		`{"source":"tt","target":"total","relation":"calls"}],`+
		`"built_at_commit":"`+head+`"}`)
	gittest.Write(t, dir, "graphify-out/manifest.json", `{"src/calc.py":{},"src/report.py":{},"tests/test_calc.py":{}}`)
	return dir
}

// #472, spec §5: with a fresh graphify graph, `explain <file>:<line>` rescans the named
// file, so a line inside total's body resolves to total (lines 10-14), not to nothing
// through graphify's start-only span.
func TestExplainFileLineRescansTheFileUnderGraphify(t *testing.T) {
	dir := explainGraphifyRepo(t)
	code, out, errOut := rtdd(t, dir, "explain", "src/calc.py:13")
	if code != 0 {
		t.Fatalf("rtdd explain src/calc.py:13 = %d, stderr %q", code, errOut)
	}
	want := "src/calc.py::total  (func, lines 10-14)\n" +
		"  tests:\n" +
		"    tests/test_calc.py::test_total  (tests/test_calc.py:4, calls)\n" +
		"  callers:\n" +
		"    src/report.py::report  (src/report.py:4, calls)\n" +
		"  callees:\n" +
		"    src/calc.py::add  (src/calc.py:1, calls)\n"
	if out != want {
		t.Errorf("rtdd explain src/calc.py:13:\n%s\nwant:\n%s", out, want)
	}
}

// #472: `explain <name>` and `explain <file>` keep reading graphify's graph for a fresh
// file, as before.
func TestExplainNameAndFileStillReadGraphify(t *testing.T) {
	dir := explainGraphifyRepo(t)
	for _, arg := range []string{"total", "src/calc.py"} {
		code, out, errOut := rtdd(t, dir, "explain", arg)
		if code != 0 {
			t.Fatalf("rtdd explain %s = %d, stderr %q", arg, code, errOut)
		}
		if !strings.Contains(out, "total  (func, lines 10-10)\n") {
			t.Errorf("rtdd explain %s should show graphify's total:\n%s", arg, out)
		}
	}
}
