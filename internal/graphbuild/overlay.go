package graphbuild

import (
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphify"
	"github.com/VocanicZ/rtdd/internal/scan"
)

// Overlay is spec §5 steps 2-3. graphify's nodes and outgoing edges in a stale file are
// dropped and the scanner's scan of that file (in scanned) stands in. A graphify edge
// from an unchanged file INTO a stale file is re-pointed to the scanner node(s) of the
// same name in that file, or dropped when there is none (renamed, deleted). The
// scanner's calls, imports and supertypes are then linked against the whole merged graph.
func Overlay(gf *graphify.Graph, stale map[string]bool, scanned []scan.FileResult) graph.Graph {
	g, _ := overlay(gf, stale, scanned)
	return g
}

// overlay is Overlay and what its link resolved.
func overlay(gf *graphify.Graph, stale map[string]bool, scanned []scan.FileResult) (graph.Graph, scan.Stats) {
	var g graph.Graph
	gnode := map[string]graph.Node{}
	for _, n := range gf.Nodes {
		gnode[n.ID] = n
		if !stale[n.File] {
			g.Nodes = append(g.Nodes, n)
		}
	}
	byFileName := map[[2]string][]string{}
	for _, r := range scanned {
		g.Nodes = append(g.Nodes, r.Nodes...)
		g.Edges = append(g.Edges, r.Edges...)
		for _, n := range r.Nodes {
			k := [2]string{n.File, n.Name}
			byFileName[k] = append(byFileName[k], n.ID)
		}
	}
	for _, e := range gf.Edges {
		from, to := gnode[e.From], gnode[e.To]
		switch {
		case stale[from.File]:
			continue
		case !stale[to.File]:
			g.Edges = append(g.Edges, e)
		default:
			for _, id := range byFileName[[2]string{to.File, to.Name}] {
				g.Edges = append(g.Edges, graph.Edge{From: e.From, To: id, Relation: e.Relation})
			}
		}
	}
	edges, st := scan.Link(g.Nodes, scan.RefsOf(scanned))
	g.Edges = append(g.Edges, edges...)
	graph.Sort(&g)
	return g, st
}
