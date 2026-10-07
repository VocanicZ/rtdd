// Package graph is the one model rtdd selects from, whatever built it (spec §3): nodes
// are functions, methods, classes and tests; edges are code relationships.
package graph

import (
	"cmp"
	"slices"
	"strings"
)

// Kind is what a node is. Only the scanner and the graphify loader assign it.
type Kind string

const (
	KindFunc   Kind = "func"
	KindMethod Kind = "method"
	KindClass  Kind = "class"
	KindTest   Kind = "test"
)

// Relation is an edge's type. The set is closed: anything else is dropped on load.
type Relation string

const (
	RelCalls      Relation = "calls"
	RelMethod     Relation = "method"
	RelInherits   Relation = "inherits"
	RelImplements Relation = "implements"
	RelReferences Relation = "references"
)

// Relations is the closed relation set of spec §3, in spec order.
var Relations = []Relation{RelCalls, RelMethod, RelInherits, RelImplements, RelReferences}

// ParseRelation reports whether s names a relation in the closed set.
func ParseRelation(s string) (Relation, bool) {
	for _, r := range Relations {
		if string(r) == s {
			return r, true
		}
	}
	return "", false
}

// Node is one function, method, class or test. File is repo-relative and slash-separated;
// Start and End are 1-based and inclusive.
type Node struct {
	ID     string
	File   string
	Name   string
	Kind   Kind
	Start  int
	End    int
	IsTest bool
}

// Edge points From one node ID To another.
type Edge struct {
	From     string
	To       string
	Relation Relation
}

// Graph is nodes and edges. Nodes are ordered by File, then Start; Edges by From, To,
// Relation — so two builds of the same tree compare equal.
type Graph struct {
	Nodes []Node
	Edges []Edge
}

// Sort puts g in its canonical order: nodes by File then Start then ID, edges by From,
// To, Relation, with duplicate edges removed.
func Sort(g *Graph) {
	slices.SortFunc(g.Nodes, func(a, b Node) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Start, b.Start); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	slices.SortFunc(g.Edges, func(a, b Edge) int {
		if c := strings.Compare(a.From, b.From); c != 0 {
			return c
		}
		if c := strings.Compare(a.To, b.To); c != 0 {
			return c
		}
		return strings.Compare(string(a.Relation), string(b.Relation))
	})
	g.Edges = slices.Compact(g.Edges)
}
