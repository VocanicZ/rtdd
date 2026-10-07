package protocol

import (
	"strings"
	"testing"
)

// flat collapses every run of whitespace to one space, so a phrase the source wraps across
// two lines still matches.
func flat(s string) string { return strings.Join(strings.Fields(s), " ") }

// fiveSteps is spec §10's process as PROTOCOL.md states it, one literal line per step, in
// the order every front-end must carry them (PRD #411 AC1).
var fiveSteps = []string{
	"1. Edit code (test first, per TDD).",
	"2. Run `rtdd which`. If `untested` names a node you changed, write its test first.",
	"3. Run **Round 1** with the project's own test command. Fix until green.",
	"4. Run **Round 2**. Fix until green; return to step 2 after any further edit.",
	"5. When the task is done — before committing or handing off — run the **full suite once**.",
}

// PRD #411 AC1: every target states the five steps of spec §10, in order.
func TestEveryFrontEndStatesTheFiveStepsInOrder(t *testing.T) {
	out := renderReal(t)
	for _, tgt := range Targets {
		t.Run(tgt.Name, func(t *testing.T) {
			body := flat(out[tgt.OutPath])
			at := -1
			for i, step := range fiveSteps {
				j := strings.Index(body, step)
				switch {
				case j < 0:
					t.Errorf("%s does not state step %d: %q", tgt.OutPath, i+1, step)
				case j < at:
					t.Errorf("%s states step %d before step %d", tgt.OutPath, i+1, i)
				default:
					at = j
				}
			}
		})
	}
}

// PRD #411 AC1: every target says rtdd runs no tests, that the agent runs each round with
// the project's own test command, and that empty Rounds 1 and 2 are no linked test, never
// a pass.
func TestEveryFrontEndSaysRtddRunsNoTestsAndEmptyRoundsAreNotAPass(t *testing.T) {
	out := renderReal(t)
	for _, tgt := range Targets {
		t.Run(tgt.Name, func(t *testing.T) {
			body := flat(out[tgt.OutPath])
			for _, want := range []string{
				"rtdd runs no tests",
				"run each round with the project's own test command",
				"no linked test, never a pass",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s does not say %q", tgt.OutPath, want)
				}
			}
		})
	}
}
