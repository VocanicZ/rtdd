// Package graphify reads a graph graphify already built. rtdd NEVER runs graphify (spec
// §5, §11): this package opens files and nothing else, and it trusts nothing it reads —
// the staleness overlay in internal/graphbuild decides which of it survives.
package graphify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// DefaultPath is where graphify writes its graph, relative to the repository root.
const DefaultPath = "graphify-out/graph.json"

// ErrAbsent is a missing graph: the project does not use graphify. Not a failure.
var ErrAbsent = errors.New("graphify: no graph")

// Graph is graphify's graph mapped into the rtdd model, plus what staleness needs.
type Graph struct {
	Nodes         []graph.Node // code nodes only; End == Start (graphify records start lines only)
	Edges         []graph.Edge // closed relation set only, both ends kept
	BuiltAtCommit string       // "" when graphify did not record one
	CodeFiles     []string     // sorted repo-relative files holding at least one code node
	Manifest      []string     // sorted repo-relative files in manifest.json; nil when it is absent
}

type nodeLink struct {
	Nodes []struct {
		ID             string `json:"id"`
		Label          string `json:"label"`
		FileType       string `json:"file_type"`
		SourceFile     string `json:"source_file"`
		SourceLocation string `json:"source_location"`
	} `json:"nodes"`
	Links []struct {
		Source   string `json:"source"`
		Target   string `json:"target"`
		Relation string `json:"relation"`
	} `json:"links"`
	BuiltAtCommit string `json:"built_at_commit"`
}

// Load reads <root>/<rel> (graphify_path) and manifest.json beside it.
func Load(root, rel string) (*Graph, error) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w at %s", ErrAbsent, rel)
	}
	if err != nil {
		return nil, err
	}
	var nl nodeLink
	if err := json.Unmarshal(b, &nl); err != nil {
		return nil, fmt.Errorf("graphify: %s: %w", rel, err)
	}

	// A method edge names its target's class; the class is part of the target's ID.
	parent := map[string]string{}
	label := map[string]string{}
	for _, n := range nl.Nodes {
		label[n.ID] = n.Label
	}
	for _, l := range nl.Links {
		if l.Relation == string(graph.RelMethod) {
			parent[l.Target] = nameOf(label[l.Source])
		}
	}

	g := &Graph{BuiltAtCommit: nl.BuiltAtCommit}
	ids := map[string]string{} // graphify id -> rtdd id
	files := map[string]bool{}
	seen := map[string]bool{}
	for _, n := range nl.Nodes {
		if n.FileType != "code" || n.SourceFile == "" {
			continue
		}
		file := filepath.ToSlash(n.SourceFile)
		files[file] = true
		if n.Label == path.Base(file) {
			continue // the file node: everything it would say, `contains` said, and that is dropped
		}
		name := nameOf(n.Label)
		kind := graph.KindClass
		if strings.HasSuffix(n.Label, ")") {
			kind = graph.KindFunc
			if _, ok := parent[n.ID]; ok {
				kind = graph.KindMethod
			}
		}
		start, _ := strconv.Atoi(strings.TrimPrefix(n.SourceLocation, "L"))
		id := file + "::" + name
		if c, ok := parent[n.ID]; ok {
			id = file + "::" + c + "::" + name
		}
		if seen[id] {
			id += "@" + strconv.Itoa(start)
		}
		seen[id] = true
		ids[n.ID] = id
		g.Nodes = append(g.Nodes, graph.Node{ID: id, File: file, Name: name, Kind: kind, Start: start, End: start})
	}
	for _, l := range nl.Links {
		rel, ok := graph.ParseRelation(l.Relation)
		from, okFrom := ids[l.Source]
		to, okTo := ids[l.Target]
		if ok && okFrom && okTo {
			g.Edges = append(g.Edges, graph.Edge{From: from, To: to, Relation: rel})
		}
	}
	whole := graph.Graph{Nodes: g.Nodes, Edges: g.Edges}
	graph.Sort(&whole)
	g.Nodes, g.Edges = whole.Nodes, whole.Edges
	for f := range files {
		g.CodeFiles = append(g.CodeFiles, f)
	}
	sort.Strings(g.CodeFiles)

	g.Manifest, err = readManifest(root, filepath.Dir(p))
	if err != nil {
		return nil, err
	}
	return g, nil
}

// nameOf turns a graphify label into a name: `.plus()` -> plus, `add()` -> add.
func nameOf(label string) string {
	return strings.TrimSuffix(strings.TrimPrefix(label, "."), "()")
}

// readManifest returns manifest.json's files relative to the root graphify recorded in
// .graphify_root (the repository root when that file is absent); entries outside it are
// dropped. A missing manifest is nil, not an error.
func readManifest(root, dir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("graphify: manifest.json: %w", err)
	}
	base := root
	if r, err := os.ReadFile(filepath.Join(dir, ".graphify_root")); err == nil {
		base = strings.TrimSpace(string(r))
	}
	out := []string{}
	for k := range m {
		rel := k
		if filepath.IsAbs(k) {
			r, err := filepath.Rel(base, k)
			r = filepath.ToSlash(r)
			if err != nil || r == ".." || strings.HasPrefix(r, "../") {
				continue
			}
			rel = r
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out, nil
}
