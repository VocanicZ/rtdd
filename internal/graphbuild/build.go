// Package graphbuild assembles the node graph a command selects from: the scanner's
// graph, cached in .rtdd/graph.json, overlaid on graphify's when the project has one and
// it can be trusted (spec §4.5, §5). It is the only graph package that talks to git, and
// it does so only through internal/gitctx.
package graphbuild

import (
	"errors"
	"path/filepath"

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
	changed, err := changedSet(root)
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
		if toScan, stale, err = useGraphify(root, cfg, gf, files, changed, res); err != nil {
			return nil, err
		}
	}

	// A file is re-scanned only when it is in the working-tree changed set (untracked
	// included) or HEAD holds it at a blob other than the one cached (spec §4.5).
	c := readCache(cachePath)
	results, scanned := scanCached(root, toScan, blobs, changed, c)
	res.Scanned = scanned
	if len(scanned) > 0 || !c.covers(head, files) {
		if err := writeCache(cachePath, head, files, blobs, changed, c, results); err != nil {
			return nil, err
		}
	}

	if stale != nil {
		res.Graph = Overlay(gf, stale, results)
	} else {
		res.Graph = scan.Assemble(results)
	}
	graph.Classify(res.Graph.Nodes, cfg)
	return res, nil
}

// useGraphify decides, from graphify's graph, which files the scanner must read: the
// stale files (spec §5 step 1), which Build overlays on graphify's graph (steps 2-3), or,
// when staleness ignores graphify entirely (step 4), every file. A nil stale set means
// graphify is not used.
func useGraphify(root string, cfg graph.Config, gf *graphify.Graph, files []string, changed map[string]bool, res *Result) ([]string, map[string]bool, error) {
	res.GraphifyCommit, res.GraphifyFiles = gf.BuiltAtCommit, len(gf.CodeFiles)
	stale, reason, err := staleness(root, gf, files, changed, cfg.MaxStaleRatio)
	if err != nil {
		return nil, nil, err
	}
	if reason != "" {
		res.GraphifyIgnored = reason
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

// changedSet is the working-tree changed set against HEAD as a path set, untracked files
// included (spec §4.5's re-scan set; spec §5 step 1's "current changed set" and
// "untracked").
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
