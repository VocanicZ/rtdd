// Package doctor ranks source files by fan-out — how many tests cover them — as a
// coupling diagnostic. Fan-out is never used as an automatic escalation trigger; see
// spec §9 and Caveat.
package doctor

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// Caveat is the limitation rtdd doctor MUST print alongside its table (spec §9).
// Every test file runs in its own process, so anything executed once per process is
// attributed to every test file that runs it, which inflates the fan-out of the code that
// setup reaches.
const Caveat = "CAVEAT: each test file runs in its own process, so anything executed once " +
	"per process — @lru_cache results, module singletons, DI container wiring, " +
	"session-scoped fixtures — is attributed to every test file that runs it. That " +
	"inflates fan-out: code reached only through shared setup looks coupled to every " +
	"test file that does the setup. Fan-out is a diagnostic only; RTDD never " +
	"escalates selection on it."

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
