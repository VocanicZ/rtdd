package main

import (
	"cmp"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
	"github.com/VocanicZ/rtdd/internal/paths"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// fileLine is an explain argument naming a line: "src/calc.py:6".
var fileLine = regexp.MustCompile(`^(.+):([0-9]+)$`)

// cmdExplain implements `rtdd explain <file[:line]|name>` (spec §8): for each node the
// argument names, its tests, callers and callees, from the graph. It runs nothing.
func cmdExplain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: rtdd explain <file[:line]|name>")
		return 2
	}
	root, err := graphRoot()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd explain: %v\n", err)
		return 3
	}
	// A file:line is resolved with real spans: the named file is rescanned rather than
	// taken from graphify, whose nodes span one line (spec §5, #472).
	var opt graphbuild.Options
	if m := fileLine.FindStringSubmatch(fs.Arg(0)); m != nil {
		opt.Rescan = []string{repoRel(root, m[1])}
	}
	_, res, code, err := buildGraph(root, opt)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd explain: %v\n", err)
		return code
	}
	nodes := explainTargets(root, res.Graph, fs.Arg(0))
	if len(nodes) == 0 {
		fmt.Fprintf(stderr, "rtdd explain: no node matches %q (give a file, file:line, node id or name)\n", fs.Arg(0))
		return 2
	}
	for i, n := range nodes {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprint(stdout, renderExplain(n, rounds.LinksOf(res.Graph, n.ID)))
	}
	return 0
}

// explainTargets resolves the argument, in order: a node ID; file:line (the innermost
// node owning that line); a file (all its nodes); a name (every node so named — all
// same-named definitions, spec §12). A path is relative to the caller's directory.
func explainTargets(root string, g graph.Graph, arg string) []graph.Node {
	var out []graph.Node
	for _, n := range g.Nodes {
		if n.ID == arg {
			return []graph.Node{n}
		}
	}
	if m := fileLine.FindStringSubmatch(arg); m != nil {
		line, _ := strconv.Atoi(m[2])
		if n, ok := rounds.Owner(g, repoRel(root, m[1]), line); ok {
			return []graph.Node{n}
		}
		return nil
	}
	file := repoRel(root, arg)
	for _, n := range g.Nodes {
		if n.File == file {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		for _, n := range g.Nodes {
			if n.Name == arg {
				out = append(out, n)
			}
		}
	}
	slices.SortFunc(out, func(a, b graph.Node) int {
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Start, b.Start), strings.Compare(a.ID, b.ID))
	})
	return out
}

// repoRel is p, a path relative to the caller's directory, relative to root.
func repoRel(root, p string) string {
	abs := p
	if !filepath.IsAbs(abs) {
		wd, _ := os.Getwd()
		abs = filepath.Join(wd, p)
	}
	r, _ := paths.Normalize(root, abs)
	return r
}

func renderExplain(n graph.Node, l rounds.Links) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  (%s, lines %d-%d)\n", n.ID, n.Kind, n.Start, n.End)
	for _, sec := range []struct {
		head  string
		links []rounds.Link
	}{{"tests", l.Tests}, {"callers", l.Callers}, {"callees", l.Callees}} {
		fmt.Fprintf(&b, "  %s:\n", sec.head)
		if len(sec.links) == 0 {
			b.WriteString("    none\n")
		}
		for _, x := range sec.links {
			fmt.Fprintf(&b, "    %s  (%s:%d, %s)\n", x.Node.ID, x.Node.File, x.Node.Start, x.Relation)
		}
	}
	return b.String()
}
