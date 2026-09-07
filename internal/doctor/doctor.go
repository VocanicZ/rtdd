// Package doctor ranks source files by fan-out — how many tests cover them — as a
// coupling diagnostic. Fan-out is never used as an automatic escalation trigger; see
// spec §9 and Caveat.
package doctor

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// Caveat is the limitation rtdd doctor MUST print alongside its table (spec §9).
// Anything executed once per process is attributed to whichever test happened to run
// first, so the repo's most coupled file can be reported as its cleanest.
const Caveat = "CAVEAT: anything executed once per process — @lru_cache results, " +
	"module singletons, DI container wiring, session-scoped fixtures — runs during " +
	"whichever test happened to go first and therefore gets a fan-out of 1. The most " +
	"coupled file in the repo can appear here as the cleanest. Fan-out is a diagnostic " +
	"only; RTDD never escalates selection on it."

// Hub is one file's fan-out.
type Hub struct {
	Path      string
	TestCount int
	Fraction  float64 // TestCount / total tests in the map
}

// Hubs returns files sorted by descending TestCount, then ascending Path so the output
// is deterministic. An empty map yields an empty, non-nil slice: Fraction's denominator
// is m.Len(), which is zero exactly when there is nothing to divide.
func Hubs(m *mapstore.Map) []Hub {
	fan := m.FanOut()
	total := m.Len()
	out := make([]Hub, 0, len(fan))
	for path, n := range fan {
		frac := 0.0
		if total > 0 {
			frac = float64(n) / float64(total)
		}
		out = append(out, Hub{Path: path, TestCount: n, Fraction: frac})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TestCount != out[j].TestCount {
			return out[i].TestCount > out[j].TestCount
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// StaticSelectionCaveat is the sentence itself, without the CAVEAT: prefix doctor's own
// table format wants. It is a separate const because `which` and `run` carry the same
// limitation into their notes and into the --json `warnings` array (issue #348), where a
// table's prefix would read as noise and, in a polyglot repository, sit between the
// adapter name and the sentence it qualifies.
//
// One const, three commands: a static selection described one way on doctor's table and
// another in the document an agent parses is a difference the agent has to reconcile, and
// the wording is the whole point — it says what a static selection can MISS and what
// passing one is worth.
const StaticSelectionCaveat = "a static selection is derived from declared correspondence " +
	"and imports, not from a recorded run, so it can miss a test that execution-derived " +
	"selection would have caught. A passing static selection is therefore weaker evidence " +
	"than a passing execution-derived one."

// StaticCaveat is the limitation rtdd doctor MUST print alongside any `static` or `none`
// selection fidelity (spec §6). A static selection is derived from declaration rather than
// from a recorded run, so it can miss a test execution-derived selection would have caught;
// saying so is what keeps a green static selection from being read as the same evidence a
// green execution-derived one is.
const StaticCaveat = "CAVEAT: " + StaticSelectionCaveat
