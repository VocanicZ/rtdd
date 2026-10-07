package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
)

// graphJSON is `rtdd graph --json`: the spec §9 envelope (schema 3) with only the
// `graph` object, which is the same object `rtdd which` will carry once N2 lands.
type graphJSON struct {
	Schema  int         `json:"schema"`
	Command string      `json:"command"`
	Graph   graphObject `json:"graph"`
}

type graphObject struct {
	Source          string `json:"source"`
	BuiltAtCommit   string `json:"built_at_commit"`
	StaleFiles      int    `json:"stale_files"`
	GraphifyIgnored string `json:"graphify_ignored,omitempty"`
	Nodes           int    `json:"nodes"`
	Edges           int    `json:"edges"`
	Tests           int    `json:"tests"`
}

// cmdGraph builds or refreshes the node graph and reports what it holds (spec §8). It
// runs no test and changes nothing but .rtdd/graph.json.
func cmdGraph(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON (schema 3)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd graph [--json]")
		return 2
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd graph: %v\n", err)
		return 3
	}
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd graph: not inside a git work tree, or git is unavailable: %v\n", err)
		return 3
	}
	cfg, err := graph.LoadConfig(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd graph: %v\n", err)
		return 2
	}
	res, err := graphbuild.Build(root, cfg, graphbuild.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "rtdd graph: %v\n", err)
		return 3
	}

	obj := graphObject{Source: res.Source, BuiltAtCommit: res.BuiltAtCommit, StaleFiles: len(res.StaleFiles),
		GraphifyIgnored: res.GraphifyIgnored, Nodes: len(res.Graph.Nodes), Edges: len(res.Graph.Edges)}
	for _, n := range res.Graph.Nodes {
		if n.IsTest {
			obj.Tests++
		}
	}
	if *asJSON {
		b, _ := json.MarshalIndent(graphJSON{Schema: 3, Command: "graph", Graph: obj}, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return 0
	}
	fmt.Fprintf(stdout, "source:          %s\n", obj.Source)
	fmt.Fprintf(stdout, "nodes:           %d\n", obj.Nodes)
	fmt.Fprintf(stdout, "edges:           %d\n", obj.Edges)
	fmt.Fprintf(stdout, "tests:           %d\n", obj.Tests)
	fmt.Fprintf(stdout, "built_at_commit: %s\n", obj.BuiltAtCommit)
	fmt.Fprintf(stdout, "stale_files:     %d\n", obj.StaleFiles)
	if res.GraphifyIgnored != "" {
		fmt.Fprintf(stdout, "graphify:        ignored — %s; run `graphify --update` to use it again\n", ignoredWhy(res, cfg))
	}
	return 0
}

// ignoredWhy is the human sentence for a graphbuild.Ignored* code.
func ignoredWhy(res *graphbuild.Result, cfg graph.Config) string {
	switch res.GraphifyIgnored {
	case graphbuild.IgnoredNoCommit:
		return "its graph records no built_at_commit"
	case graphbuild.IgnoredUnknownCommit:
		return fmt.Sprintf("its built_at_commit %s is unknown to git", res.GraphifyCommit)
	case graphbuild.IgnoredTooStale:
		return fmt.Sprintf("%d of its %d code files are stale, more than max_stale_ratio %.2f",
			res.StaleCodeFiles, res.GraphifyFiles, cfg.MaxStaleRatio)
	}
	return res.GraphifyIgnored
}
