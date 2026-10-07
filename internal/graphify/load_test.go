package graphify

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// copyFixture puts testdata/graphify-out at rel/.. under a fresh root.
func copyFixture(t *testing.T, rel string) string {
	t.Helper()
	root := t.TempDir()
	dst := filepath.Join(root, filepath.Dir(filepath.FromSlash(rel)))
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(dst, os.DirFS("testdata/graphify-out")); err != nil {
		t.Fatal(err)
	}
	return root
}

// PRD #409 AC4: only code nodes survive, and only the closed relation set's edges
// between surviving nodes.
func TestLoadKeepsCodeNodesAndDropsNonCodeRelations(t *testing.T) {
	root := copyFixture(t, DefaultPath)
	g, err := Load(root, DefaultPath)
	if err != nil {
		t.Fatal(err)
	}
	wantNodes := []graph.Node{
		{ID: "src/base.py::Base", File: "src/base.py", Name: "Base", Kind: graph.KindClass, Start: 1, End: 1},
		{ID: "src/base.py::Proto", File: "src/base.py", Name: "Proto", Kind: graph.KindClass, Start: 5, End: 5},
		{ID: "src/calc.py::add", File: "src/calc.py", Name: "add", Kind: graph.KindFunc, Start: 3, End: 3},
		{ID: "src/calc.py::Calc", File: "src/calc.py", Name: "Calc", Kind: graph.KindClass, Start: 7, End: 7},
		{ID: "src/calc.py::Calc::plus", File: "src/calc.py", Name: "plus", Kind: graph.KindMethod, Start: 8, End: 8},
		{ID: "tests/test_calc.py::test_add", File: "tests/test_calc.py", Name: "test_add", Kind: graph.KindFunc, Start: 4, End: 4},
	}
	if !reflect.DeepEqual(g.Nodes, wantNodes) {
		t.Errorf("nodes:\n got %+v\nwant %+v", g.Nodes, wantNodes)
	}
	wantEdges := []graph.Edge{
		{From: "src/calc.py::Calc", To: "src/base.py::Base", Relation: graph.RelInherits},
		{From: "src/calc.py::Calc", To: "src/base.py::Proto", Relation: graph.RelImplements},
		{From: "src/calc.py::Calc", To: "src/calc.py::Calc::plus", Relation: graph.RelMethod},
		{From: "src/calc.py::Calc::plus", To: "src/calc.py::Calc", Relation: graph.RelReferences},
		{From: "src/calc.py::Calc::plus", To: "src/calc.py::add", Relation: graph.RelCalls},
		{From: "tests/test_calc.py::test_add", To: "src/calc.py::add", Relation: graph.RelCalls},
	}
	if !reflect.DeepEqual(g.Edges, wantEdges) {
		t.Errorf("edges:\n got %+v\nwant %+v", g.Edges, wantEdges)
	}
	for _, e := range g.Edges {
		for _, dropped := range []graph.Relation{"contains", "conceptually_related_to", "semantically_similar_to", "shares_data_with", "cites"} {
			if e.Relation == dropped {
				t.Errorf("a %s edge survived the load: %+v", dropped, e)
			}
		}
	}
	if g.BuiltAtCommit != "fab6c1ad2d93c5cd157dec646024b8e2a8c080f0" {
		t.Errorf("BuiltAtCommit = %q", g.BuiltAtCommit)
	}
	if want := []string{"src/base.py", "src/calc.py", "tests/test_calc.py"}; !reflect.DeepEqual(g.CodeFiles, want) {
		t.Errorf("CodeFiles = %v, want %v", g.CodeFiles, want)
	}
	if want := []string{"README.md", "src/base.py", "src/calc.py", "tests/test_calc.py"}; !reflect.DeepEqual(g.Manifest, want) {
		t.Errorf("Manifest = %v, want %v (relative to .graphify_root, outside paths dropped)", g.Manifest, want)
	}
}

func TestLoadHonoursGraphifyPath(t *testing.T) {
	root := copyFixture(t, "tools/kg/graph.json")
	if _, err := Load(root, DefaultPath); !errors.Is(err, ErrAbsent) {
		t.Errorf("default path: err = %v, want ErrAbsent", err)
	}
	g, err := Load(root, "tools/kg/graph.json")
	if err != nil || len(g.Nodes) != 6 {
		t.Fatalf("Load(graphify_path) = %v nodes, %v", g, err)
	}
}

func TestLoadAbsentIsErrAbsentAndMalformedIsAnError(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(root, DefaultPath); !errors.Is(err, ErrAbsent) {
		t.Errorf("absent: err = %v, want ErrAbsent", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "graphify-out"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "graphify-out", "graph.json"), []byte("{\"nodes\": ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, DefaultPath); err == nil || errors.Is(err, ErrAbsent) {
		t.Errorf("malformed: err = %v, want an error that is not ErrAbsent", err)
	}
}

// writeGraph writes graph.json at the default path under a fresh root.
func writeGraph(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "graphify-out"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "graphify-out", "graph.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// PRD #409 AC4: each dropped relation, alone between two code nodes that both survive,
// yields no edge — while `calls` between the same two nodes does.
func TestLoadDropsEachNonCodeRelation(t *testing.T) {
	const tmpl = `{"nodes": [
		{"id": "a", "label": "a()", "file_type": "code", "source_file": "a.py", "source_location": "L1"},
		{"id": "b", "label": "b()", "file_type": "code", "source_file": "b.py", "source_location": "L2"}
	], "links": [{"relation": %q, "source": "a", "target": "b"}]}`
	for _, rel := range []string{"contains", "conceptually_related_to", "semantically_similar_to", "shares_data_with", "cites", "imports", "defines"} {
		t.Run(rel, func(t *testing.T) {
			g, err := Load(writeGraph(t, fmt.Sprintf(tmpl, rel)), DefaultPath)
			if err != nil {
				t.Fatal(err)
			}
			if len(g.Nodes) != 2 {
				t.Fatalf("nodes = %+v, want both code nodes kept", g.Nodes)
			}
			if len(g.Edges) != 0 {
				t.Errorf("a %s edge survived the load: %+v", rel, g.Edges)
			}
		})
	}
	g, err := Load(writeGraph(t, fmt.Sprintf(tmpl, "calls")), DefaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := []graph.Edge{{From: "a.py::a", To: "b.py::b", Relation: graph.RelCalls}}; !reflect.DeepEqual(g.Edges, want) {
		t.Errorf("control: edges = %+v, want %+v", g.Edges, want)
	}
}

func TestLoadWithoutManifestHasNilManifest(t *testing.T) {
	g, err := Load(writeGraph(t, `{"nodes": [], "links": []}`), DefaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if g.Manifest != nil {
		t.Errorf("Manifest = %v, want nil when manifest.json is absent", g.Manifest)
	}
}

// PRD #409 AC4, spec §5: rtdd never runs graphify. The package opens files only, so it
// must not import anything that can start a process.
func TestPackageNeverExecutesAProcess(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range af.Imports {
			switch strings.Trim(imp.Path.Value, `"`) {
			case "os/exec", "syscall", "golang.org/x/sys/unix", "github.com/VocanicZ/rtdd/internal/runner":
				t.Errorf("%s imports %s: internal/graphify must only read files", f, imp.Path.Value)
			}
		}
	}
}
