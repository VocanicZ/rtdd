package selector

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// Select computes the tier and the ranked test list.
//
// Order of evaluation, and it matters:
//  1. the direct set, BEFORE any map lookup — a test the agent just wrote has no map row,
//     and in v1 it was therefore in no tier and never ran;
//  2. T2 escalations (full-escalate file, drift guard);
//  3. T1 escalations (merge commit, opaque file, import-time-only file, stale row);
//  4. T0;
//  5. TS — static correspondence and imports, reached only when the coverage relation
//     cannot answer: an unseeded map, or an adapter declaring selection: static. TS
//     never overrides a usable map (spec §4.1), so steps 3 and 4 are skipped on this
//     path and a seeded repository never reaches it;
//  6. TierEmpty, reported explicitly with a Reason — or, when the static tier had
//     nothing to offer either, the full suite at T2 with a reason naming what was
//     missing.
//
// Select is pure: it makes no git calls, touches no filesystem, and prints nothing.
// Everything it needs about the world arrives through Inputs.
func Select(in Inputs) Selection {
	m := in.Map
	if m == nil {
		m = mapstore.New()
	}
	cfg := in.Cfg.orDefault()

	changedFiles := make([]string, 0, len(in.Changes)*2)
	for _, c := range in.Changes {
		changedFiles = append(changedFiles, c.Path)
		if c.OldPath != "" {
			changedFiles = append(changedFiles, c.OldPath)
		}
	}
	sort.Strings(changedFiles)
	changedFiles = dedupe(changedFiles)

	// (1) direct tier, first, with no map lookup at all.
	direct := make([]string, 0, len(in.Changes))
	for _, c := range in.Changes {
		if c.Status == gitctx.Deleted {
			continue // a deleted test file cannot be executed
		}
		if in.Adapter.IsTestFile(c.Path) {
			direct = append(direct, c.Path)
		}
	}
	sort.Strings(direct)
	direct = dedupe(direct)

	// (2) T2.
	if reason, escalate := escalateFull(in, m, cfg); escalate {
		return Selection{
			Tier:   TierT2,
			Direct: direct,
			Tests:  mergeFirst(direct, in.AllTests),
			Reason: reason,
		}
	}

	// (5) TS. The map cannot answer, so T1 and T0 have nothing to consult and are
	// skipped entirely; static evidence is the only evidence there is.
	if mapCannotAnswer(in, m) {
		cands := staticCandidates(in)
		if ranked := rankStatic(cands, changedFiles); len(ranked) > 0 {
			return Selection{
				Tier:   TierTS,
				Direct: direct,
				Tests:  mergeFirst(direct, ranked),
				Reason: staticReason(cands),
			}
		}
		return Selection{
			Tier:   TierT2,
			Direct: direct,
			Tests:  mergeFirst(direct, in.AllTests),
			Reason: unanswerableReason(in),
		}
	}

	t0 := m.TestsCovering(changedFiles)

	// (3) T1.
	if extra, reason := escalateT1(in, m, cfg, t0); reason != "" {
		ranked := Rank(m, dedupe(append(append([]string{}, t0...), extra...)), changedFiles)
		tests := mergeFirst(direct, ranked)
		if len(tests) == 0 {
			return Selection{
				Tier:   TierEmpty,
				Direct: direct,
				Reason: reason + "; but nothing in the map covers the changed set",
			}
		}
		return Selection{Tier: TierT1, Direct: direct, Tests: tests, Reason: reason}
	}

	// (4) T0, and (5) empty.
	ranked := Rank(m, t0, changedFiles)
	tests := mergeFirst(direct, ranked)
	switch {
	case len(tests) == 0:
		return Selection{
			Tier:   TierEmpty,
			Direct: direct,
			Reason: emptyReason(in),
		}
	case len(ranked) == 0:
		return Selection{
			Tier:   TierDirect,
			Direct: direct,
			Tests:  tests,
			Reason: "changed test files only; no mapped test covers the changed set",
		}
	default:
		return Selection{
			Tier:   TierT0,
			Direct: direct,
			Tests:  tests,
			Reason: "tests whose recorded coverage intersects the changed set",
		}
	}
}

