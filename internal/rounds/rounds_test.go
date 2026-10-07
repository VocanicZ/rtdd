package rounds

import (
	"go/parser"
	"go/token"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// n is a non-test node. path is the node's names below the file, outermost first
// ("Calc::plus"); the node's Name is its last segment, as internal/graph IDs it.
func n(file, path string, start, end int) graph.Node {
	name := path
	if i := strings.LastIndex(path, "::"); i >= 0 {
		name = path[i+2:]
	}
	return graph.Node{ID: file + "::" + path, File: file, Name: name, Kind: graph.KindFunc, Start: start, End: end}
}

// tn is a test node.
func tn(file, path string, start, end int) graph.Node {
	x := n(file, path, start, end)
	x.IsTest = true
	return x
}

func calls(from, to graph.Node) graph.Edge {
	return graph.Edge{From: from.ID, To: to.ID, Relation: graph.RelCalls}
}

func ids(ns []graph.Node) []string {
	out := []string{}
	for _, x := range ns {
		out = append(out, x.ID)
	}
	return out
}

type want struct{ changed, round1, round2, untested []string }

func check(t *testing.T, got Result, w want) {
	t.Helper()
	for _, c := range []struct {
		set       string
		got, want []string
	}{
		{"changed_nodes", ids(got.ChangedNodes), w.changed},
		{"round1", ids(got.Round1), w.round1},
		{"round2", ids(got.Round2), w.round2},
		{"untested", ids(got.Untested), w.untested},
	} {
		if c.want == nil {
			c.want = []string{}
		}
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s = %q, want %q", c.set, c.got, c.want)
		}
	}
}

func lines(path string, rs ...LineRange) map[string][]LineRange {
	return map[string][]LineRange{path: rs}
}

// Spec §7, changed_nodes: the innermost node containing each changed line.
func TestInnermostNodeOwnsAChangedLine(t *testing.T) {
	k := n("a.py", "K", 1, 10)
	k.Kind = graph.KindClass
	m := n("a.py", "K::m", 2, 6)
	step := n("a.py", "K::m::step", 3, 4)
	g := graph.Graph{Nodes: []graph.Node{k, m, step}}
	got := Rounds(g, lines("a.py", LineRange{3, 3}, LineRange{6, 6}, LineRange{9, 9}))
	check(t, got, want{
		changed:  []string{"a.py::K", "a.py::K::m", "a.py::K::m::step"},
		untested: []string{"a.py::K", "a.py::K::m", "a.py::K::m::step"},
	})
}

// Owner answers the same question for one line, for `rtdd explain file:line`.
func TestOwnerIsTheInnermostNodeOfALine(t *testing.T) {
	g := graph.Graph{Nodes: []graph.Node{n("a.py", "K", 1, 10), n("a.py", "K::m", 2, 6), n("a.py", "K::m::step", 3, 4)}}
	for line, want := range map[int]string{3: "a.py::K::m::step", 6: "a.py::K::m", 9: "a.py::K"} {
		if got, ok := Owner(g, "a.py", line); !ok || got.ID != want {
			t.Errorf("Owner(a.py:%d) = %q, %v; want %q", line, got.ID, ok, want)
		}
	}
	if _, ok := Owner(g, "a.py", 11); ok {
		t.Error("a line in no node has an owner")
	}
}

// Spec §7: a changed line in no node maps to file::<module>, whose callers are the nodes
// of OTHER files that call any node in that file.
func TestTopLevelLineIsTheModuleNodeAndItsCrossFileCallers(t *testing.T) {
	f := n("src/lib.py", "f", 3, 4)
	g2 := n("src/lib.py", "g", 6, 7)
	run := n("src/app.py", "run", 1, 2)
	testF := tn("tests/test_lib.py", "test_f", 1, 2)
	testG := tn("tests/test_lib.py", "test_g", 4, 5)
	testRun := tn("tests/test_app.py", "test_run", 1, 2)
	g := graph.Graph{
		Nodes: []graph.Node{f, g2, run, testF, testG, testRun},
		Edges: []graph.Edge{calls(g2, f), calls(run, f), calls(testF, f), calls(testG, g2), calls(testRun, run)},
	}
	got := Rounds(g, lines("src/lib.py", LineRange{1, 1}))
	check(t, got, want{
		changed: []string{"src/lib.py::<module>"},
		round1:  []string{"tests/test_lib.py::test_f", "tests/test_lib.py::test_g"},
		round2:  []string{"tests/test_app.py::test_run"},
	})
	mod := got.ChangedNodes[0]
	if mod.Name != ModuleName || mod.Kind != KindModule || mod.File != "src/lib.py" || mod.Start != 1 || mod.End != 1 {
		t.Errorf("module node = %+v, want name %q kind %q file src/lib.py lines 1-1", mod, ModuleName, KindModule)
	}
}

// A changed line in a file the graph holds no node for (README.md, a config file) is no
// changed node at all: there is nothing in it a test can call.
func TestALineInAFileWithNoNodeIsNoChangedNode(t *testing.T) {
	g := graph.Graph{Nodes: []graph.Node{n("src/a.py", "f", 1, 2)}}
	check(t, Rounds(g, lines("README.md", LineRange{1, 5})), want{})
}

