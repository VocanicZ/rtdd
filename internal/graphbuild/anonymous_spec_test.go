package graphbuild

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// Issue #570: a Luau spec that returns its cases as a table of anonymous
// `fn = function() ... end` bodies defines no node, so it is a whole-file test; the calls
// in those bodies are its file node's, and a change to what they call selects the spec.
func TestCallsInASpecsAnonymousCaseBodiesSelectTheSpec(t *testing.T) {
	root := repo(t, map[string]string{
		"src/Server/Friends.luau": "local M = {}\n\nfunction M.new()\n\treturn { links = {} }\nend\n\n" +
			"function M.areFriends(f, a, b)\n\treturn f.links[a .. \":\" .. b] == true\nend\n\nreturn M\n",
		"src/Server/Friends.spec.luau": "local Friends = require(\"./Friends\")\n\nreturn {\n\t{\n" +
			"\t\tname = \"strangers\",\n\t\tfn = function()\n\t\t\tlocal f = Friends.new()\n" +
			"\t\t\tassert(not Friends.areFriends(f, 1, 2))\n\t\tend,\n\t},\n}\n",
	})
	cold := build(t, root)
	warm := build(t, root) // from the cache: the file's calls survive it
	if !warm.CacheWarm {
		t.Fatal("second build read no cache")
	}
	if !reflect.DeepEqual(warm.Graph, cold.Graph) {
		t.Errorf("warm graph differs from cold:\n got %+v\nwant %+v", warm.Graph, cold.Graph)
	}
	g := warm.Graph

	for _, to := range []string{"src/Server/Friends.luau::new", "src/Server/Friends.luau::areFriends"} {
		e := graph.Edge{From: "src/Server/Friends.spec.luau", To: to, Relation: graph.RelCalls}
		found := false
		for _, x := range g.Edges {
			found = found || x == e
		}
		if !found {
			t.Errorf("no edge %+v in %+v", e, g.Edges)
		}
	}

	res := rounds.Rounds(g, map[string][]rounds.LineRange{"src/Server/Friends.luau": {{Start: 8, End: 8}}})
	var round1 []string
	for _, n := range res.Round1 {
		round1 = append(round1, n.ID)
	}
	if want := []string{"src/Server/Friends.spec.luau"}; !reflect.DeepEqual(round1, want) {
		t.Errorf("Round 1 = %v, want %v", round1, want)
	}
	if len(res.Untested) != 0 {
		t.Errorf("untested = %+v, want none", res.Untested)
	}
}
