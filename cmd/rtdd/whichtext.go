package main

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// RenderWhich formats the ranked selection and the unmapped-file notice.
//
// An empty selection is stated explicitly and is never allowed to read as "all passed"
// (spec §5, tier "empty").
//
// The unmapped-file notice is the ONLY file-level signal `which` may honestly print:
// it runs nothing, so it has no fresh coverage and therefore no line-level report. ad is
// the adapter that made the selection, and it decides whether the notice is honest at
// all — see unmappedNoticeApplies.
func RenderWhich(sel selector.Selection, unmapped []string, ad *adapter.Adapter) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  tier: %s  (%d %s selected, ranked)\n",
		sel.Tier.String(), len(sel.Tests), plural(len(sel.Tests), "test", "tests"))
	if len(sel.Direct) > 0 {
		fmt.Fprintf(&b, "  direct: %s\n", strings.Join(sel.Direct, ", "))
	}
	if sel.Reason != "" {
		fmt.Fprintf(&b, "  reason: %s\n", sel.Reason)
	}
	for _, t := range sel.Tests {
		fmt.Fprintf(&b, "    %s\n", t)
	}
	if len(sel.Tests) == 0 {
		b.WriteString("  NOTHING SELECTED — this is not the same as \"all passed\".\n")
	}
	if !unmappedNoticeApplies(ad) {
		return b.String()
	}
	for _, f := range unmapped {
		fmt.Fprintf(&b, "  no map row covers: %s  (import-time-only or untested; "+
			"tests selected by static import scan)\n", f)
	}
	return b.String()
}

// unmappedNoticeApplies reports whether the no-map-row notice is true of this adapter.
//
// For a `selection: static` adapter it is not, on all three of its claims (issue #279).
// There is no map for a row to be absent from — the adapter declares `coverage: none`, so
// nothing is ever recorded and EVERY changed file is trivially "unmapped".
// `import-time-only` is a coverage-attribution concept, and nothing was instrumented to
// attribute. And "tests selected by static import scan" names the wrong evidence twice
// over: the selection came from `test_for` correspondence, and an adapter that declares
// no importscan ran no scan at all — the #275 rule that a skipped level is skipped, on the
// uncovered-lines surface instead of the `reason` string.
//
// The notice is suppressed rather than reworded because a static selection has no
// file-level signal left to report here: what the selection rests on is already the
// `reason` line above, and an empty selection already says NOTHING SELECTED in as many
// words. A nil adapter keeps the notice — nothing declared otherwise, and the coverage
// reading is the one every existing repository has.
func unmappedNoticeApplies(ad *adapter.Adapter) bool {
	return ad == nil || ad.Selection != adapter.SelectionStatic
}