// Spec §7, round1: changed test nodes.
func TestAChangedTestNodeIsInRound1(t *testing.T) {
	x := tn("tests/test_a.py", "test_x", 1, 3)
	check(t, Rounds(graph.Graph{Nodes: []graph.Node{x}}, lines("tests/test_a.py", LineRange{2, 2})), want{
		changed: []string{"tests/test_a.py::test_x"},
		round1:  []string{"tests/test_a.py::test_x"},
	})
}

// A top-level line of a test file (an import, a fixture) changes every test in it: the
// module node of a file holding test nodes selects them, and is never untested.
func TestATopLevelLineOfATestFileSelectsItsTests(t *testing.T) {
	a := tn("tests/test_m.py", "test_a", 3, 4)
	b := tn("tests/test_m.py", "test_b", 6, 7)
	check(t, Rounds(graph.Graph{Nodes: []graph.Node{a, b}}, lines("tests/test_m.py", LineRange{1, 1})), want{
		changed: []string{"tests/test_m.py::<module>"},
		round1:  []string{"tests/test_m.py::test_a", "tests/test_m.py::test_b"},
	})
}

// Spec §7: round1 by an edge to a changed node; round2 by an edge to a neighbour — a
// callee AND a caller (spec §12, #414: both directions).
func TestRound1IsTestsOfTheChangedNodeAndRound2TestsOfCallersAndCallees(t *testing.T) {
	a := n("src/m.py", "a", 1, 2)
	b := n("src/m.py", "b", 4, 5)
	tt := n("src/m.py", "t", 7, 9)
	c := n("src/m.py", "c", 11, 12)
	testA := tn("tests/test_m.py", "test_a", 1, 2)
	testB := tn("tests/test_m.py", "test_b", 4, 5)
	testC := tn("tests/test_m.py", "test_c", 7, 8)
	testT := tn("tests/test_m.py", "test_t", 10, 11)
	g := graph.Graph{
		Nodes: []graph.Node{a, b, tt, c, testA, testB, testC, testT},
		Edges: []graph.Edge{calls(tt, b), calls(c, tt), calls(testA, a), calls(testB, b), calls(testC, c), calls(testT, tt)},
	}
	check(t, Rounds(g, lines("src/m.py", LineRange{8, 8})), want{
		changed: []string{"src/m.py::t"},
		round1:  []string{"tests/test_m.py::test_t"},
		round2:  []string{"tests/test_m.py::test_b", "tests/test_m.py::test_c"},
	})
}

// Spec §7: neighbours are reached over calls|method|inherits|implements|references, in
// either direction.
func TestEveryRelationIsTraversedInBothDirections(t *testing.T) {
	for _, rel := range graph.Relations {
		for _, outward := range []bool{true, false} {
			t.Run(string(rel)+"/outward="+strconv.FormatBool(outward), func(t *testing.T) {
				k := n("x.py", "k", 1, 2)
				nb := n("x.py", "nb", 4, 5)
				testNb := tn("tests/test_x.py", "test_nb", 1, 2)
				e := graph.Edge{From: k.ID, To: nb.ID, Relation: rel}
				if !outward {
					e = graph.Edge{From: nb.ID, To: k.ID, Relation: rel}
				}
				g := graph.Graph{Nodes: []graph.Node{k, nb, testNb}, Edges: []graph.Edge{e, calls(testNb, nb)}}
				check(t, Rounds(g, lines("x.py", LineRange{1, 1})), want{
					changed: []string{"x.py::k"},
					round2:  []string{"tests/test_x.py::test_nb"},
				})
			})
		}
	}
}

// Spec §7: test nodes are never neighbours, so a test that only calls another test is
// in no round.
func TestATestNodeIsNeverANeighbour(t *testing.T) {
	k := n("src/k.py", "k", 1, 2)
	testK := tn("tests/test_k.py", "test_k", 1, 2)
	testOther := tn("tests/test_k.py", "test_other", 4, 5)
	g := graph.Graph{Nodes: []graph.Node{k, testK, testOther}, Edges: []graph.Edge{calls(testK, k), calls(k, testK), calls(testOther, testK)}}
	check(t, Rounds(g, lines("src/k.py", LineRange{1, 1})), want{
		changed: []string{"src/k.py::k"},
		round1:  []string{"tests/test_k.py::test_k"},
	})
}

// Spec §7: round2 is minus round1.
func TestRound2ExcludesRound1(t *testing.T) {
	tt := n("src/m.py", "t", 1, 2)
	b := n("src/m.py", "b", 4, 5)
	both := tn("tests/test_m.py", "test_both", 1, 3)
	g := graph.Graph{Nodes: []graph.Node{tt, b, both}, Edges: []graph.Edge{calls(tt, b), calls(both, tt), calls(both, b)}}
	check(t, Rounds(g, lines("src/m.py", LineRange{1, 1})), want{
		changed: []string{"src/m.py::t"},
		round1:  []string{"tests/test_m.py::test_both"},
	})
}

