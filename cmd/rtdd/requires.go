package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// lookPath resolves a prerequisite binary on this machine.
//
// It is a package variable rather than a direct exec.LookPath call at each site so tests
// can fix what is installed: a guard that asked the real PATH would pass on a laptop with
// node installed and fail on a minimal CI image, which is the opposite of a guard.
var lookPath = exec.LookPath

// RenderRequirements formats the unmet-prerequisite findings. Pure; no findings renders
// the empty string, because a heading over an empty list reads as a problem.
//
// Spec §4.3: an unmet prerequisite surfaces here, naming the binary, the adapter that
// needs it and the declared reason — never as a mid-run parse failure against a report
// file that was never written. It is shared between `doctor` and `init` because the spec
// requires the same block at both, and a copy in each is two chances to drift apart.
//
// One finding per line, in the order given, which is each adapter's declaration order.
func RenderRequirements(findings []adapter.UnmetFinding) string {
	if len(findings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("prerequisites\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "  %s is not on PATH — needed by adapter %s: %s\n", f.Req.Bin, f.Adapter, f.Req.Reason)
	}
	b.WriteString("\n")
	return b.String()
}
