package graphbuild

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/scan"
)

// cacheVersion is the rtdd_cache value this build reads and writes. Any other value —
// and any other scanner fingerprint — is a cache miss, never an error. 2: a method's
// class is rtdd_parent on its node, so a read never decodes links. 3: a file's import
// specs are rtdd_imports, a class's supertypes rtdd_supers on its node. 4: a file's line
// count is rtdd_lines, the path tokens of its lines in no definition rtdd_refs, and a
// node's rtdd_refs on the node (#504).
const cacheVersion = 4

// cacheFile is .rtdd/graph.json: graphify's graph.json node-link shape (so a graphify
// reader can open it), plus rtdd's own keys, all prefixed rtdd_ (spec §4.5). Links are
// written last, and the order is load-bearing: readCache stops before them (cacheRead).
type cacheFile struct {
	Directed      bool                `json:"directed"`
	Multigraph    bool                `json:"multigraph"`
	Graph         struct{}            `json:"graph"`
	BuiltAtCommit string              `json:"built_at_commit"`
	Cache         int                 `json:"rtdd_cache"`
	Scanner       string              `json:"rtdd_scanner"`
	Files         map[string]string   `json:"rtdd_files"`   // path -> blob id it was scanned from; "" = working-tree content
	Imports       map[string][]string `json:"rtdd_imports"` // path -> its import specs, when it has any
	Lines         map[string]int      `json:"rtdd_lines"`   // path -> its line count
	Refs          map[string][]string `json:"rtdd_refs"`    // path -> the path tokens of its lines in no definition
	Nodes         []cacheNode         `json:"nodes"`
	Links         []cacheLink         `json:"links"`
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
	Parent         string     `json:"rtdd_parent,omitempty"` // the class of a method: its method edge
	Supers         []string   `json:"rtdd_supers,omitempty"` // a class's supertypes, "<relation>:<name>"
	Refs           []string   `json:"rtdd_refs,omitempty"`   // the path tokens in its code
}

// cacheRead is cacheFile without links. They are written for a graphify reader; rtdd
// re-links calls edges from rtdd_calls and method edges from rtdd_parent, and even
// skipping tens of thousands of links it would drop is most of a warm build — so
// decodeCache stops reading once it has every key below, which cacheFile writes first.
type cacheRead struct {
	Nodes         []cacheNode         `json:"nodes"`
	BuiltAtCommit string              `json:"built_at_commit"`
	Cache         int                 `json:"rtdd_cache"`
	Scanner       string              `json:"rtdd_scanner"`
	Files         map[string]string   `json:"rtdd_files"`
	Imports       map[string][]string `json:"rtdd_imports"`
	Lines         map[string]int      `json:"rtdd_lines"`
	Refs          map[string][]string `json:"rtdd_refs"`
}

type cacheLink struct {
	Source   string         `json:"source"`
	Target   string         `json:"target"`
	Relation graph.Relation `json:"relation"`
}

// cache is a read cache: per file, the blob it was scanned from and the scan.
type cache struct {
	hit     bool // the cache file was read: right version, right scanner
	builtAt string
	blob    map[string]string
	results map[string]scan.FileResult
}

// scannerID is what a cache must have been scanned under to be read: the scanner's
// fingerprint and the files it read as prose (definition_exclude), which ScanProse scans.
func scannerID(definitionExclude []string) string {
	return scan.Fingerprint() + " " + strings.Join(definitionExclude, " ")
}

