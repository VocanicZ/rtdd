// Package rounds turns a node graph and the changed lines of a diff into the rounds an
// agent runs (spec §7). It is pure: it reads no file, runs no git and starts no process —
// the caller hands it the graph and the ranges, so every rule is testable on a graph
// built by hand.
package rounds

import (
	"cmp"
	"slices"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// LineRange is a 1-based, inclusive range of changed lines on the new side of a diff.
type LineRange struct{ Start, End int }

// ModuleName names the synthetic node a changed top-level line maps to (spec §7); its ID
// is "<file>::<module>".
const ModuleName = "<module>"

// KindModule is the synthetic node's kind. No graph source ever produces it.
const KindModule graph.Kind = "module"

// Result is spec §7's four sets. Every slice is non-nil and ordered by File, then Start,
// then ID.
type Result struct {
	ChangedNodes []graph.Node // the innermost node of each changed line, or the file's <module>
	Round1       []graph.Node // changed tests, and tests with an edge to a changed node
	Round2       []graph.Node // tests with an edge to a neighbour, minus Round1
	Untested     []graph.Node // changed non-test nodes no test in Round1 or Round2 reaches
}

// traversed is the relation set neighbours are reached over (spec §7) — all of §3's.
var traversed = func() map[graph.Relation]bool {
	m := map[graph.Relation]bool{}
	for _, r := range graph.Relations {
		m[r] = true
	}
	return m
}()

// index is the graph by ID, by file, and as adjacency lists in both directions.
type index struct {
	byID   map[string]graph.Node
	byFile map[string][]graph.Node // ordered by Start, then End descending (outer first)
	out    map[string][]graph.Edge
	in     map[string][]graph.Edge
}

func newIndex(g graph.Graph) *index {
	ix := &index{byID: map[string]graph.Node{}, byFile: map[string][]graph.Node{},
		out: map[string][]graph.Edge{}, in: map[string][]graph.Edge{}}
	for _, n := range g.Nodes {
		ix.byID[n.ID] = n
		ix.byFile[n.File] = append(ix.byFile[n.File], n)
	}
	for _, ns := range ix.byFile {
		slices.SortFunc(ns, outerFirst)
	}
	for _, e := range g.Edges {
		if !traversed[e.Relation] {
			continue
		}
		ix.out[e.From] = append(ix.out[e.From], e)
		ix.in[e.To] = append(ix.in[e.To], e)
	}
	return ix
}

// outerFirst is the order a file's nodes are painted onto its lines: by Start, then End
// descending, then ID. The node it puts last among those containing a line owns it.
func outerFirst(a, b graph.Node) int {
	if c := cmp.Compare(a.Start, b.Start); c != 0 {
		return c
	}
	if c := cmp.Compare(b.End, a.End); c != 0 {
		return c
	}
	return strings.Compare(a.ID, b.ID)
}

// owners maps each line of file up to maxLine to the innermost node containing it.
// Nodes are painted outer first, so a nested node overwrites its parent's lines.
func (ix *index) owners(file string, maxLine int) []string {
	own := make([]string, maxLine+1)
	for _, n := range ix.byFile[file] {
		for l := max(n.Start, 1); l <= min(n.End, maxLine); l++ {
			own[l] = n.ID
		}
	}
	return own
}

// Owner returns the innermost node of g containing file:line (spec §7's line-to-node
// rule), and false when the line is in no node.
func Owner(g graph.Graph, file string, line int) (graph.Node, bool) {
	var best graph.Node
	found := false
	for _, n := range g.Nodes {
		if n.File != file || line < n.Start || line > n.End {
			continue
		}
		if !found || outerFirst(n, best) > 0 {
			best, found = n, true
		}
	}
	return best, found
}

// Rounds computes spec §7 for the changed line ranges of each repo-relative file.
func Rounds(g graph.Graph, changed map[string][]LineRange) Result {
	ix := newIndex(g)
	changedSet := map[string]graph.Node{}
	moduleIn := map[string][]graph.Edge{} // a <module> node's callers, as edges into it
	round1 := map[string]graph.Node{}

	for file, ranges := range changed {
		nodes := ix.byFile[file]
		if len(nodes) == 0 || len(ranges) == 0 {
			continue // a file with no node holds nothing a test can call
		}
		maxLine := 0
		for _, r := range ranges {
			maxLine = max(maxLine, r.End)
		}
		own := ix.owners(file, maxLine)
		var mod *graph.Node
		for _, r := range ranges {
			for l := max(r.Start, 1); l <= r.End; l++ {
				if id := own[l]; id != "" {
					changedSet[id] = ix.byID[id]
					continue
				}
				if mod == nil {
					mod = &graph.Node{ID: file + "::" + ModuleName, File: file, Name: ModuleName, Kind: KindModule, Start: l, End: l}
				}
				mod.Start, mod.End = min(mod.Start, l), max(mod.End, l)
			}
		}
		if mod == nil {
			continue
		}
		for _, n := range nodes {
			if n.IsTest {
				mod.IsTest = true // a test file's top level: its tests are the changed tests
				round1[n.ID] = n
			}
			for _, e := range ix.in[n.ID] {
				if e.Relation == graph.RelCalls && ix.byID[e.From].File != file {
					moduleIn[mod.ID] = append(moduleIn[mod.ID], graph.Edge{From: e.From, To: mod.ID, Relation: graph.RelCalls})
				}
			}
		}
		changedSet[mod.ID] = *mod
	}

	in := func(id string) []graph.Edge {
		if es, ok := moduleIn[id]; ok {
			return es
		}
		return ix.in[id]
	}
	// testsOf is memoized: a widely called neighbour is reached from many changed nodes,
	// and re-walking its callers for each one made a large diff quadratic (#454).
	tests := map[string][]graph.Node{}
	testsOf := func(id string) []graph.Node {
		if out, ok := tests[id]; ok {
			return out
		}
		var out []graph.Node
		for _, e := range in(id) {
			if t, ok := ix.byID[e.From]; ok && t.IsTest {
				out = append(out, t)
			}
		}
		tests[id] = out
		return out
	}

	neighbours := map[string][]string{} // changed ID -> its non-test neighbours
	for id, c := range changedSet {
		if c.IsTest && c.Kind != KindModule {
			round1[id] = c
		}
		for _, t := range testsOf(id) {
			round1[t.ID] = t
		}
		for _, e := range ix.out[id] {
			if n, ok := ix.byID[e.To]; ok && !n.IsTest {
				neighbours[id] = append(neighbours[id], n.ID)
			}
		}
		for _, e := range in(id) {
			if n, ok := ix.byID[e.From]; ok && !n.IsTest {
				neighbours[id] = append(neighbours[id], n.ID)
			}
		}
	}

	round2 := map[string]graph.Node{}
	seen := map[string]bool{}
	for _, ns := range neighbours {
		for _, nb := range ns {
			if seen[nb] {
				continue
			}
			seen[nb] = true
			for _, t := range testsOf(nb) {
				if _, in1 := round1[t.ID]; !in1 {
					round2[t.ID] = t
				}
			}
		}
	}

	untested := map[string]graph.Node{}
	for id, c := range changedSet {
		if c.IsTest || len(testsOf(id)) > 0 {
			continue
		}
		reached := false
		for _, nb := range neighbours[id] {
			if len(testsOf(nb)) > 0 {
				reached = true
				break
			}
		}
		if !reached {
			untested[id] = c
		}
	}

	return Result{
		ChangedNodes: ordered(changedSet),
		Round1:       ordered(round1),
		Round2:       ordered(round2),
		Untested:     ordered(untested),
	}
}

// ordered is a set's nodes by File, then Start, then ID — never nil.
func ordered(set map[string]graph.Node) []graph.Node {
	out := make([]graph.Node, 0, len(set))
	for _, n := range set {
		out = append(out, n)
	}
	slices.SortFunc(out, byFileThenLine)
	return out
}

func byFileThenLine(a, b graph.Node) int {
	if c := strings.Compare(a.File, b.File); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Start, b.Start); c != 0 {
		return c
	}
	return strings.Compare(a.ID, b.ID)
}
