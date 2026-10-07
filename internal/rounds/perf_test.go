package rounds_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// thisRepositoryGraph is the scanner graph of this repository, built with its cache in
// a temp dir so neither the test nor the benchmark writes into the checkout.
func thisRepositoryGraph(b testing.TB) graph.Graph {
	b.Helper()
	wd, _ := os.Getwd()
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		b.Skipf("not in a git checkout: %v", err)
	}
	cfg, err := graph.LoadConfig(root)
	if err != nil {
		b.Fatal(err)
	}
	res, err := graphbuild.Build(root, cfg, graphbuild.Options{CachePath: filepath.Join(b.TempDir(), "graph.json")})
	if err != nil {
		b.Fatal(err)
	}
	return res.Graph
}

// oneLine is the small edit `rtdd which` answers in a TDD loop: the first body line of
// Rounds itself.
func oneLine(b testing.TB, g graph.Graph) map[string][]rounds.LineRange {
	b.Helper()
	for _, n := range g.Nodes {
		if n.File == "internal/rounds/rounds.go" && n.Name == "Rounds" {
			return map[string][]rounds.LineRange{n.File: {{Start: n.Start + 1, End: n.Start + 1}}}
		}
	}
	b.Fatal("no internal/rounds/rounds.go::Rounds node in this repository's graph")
	return nil
}

// everyLine changes every node of every file: the most Rounds can be asked, as a
// `--base` far behind HEAD asks it.
func everyLine(g graph.Graph) map[string][]rounds.LineRange {
	changed := map[string][]rounds.LineRange{}
	for _, n := range g.Nodes {
		changed[n.File] = append(changed[n.File], rounds.LineRange{Start: n.Start, End: n.End})
	}
	return changed
}

// #454: a diff that changes every node of this repository still leaves `rtdd which`
// interactive. Rounds walked a neighbour's callers once per changed node that reached
// it, so this took ~3 s; walking each once, it takes well under a tenth of that.
func TestRoundsOfEveryLineOfThisRepositoryIsUnder1s(t *testing.T) {
	g := thisRepositoryGraph(t)
	changed := everyLine(g)
	start := time.Now()
	r := rounds.Rounds(g, changed)
	took := time.Since(start)
	t.Logf("Rounds of every line: %d nodes, %d edges, %d changed nodes, in %v", len(g.Nodes), len(g.Edges), len(r.ChangedNodes), took)
	if took >= time.Second {
		t.Errorf("Rounds of every line took %v, want < 1s", took)
	}
}

// #454: Rounds on this repository's graph (~5k nodes, ~60k edges), for a one-line edit
// and for a change to every line.
func BenchmarkRoundsOnThisRepository(b *testing.B) {
	g := thisRepositoryGraph(b)
	b.Logf("graph: %d nodes, %d edges", len(g.Nodes), len(g.Edges))
	for _, c := range []struct {
		name    string
		changed map[string][]rounds.LineRange
	}{{"one line", oneLine(b, g)}, {"every line", everyLine(g)}} {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				rounds.Rounds(g, c.changed)
			}
		})
	}
}
