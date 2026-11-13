package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// fanoutRepo is #573's reproduction: four modules with a test each, test_all calling three
// of them, a CHANGELOG naming them, a doc a test reads, a log whose lines look like
// definitions, and shell tests.
func fanoutRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	for _, x := range []string{"a", "b", "c", "d"} {
		gittest.Write(t, dir, "src/"+x+".py", fmt.Sprintf("def f_%s(v):\n    return v + 1\n", x))
		gittest.Write(t, dir, "tests/test_"+x+".py", fmt.Sprintf("from src.%s import f_%s\n\ndef test_%s():\n    assert f_%s(1) == 2\n", x, x, x, x))
	}
	gittest.Write(t, dir, "tests/test_all.py", "from src.a import f_a\nfrom src.b import f_b\nfrom src.c import f_c\n\ndef test_all():\n    assert f_a(1) + f_b(1) + f_c(1) == 6\n")
	gittest.Write(t, dir, "CHANGELOG.md", "# Changelog\n\n## 0.1\n- fix src/a.py\n- fix src/b.py\n- fix src/c.py\n- tweak src/d.py\n")
	gittest.Write(t, dir, "docs/guide.md", "# Guide\n\nUse it.\n")
	gittest.Write(t, dir, "tests/test_docs.py", "def test_guide():\n    assert open('docs/guide.md').read()\n")
	gittest.Write(t, dir, "test/smoke/logs/run.log", "+ setup() {\nsetup() {\n  f_a 1\n}\nrunning src/a.py\n")
	gittest.Write(t, dir, "tests/test_c_cli.sh", "#!/bin/sh\npython3 src/c.py && echo ok\n")
	gittest.Write(t, dir, "tests/test_d_cli.sh", "#!/bin/sh\npython3 src/d.py && echo ok\n")
	gittest.Write(t, dir, "tests/test_unrelated.sh", "#!/bin/sh\npython3 src/a.py\n")
	gittest.Write(t, dir, "scripts/release.sh", "#!/bin/sh\nnotes() {\n  head CHANGELOG.md\n}\nnotes\n")
	gittest.Write(t, dir, "tests/test_release.sh", "#!/bin/sh\nsh scripts/release.sh | grep -q Changelog && echo ok\n")
	gittest.Write(t, dir, "tests/test_hermetic.sh", "#!/bin/sh\nrun_all() {\n  python3 src/c.py\n  python3 src/d.py\n  sh scripts/release.sh\n}\nrun_all && echo ok\n")
	gittest.Commit(t, dir, "fixture")
	return dir
}

// #573: appending to a CHANGELOG that names the repo selects nothing, and no .log is a test.
func TestWhichAChangelogAppendSelectsNothing(t *testing.T) {
	dir := fanoutRepo(t)
	gittest.Write(t, dir, "CHANGELOG.md", "# Changelog\n\n## 0.1\n- fix src/a.py\n- fix src/b.py\n- fix src/c.py\n- tweak src/d.py\n- new entry\n")
	w, _ := runE2EWhich(t, dir)
	assertIDs(t, "changed_nodes", w.changed(), []string{"CHANGELOG.md"})
	assertIDs(t, "Round 1", w.round(1), []string{})
	assertIDs(t, "untested", w.Untested, []string{"CHANGELOG.md"})
	assertIDs(t, "Round 2", w.round(2), []string{})
}

// #573: a changed doc a test reads is that test's Round 1, and nothing is Round 2.
func TestWhichAChangedDocSelectsOnlyTheTestThatReadsIt(t *testing.T) {
	dir := fanoutRepo(t)
	gittest.Write(t, dir, "docs/guide.md", "# Guide\n\nUse it, see src/a.py and src/b.py.\n")
	w, _ := runE2EWhich(t, dir)
	assertIDs(t, "Round 1", w.round(1), []string{"tests/test_docs.py::test_guide"})
	assertIDs(t, "Round 2", w.round(2), []string{})
}

// #573: editing a test selects it and nothing in Round 2.
func TestWhichAChangedTestSelectsOnlyItself(t *testing.T) {
	dir := fanoutRepo(t)
	e2eEdit(t, dir, "tests/test_all.py", "== 6", "== 3 + 3")
	w, _ := runE2EWhich(t, dir)
	assertIDs(t, "Round 1", w.round(1), []string{"tests/test_all.py::test_all"})
	assertIDs(t, "Round 2", w.round(2), []string{})

	dir = fanoutRepo(t)
	e2eEdit(t, dir, "tests/test_hermetic.sh", "  python3 src/c.py\n", "  python3 src/c.py --x\n")
	w, _ = runE2EWhich(t, dir)
	assertIDs(t, "changed_nodes", w.changed(), []string{"tests/test_hermetic.sh::run_all"})
	assertIDs(t, "Round 1", w.round(1), []string{"tests/test_hermetic.sh"})
	assertIDs(t, "Round 2", w.round(2), []string{})
}

