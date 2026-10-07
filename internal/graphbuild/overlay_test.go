package graphbuild

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphify"
)

// gfNode is one graphify code node: a function `name()` at line in file.
type gfNode struct {
	id, label, file string
	line            int
}

// writeGraphify writes graphify-out/graph.json and manifest.json (keys absolute under
// root, as graphify writes them) the way graphify would have at builtAt. It is untracked
// and under scan_exclude, as a real graphify-out/ is.
func writeGraphify(t *testing.T, root, builtAt string, nodes []gfNode, calls [][2]string, manifest []string) {
	t.Helper()
	type n struct {
		ID             string `json:"id"`
		Label          string `json:"label"`
		FileType       string `json:"file_type"`
		SourceFile     string `json:"source_file"`
		SourceLocation string `json:"source_location"`
	}
	type l struct {
		Source   string `json:"source"`
		Target   string `json:"target"`
		Relation string `json:"relation"`
	}
	doc := struct {
		Nodes         []n    `json:"nodes"`
		Links         []l    `json:"links"`
		BuiltAtCommit string `json:"built_at_commit,omitempty"`
	}{BuiltAtCommit: builtAt}
	for _, x := range nodes {
		doc.Nodes = append(doc.Nodes, n{x.id, x.label, "code", x.file, "L" + strconv.Itoa(x.line)})
	}
	for _, c := range calls {
		doc.Links = append(doc.Links, l{c[0], c[1], "calls"})
	}
	b, _ := json.Marshal(doc)
	gittest.Write(t, root, "graphify-out/graph.json", string(b))
	m := map[string]any{}
	for _, f := range manifest {
		m[filepath.Join(root, filepath.FromSlash(f))] = map[string]any{"mtime": 0}
	}
	mb, _ := json.Marshal(m)
	gittest.Write(t, root, "graphify-out/manifest.json", string(mb))
}

func fullHead(t *testing.T, root string) string {
	return strings.TrimSpace(gittest.Run(t, root, "rev-parse", "HEAD"))
}

var overlayProject = map[string]string{
	"src/calc.py":        "def add(a, b):\n    return a + b\n\n\ndef run(values):\n    def step(v):\n        return add(v, 1)\n\n    return [step(v) for v in values]\n",
	"src/other.py":       "def helper():\n    return 1\n",
	"tests/test_calc.py": "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n\n\ndef test_step():\n    assert step(1) == 2\n",
}

// overlayGraphify is what graphify would have recorded for overlayProject: start lines
// only, and `step` flat at file level where the scanner nests it under `run`.
func overlayGraphify(t *testing.T, root string) {
	t.Helper()
	all := []string{"src/calc.py", "src/other.py", "tests/test_calc.py"}
	writeGraphify(t, root, fullHead(t, root), []gfNode{
		{"calc_add", "add()", "src/calc.py", 1},
		{"calc_run", "run()", "src/calc.py", 5},
		{"calc_step", "step()", "src/calc.py", 6},
		{"other_helper", "helper()", "src/other.py", 1},
		{"test_add", "test_add()", "tests/test_calc.py", 4},
		{"test_step", "test_step()", "tests/test_calc.py", 8},
	}, [][2]string{{"test_add", "calc_add"}, {"test_step", "calc_step"}, {"calc_run", "calc_step"}, {"calc_step", "calc_add"}}, all)
}

