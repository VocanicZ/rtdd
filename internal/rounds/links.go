package rounds

import (
	"slices"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// Link is a node one edge away, and the relation of that edge.
type Link struct {
	Node     graph.Node
	Relation graph.Relation
}

// Links is one node's depth-1 surroundings, as `rtdd explain` prints them: the tests with
// an edge to it, the non-test nodes with an edge to it, and the nodes it has an edge to.
// Each list is ordered by File, then Start, then ID, then Relation, and is non-nil.
type Links struct {
	Tests   []Link
	Callers []Link
	Callees []Link
}

// LinksOf returns the links of the node with the given ID; an unknown ID has none.
func LinksOf(g graph.Graph, id string) Links {
	ix := newIndex(g)
	l := Links{Tests: []Link{}, Callers: []Link{}, Callees: []Link{}}
	for _, e := range ix.in[id] {
		if n, ok := ix.byID[e.From]; ok {
			if n.IsTest {
				l.Tests = append(l.Tests, Link{n, e.Relation})
			} else {
				l.Callers = append(l.Callers, Link{n, e.Relation})
			}
		}
	}
	for _, e := range ix.out[id] {
		if n, ok := ix.byID[e.To]; ok {
			l.Callees = append(l.Callees, Link{n, e.Relation})
		}
	}
	for _, ls := range [][]Link{l.Tests, l.Callers, l.Callees} {
		slices.SortFunc(ls, func(a, b Link) int {
			if c := byFileThenLine(a.Node, b.Node); c != 0 {
				return c
			}
			return strings.Compare(string(a.Relation), string(b.Relation))
		})
	}
	return l
}