// readCache never fails: a missing, corrupt, other-version or other-scanner cache is
// empty. id is the scannerID the cache must have been written under.
func readCache(p, id string) *cache {
	c := &cache{blob: map[string]string{}, results: map[string]scan.FileResult{}}
	fh, err := os.Open(p)
	if err != nil {
		return c
	}
	defer fh.Close()
	f, err := decodeCache(fh)
	if err != nil || f.Cache != cacheVersion || f.Scanner != id {
		return c
	}
	c.hit = true
	c.builtAt = f.BuiltAtCommit
	for _, n := range f.Nodes {
		r := c.results[n.SourceFile]
		if r.Calls == nil {
			r = scan.FileResult{Path: n.SourceFile, Calls: map[string][]string{}}
		}
		r.Nodes = append(r.Nodes, graph.Node{ID: n.ID, File: n.SourceFile, Name: n.Name, Kind: n.Kind, Start: n.Start, End: n.End})
		if len(n.Calls) > 0 {
			r.Calls[n.ID] = n.Calls
		}
		if len(n.Refs) > 0 {
			if r.Refs == nil {
				r.Refs = map[string][]string{}
			}
			r.Refs[n.ID] = n.Refs
		}
		// Only method edges are cached: calls edges are re-linked from rtdd_calls on every
		// build, which is what keeps a cached caller correct when its callee's file changes.
		if n.Parent != "" {
			r.Edges = append(r.Edges, graph.Edge{From: n.Parent, To: n.ID, Relation: graph.RelMethod})
		}
		for _, s := range n.Supers {
			rel, name, _ := strings.Cut(s, ":")
			if r.Supers == nil {
				r.Supers = map[string][]scan.Super{}
			}
			r.Supers[n.ID] = append(r.Supers[n.ID], scan.Super{Name: name, Relation: graph.Relation(rel)})
		}
		c.results[n.SourceFile] = r
	}
	for p, blob := range f.Files {
		c.blob[p] = blob
		if _, ok := c.results[p]; !ok {
			c.results[p] = scan.FileResult{Path: p, Calls: map[string][]string{}} // a file with no nodes
		}
	}
	for p, specs := range f.Imports {
		if r, ok := c.results[p]; ok {
			r.Imports = specs
			c.results[p] = r
		}
	}
	for p, n := range f.Lines {
		if r, ok := c.results[p]; ok {
			r.Lines = n
			c.results[p] = r
		}
	}
	for p, toks := range f.Refs {
		if r, ok := c.results[p]; ok {
			if r.Refs == nil {
				r.Refs = map[string][]string{}
			}
			r.Refs[p] = toks
			c.results[p] = r
		}
	}
	return c
}

// decodeCache reads cacheRead's keys from a cache object, in whatever order they come,
// and returns as soon as it has all of them, without reading what follows.
func decodeCache(r io.Reader) (cacheRead, error) {
	var f cacheRead
	dec := json.NewDecoder(r)
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return f, errors.New("cache is not a JSON object")
	}
	seen := map[string]bool{}
	for len(seen) < 8 && dec.More() {
		t, err := dec.Token()
		if err != nil {
			return f, err
		}
		key, _ := t.(string)
		var v any
		switch key {
		case "nodes":
			v = &f.Nodes
		case "built_at_commit":
			v = &f.BuiltAtCommit
		case "rtdd_cache":
			v = &f.Cache
		case "rtdd_scanner":
			v = &f.Scanner
		case "rtdd_files":
			v = &f.Files
		case "rtdd_imports":
			v = &f.Imports
		case "rtdd_lines":
			v = &f.Lines
		case "rtdd_refs":
			v = &f.Refs
		default:
			v = &json.RawMessage{}
		}
		if err := dec.Decode(v); err != nil {
			return f, err
		}
		if _, ok := v.(*json.RawMessage); !ok {
			seen[key] = true
		}
	}
	return f, nil
}

// fresh reports whether the cached scan of f can stand: f is not in the working-tree
// changed set and HEAD holds it at the blob it was scanned from.
func (c *cache) fresh(f string, blobs map[string]string, changed map[string]bool) bool {
	return !changed[f] && blobs[f] != "" && c.blob[f] == blobs[f]
}

