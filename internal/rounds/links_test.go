package rounds

import (
	"reflect"
	"slices"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

func linkIDs(ls []Link) []string {
	out := []string{}
	for _, l := range ls {
		out = append(out, l.Node.ID+" "+string(l.Relation))
	}
	return out
}

// LinksOf splits a node's in-edges into tests and callers, lists its out-edges as callees,
// and names each edge's relation; every list is ordered by file, then line.
func TestLinksOfSplitsTestsCallersAndCallees(t *testing.T) {
	add := n("src/calc.py", "add", 1, 2)
	total := n("src/calc.py", "total", 5, 6)
	report := n("src/report.py", "report", 4, 5)
	base := n("src/a.py", "Base", 1, 3)
	testTotal := tn("tests/test_calc.py", "test_total", 8, 9)
	testZ := tn("tests/a_test.py", "test_z", 1, 2)
	g := graph.Graph{
		Nodes: []graph.Node{add, total, report, base, testTotal, testZ},
		Edges: []graph.Edge{
			calls(total, add),
			calls(report, total),
			calls(testTotal, total),
			calls(testZ, total),
			pair{From: total, To: base}.edge(graph.RelInherits),
		},
	}
	got := LinksOf(g, total.ID)
	for _, c := range []struct {
		name      string
		got, want []string
	}{
		{"tests", linkIDs(got.Tests), []string{"tests/a_test.py::test_z calls", "tests/test_calc.py::test_total calls"}},
		{"callers", linkIDs(got.Callers), []string{"src/report.py::report calls"}},
		{"callees", linkIDs(got.Callees), []string{"src/a.py::Base inherits", "src/calc.py::add calls"}},
	} {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// An unknown ID, or a node with no edges, has empty — never nil — lists.
func TestLinksOfAnUnknownNodeIsEmptyNotNil(t *testing.T) {
	got := LinksOf(graph.Graph{Nodes: []graph.Node{n("a.py", "f", 1, 2)}}, "nope")
	if got.Tests == nil || got.Callers == nil || got.Callees == nil {
		t.Errorf("LinksOf(unknown) = %#v; want non-nil empty lists", got)
	}
}

type pair struct{ From, To graph.Node }

func (p pair) edge(r graph.Relation) graph.Edge {
	return graph.Edge{From: p.From.ID, To: p.To.ID, Relation: r}
}

// The Linker answers every lookup from one index and agrees with LinksOf for each node of a
// graph — including a test file's file node with an edge, whose edge counts as from each of
// the file's tests (#504) — and for an unknown ID, which has non-nil empty lists.
func TestLinkerMatchesLinksOf(t *testing.T) {
	add := n("src/calc.py", "add", 1, 2)
	total := n("src/calc.py", "total", 5, 6)
	report := n("src/report.py", "report", 4, 5)
	testAdd := tn("tests/test_calc.py", "test_add", 4, 5)
	testTotal := tn("tests/test_calc.py", "test_total", 8, 9)
	testFile := graph.Node{ID: "tests/test_calc.py", File: "tests/test_calc.py", Name: "test_calc.py", Kind: graph.KindFile, Start: 1, End: 9, IsTest: true}
	g := graph.Graph{
		Nodes: []graph.Node{add, total, report, testAdd, testTotal, testFile},
		Edges: []graph.Edge{
			calls(total, add),
			calls(report, total),
			calls(testTotal, total),
			calls(testFile, add),
		},
	}
	if got := LinksOf(g, add.ID); !slices.Contains(linkIDs(got.Tests), testTotal.ID+" calls") {
		t.Fatalf("fixture misses the #504 testsIn branch: tests of add = %v", linkIDs(got.Tests))
	}
	k := NewLinker(g)
	for _, node := range g.Nodes {
		if got, want := k.Links(node.ID), LinksOf(g, node.ID); !reflect.DeepEqual(got, want) {
			t.Errorf("Linker.Links(%s) = %#v, want %#v", node.ID, got, want)
		}
	}
	got := k.Links("nope")
	if got.Tests == nil || got.Callers == nil || got.Callees == nil {
		t.Errorf("Linker.Links(unknown) = %#v; want non-nil empty lists", got)
	}
	if !reflect.DeepEqual(got, LinksOf(g, "nope")) {
		t.Errorf("Linker.Links(unknown) = %#v, LinksOf = %#v", got, LinksOf(g, "nope"))
	}
}
