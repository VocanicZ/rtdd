package main

import (
	"cmp"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
	"github.com/VocanicZ/rtdd/internal/scan"
)

// overLinkDefinitions is how many definitions of one name make it an over-link hub:
// every call to the name links to all of them (spec §4.3, §12, #417).
const overLinkDefinitions = 8

// cmdDoctor implements `rtdd doctor` (spec §8): where the graph came from, how stale
// graphify is, what the test_files globs found, and the names over-linked enough to
// widen every round that touches them. It runs nothing; it changes only the graph cache.
func cmdDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd doctor")
		return 2
	}
	root, err := graphRoot()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", err)
		return 3
	}
	cfg, res, code, err := buildGraph(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", err)
		return code
	}
	listed, err := gitctx.ListFiles(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", err)
		return 3
	}
	fmt.Fprint(stdout, renderDoctor(res, cfg, scan.Filter(root, listed, cfg.ScanExclude)))
	return 0
}

func renderDoctor(res *graphbuild.Result, cfg graph.Config, files []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "graph source:  %s\n", res.Source)
	fmt.Fprintf(&b, "built at:      %s\n", res.BuiltAtCommit)
	fmt.Fprintf(&b, "graphify:      %s\n", graphifyState(res, cfg))
	fmt.Fprintf(&b, "nodes:         %d\n", len(res.Graph.Nodes))
	fmt.Fprintf(&b, "edges:         %d\n", len(res.Graph.Edges))

	testFiles, withTests, testNodes := 0, map[string]bool{}, 0
	for _, f := range files {
		if graph.IsTestFile(f, cfg) {
			testFiles++
		}
	}
	for _, n := range res.Graph.Nodes {
		if n.IsTest {
			testNodes++
			withTests[n.File] = true
		}
	}
	fmt.Fprintf(&b, "test files:    %d matched by test_files, %d with a test node\n", testFiles, len(withTests))
	fmt.Fprintf(&b, "test nodes:    %d\n", testNodes)

	fmt.Fprintf(&b, "names defined %d or more times (a call to one links to every definition):\n", overLinkDefinitions)
	hubs := overLinked(res.Graph, overLinkDefinitions)
	if len(hubs) == 0 {
		b.WriteString("  none\n")
	}
	for _, h := range hubs {
		fmt.Fprintf(&b, "  %s  %d\n", h.name, h.count)
	}
	return b.String()
}

// graphifyState is doctor's one line on graphify: absent, used (and how stale), or
// ignored (and why).
func graphifyState(res *graphbuild.Result, cfg graph.Config) string {
	switch {
	case res.GraphifyIgnored != "":
		return fmt.Sprintf("ignored — %s; run `graphify --update` to use it again", ignoredWhy(res, cfg))
	case res.Source == graphbuild.SourceGraphifyScanner:
		return fmt.Sprintf("used, built at %s — %d stale %s, %d of its %d code files (max_stale_ratio %.2f)",
			res.GraphifyCommit, len(res.StaleFiles), plural(len(res.StaleFiles), "file", "files"),
			res.StaleCodeFiles, res.GraphifyFiles, cfg.MaxStaleRatio)
	}
	return "not present (" + cfg.GraphifyPath + ")"
}

type nameCount struct {
	name  string
	count int
}

// overLinked is every non-test name defined at least atLeast times, most-defined first.
func overLinked(g graph.Graph, atLeast int) []nameCount {
	counts := map[string]int{}
	for _, n := range g.Nodes {
		if !n.IsTest {
			counts[n.Name]++
		}
	}
	var out []nameCount
	for name, c := range counts {
		if c >= atLeast {
			out = append(out, nameCount{name, c})
		}
	}
	slices.SortFunc(out, func(a, b nameCount) int {
		if c := cmp.Compare(b.count, a.count); c != 0 {
			return c
		}
		return strings.Compare(a.name, b.name)
	})
	return out
}
