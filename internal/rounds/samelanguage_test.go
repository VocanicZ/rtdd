package rounds_test

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/rounds"
)

// #467: a one-line edit inside Go's Rounds selected 206 tests in Round 2, 68 of them
// Python tests under bench/ — a Go call linked to every same-named definition in any
// language, and a Python builtin call fell back onto stray Go and Markdown definitions.
// Neither round may hold a test a Go change cannot reach.
func TestAOneLineGoEditSelectsNoPythonBenchTest(t *testing.T) {
	g := thisRepositoryGraph(t)
	r := rounds.Rounds(g, oneLine(t, g))
	t.Logf("Round 1: %d tests, Round 2: %d tests", len(r.Round1), len(r.Round2))
	for _, n := range append(r.Round1, r.Round2...) {
		if strings.HasPrefix(n.File, "bench/") {
			t.Errorf("a one-line edit of Rounds selected %s", n.ID)
		}
	}
}
