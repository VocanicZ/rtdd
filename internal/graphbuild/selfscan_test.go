package graphbuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
)

// Issue #466: on this repository the graph holds no node named after a keyword or a
// builtin call — an anonymous literal (`func(`, `function (`) or a statement read as a
// definition (`if n := len(x); n > 0 {`). Each one was linked from every call of its
// name, which is what blew a one-line edit's Round 2 up to 206 tests.
func TestThisRepositoryHasNoKeywordOrBuiltinNamedNodes(t *testing.T) {
	wd, _ := os.Getwd()
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	cfg, err := graph.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Build(root, cfg, Options{CachePath: filepath.Join(t.TempDir(), "graph.json")})
	if err != nil {
		t.Fatal(err)
	}
	banned := map[string]bool{"func": true, "function": true, "len": true, "min": true, "max": true,
		"if": true, "for": true, "switch": true, "return": true}
	for _, n := range res.Graph.Nodes {
		if banned[n.Name] {
			t.Errorf("node %s (L%d-%d) is named %q", n.ID, n.Start, n.End, n.Name)
		}
	}
}