// emptyReason explains an empty selection without claiming more than the selector
// checked. "no test file changed" is only sayable when the adapter was there to say it:
// with no adapter every classifier returns false, so the sentence would deny a changed
// test file the selector was never able to see. A deleted test file did change too — it
// simply cannot be executed.
func emptyReason(in Inputs) string {
	const noCoverage = "no test in the map covers the changed set"
	if in.Adapter == nil {
		return noCoverage + ", and file classification is disabled (no adapter): " +
			"a changed test file cannot be recognised"
	}
	var deleted []string
	for _, c := range in.Changes {
		if c.Status == gitctx.Deleted && in.Adapter.IsTestFile(c.Path) {
			deleted = append(deleted, c.Path)
		}
	}
	if len(deleted) > 0 {
		return noCoverage + "; the only changed test files were deleted and cannot be run: " +
			strings.Join(deleted, ", ")
	}
	return noCoverage + ", and no test file changed"
}

// escalateFull covers the T2 escalations that do not depend on the map. The unseeded-map
// branch used to live here and now belongs to the TS gate: an unseeded map is not an
// escalation, it is the coverage relation being unable to answer at all, and what to do
// about that depends on what the adapter can declare instead.
func escalateFull(in Inputs, m *mapstore.Map, cfg Config) (string, bool) {
	if !in.fullRunCoversCurrentConfig() {
		for _, c := range in.Changes {
			if in.Adapter.IsFullEscalate(c.Path) {
				return "full-escalate file changed: " + c.Path, true
			}
		}
	}
	if cfg.DriftGuard > 0 && in.Cycles >= cfg.DriftGuard {
		return fmt.Sprintf("drift guard reached: %d cycles since the last full run (limit %d)",
			in.Cycles, cfg.DriftGuard), true
	}
	return "", false
}

// fullRunCoversCurrentConfig reports whether a completed full run already saw the
// `full_escalate` files in the state they are in now.
//
// It is the difference between "the config changed" and "the config changed and
// nothing has run everything since". Only the second is a reason to run everything:
// once the full suite has been through this config, a diff that still mentions
// conftest.py is describing an edit that has already been paid for. An unknown
// last-full state is never treated as covered.
func (in Inputs) fullRunCoversCurrentConfig() bool {
	return in.EscalateDigestAtLastFull != "" && in.EscalateDigestAtLastFull == in.EscalateDigest
}

// escalateT1 returns the tests T1 adds to T0, and the reason for the escalation.
// An empty reason means no escalation.
func escalateT1(in Inputs, m *mapstore.Map, cfg Config, t0 []string) (extra []string, reason string) {
	if in.Merge {
		reason = "HEAD is a merge commit: a union-merged map can be stale relative to the merged code"
	}

	for _, c := range in.Changes {
		if !in.Adapter.IsOpaque(c.Path) {
			continue
		}
		extra = append(extra, m.TestsCovering(filesUnder(m, path.Dir(c.Path)))...)
		if reason == "" {
			reason = "opaque file changed (coverage cannot see inside it): " + c.Path
		}
	}

	if in.ImportOnly != nil {
		for _, c := range in.Changes {
			ids := in.ImportOnly(c.Path)
			if len(ids) == 0 {
				continue
			}
			extra = append(extra, ids...)
			if reason == "" {
				reason = "import-time-only file changed: " + c.Path
			}
		}
	}

	if in.Distance != nil && cfg.StaleCommits > 0 {
		// t0 arrives in mapstore's map-iteration order, which Go randomises per run. The
		// scan below names ONE row in the reason, so an unordered scan would put map
		// iteration order into the answer: identical inputs, a different sentence each
		// run, and a seeded repository whose selection is no longer byte-identical.
		scan := append([]string(nil), t0...)
		sort.Strings(scan)
		for _, id := range scan {
			r, ok := m.Get(id)
			if !ok {
				continue
			}
			d := in.Distance(r.C)
			if d < 0 {
				if reason == "" {
					reason = fmt.Sprintf("row %s records commit %q, which is unreachable: "+
						"age unknown, treated as stale", id, r.C)
				}
				break
			}
			if d > cfg.StaleCommits {
				if reason == "" {
					reason = fmt.Sprintf("row %s is %d commits stale (limit %d)", id, d, cfg.StaleCommits)
				}
				break
			}
		}
	}

	return dedupe(extra), reason
}

// filesUnder returns every file in the map that lives under dir.
func filesUnder(m *mapstore.Map, dir string) []string {
	out := []string{}
	prefix := dir + "/"
	for f := range m.FanOut() {
		if dir == "." || strings.HasPrefix(f, prefix) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// mergeFirst returns first, then every member of rest not already present.
// This is what keeps the direct tier ahead of everything else.
func mergeFirst(first, rest []string) []string {
	out := make([]string, 0, len(first)+len(rest))
	seen := make(map[string]struct{}, len(first)+len(rest))
	for _, group := range [][]string{first, rest} {
		for _, s := range group {
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
