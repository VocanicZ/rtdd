package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// RenderUncovered formats the post-run uncovered report exactly as spec §6 shows:
//
//	UNCOVERED: src/auth.py:52-58  (7 changed lines, no executing test)
//	import-time: src/constants.py:1-12  (executed during collection, not attributed)
//
// Import-time ranges are reported on their own line and are NEVER rendered as UNCOVERED.
// Returns "" when every changed line is Covered.
func RenderUncovered(reports []uncovered.FileReport) string {
	var b strings.Builder
	for _, r := range reports {
		for _, cr := range r.Ranges {
			if cr.Class != uncovered.Uncovered {
				continue
			}
			n := cr.Range.End - cr.Range.Start + 1
			fmt.Fprintf(&b, "  UNCOVERED: %s:%s  (%d changed %s, no executing test)\n",
				r.Path, spanText(cr.Range), n, plural(n, "line", "lines"))
		}
	}
	for _, r := range reports {
		for _, cr := range r.Ranges {
			if cr.Class != uncovered.ImportTime {
				continue
			}
			fmt.Fprintf(&b, "  import-time: %s:%s  (executed during collection, not attributed)\n",
				r.Path, spanText(cr.Range))
		}
	}
	return b.String()
}

// spanText renders a line range as spec §6 does: "52-58", or bare "9" for a single line.
func spanText(r gitctx.LineRange) string {
	if r.Start == r.End {
		return fmt.Sprintf("%d", r.Start)
	}
	return fmt.Sprintf("%d-%d", r.Start, r.End)
}

// SignalInput is everything the post-run uncovered signal needs.
//
// Cov MUST be the coverage produced by the run that just finished. Line data is never
// persisted in map.jsonl, so both the changed line ranges and the coverage are current by
// construction and there is no line-drift problem (spec §4, §6). A nil Cov means no test
// executed, which classifies every changed line Uncovered — honest only for a command
// that actually ran the selection.
type SignalInput struct {
	Changes          []gitctx.Change
	Cov              *coverage.Result
	IsInstrumentable func(rel string) bool
	Map              *mapstore.Map
}

// SignalOutput carries the classification and the import-fallback trigger set.
//
// Instrumentable is keyed by every changed path, instrumentable or not, because the
// --json changed set reports the verdict for each one.
type SignalOutput struct {
	Reports        []uncovered.FileReport
	Instrumentable map[string]bool
	UnmappedFiles  []string
}

// BuildSignal filters the changed set to instrumentable files and classifies them
// against fresh post-run coverage.
//
// The filter runs BEFORE Classify on purpose: Classify reports a file absent from
// coverage as wholly Uncovered, which is right for a new source file and wrong for a test
// file or an opaque asset, neither of which coverage ever measures.
//
// UnmappedFiles is the set of changed instrumentable files that NO map row covers.
// Because import-time lines are attributed to no test, they never enter any row's f, so
// this set is exactly the static-import fallback's trigger set (spec §6, D14). It is
// never nil: callers range over it unconditionally.
func BuildSignal(in SignalInput) SignalOutput {
	out := SignalOutput{
		Instrumentable: map[string]bool{},
		UnmappedFiles:  []string{},
	}
	var keep []gitctx.Change
	for _, c := range in.Changes {
		ok := in.IsInstrumentable != nil && in.IsInstrumentable(c.Path)
		out.Instrumentable[c.Path] = ok
		if !ok || c.Status == gitctx.Deleted {
			continue
		}
		keep = append(keep, c)
		if in.Map != nil && len(in.Map.TestsCovering([]string{c.Path})) == 0 {
			out.UnmappedFiles = append(out.UnmappedFiles, c.Path)
		}
	}
	sort.Strings(out.UnmappedFiles)
	out.Reports = uncovered.Classify(keep, in.Cov)
	return out
}
