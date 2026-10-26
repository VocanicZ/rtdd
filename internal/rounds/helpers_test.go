package rounds

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// hn is a helper: a func in a test file that is not a test.
func hn(file, path string, start, end int) graph.Node {
	x := n(file, path, start, end)
	x.Helper = true
	return x
}

// Issue #505: a helper is never listed; a test that calls it reaches what it calls, in
// Round 1, and a change to the helper itself selects the tests that call it.
func TestHelperIsTransparentToTheTestsThatCallIt(t *testing.T) {
	code := n("src/calc.py", "add", 1, 2)
	helper := hn("tests/test_calc.py", "make", 1, 2)
	test := tn("tests/test_calc.py", "test_add", 4, 5)
	g := graph.Graph{Nodes: []graph.Node{code, helper, test}, Edges: []graph.Edge{calls(test, helper), calls(helper, code)}}

	check(t, Rounds(g, lines("src/calc.py", LineRange{2, 2})), want{
		changed: []string{"src/calc.py::add"},
		round1:  []string{"tests/test_calc.py::test_add"},
	})
	check(t, Rounds(g, lines("tests/test_calc.py", LineRange{2, 2})), want{
		changed: []string{"tests/test_calc.py::make"},
		round1:  []string{"tests/test_calc.py::test_add"},
	})
}

// Issue #505: a helper no test calls (a fixture, a setUp) is run by running its file.
func TestHelperNoTestCallsSelectsItsWholeFile(t *testing.T) {
	code := n("src/calc.py", "add", 1, 2)
	fixture := hn("tests/test_calc.py", "calc", 1, 2)
	test := tn("tests/test_calc.py", "test_other", 4, 5)
	g := graph.Graph{Nodes: []graph.Node{code, fixture, test}, Edges: []graph.Edge{calls(fixture, code)}}

	check(t, Rounds(g, lines("src/calc.py", LineRange{2, 2})), want{
		changed: []string{"src/calc.py::add"},
		round1:  []string{"tests/test_calc.py"},
	})
}

// Issue #505: in a whole-file test the stubs are helpers under one file node; a change
// any of them reaches selects the file path, once — no `::git`, no `@line`.
func TestWholeFileTestIsSelectedOnceByItsPath(t *testing.T) {
	const f = "test/test_drive.sh"
	code := n("lib/drive.sh", "spawn", 1, 3)
	file := graph.WholeFileNode(f)
	git := hn(f, "git", 3, 5)
	gh := hn(f, "gh", 7, 9)
	gh2 := hn(f, "gh@11", 11, 13)
	gh2.Name = "gh"
	spawn := hn(f, "spawn_impl", 15, 17)
	recursive := hn(f, "retry", 19, 21)
	g := graph.Graph{
		Nodes: []graph.Node{code, file, git, gh, gh2, spawn, recursive},
		Edges: []graph.Edge{calls(file, git), calls(file, gh), calls(file, gh2), calls(file, spawn), calls(file, recursive),
			calls(gh, code), calls(gh2, code), calls(spawn, code), calls(recursive, recursive), calls(recursive, code)},
	}
	check(t, Rounds(g, lines("lib/drive.sh", LineRange{2, 2})), want{
		changed: []string{"lib/drive.sh::spawn"},
		round1:  []string{f},
	})
	check(t, Rounds(g, lines(f, LineRange{1, 1})), want{
		changed: []string{f + "::<module>"},
		round1:  []string{f},
		round2:  nil,
	})
}

// Issue #505: a runnable id drops the scanner's "@line" suffix, and a round lists it once.
func TestRoundListsEachRunnableIDOnce(t *testing.T) {
	code := n("src/a.js", "f", 1, 2)
	t1 := tn("src/a.test.js", "works", 1, 3)
	t2 := tn("src/a.test.js", "works@5", 5, 7)
	t2.Name = "works"
	g := graph.Graph{Nodes: []graph.Node{code, t1, t2}, Edges: []graph.Edge{calls(t1, code), calls(t2, code)}}

	check(t, Rounds(g, lines("src/a.js", LineRange{2, 2})), want{
		changed: []string{"src/a.js::f"},
		round1:  []string{"src/a.test.js::works"},
	})
}
