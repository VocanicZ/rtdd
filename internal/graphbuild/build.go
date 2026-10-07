// Package graphbuild assembles the node graph a command selects from: the scanner's
// graph, cached in .rtdd/graph.json, overlaid on graphify's when the project has one and
// it can be trusted (spec §4.5, §5). It is the only graph package that talks to git, and
// it does so only through internal/gitctx.
package graphbuild

import (
	"errors"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphify"
	"github.com/VocanicZ/rtdd/internal/scan"
)

// Sources of a built graph (spec §5, `graph.source`).
const (
	SourceScanner         = "scanner"
	SourceGraphifyScanner = "graphify+scanner"
)

// Why graphify was ignored entirely (spec §5 step 4). The JSON carries the code; the
// human output explains it.
const (
	IgnoredNoCommit      = "built_at_commit_missing"
	IgnoredUnknownCommit = "built_at_commit_unknown"
	IgnoredTooStale      = "too_stale"
)

// Options tune a build. The zero value is the CLI's behaviour.
type Options struct {
	CachePath string // "" is <root>/.rtdd/graph.json
}

// Result is a built graph and how it was built. Every field is declared here, in Task 6,
// so Tasks 7, 8 and 9 — which may land in either order — only fill them.
type Result struct {
	Graph           graph.Graph // IsTest set on every node
	Source          string      // SourceScanner | SourceGraphifyScanner
	BuiltAtCommit   string      // graphify's when it is used, else HEAD's short sha ("" on an unborn HEAD)
	StaleFiles      []string    // files graphify was not trusted for, sorted; empty for SourceScanner
	GraphifyIgnored string      // an Ignored* code, or "" when graphify was used or absent
	GraphifyCommit  string      // graphify's built_at_commit as it recorded it, when it was read
	GraphifyFiles   int         // graphify's code-file count, when it was read
	Scanned         []string    // files the scanner read on this call, sorted
}

// Build builds the graph for the repository at root.
func Build(root string, cfg graph.Config, opt Options) (*Result, error) {
	listed, err := gitctx.ListFiles(root)
	if err != nil {
		return nil, err
	}
	files := scan.Filter(root, listed, cfg.ScanExclude)
	head, _ := gitctx.HeadSHA(root) // "" on an unborn HEAD

	res := &Result{Source: SourceScanner, BuiltAtCommit: head, Scanned: files}
	gf, err := graphify.Load(root, cfg.GraphifyPath)
	switch {
	case errors.Is(err, graphify.ErrAbsent):
		res.Graph = scan.Assemble(scan.ScanFiles(root, files))
	case err != nil:
		return nil, err
	default:
		if err := useGraphify(root, cfg, gf, files, res); err != nil {
			return nil, err
		}
	}
	graph.Classify(res.Graph.Nodes, cfg)
	return res, nil
}

// useGraphify fills res from graphify's graph: the stale files (spec §5 step 1) are
// read by the scanner and overlaid (steps 2-3), or, when staleness ignores graphify
// entirely (step 4), the scanner reads every file.
func useGraphify(root string, cfg graph.Config, gf *graphify.Graph, files []string, res *Result) error {
	res.GraphifyCommit, res.GraphifyFiles = gf.BuiltAtCommit, len(gf.CodeFiles)
	changed, err := changedSet(root)
	if err != nil {
		return err
	}
	stale, reason, err := staleness(root, gf, files, changed, cfg.MaxStaleRatio)
	if err != nil {
		return err
	}
	if reason != "" {
		res.GraphifyIgnored = reason
		res.Graph = scan.Assemble(scan.ScanFiles(root, files))
		return nil
	}
	isStale := map[string]bool{}
	for _, f := range stale {
		isStale[f] = true
	}
	var toScan []string
	for _, f := range files {
		if isStale[f] {
			toScan = append(toScan, f)
		}
	}
	res.Source, res.BuiltAtCommit, res.StaleFiles, res.Scanned = SourceGraphifyScanner, gf.BuiltAtCommit, stale, toScan
	res.Graph = Overlay(gf, isStale, scan.ScanFiles(root, toScan))
	return nil
}

// changedSet is the working-tree changed set against HEAD as a path set, untracked files
// included (spec §5 step 1's "current changed set" and "untracked").
func changedSet(root string) (map[string]bool, error) {
	cs, err := gitctx.ChangedSet(root, "HEAD")
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{}
	for _, c := range cs {
		changed[c.Path] = true
	}
	return changed, nil
}
