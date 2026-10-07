package graphbuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/scan"
)

// cacheVersion is the rtdd_cache value this build reads and writes. Any other value —
// and any other scanner fingerprint — is a cache miss, never an error.
const cacheVersion = 1

// cacheFile is .rtdd/graph.json: graphify's graph.json node-link shape (so a graphify
// reader can open it), plus rtdd's own keys, all prefixed rtdd_ (spec §4.5).
type cacheFile struct {
	Directed      bool              `json:"directed"`
	Multigraph    bool              `json:"multigraph"`
	Graph         struct{}          `json:"graph"`
	Nodes         []cacheNode       `json:"nodes"`
	Links         []cacheLink       `json:"links"`
	BuiltAtCommit string            `json:"built_at_commit"`
	Cache         int               `json:"rtdd_cache"`
	Scanner       string            `json:"rtdd_scanner"`
	Files         map[string]string `json:"rtdd_files"` // path -> blob id it was scanned from; "" = working-tree content
}

type cacheNode struct {
	ID             string     `json:"id"`
	Label          string     `json:"label"`
	FileType       string     `json:"file_type"`
	SourceFile     string     `json:"source_file"`
	SourceLocation string     `json:"source_location"`
	Name           string     `json:"rtdd_name"`
	Kind           graph.Kind `json:"rtdd_kind"`
	Start          int        `json:"rtdd_start"`
	End            int        `json:"rtdd_end"`
	Calls          []string   `json:"rtdd_calls,omitempty"`
}

type cacheLink struct {
	Source   string         `json:"source"`
	Target   string         `json:"target"`
	Relation graph.Relation `json:"relation"`
}

// cache is a read cache: per file, the blob it was scanned from and the scan.
type cache struct {
	builtAt string
	blob    map[string]string
	results map[string]scan.FileResult
}

// readCache never fails: a missing, corrupt, other-version or other-scanner cache is empty.
func readCache(p string) *cache {
	c := &cache{blob: map[string]string{}, results: map[string]scan.FileResult{}}
	b, err := os.ReadFile(p)
	if err != nil {
		return c
	}
	var f cacheFile
	if json.Unmarshal(b, &f) != nil || f.Cache != cacheVersion || f.Scanner != scan.Fingerprint() {
		return c
	}
	c.builtAt = f.BuiltAtCommit
	file := map[string]string{}
	for _, n := range f.Nodes {
		r := c.results[n.SourceFile]
		if r.Calls == nil {
			r = scan.FileResult{Path: n.SourceFile, Calls: map[string][]string{}}
		}
		r.Nodes = append(r.Nodes, graph.Node{ID: n.ID, File: n.SourceFile, Name: n.Name, Kind: n.Kind, Start: n.Start, End: n.End})
		if len(n.Calls) > 0 {
			r.Calls[n.ID] = n.Calls
		}
		c.results[n.SourceFile] = r
		file[n.ID] = n.SourceFile
	}
	// Only method edges are read back: calls edges are re-linked from rtdd_calls on every
	// build, which is what keeps a cached caller correct when its callee's file changes.
	for _, l := range f.Links {
		src, ok := file[l.Source]
		if l.Relation != graph.RelMethod || !ok {
			continue
		}
		r := c.results[src]
		r.Edges = append(r.Edges, graph.Edge{From: l.Source, To: l.Target, Relation: l.Relation})
		c.results[src] = r
	}
	for p, blob := range f.Files {
		c.blob[p] = blob
		if _, ok := c.results[p]; !ok {
			c.results[p] = scan.FileResult{Path: p, Calls: map[string][]string{}} // a file with no nodes
		}
	}
	return c
}

// fresh reports whether the cached scan of f can stand: f is not in the working-tree
// changed set and HEAD holds it at the blob it was scanned from.
func (c *cache) fresh(f string, blobs map[string]string, changed map[string]bool) bool {
	return !changed[f] && blobs[f] != "" && c.blob[f] == blobs[f]
}

// scanCached returns a scan of every file in toScan, re-reading only a file whose HEAD
// blob differs from the one cached, or that is in the working-tree changed set. The
// second result is the files it re-read.
func scanCached(root string, toScan []string, blobs map[string]string, changed map[string]bool, c *cache) ([]scan.FileResult, []string) {
	var out []scan.FileResult
	var reread []string
	for _, f := range toScan {
		if c.fresh(f, blobs, changed) {
			out = append(out, c.results[f])
			continue
		}
		reread = append(reread, f)
	}
	out = append(out, scan.ScanFiles(root, reread)...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, reread
}

// writeCache records results, plus every older entry still valid for a listed file.
func writeCache(p, head string, files []string, blobs map[string]string, changed map[string]bool, old *cache, results []scan.FileResult) error {
	keep := map[string]scan.FileResult{}
	for _, f := range files {
		if r, ok := old.results[f]; ok && old.fresh(f, blobs, changed) {
			keep[f] = r
		}
	}
	for _, r := range results {
		keep[r.Path] = r
	}
	f := cacheFile{BuiltAtCommit: head, Cache: cacheVersion, Scanner: scan.Fingerprint(), Files: map[string]string{}}
	all := make([]scan.FileResult, 0, len(keep))
	for path, r := range keep {
		blob := blobs[path]
		if changed[path] {
			blob = ""
		}
		f.Files[path] = blob
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Path < all[j].Path })
	g := scan.Assemble(all)
	calls := map[string][]string{}
	for _, r := range all {
		for id, names := range r.Calls {
			calls[id] = names
		}
	}
	f.Nodes = []cacheNode{}
	for _, n := range g.Nodes {
		f.Nodes = append(f.Nodes, cacheNode{ID: n.ID, Label: label(n), FileType: "code", SourceFile: n.File,
			SourceLocation: "L" + strconv.Itoa(n.Start), Name: n.Name, Kind: n.Kind, Start: n.Start, End: n.End, Calls: calls[n.ID]})
	}
	f.Links = []cacheLink{}
	for _, e := range g.Edges {
		f.Links = append(f.Links, cacheLink{Source: e.From, Target: e.To, Relation: e.Relation})
	}
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".graph.json.*")
	if err != nil {
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// label is graphify's label convention: `add()`, `.plus()`, `Calc`.
func label(n graph.Node) string {
	switch n.Kind {
	case graph.KindFunc:
		return n.Name + "()"
	case graph.KindMethod:
		return "." + n.Name + "()"
	}
	return n.Name
}

// covers reports whether the cache was written at head and lists no file outside files —
// so a call that re-scanned nothing (every file it needed was fresh in the cache) need
// not rewrite it.
func (c *cache) covers(head string, files []string) bool {
	if c.builtAt != head {
		return false
	}
	listed := make(map[string]bool, len(files))
	for _, f := range files {
		listed[f] = true
	}
	for f := range c.blob {
		if !listed[f] {
			return false
		}
	}
	return true
}
