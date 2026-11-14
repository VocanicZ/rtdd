package rounds

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// #578 rule 2: a top-level code line in a file many other files call selects the tests
// that name the file, not the tests of every caller.
func TestTopLevelChangeIgnoresTheCallersOfTheFilesFunctions(t *testing.T) {
	file := graph.FileNode("lib/w.py", 40)
	set := n("lib/w.py", "cmd_set", 5, 20)
	nodes := []graph.Node{file, set}
	var edges []graph.Edge
	for _, name := range []string{"a", "b", "c"} {
		caller := n("src/"+name+".py", "use", 1, 2)
		test := tn("tests/test_"+name+".py", "test_use", 1, 2)
		nodes = append(nodes, caller, test)
		edges = append(edges, calls(caller, set), calls(test, caller))
	}
	direct := tn("tests/test_w.py", "test_w", 1, 2)
	nodes = append(nodes, direct)
	edges = append(edges, refs(direct, file))
	g := graph.Graph{Nodes: nodes, Edges: edges}
	check(t, Rounds(g, lines("lib/w.py", LineRange{30, 30})), want{
		changed: []string{"lib/w.py::<module>"},
		round1:  []string{"tests/test_w.py::test_w"},
	})
}

// #578 rule 3: a prose file naming a file does not make the tests that read the prose
// the file's tests; a code node naming it still does.
func TestAProseNodeDoesNotHopItsTestsIntoTheFileItNames(t *testing.T) {
	file := graph.FileNode("lib/w.py", 10)
	doc := graph.FileNode("docs/ref.md", 5)
	doc.Prose = true
	docTest := tn("tests/test_docs.py", "test_docs", 1, 2)
	codeUser := n("src/u.py", "u", 1, 2)
	userTest := tn("tests/test_u.py", "test_u", 1, 2)
	set := n("lib/w.py", "cmd_set", 3, 6)
	g := graph.Graph{
		Nodes: []graph.Node{file, doc, docTest, codeUser, userTest, set},
		Edges: []graph.Edge{refs(doc, file), refs(docTest, doc), refs(codeUser, file), calls(userTest, codeUser)},
	}
	check(t, Rounds(g, lines("lib/w.py", LineRange{4, 4})), want{
		changed: []string{"lib/w.py::cmd_set"},
		round1:  []string{"tests/test_u.py::test_u"},
	})
}
