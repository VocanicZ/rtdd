package main

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/selector"
)

// RenderWhich formats the ranked selection and the unmapped-file notice.
//
// An empty selection is stated explicitly and is never allowed to read as "all passed"
// (spec §5, tier "empty").
//
// The unmapped-file notice is the ONLY file-level signal `which` may honestly print:
// it runs nothing, so it has no fresh coverage and therefore no line-level report.
func RenderWhich(sel selector.Selection, unmapped []string) string {
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
	for _, f := range unmapped {
		fmt.Fprintf(&b, "  no map row covers: %s  (import-time-only or untested; "+
			"tests selected by static import scan)\n", f)
	}
	return b.String()
}
