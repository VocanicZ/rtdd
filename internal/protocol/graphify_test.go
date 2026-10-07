package protocol

import (
	"strings"
	"testing"
)

// PRD #411 AC3: every target states the three facts about graphify — used when its graph
// exists, never trusted for changed files, never run by rtdd.
func TestEveryFrontEndDescribesGraphifyAsOptional(t *testing.T) {
	out := renderReal(t)
	for _, tgt := range Targets {
		t.Run(tgt.Name, func(t *testing.T) {
			body := flat(out[tgt.OutPath])
			for _, want := range []string{
				"graphify is optional",
				"used when `graphify-out/graph.json` exists",
				"never trusted for changed files",
				"never run by rtdd",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s does not say %q", tgt.OutPath, want)
				}
			}
		})
	}
}

// PRD #411 AC3: no target tells the agent graphify is required, or that rtdd runs or
// updates it.
func TestNoFrontEndRequiresGraphifyOrHasRtddRunIt(t *testing.T) {
	out := renderReal(t)
	for _, tgt := range Targets {
		body := strings.ToLower(flat(out[tgt.OutPath]))
		for _, bad := range []string{"graphify is required", "requires graphify", "install graphify",
			"rtdd runs graphify", "rtdd updates graphify", "rtdd will run graphify", "rtdd will update graphify"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s says %q", tgt.OutPath, bad)
			}
		}
	}
}
