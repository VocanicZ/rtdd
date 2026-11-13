package rounds

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// fanoutGraph is #573's fixture: src/a..d.py, a test each, test_all calling a, b and c,
// a CHANGELOG.md that names every source file, a guide a test reads, and a .tsv of data
// that names source files too.
func fanoutGraph() graph.Graph {
	var nodes []graph.Node
	var edges []graph.Edge
	changelog := graph.FileNode("CHANGELOG.md", 8)
	guide := graph.FileNode("docs/guide.md", 4)
	twins := graph.FileNode("tests/twins.tsv", 4)
	docsTest := tn("tests/test_docs.py", "test_guide", 1, 2)
	twinsTest := tn("tests/test_twins.py", "test_twins", 1, 2)
	nodes = append(nodes, changelog, guide, twins, docsTest, twinsTest)
	edges = append(edges, refs(docsTest, guide), refs(twinsTest, twins))
	all := tn("tests/test_all.py", "test_all", 1, 5)
	nodes = append(nodes, all)
	for _, x := range []string{"a", "b", "c", "d"} {
		code := n("src/"+x+".py", "f_"+x, 1, 2)
		test := tn("tests/test_"+x+".py", "test_"+x, 1, 2)
		nodes = append(nodes, code, test)
		edges = append(edges, calls(test, code), refs(changelog, code), refs(guide, code), refs(twins, code))
		if x != "d" {
			edges = append(edges, calls(all, code))
		}
	}
	return graph.Graph{Nodes: nodes, Edges: edges}
}

// #573: prose that names most of the repo adds nothing to Round 2.
func TestAChangedProseFileAddsNothingToRound2(t *testing.T) {
	check(t, Rounds(fanoutGraph(), lines("CHANGELOG.md", LineRange{9, 9})), want{
		changed:  []string{"CHANGELOG.md"},
		untested: []string{"CHANGELOG.md"},
	})
}

// #573: a prose file a test reads is that test's Round 1, even when it names source files.
func TestAChangedProseFileATestReadsIsRound1Only(t *testing.T) {
	check(t, Rounds(fanoutGraph(), lines("docs/guide.md", LineRange{2, 2})), want{
		changed: []string{"docs/guide.md"},
		round1:  []string{"tests/test_docs.py::test_guide"},
	})
}

// #573: a data file that defines nothing is a whole-file node: no Round 2.
func TestAChangedDataFileAddsNothingToRound2(t *testing.T) {
	check(t, Rounds(fanoutGraph(), lines("tests/twins.tsv", LineRange{2, 2})), want{
		changed: []string{"tests/twins.tsv"},
		round1:  []string{"tests/test_twins.py::test_twins"},
	})
}

// #573: a changed test is Round 1 and adds nothing to Round 2.
func TestAChangedTestAddsNothingToRound2(t *testing.T) {
	check(t, Rounds(fanoutGraph(), lines("tests/test_all.py", LineRange{3, 3})), want{
		changed: []string{"tests/test_all.py::test_all"},
		round1:  []string{"tests/test_all.py::test_all"},
	})
}

// #573: a changed helper of a whole-file test, its top level and its file node add
// nothing to Round 2.
func TestAChangedWholeFileTestAddsNothingToRound2(t *testing.T) {
	const f = "tests/test_hermetic.sh"
	a, d := n("src/a.sh", "run_a", 1, 2), n("src/d.sh", "run_d", 1, 2)
	whole := graph.WholeFileNode(f)
	helper := hn(f, "run_all", 2, 5)
	other := tn("tests/test_a.sh", "t", 1, 2)
	file := graph.FileNode(f, 6)
	g := graph.Graph{
		Nodes: []graph.Node{a, d, whole, helper, other, file},
		Edges: []graph.Edge{calls(whole, helper), calls(helper, a), calls(helper, d), calls(other, a), calls(other, d)},
	}
	check(t, Rounds(g, lines(f, LineRange{3, 3})), want{
		changed: []string{f + "::run_all"},
		round1:  []string{f},
	})
	got := Rounds(g, lines(f, LineRange{1, 1}))
	if len(got.Round2) != 0 {
		t.Errorf("a whole-file test's top level: round2 = %q, want none", ids(got.Round2))
	}
}

// #573: a PR that changes code and a test together still takes Round 2 from the code.
func TestRound2OfCodeAndTestChangedTogetherComesFromTheCode(t *testing.T) {
	g := fanoutGraph()
	got := Rounds(g, map[string][]LineRange{"tests/test_all.py": {{3, 3}}, "src/d.py": {{1, 1}}})
	check(t, got, want{
		changed: []string{"src/d.py::f_d", "tests/test_all.py::test_all"},
		round1:  []string{"tests/test_all.py::test_all", "tests/test_d.py::test_d"},
		round2:  []string{"tests/test_docs.py::test_guide", "tests/test_twins.py::test_twins"},
	})
}