// #573: a log under test/ is never a test: editing the source it names lists no .log, and
// an explicit config holding the pre-fix defaults changes nothing.
func TestWhichNeverListsALogAsATest(t *testing.T) {
	for _, preFix := range []bool{false, true} {
		dir := fanoutRepo(t)
		if preFix {
			gittest.Write(t, dir, ".rtdd/config.yaml", install.DefaultConfig())
			gittest.Commit(t, dir, "config")
		}
		e2eEdit(t, dir, "src/a.py", "v + 1", "v + 2")
		w, text := runE2EWhich(t, dir)
		if strings.Contains(text, "run.log") {
			t.Errorf("preFix=%v: run.log is in the answer:\n%s", preFix, text)
		}
		for _, r := range []int{1, 2} {
			for _, id := range w.round(r) {
				if strings.HasSuffix(id, ".log") {
					t.Errorf("preFix=%v: Round %d lists %s", preFix, r, id)
				}
			}
		}
	}
}

func testNodes(n int) graph.Graph {
	var g graph.Graph
	for i := range n {
		g.Nodes = append(g.Nodes, graph.Node{ID: fmt.Sprintf("t.py::test_%d", i), File: "t.py", Name: fmt.Sprintf("test_%d", i), IsTest: true})
	}
	return g
}

func pick(g graph.Graph, n int) []graph.Node { return g.Nodes[:n] }

// #573: more than half of all tests selected warns — but not on a tiny suite.
func TestLargeSelectionWarnsPastHalfOfAMinimumSuite(t *testing.T) {
	g := testNodes(20)
	for _, c := range []struct {
		r1, r2 int
		warn   bool
	}{{10, 0, false}, {8, 3, true}, {6, 5, true}, {1, 0, false}, {20, 0, true}} {
		r := rounds.Result{Round1: pick(g, c.r1), Round2: g.Nodes[c.r1 : c.r1+c.r2]}
		warn, msg := largeSelection(g, r)
		if warn != c.warn || (warn && !strings.Contains(msg, "more than half")) || (!warn && msg != "") {
			t.Errorf("round1 %d + round2 %d of 20: warn=%v msg=%q, want warn=%v", c.r1, c.r2, warn, msg, c.warn)
		}
	}
	small := testNodes(4)
	if warn, _ := largeSelection(small, rounds.Result{Round1: pick(small, 4)}); warn {
		t.Error("a 4-test repo warns on every change")
	}
}

// #573: the text prints the warning and --json carries it as a field and a warnings entry.
func TestWhichWarnsWhenItSelectsMoreThanHalfTheSuite(t *testing.T) {
	dir := gittest.Init(t)
	for i := range 12 {
		gittest.Write(t, dir, fmt.Sprintf("src/m%d.py", i), fmt.Sprintf("def f%d():\n    return 1\n", i))
		gittest.Write(t, dir, fmt.Sprintf("tests/test_m%d.py", i), fmt.Sprintf("from src.m%d import f%d\n\ndef test_m%d():\n    assert f%d()\n", i, i, i, i))
	}
	gittest.Commit(t, dir, "fixture")
	e2eEdit(t, dir, "src/m0.py", "return 1", "return 2")
	code, out, errOut := rtdd(t, dir, "which")
	if code != 0 || strings.Contains(out+errOut, "more than half") {
		t.Fatalf("one change in twelve warns: %q %q", out, errOut)
	}
	for i := 1; i < 8; i++ {
		e2eEdit(t, dir, fmt.Sprintf("src/m%d.py", i), "return 1", "return 2")
	}
	code, out, errOut = rtdd(t, dir, "which")
	if code != 0 || !strings.Contains(out+errOut, "more than half") {
		t.Errorf("8 of 12 tests selected, no warning: %q %q", out, errOut)
	}
	_, js, _ := rtdd(t, dir, "which", "--json")
	if !strings.Contains(js, `"large_selection": true`) || !strings.Contains(js, "more than half") {
		t.Errorf("--json lacks the field or the warnings entry:\n%s", js)
	}
}