// PRD #409 AC5, against a real repository: commit a change after graphify's commit; the
// changed file's nodes come from the scanner and an unchanged file's edge into it is
// re-pointed by name.
func TestOverlayReplacesAStaleFileAndRepointsEdgesIntoIt(t *testing.T) {
	root := repo(t, overlayProject)
	overlayGraphify(t, root)
	gittest.Write(t, root, "src/calc.py", "# moved down two lines\n\n"+overlayProject["src/calc.py"])
	gittest.Run(t, root, "add", "src/calc.py")
	gittest.Run(t, root, "commit", "-q", "-m", "shift")

	res := build(t, root)
	if res.Source != SourceGraphifyScanner || !reflect.DeepEqual(res.StaleFiles, []string{"src/calc.py"}) {
		t.Fatalf("Source %q, StaleFiles %v; want graphify+scanner with src/calc.py stale", res.Source, res.StaleFiles)
	}
	nodes := map[string]graph.Node{}
	for _, n := range res.Graph.Nodes {
		nodes[n.ID] = n
	}
	if n := nodes["src/calc.py::add"]; n.Start != 3 || n.End != 4 {
		t.Errorf("src/calc.py::add = %+v, want the scanner's span 3-4", n)
	}
	if _, ok := nodes["src/calc.py::step"]; ok {
		t.Error("graphify's src/calc.py::step survived in a stale file")
	}
	if n := nodes["tests/test_calc.py::test_step"]; n.Start != 8 || n.End != 8 {
		t.Errorf("tests/test_calc.py::test_step = %+v, want graphify's start-only node", n)
	}
	has := func(from, to string) bool {
		for _, e := range res.Graph.Edges {
			if e.From == from && e.To == to && e.Relation == graph.RelCalls {
				return true
			}
		}
		return false
	}
	if !has("tests/test_calc.py::test_step", "src/calc.py::run::step") {
		t.Error("graphify's test_step -> step edge was not re-pointed to the scanner's src/calc.py::run::step")
	}
	if !has("tests/test_calc.py::test_add", "src/calc.py::add") || !has("src/calc.py::run::step", "src/calc.py::add") {
		t.Errorf("edges = %+v; want test_add -> add kept and the scanner's step -> add", res.Graph.Edges)
	}
}

func TestOverlayDropsAnEdgeIntoARenamedNode(t *testing.T) {
	root := repo(t, overlayProject)
	overlayGraphify(t, root)
	gittest.Write(t, root, "src/calc.py", strings.ReplaceAll(overlayProject["src/calc.py"], "add", "plus"))
	res := build(t, root)
	for _, e := range res.Graph.Edges {
		if e.From == "tests/test_calc.py::test_add" {
			t.Errorf("an edge into the renamed add survived: %+v", e)
		}
	}
}

// Each source of spec §5 step 1, alone, makes a file stale.
func TestStaleSetSources(t *testing.T) {
	files := map[string]string{"a.py": "def a():\n    pass\n", "b.py": "def b():\n    pass\n",
		"c.py": "def c():\n    pass\n", "README.md": "# r\n"}
	code := []string{"a.py", "b.py", "c.py"}
	cases := []struct {
		name     string
		mutate   func(t *testing.T, root string)
		manifest []string
		changed  map[string]bool
		want     []string
	}{
		{"diff since built_at_commit", func(t *testing.T, root string) {
			gittest.Write(t, root, "a.py", "def a():\n    return 1\n")
			gittest.Commit(t, root, "a")
		}, append(code, "README.md"), nil, []string{"a.py"}},
		{"untracked file", func(t *testing.T, root string) {
			gittest.Write(t, root, "d.py", "def d():\n    pass\n")
		}, append(code, "d.py"), nil, []string{"d.py"}},
		{"working-tree changed set", func(*testing.T, string) {}, code, map[string]bool{"b.py": true}, []string{"b.py"}},
		{"absent from manifest.json", func(*testing.T, string) {}, []string{"a.py", "b.py"}, nil, []string{"c.py"}},
		// #441: a changed non-code file is still scanned and overlaid; it only does not
		// count toward max_stale_ratio (TestNonCodeAndNewLanguageFilesDoNotCountTowardTheRatio).
		{"a changed non-code file", func(t *testing.T, root string) {
			gittest.Write(t, root, "README.md", "# changed\n")
		}, code, nil, []string{"README.md"}},
		{"a non-code file absent from manifest.json is not", func(*testing.T, string) {}, code, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := repo(t, files)
			gf := &graphify.Graph{BuiltAtCommit: fullHead(t, root), CodeFiles: code, Manifest: c.manifest}
			c.mutate(t, root)
			listed, err := gitctx.ListFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			changed := c.changed
			if changed == nil {
				changed = map[string]bool{}
				cs, err := gitctx.ChangedSet(root, "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				for _, ch := range cs {
					changed[ch.Path] = true
				}
			}
			got, err := StaleSet(root, gf, listed, changed)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("StaleSet = %v, want %v", got, c.want)
			}
		})
	}
}
