// Package graphbuild assembles the node graph a command selects from: the scanner's
// graph, cached in .rtdd/graph.json, overlaid on graphify's when the project has one and
// it can be trusted (spec §4.5, §5). It is the only graph package that talks to git, and
// it does so only through internal/gitctx.
package graphbuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphify"
	"github.com/VocanicZ/rtdd/internal/paths"
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
	// Base is the ref the caller measures changes from (`rtdd which --base`); "" is HEAD.
	// Files changed against it join graphify's stale set (spec §5 step 1, "the current
	// changed set"), so no changed line maps through graphify's start-only spans.
	Base string
	// Rescan names files (repo-relative) the scanner reads for this build whatever
	// graphify holds for them, as if changed (`rtdd explain <file>:<line>`), so a line
	// inside them never maps through graphify's start-only spans (spec §5).
	Rescan []string
}

// Result is a built graph and how it was built. Every field is declared here, in Task 6,
// so Tasks 7, 8 and 9 — which may land in either order — only fill them.
type Result struct {
	Graph           graph.Graph // IsTest set on every node
	Source          string      // SourceScanner | SourceGraphifyScanner
	BuiltAtCommit   string      // graphify's when it is used, else HEAD's short sha ("" on an unborn HEAD)
	StaleFiles      []string    // files graphify was not trusted for, sorted; empty for SourceScanner unless IgnoredTooStale
	StaleCodeFiles  int         // how many of StaleFiles are code files: max_stale_ratio's numerator
	GraphifyIgnored string      // an Ignored* code, or "" when graphify was used or absent
	GraphifyCommit  string      // graphify's built_at_commit as it recorded it, when it was read
	GraphifyFiles   int         // graphify's code-file count, when it was read
	Scanned         []string    // files the scanner read on this call, sorted
	Imports         int         // file -> file imports edges the scanner resolved (spec §4.3)
	AmbiguousCalls  int         // calls linked to no definition because too many matched (spec §4.3)
	CacheWarm       bool        // .rtdd/graph.json was read and usable, so only changed files were scanned
}

// Build builds the graph for the repository at root.
func Build(root string, cfg graph.Config, opt Options) (*Result, error) {
	cachePath := opt.CachePath
	if cachePath == "" {
		cachePath = filepath.Join(root, ".rtdd", "graph.json")
	}
	listed, err := gitctx.ListFiles(root)
	if err != nil {
		return nil, err
	}
	files := scan.Filter(root, listed, cfg.ScanExclude)
	head, _ := gitctx.HeadSHA(root) // "" on an unborn HEAD
	changed, err := changedSet(root, "HEAD")
	if err != nil {
		return nil, err
	}
	blobs, err := gitctx.BlobIDs(root)
	if err != nil {
		return nil, err
	}

	res := &Result{Source: SourceScanner, BuiltAtCommit: head}
	toScan := files
	var stale map[string]bool
	gf, err := graphify.Load(root, cfg.GraphifyPath)
	switch {
	case errors.Is(err, graphify.ErrAbsent):
	case err != nil:
		return nil, err
	default:
		current, err := currentChangedSet(root, opt.Base, opt.Rescan, changed)
		if err != nil {
			return nil, err
		}
		if toScan, stale, err = useGraphify(root, cfg, gf, files, current, res); err != nil {
			return nil, err
		}
	}

	// A file is re-scanned only when it is in the working-tree changed set (untracked
	// included) or HEAD holds it at a blob other than the one cached (spec §4.5).
	id := scannerID(cfg.DefinitionExclude)
	c := readCache(cachePath, id)
	prose := func(f string) bool { return matchAny(cfg.DefinitionExclude, f) }
	results, scanned := scanCached(root, toScan, blobs, changed, c, prose)
	res.Scanned = scanned
	res.CacheWarm = c.hit
	var whole *graph.Graph // results assembled, when they are the whole scanner graph
	var st scan.Stats
	if stale != nil {
		res.Graph, st = overlay(gf, stale, results)
	} else {
		res.Graph, st = scan.AssembleStats(results)
		whole = &res.Graph
	}
	res.Imports, res.AmbiguousCalls = st.Imports, st.AmbiguousCalls
	if !c.current(head, files, changed, results, scanned) {
		if err := writeCache(cachePath, id, head, files, blobs, changed, c, results, whole); err != nil {
			return nil, err
		}
	}
	addFiles(&res.Graph, root, files, results)
	graph.Classify(res.Graph.Nodes, cfg)
	graph.SplitTests(&res.Graph, files, cfg)
	return res, nil
}

