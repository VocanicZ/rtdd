// Package graph is the one model rtdd selects from, whatever built it (spec §3): nodes
// are functions, methods, classes and tests; edges are code relationships.
package graph

import (
	"slices"
	"sort"
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
	sort.Slice(g.Nodes, func(i, j int) bool {
		a, b := g.Nodes[i], g.Nodes[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		return a.ID < b.ID
	})
	sort.Slice(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Relation < b.Relation
	})
	g.Edges = slices.Compact(g.Edges)
}
