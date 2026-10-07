package graphbuild

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

// repo commits files into a fresh git repository and returns its root.
func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := gittest.Init(t)
	for rel, body := range files {
		gittest.Write(t, dir, rel, body)
	}
	gittest.Commit(t, dir, "init")
	return dir
}

// build is Build with the CLI's defaults, failing the test on error.
func build(t *testing.T, root string) *Result {
	t.Helper()
	res, err := Build(root, graph.DefaultConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

var calcProject = map[string]string{
	"src/calc.py":        "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n",
	"tests/test_calc.py": "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n",
	"config.yaml":        "name: calc\n",
}

// PRD #409 AC9, scanner half: with no graphify graph the scanner builds everything.
func TestBuildWithoutGraphifyIsTheScannersGraph(t *testing.T) {
	root := repo(t, calcProject)
	res, err := Build(root, graph.DefaultConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != SourceScanner || res.BuiltAtCommit != gittest.HeadShort(t, root) || len(res.StaleFiles) != 0 || res.GraphifyIgnored != "" {
		t.Errorf("Result = {Source:%q BuiltAtCommit:%q StaleFiles:%v GraphifyIgnored:%q}, want scanner at HEAD, nothing stale",
			res.Source, res.BuiltAtCommit, res.StaleFiles, res.GraphifyIgnored)
	}
	wantNodes := []graph.Node{
		{ID: "src/calc.py::add", File: "src/calc.py", Name: "add", Kind: graph.KindFunc, Start: 1, End: 2},
		{ID: "src/calc.py::total", File: "src/calc.py", Name: "total", Kind: graph.KindFunc, Start: 5, End: 6},
		{ID: "tests/test_calc.py::test_add", File: "tests/test_calc.py", Name: "test_add", Kind: graph.KindFunc, Start: 4, End: 5, IsTest: true},
	}
	if !reflect.DeepEqual(res.Graph.Nodes, wantNodes) {
		t.Errorf("nodes:\n got %+v\nwant %+v", res.Graph.Nodes, wantNodes)
	}
	wantEdges := []graph.Edge{
		{From: "src/calc.py::total", To: "src/calc.py::add", Relation: graph.RelCalls},
		{From: "tests/test_calc.py::test_add", To: "src/calc.py::add", Relation: graph.RelCalls},
	}
	if !reflect.DeepEqual(res.Graph.Edges, wantEdges) {
		t.Errorf("edges:\n got %+v\nwant %+v", res.Graph.Edges, wantEdges)
	}
}
