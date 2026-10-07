package rounds

import (
	"reflect"
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