// useGraphify decides, from graphify's graph, which files the scanner must read: the
// stale files (spec §5 step 1), which Build overlays on graphify's graph (steps 2-3), or,
// when staleness ignores graphify entirely (step 4), every file. A nil stale set means
// graphify is not used.
func useGraphify(root string, cfg graph.Config, gf *graphify.Graph, files []string, changed map[string]bool, res *Result) ([]string, map[string]bool, error) {
	res.GraphifyCommit, res.GraphifyFiles = gf.BuiltAtCommit, len(gf.CodeFiles)
	stale, code, reason, err := staleness(root, gf, files, changed, cfg.MaxStaleRatio)
	if err != nil {
		return nil, nil, err
	}
	res.StaleCodeFiles = code
	if reason != "" {
		res.GraphifyIgnored, res.StaleFiles = reason, stale
		return files, nil, nil
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
	res.Source, res.BuiltAtCommit, res.StaleFiles = SourceGraphifyScanner, gf.BuiltAtCommit, stale
	return toScan, isStale, nil
}

// changedSet is the working-tree changed set against base as a path set, untracked files
// included. Against HEAD it is spec §4.5's re-scan set.
func changedSet(root, base string) (map[string]bool, error) {
	cs, err := gitctx.ChangedSet(root, base)
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{}
	for _, c := range cs {
		changed[c.Path] = true
	}
	return changed, nil
}

// currentChangedSet is spec §5 step 1's "current changed set" (untracked included): the
// changed set against HEAD ∪ the one against base, the ref the caller selects from, ∪
// rescan. The union keeps a file changed against HEAD but not against base (edited back)
// scanned too. The cache's re-scan set stays the one against HEAD: HEAD's blobs key the
// cache.
func currentChangedSet(root, base string, rescan []string, againstHEAD map[string]bool) (map[string]bool, error) {
	if (base == "" || base == "HEAD") && len(rescan) == 0 {
		return againstHEAD, nil
	}
	current := make(map[string]bool, len(againstHEAD)+len(rescan))
	for f := range againstHEAD {
		current[f] = true
	}
	if base != "" && base != "HEAD" {
		againstBase, err := changedSet(root, base)
		if err != nil {
			return nil, err
		}
		for f := range againstBase {
			current[f] = true
		}
	}
	for _, f := range rescan {
		current[f] = true
	}
	return current, nil
}

// addFiles gives every file the scanner may read its file node, defining anything or not,
// and makes the references edges from the scanned files to them (#504). A file graphify
// vouches for was not scanned: its node's end is its line count, read here.
func addFiles(g *graph.Graph, root string, files []string, results []scan.FileResult) {
	lines := make(map[string]int, len(results))
	for _, r := range results {
		lines[r.Path] = r.Lines
	}
	for _, f := range files {
		n, ok := lines[f]
		if !ok {
			n = countLines(filepath.Join(root, filepath.FromSlash(f)))
		}
		g.Nodes = append(g.Nodes, graph.FileNode(f, n))
	}
	g.Edges = append(g.Edges, scan.References(files, results)...)
	graph.Sort(g)
}

// countLines is how many lines the file at p holds, 0 when it cannot be read.
func countLines(p string) int {
	b, err := os.ReadFile(p)
	if err != nil || len(b) == 0 {
		return 0
	}
	n := bytes.Count(b, []byte{'\n'})
	if b[len(b)-1] != '\n' {
		n++
	}
	return n
}

func matchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if paths.MatchGlob(g, rel) {
			return true
		}
	}
	return false
}