// Spec §7: untested is the changed non-test nodes with no test in round1 or round2 — a
// node is tested when a test links to it or to one of its neighbours.
func TestUntestedIsTheChangedNodesNoRoundReaches(t *testing.T) {
	lone := n("src/u.py", "lone", 1, 2)
	direct := n("src/u.py", "direct", 4, 5)
	viaNeighbour := n("src/u.py", "via_neighbour", 7, 8)
	b := n("src/u.py", "b", 10, 11)
	testDirect := tn("tests/test_u.py", "test_direct", 1, 2)
	testB := tn("tests/test_u.py", "test_b", 4, 5)
	g := graph.Graph{
		Nodes: []graph.Node{lone, direct, viaNeighbour, b, testDirect, testB},
		Edges: []graph.Edge{calls(testDirect, direct), calls(viaNeighbour, b), calls(testB, b)},
	}
	check(t, Rounds(g, lines("src/u.py", LineRange{1, 2}, LineRange{4, 5}, LineRange{7, 8})), want{
		changed:  []string{"src/u.py::lone", "src/u.py::direct", "src/u.py::via_neighbour"},
		round1:   []string{"tests/test_u.py::test_direct"},
		round2:   []string{"tests/test_u.py::test_b"},
		untested: []string{"src/u.py::lone"},
	})
}

// Spec §7: within a round tests are ordered by file then line, and the answer does not
// depend on the order the graph lists nodes and edges in.
func TestRoundsAreOrderedByFileThenLineAndDeterministic(t *testing.T) {
	tt := n("src/m.py", "t", 1, 2)
	var nodes []graph.Node
	var edges []graph.Edge
	nodes = append(nodes, tt)
	for _, f := range []string{"tests/z_test.py", "tests/a_test.py", "tests/m_test.py"} {
		for _, line := range []int{30, 10, 20} {
			x := tn(f, "test_"+strconv.Itoa(line), line, line+1)
			nodes = append(nodes, x)
			edges = append(edges, calls(x, tt))
		}
	}
	var wantR1 []string
	for _, f := range []string{"tests/a_test.py", "tests/m_test.py", "tests/z_test.py"} {
		for _, line := range []int{10, 20, 30} {
			wantR1 = append(wantR1, f+"::test_"+strconv.Itoa(line))
		}
	}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		rng.Shuffle(len(nodes), func(a, b int) { nodes[a], nodes[b] = nodes[b], nodes[a] })
		rng.Shuffle(len(edges), func(a, b int) { edges[a], edges[b] = edges[b], edges[a] })
		g := graph.Graph{Nodes: append([]graph.Node(nil), nodes...), Edges: append([]graph.Edge(nil), edges...)}
		check(t, Rounds(g, lines("src/m.py", LineRange{1, 1})), want{changed: []string{"src/m.py::t"}, round1: wantR1})
	}
}

// No changed range is an empty answer, not an error, and every slice is non-nil so
// `--json` prints [] rather than null.
func TestNoChangedRangeIsEmptyRounds(t *testing.T) {
	g := graph.Graph{Nodes: []graph.Node{n("a.py", "f", 1, 2)}}
	for _, changed := range []map[string][]LineRange{nil, {}, {"a.py": nil}} {
		got := Rounds(g, changed)
		if got.ChangedNodes == nil || got.Round1 == nil || got.Round2 == nil || got.Untested == nil {
			t.Errorf("Rounds(%v) has a nil slice: %+v", changed, got)
		}
		check(t, got, want{})
	}
}

// Rounds is pure (issue #446): its package imports internal/graph and the standard
// library's pure packages, nothing that can touch a file, git or a process.
func TestRoundsPackageImportsNoIO(t *testing.T) {
	allowed := map[string]bool{"cmp": true, "slices": true, "strings": true, "github.com/VocanicZ/rtdd/internal/graph": true}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), e.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if !allowed[p] {
				t.Errorf("%s imports %q; internal/rounds may import only %v", e.Name(), p, allowed)
			}
		}
	}
}

// Two nodes spanning the same lines (same-named overloads, a decorator the scanner reads
// twice) have one owner, whatever order the graph lists them in, and Owner agrees with
// the owner Rounds reports as changed.
func TestOwnerOfTwoNodesWithOneSpanIsTheSameInEveryOrder(t *testing.T) {
	x := n("a.py", "x", 1, 5)
	y := n("a.py", "y", 1, 5)
	for _, nodes := range [][]graph.Node{{x, y}, {y, x}} {
		g := graph.Graph{Nodes: nodes}
		changed := Rounds(g, lines("a.py", LineRange{3, 3})).ChangedNodes
		got, ok := Owner(g, "a.py", 3)
		if !ok || len(changed) != 1 || got.ID != changed[0].ID {
			t.Errorf("nodes %q: Owner = %q, Rounds changed = %q; want one and the same", ids(nodes), got.ID, ids(changed))
		}
		if got.ID != y.ID {
			t.Errorf("nodes %q: Owner = %q, want %q", ids(nodes), got.ID, y.ID)
		}
	}
}
