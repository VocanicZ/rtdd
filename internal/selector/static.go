package selector

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// staticCandidate is one test the static tier found, with the evidence that found it.
// Level is spec §4.1's ranking level: 1 is declared test_for correspondence, 2 is a
// transitive import. There is no level 3 here on purpose — path proximity orders a
// candidate set, it never contributes one (docs/plans/06-m6b-static-tier.md, decision 1).
type staticCandidate struct {
	Test     string
	Level    int
	Distance int // import hops; 0 when the test was never reached through imports
}

// staticCandidates is the membership half of the static tier: which tests are related to
// the changed set at all, by evidence rather than by neighbourhood.
//
// A missing resolver is a skipped level, never an error. An adapter with no test_for
// templates, or one that declares no importscan, still gets whichever level it can
// answer (PRD #230 AC7); an adapter that can answer none returns nothing, and Select
// turns that into an honest T2 rather than an empty TS.
func staticCandidates(in Inputs) []staticCandidate {
	byTest := map[string]staticCandidate{}

	add := func(test string, level, distance int) {
		prev, seen := byTest[test]
		if !seen {
			byTest[test] = staticCandidate{Test: test, Level: level, Distance: distance}
			return
		}
		// The most confident level wins; the import distance is kept either way so the
		// level-2 tiebreak has a value even for a test found both ways.
		if level < prev.Level {
			prev.Level = level
		}
		if distance > 0 && (prev.Distance == 0 || distance < prev.Distance) {
			prev.Distance = distance
		}
		byTest[test] = prev
	}

	for _, c := range in.Changes {
		if c.Status == gitctx.Deleted {
			continue // nothing corresponds to a file that is gone
		}
		if in.Adapter != nil && in.Adapter.IsTestFile(c.Path) {
			continue // a changed test file is the direct tier's, and it already has it
		}

		if in.Adapter != nil && in.Exists != nil {
			if test, ok := in.Adapter.TestForCandidate(c.Path, in.Exists); ok {
				add(test, 1, 0)
			}
		}
		// Level 2. A nil ImportDistance is an adapter that declares no importscan: the
		// level is skipped and the selection is narrower, not absent and not an error.
		if in.ImportDistance != nil {
			for test, hops := range in.ImportDistance(c.Path) {
				add(test, 2, hops)
			}
		}
	}

	out := make([]staticCandidate, 0, len(byTest))
	for _, c := range byTest {
		out = append(out, c)
	}
	// Deterministic before ranking: map iteration order must never reach a selection.
	sort.Slice(out, func(i, j int) bool { return out[i].Test < out[j].Test })
	return out
}

// rankStatic orders the candidate set by spec §4.1's levels, most confident first, and
// falls back to lexicographic order so a full tie is reproducible.
//
//  1. declared test_for correspondence;
//  2. import distance, shortest transitive path first.
//
// The level key is compared before the distance key, so a level-1 candidate that also
// happens to carry an import distance is never reordered by it: correspondence outranks
// any import relationship. The distance key is read only WITHIN level 2, where every
// candidate was admitted by an import and the hop count is therefore the evidence
// itself — at level 1 a distance of zero means "never reached through imports", not
// "reached in zero hops", and ordering on it would be ordering on a sentinel.
//
// The level-3 key (path proximity over changedFiles, longest shared directory prefix
// first) is added by the sibling slice; changedFiles is the input that tiebreak will read.
func rankStatic(cands []staticCandidate, changedFiles []string) []string {
	ranked := append([]staticCandidate(nil), cands...)
	sort.Slice(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.Level != b.Level {
			return a.Level < b.Level
		}
		if a.Level == 2 && a.Distance != b.Distance {
			return a.Distance < b.Distance
		}
		return a.Test < b.Test
	})
	out := make([]string, 0, len(ranked))
	for _, c := range ranked {
		out = append(out, c.Test)
	}
	return out
}

// mapCannotAnswer is the TS gate (spec §4.1: "when the map cannot answer — unseeded, or
// the adapter declares selection: static").
//
// It reads the DECLARATION and the map, never Fidelity(). Fidelity answers "what is the
// best this adapter could ever produce", which is the question rtdd doctor asks; the gate
// asks "can the coverage relation answer this call", and the two differ exactly where it
// matters: a fidelity-none static adapter must still enter the gate, or it falls through
// to the unseeded-map branch and is told to seed a map it can never build.
//
// A SEEDED map that legitimately selects zero tests has m.Len() > 0 and a coverage
// adapter, so the gate stays shut and the answer stays an explicit empty: the static tier
// never launders an honest "nothing is related" into a speculative selection.
func mapCannotAnswer(in Inputs, m *mapstore.Map) bool {
	if in.Adapter != nil && in.Adapter.Selection == adapter.SelectionStatic {
		return true
	}
	return m.Len() == 0
}

// staticReason names the evidence the selection actually rests on. It may never contain
// "recorded coverage": spec §2 splits fidelity into two axes precisely so a static
// selection cannot borrow an execution-derived one's authority.
func staticReason(cands []staticCandidate) string {
	corresponds, imports := false, false
	for _, c := range cands {
		if c.Level == 1 {
			corresponds = true
		} else {
			imports = true
		}
	}
	switch {
	case corresponds && imports:
		return "static selection: tests whose declared test_for correspondence, " +
			"or whose transitive imports, reach the changed set"
	case corresponds:
		return "static selection: tests whose declared test_for correspondence names the changed set"
	default:
		return "static selection: tests that transitively import the changed set, shortest import path first"
	}
}

// unanswerableReason explains a full suite that neither the map nor the static tier could
// narrow. A static adapter is never advised to seed: coverage: none means no map is ever
// built, so `rtdd seed` is a command that cannot help it.
func unanswerableReason(in Inputs) string {
	if in.Adapter != nil && in.Adapter.Selection == adapter.SelectionStatic {
		if in.Adapter.Fidelity() == adapter.FidelityNone {
			return "the " + in.Adapter.Name + " adapter declares selection: static but no " +
				"test_for templates and no importscan command, so nothing narrower than " +
				"the full suite can be derived"
		}
		return "the " + in.Adapter.Name + " adapter selects statically, and neither " +
			"declared correspondence nor imports reach the changed set"
	}
	return "the map is unseeded, so no selection is trustworthy: run rtdd seed"
}
