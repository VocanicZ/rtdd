package selector

import "testing"

// `rtdd seed` is advice that only applies to an adapter that records coverage. A
// selection: static adapter declares coverage: none, so no map is ever built and the
// command can do nothing at all — an agent that follows the instruction burns a cycle
// and learns nothing.
//
// This is the seed-advice half of the two-axis split spec §2 draws, asserted in the same
// shape as the "recorded coverage" assertion the TS tier already carries: a static
// selection may neither borrow an execution-derived one's words nor send the caller to
// an execution-derived one's commands.
func TestStaticSelectionReasonsNeverAdviseSeeding(t *testing.T) {
	const forbidden = "run rtdd seed"

	fidelityNone := staticInputs()
	noTemplates := staticFixtureAdapter()
	noTemplates.TestFor = nil
	fidelityNone.Adapter = noTemplates

	nothingFound := staticInputs()
	nothingFound.Exists = existsIn() // the templates expand and name nothing on disk

	for _, tc := range []struct {
		name string
		in   Inputs
	}{
		{"a static tier that resolved", staticInputs()},
		{"a static adapter that can declare nothing", fidelityNone},
		{"a static adapter that found nothing for this change", nothingFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Select(tc.in)
			if contains(got.Reason, forbidden) {
				t.Errorf("Reason = %q, must not contain %q: coverage: none means no map is ever built",
					got.Reason, forbidden)
			}
		})
	}
}
