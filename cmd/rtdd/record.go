package main

import (
	"fmt"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// The two --record modes.
//
// `always` is the default and is exactly what every release before this flag did: every
// cycle executes the selection with coverage recording on. `auto` records only on the
// cycles where the map has something to learn, and runs the selection plain otherwise.
//
// The flag exists because recording is the expensive half, by an order of magnitude.
// Per-test coverage contexts made executing a 23% selection cost MORE than running the
// whole suite plain — a median flask cycle at 14675 ms against the suite's 3001 ms — so
// on the loop this tool exists to make fast, RTDD was charging for a map it then used to
// skip tests that were cheaper to run than to skip.
//
// `auto` is not the default, and should not become one on this evidence. It trades away
// the uncovered report on every plain cycle: no `--cov` means no fresh coverage, and the
// previous cycle's coverage describes lines this run never watched. That report is the
// single best-measured thing RTDD does — 12 firings across flask and httpie, 0/12 and
// 0/52 false signals — and what a staler map costs in missed selections has not been
// measured at all. Both are questions for the replay corpus, not for a default.
const (
	recordAlways = "always"
	recordAuto   = "auto"
)

// shouldRecord decides whether this cycle records coverage, and says why when it does
// not. The reason is returned rather than logged so it reaches the --json document's
// warnings too: a consumer that discards stderr must not read a plain cycle as a normal
// one.
//
// Under `auto` a cycle records when the map would otherwise stay ignorant of something
// this run could have told it:
//
//   - T2 — the whole suite is running, which is the map's one chance to refresh
//     wholesale, and the cycle is already paying full-suite cost.
//   - A changed file no row covers — the map has nothing for it, so the next selection
//     is guessing until some run records it.
//   - An adapter that declares no `subset_plain` — it has no uninstrumented command, so
//     there is no plain cycle to choose.
//
// Anything else is a cycle whose selection the map already answered, and re-recording it
// buys a refresh of coverage for code that did not change.
func shouldRecord(mode string, sel selector.Selection, unmapped []string, ads []*adapter.Adapter) (bool, string) {
	if mode != recordAuto {
		return true, ""
	}
	switch {
	case sel.Tier == selector.TierT2:
		return true, ""
	case len(unmapped) > 0:
		return true, ""
	case !allCanRunPlain(ads):
		return true, ""
	}
	return false, fmt.Sprintf("--record=%s: this cycle ran the selection without recording coverage, "+
		"so the map is unchanged and no uncovered report is available for it", recordAuto)
}

// allCanRunPlain reports whether EVERY detected adapter can run without instrumentation.
// One that cannot would have to record anyway, and a cycle where half the adapters
// recorded and half did not is a map whose freshness nobody could reason about.
func allCanRunPlain(ads []*adapter.Adapter) bool {
	if len(ads) == 0 {
		return false
	}
	for _, ad := range ads {
		if !ad.CanRunPlain() {
			return false
		}
	}
	return true
}
