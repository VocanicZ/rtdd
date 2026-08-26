package main

import (
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// ExitCodeFor returns the process exit code for a completed run.
//
// It is 1 if and only if a test failed or errored. The uncovered report is accepted as a
// parameter precisely so that this function's tests can assert it is IGNORED: RTDD never
// exits nonzero to express a policy opinion (spec §2 non-goals, §6, decision D3).
func ExitCodeFor(outcomes []report.Outcome, _ []uncovered.FileReport) int {
	for _, o := range outcomes {
		if o.Status == "fail" || o.Status == "error" {
			return 1
		}
	}
	return 0
}