// scanCached returns a scan of every file in toScan, re-reading only a file whose HEAD
// blob differs from the one cached, or that is in the working-tree changed set. The
// second result is the files it re-read.
func scanCached(root string, toScan []string, blobs map[string]string, changed map[string]bool, c *cache, prose func(string) bool) ([]scan.FileResult, []string) {
	var out []scan.FileResult
	var reread []string
	for _, f := range toScan {
		if c.fresh(f, blobs, changed) {
			out = append(out, c.results[f])
			continue
		}
		reread = append(reread, f)
	}
	out = append(out, scan.ScanFilesProse(root, reread, prose)...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, reread
}

// writeCache records results, plus every older entry still valid for a listed file.
// whole, when not nil, is results already assembled; it is reused when results are every
// file the cache keeps, so a build assembles its scanner graph once, not twice.
func writeCache(p, id, head string, files []string, blobs map[string]string, changed map[string]bool, old *cache, results []scan.FileResult, whole *graph.Graph) error {
	keep := map[string]scan.FileResult{}
	for _, f := range files {
		if r, ok := old.results[f]; ok && old.fresh(f, blobs, changed) {
			keep[f] = r
		}
	}
	for _, r := range results {
		keep[r.Path] = r
	}
	f := cacheFile{BuiltAtCommit: head, Cache: cacheVersion, Scanner: id, Files: map[string]string{}, Imports: map[string][]string{},
		Lines: map[string]int{}, Refs: map[string][]string{}}
	all := make([]scan.FileResult, 0, len(keep))
	for path, r := range keep {
		blob := blobs[path]
		if changed[path] {
			blob = ""
		}
		f.Files[path] = blob
		if len(r.Imports) > 0 {
			f.Imports[path] = r.Imports
		}
		f.Lines[path] = r.Lines
		if toks := r.Refs[path]; len(toks) > 0 {
			f.Refs[path] = toks
		}
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Path < all[j].Path })
	var g graph.Graph
	if whole != nil && len(all) == len(results) { // keep holds every result, so the sets are equal
		g = *whole
	} else {
		g = scan.Assemble(all)
	}
	refs := scan.RefsOf(all)
	nodeRefs := map[string][]string{}
	for _, r := range all {
		for id, toks := range r.Refs {
			if id != r.Path {
				nodeRefs[id] = toks
			}
		}
	}
	supers := map[string][]string{}
	for id, sups := range refs.Supers {
		for _, s := range sups {
			supers[id] = append(supers[id], string(s.Relation)+":"+s.Name)
		}
	}
	parent := map[string]string{}
	for _, e := range g.Edges {
		if e.Relation == graph.RelMethod {
			parent[e.To] = e.From
		}
	}
	f.Nodes = []cacheNode{}
	for _, n := range g.Nodes {
		f.Nodes = append(f.Nodes, cacheNode{ID: n.ID, Label: label(n), FileType: "code", SourceFile: n.File,
			SourceLocation: "L" + strconv.Itoa(n.Start), Name: n.Name, Kind: n.Kind, Start: n.Start, End: n.End,
			Calls: refs.Calls[n.ID], Parent: parent[n.ID], Supers: supers[n.ID], Refs: nodeRefs[n.ID]})
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

// current reports whether writing the cache would reproduce it: it covers head and
// files, and every file re-read is a working-tree change, cached from the working tree,
// whose re-scan matches what the cache holds. A file being edited is re-read on every
// build; rewriting the whole cache each time for it is most of a warm build.
func (c *cache) current(head string, files []string, changed map[string]bool, results []scan.FileResult, scanned []string) bool {
	if !c.covers(head, files) {
		return false
	}
	reread := make(map[string]bool, len(scanned))
	for _, f := range scanned {
		reread[f] = true
	}
	for _, r := range results {
		if !reread[r.Path] {
			continue
		}
		if blob, ok := c.blob[r.Path]; !ok || blob != "" || !changed[r.Path] || !sameScan(c.results[r.Path], r) {
			return false
		}
	}
	return true
}

// sameScan reports whether two scans of one file hold the same nodes, method edges,
// calls, line count and path tokens, whatever their order — a cached scan comes back in graph order.
func sameScan(a, b scan.FileResult) bool {
	return reflect.DeepEqual(canonical(a), canonical(b))
}

func canonical(r scan.FileResult) scan.FileResult {
	g := graph.Graph{Nodes: slices.Clone(r.Nodes), Edges: slices.Clone(r.Edges)}
	graph.Sort(&g)
	out := scan.FileResult{Path: r.Path, Calls: map[string][]string{}, Supers: map[string][]scan.Super{}, Lines: r.Lines, Refs: map[string][]string{}}
	if len(r.Imports) > 0 {
		out.Imports = r.Imports
	}
	for id, sups := range r.Supers {
		if len(sups) > 0 {
			out.Supers[id] = sups
		}
	}
	if len(g.Nodes) > 0 {
		out.Nodes = g.Nodes
	}
	if len(g.Edges) > 0 {
		out.Edges = g.Edges
	}
	for id, names := range r.Calls {
		if len(names) > 0 {
			out.Calls[id] = names
		}
	}
	for id, toks := range r.Refs {
		if len(toks) > 0 {
			out.Refs[id] = toks
		}
	}
	return out
}
