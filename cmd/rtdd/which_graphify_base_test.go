package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// baseCalcPy is src/calc.py before the edit: total spans lines 10-14.
const baseCalcPy = "def add(a, b):\n    return a + b\n\n\ndef scale(x, k):\n    return x * k\n\n\n\n" +
	"def total(xs):\n    s = 0\n    for x in xs:\n        s = add(s, x)\n    return s\n"

// graphifyBaseRepo is #471's reproduction: a commit editing line 14, the body of total,
// on top of a base commit, and a graphify graph built at that edit's commit, so graphify
// is fresh against HEAD and knows total only by its start line (L10).
func graphifyBaseRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", baseCalcPy)
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add, scale, total\n\n\n"+
		"def test_add():\n    assert add(1, 2) == 3\n\n\n"+
		"def test_total():\n    assert total([1, 2]) == 3\n\n\n"+
		"def test_scale():\n    assert scale(2, 3) == 6\n")
	gittest.Commit(t, dir, "base")
	gittest.Write(t, dir, "src/calc.py", strings.Replace(baseCalcPy, "    return s\n", "    return s + 0\n", 1))
	gittest.Commit(t, dir, "edit total")
	head := strings.TrimSpace(gittest.Run(t, dir, "rev-parse", "HEAD"))
	gittest.Write(t, dir, "graphify-out/graph.json", `{"nodes":[`+
		`{"id":"add","label":"add()","file_type":"code","source_file":"src/calc.py","source_location":"L1"},`+
		`{"id":"scale","label":"scale()","file_type":"code","source_file":"src/calc.py","source_location":"L5"},`+
		`{"id":"total","label":"total()","file_type":"code","source_file":"src/calc.py","source_location":"L10"},`+
		`{"id":"ta","label":"test_add()","file_type":"code","source_file":"tests/test_calc.py","source_location":"L4"},`+
		`{"id":"tt","label":"test_total()","file_type":"code","source_file":"tests/test_calc.py","source_location":"L8"},`+
		`{"id":"ts","label":"test_scale()","file_type":"code","source_file":"tests/test_calc.py","source_location":"L12"}],`+
		`"links":[{"source":"total","target":"add","relation":"calls"},`+
		`{"source":"ta","target":"add","relation":"calls"},`+
		`{"source":"tt","target":"total","relation":"calls"},`+
		`{"source":"ts","target":"scale","relation":"calls"}],`+
		`"built_at_commit":"`+head+`"}`)
	gittest.Write(t, dir, "graphify-out/manifest.json", `{"src/calc.py":{},"tests/test_calc.py":{}}`)
	return dir
}

// #471, spec §5 step 1: with a fresh graphify graph, `which --base HEAD~1` still scans
// the files changed against the base, so line 14 maps to total (lines 10-14) rather than
// through graphify's start-only span to <module>.
func TestWhichBaseRescansFilesChangedAgainstTheBaseUnderGraphify(t *testing.T) {
	dir := graphifyBaseRepo(t)

	code, out, errOut := rtdd(t, dir, "which", "--base", "HEAD~1")
	if code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "graph: graphify+scanner") {
		t.Errorf("graphify should be used:\n%s", out)
	}
	if !strings.Contains(out, "changed nodes:\n  src/calc.py::total  (lines 10-14)\nRound 1 — run these first:\n  tests/test_calc.py::test_total\nRound 2") {
		t.Errorf("want total (lines 10-14) changed and Round 1 = test_total:\n%s", out)
	}

	code, out, errOut = rtdd(t, dir, "which", "--base", "HEAD~1", "--json")
	if code != 0 {
		t.Fatalf("rtdd which --json = %d, stderr %q", code, errOut)
	}
	var doc struct {
		Graph struct {
			Source     string `json:"source"`
			StaleFiles int    `json:"stale_files"`
		} `json:"graph"`
		ChangedNodes []struct {
			ID    string `json:"id"`
			Start int    `json:"start"`
			End   int    `json:"end"`
		} `json:"changed_nodes"`
		Rounds []json.RawMessage `json:"rounds"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Graph.Source != "graphify+scanner" || doc.Graph.StaleFiles != 1 {
		t.Errorf("graph = %+v, want graphify+scanner with src/calc.py its 1 stale file", doc.Graph)
	}
	if len(doc.ChangedNodes) != 1 || doc.ChangedNodes[0].ID != "src/calc.py::total" ||
		doc.ChangedNodes[0].Start != 10 || doc.ChangedNodes[0].End != 14 {
		t.Errorf("changed_nodes = %+v, want src/calc.py::total 10-14", doc.ChangedNodes)
	}
	var r1 struct {
		Tests []struct {
			ID string `json:"id"`
		} `json:"tests"`
	}
	if len(doc.Rounds) == 0 {
		t.Fatalf("no rounds:\n%s", out)
	}
	if err := json.Unmarshal(doc.Rounds[0], &r1); err != nil {
		t.Fatal(err)
	}
	if len(r1.Tests) != 1 || r1.Tests[0].ID != "tests/test_calc.py::test_total" {
		t.Errorf("Round 1 = %+v, want test_total alone", r1.Tests)
	}
}

// #471: `rtdd graph` keeps its HEAD behaviour: the same fresh graphify graph has no
// stale file, since nothing differs from HEAD.
func TestGraphStaysMeasuredAgainstHEADUnderGraphify(t *testing.T) {
	dir := graphifyBaseRepo(t)
	code, out, errOut := rtdd(t, dir, "graph")
	if code != 0 {
		t.Fatalf("rtdd graph = %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "stale_files:     0\n") {
		t.Errorf("rtdd graph should measure staleness against HEAD:\n%s", out)
	}
}
