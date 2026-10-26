package rounds

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

func refs(from, to graph.Node) graph.Edge {
	return graph.Edge{From: from.ID, To: to.ID, Relation: graph.RelReferences}
}

// #504: a changed line of a file that defines nothing maps to its file node, and a test
// that names the file is in Round 1.
func TestAChangedFileWithNoDefinitionSelectsTheTestsThatNameIt(t *testing.T) {
	prompt := graph.FileNode("prompts/x.md", 20)
	testLoad := tn("tests/test_load.py", "test_load", 1, 3)
	other := tn("tests/test_load.py", "test_other", 5, 6)
	g := graph.Graph{Nodes: []graph.Node{prompt, testLoad, other}, Edges: []graph.Edge{refs(testLoad, prompt)}}
	check(t, Rounds(g, lines("prompts/x.md", LineRange{4, 4})), want{
		changed: []string{"prompts/x.md"},
		round1:  []string{"tests/test_load.py::test_load"},
	})
}

// #504: a changed file nothing names is listed under untested, never reported as none.
func TestAChangedFileNothingNamesIsUntested(t *testing.T) {
	attrs := graph.FileNode(".gitattributes", 2)
	g := graph.Graph{Nodes: []graph.Node{attrs}}
	check(t, Rounds(g, lines(".gitattributes", LineRange{1, 1})), want{
		changed:  []string{".gitattributes"},
		untested: []string{".gitattributes"},
	})
}

// #504: Round 1 also holds the tests of a node that names the changed file — they run
// what loads it — over one references edge, not two.
func TestTheTestsOfWhatLoadsAChangedFileAreRoundOne(t *testing.T) {
	conf := graph.FileNode("conf/app.yaml", 3)
	load := n("src/cfg.py", "load", 1, 4)
	testLoad := tn("tests/test_cfg.py", "test_load", 1, 2)
	guide := graph.FileNode("docs/guide.md", 9)
	index := graph.FileNode("docs/index.md", 9)
	testDocs := tn("tests/test_docs.py", "test_docs", 1, 2)
	g := graph.Graph{
		Nodes: []graph.Node{conf, load, testLoad, guide, index, testDocs},
		Edges: []graph.Edge{refs(load, conf), calls(testLoad, load), refs(guide, conf), refs(index, guide), refs(testDocs, index)},
	}
	check(t, Rounds(g, lines("conf/app.yaml", LineRange{2, 2})), want{
		changed: []string{"conf/app.yaml"},
		round1:  []string{"tests/test_cfg.py::test_load"},
		round2:  []string{"tests/test_docs.py::test_docs"},
	})
}

// #504: lines of a test file in no test (a module-level path) stand for all its tests;
// a whole-file test's are its file node's.
func TestATestFilesTopLevelReferenceSelectsItsTests(t *testing.T) {
	prompt := graph.FileNode("prompts/x.md", 20)
	pyFile := graph.FileNode("tests/test_p.py", 9)
	testA := tn("tests/test_p.py", "test_a", 3, 4)
	testB := tn("tests/test_p.py", "test_b", 6, 7)
	shFile := graph.FileNode("test/test_flow.sh", 30)
	whole := graph.WholeFileNode("test/test_flow.sh")
	g := graph.Graph{
		Nodes: []graph.Node{prompt, pyFile, testA, testB, shFile, whole},
		Edges: []graph.Edge{refs(pyFile, prompt), refs(shFile, prompt)},
	}
	check(t, Rounds(g, lines("prompts/x.md", LineRange{1, 1})), want{
		changed: []string{"prompts/x.md"},
		round1:  []string{"test/test_flow.sh", "tests/test_p.py::test_a", "tests/test_p.py::test_b"},
	})
}

// #504: a test that names a file loads it, so a change to any of its lines — inside a
// definition too — selects that test.
func TestATestThatNamesAFileIsATestOfEveryChangeInIt(t *testing.T) {
	script := graph.FileNode("scripts/cfg.sh", 10)
	setup := n("scripts/cfg.sh", "setup", 2, 5)
	whole := graph.WholeFileNode("test/test_cfg.sh")
	shFile := graph.FileNode("test/test_cfg.sh", 12)
	g := graph.Graph{
		Nodes: []graph.Node{script, setup, whole, shFile},
		Edges: []graph.Edge{refs(shFile, script)},
	}
	check(t, Rounds(g, lines("scripts/cfg.sh", LineRange{3, 3}, LineRange{8, 8})), want{
		changed: []string{"scripts/cfg.sh::setup", "scripts/cfg.sh::<module>"},
		round1:  []string{"test/test_cfg.sh"},
	})
}

// #504: the file node owns no line a definition or <module> owns; Owner still finds it
// for a line in no definition (explain file:line).
func TestTheFileNodeOwnsOnlyLinesInNoDefinition(t *testing.T) {
	file := graph.FileNode("a.py", 10)
	f := n("a.py", "f", 1, 10)
	g := graph.Graph{Nodes: []graph.Node{file, f}}
	if got, _ := Owner(g, "a.py", 3); got.ID != "a.py::f" {
		t.Errorf("Owner(a.py:3) = %q, want a.py::f", got.ID)
	}
	g = graph.Graph{Nodes: []graph.Node{file, n("a.py", "f", 2, 3)}}
	if got, _ := Owner(g, "a.py", 7); got.ID != "a.py" {
		t.Errorf("Owner(a.py:7) = %q, want the file node a.py", got.ID)
	}
	check(t, Rounds(g, lines("a.py", LineRange{7, 7})), want{
		changed:  []string{"a.py::<module>"},
		untested: []string{"a.py::<module>"},
	})
}

// #504: explain lists a test file's top-level reference under the file's tests.
func TestLinksOfAFileListsATestFilesReferenceAsItsTests(t *testing.T) {
	prompt := graph.FileNode("prompts/x.md", 20)
	shFile := graph.FileNode("test/test_flow.sh", 30)
	whole := graph.WholeFileNode("test/test_flow.sh")
	doc := graph.FileNode("README.md", 5)
	g := graph.Graph{Nodes: []graph.Node{prompt, shFile, whole, doc}, Edges: []graph.Edge{refs(shFile, prompt), refs(doc, prompt)}}
	l := LinksOf(g, prompt.ID)
	if len(l.Tests) != 1 || l.Tests[0].Node.ID != whole.ID || len(l.Callers) != 1 || l.Callers[0].Node.ID != "README.md" {
		t.Errorf("LinksOf(prompts/x.md) = tests %v, callers %v; want the whole-file test, then README.md", l.Tests, l.Callers)
	}
}
