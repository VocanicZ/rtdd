package scan

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// #504: a path token is a whole run of path characters that names a file by its path or
// by a base name with a lower-case extension — string or bare — and is not a call.
func TestPathTokensAreTheRunsThatMayNameAFile(t *testing.T) {
	for line, want := range map[string][]string{
		`cat prompts/impl.md > out`:                 {"prompts/impl.md"},
		`open("impl.md").read()`:                    {"impl.md"},
		`load config.yaml.`:                         {"config.yaml"},
		`CFG="$ROOT/scripts/cfg.json"`:              {"ROOT/scripts/cfg.json"},
		`run ../bin/harness --flag`:                 {"../bin/harness"},
		`git check-attr -a .gitattributes`:          {".gitattributes"},
		`fmt.Println(n.ID, os.Exit)`:                nil,
		`x := strings.Split(s, ".")`:                nil,
		`self.name = path.Join(dir, "a.d.ts")`:      {"a.d.ts", "self.name"},
		`url = "https://example.com/a" // a/b.json`: {"//example.com/a", "a/b.json"},
	} {
		set := map[string]bool{}
		pathTokens(line, set)
		if got := sortedSet(set); !reflect.DeepEqual(got, want) {
			t.Errorf("pathTokens(%q) = %q, want %q", line, got, want)
		}
	}
}

// #504: a prose file is read for its paths only: no definition, however code-like a
// sample in it is.
func TestScanProseReadsPathsAndNoDefinition(t *testing.T) {
	r := ScanProse("docs/a.md", []byte("# A\n\nSee `conf/app.yaml`.\n\n```python\ndef fake():\n    return run(1)\n```\n"))
	if len(r.Nodes) != 0 || len(r.Calls) != 0 || r.Lines != 8 {
		t.Errorf("ScanProse = %d nodes, %d calls, %d lines; want 0, 0, 8", len(r.Nodes), len(r.Calls), r.Lines)
	}
	if want := map[string][]string{"docs/a.md": {"conf/app.yaml"}}; !reflect.DeepEqual(r.Refs, want) {
		t.Errorf("Refs = %v, want %v", r.Refs, want)
	}
}

// #504: a code file's path tokens belong to the definition that holds them, or to the
// file for its lines in none; a comment names nothing.
func TestScanFileAttributesPathTokensToTheirOwner(t *testing.T) {
	r := ScanFile("t/test_a.py", []byte("FIXTURE = \"data/in.json\"  # see notes.txt\n\n\ndef test_a():\n    open(\"prompts/x.md\")\n"))
	want := map[string][]string{
		"t/test_a.py":         {"data/in.json"},
		"t/test_a.py::test_a": {"prompts/x.md"},
	}
	if !reflect.DeepEqual(r.Refs, want) || r.Lines != 5 {
		t.Errorf("Refs = %v, Lines = %d; want %v, 5", r.Refs, r.Lines, want)
	}
}

// #504: a token names the file at its path, or at a trailing run of its segments; else
// every file of that base name. A file never references itself.
func TestReferencesResolvesPathsThenBaseNames(t *testing.T) {
	files := []string{"prompts/impl.md", "docs/impl.md", "scripts/cfg.json", "bin/harness", "a.sh"}
	results := []FileResult{{Path: "a.sh", Refs: map[string][]string{
		"a.sh":      {"ROOT/scripts/cfg.json", "a.sh", "harness"},
		"a.sh::run": {"../bin/harness", "prompts/impl.md"},
		"a.sh::doc": {"impl.md", "missing.md"},
	}}}
	g := graph.Graph{Edges: References(files, results)}
	graph.Sort(&g)
	want := []graph.Edge{
		{From: "a.sh", To: "scripts/cfg.json", Relation: graph.RelReferences},
		{From: "a.sh::doc", To: "docs/impl.md", Relation: graph.RelReferences},
		{From: "a.sh::doc", To: "prompts/impl.md", Relation: graph.RelReferences},
		{From: "a.sh::run", To: "bin/harness", Relation: graph.RelReferences},
		{From: "a.sh::run", To: "prompts/impl.md", Relation: graph.RelReferences},
	}
	if !reflect.DeepEqual(g.Edges, want) {
		t.Errorf("References =\n %v\nwant\n %v", g.Edges, want)
	}
}

// #504 acceptance, scanner half: in testdata/filenodes the test that reads prompts/x.md
// and conf/app.yaml references both; the prompt defines nothing.
func TestFileNodesFixtureReferences(t *testing.T) {
	dir := filepath.Join("testdata", "filenodes")
	var files []string
	if err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		files = append(files, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	results := ScanFilesProse(dir, files, func(f string) bool { return strings.HasSuffix(f, ".md") })
	var defined []string
	for _, r := range results {
		for _, n := range r.Nodes {
			defined = append(defined, n.ID)
		}
	}
	if want := []string{"tests/test_load.py::test_load", "tests/test_load.py::test_unrelated"}; !reflect.DeepEqual(defined, want) {
		t.Errorf("nodes = %v, want %v", defined, want)
	}
	g := graph.Graph{Edges: References(files, results)}
	graph.Sort(&g)
	want := []graph.Edge{
		{From: "prompts/x.md", To: "conf/app.yaml", Relation: graph.RelReferences},
		{From: "tests/test_load.py::test_load", To: "conf/app.yaml", Relation: graph.RelReferences},
		{From: "tests/test_load.py::test_load", To: "prompts/x.md", Relation: graph.RelReferences},
	}
	if !reflect.DeepEqual(g.Edges, want) {
		t.Errorf("References =\n %v\nwant\n %v", g.Edges, want)
	}
}
