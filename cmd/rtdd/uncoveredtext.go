package main

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
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
