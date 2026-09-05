package adapter

import (
	"errors"
	"testing"
)

// lookPath is injected: a test that asked the machine whether `node` exists would pass on
// a developer's laptop and fail on a minimal CI image, which is the opposite of a guard.
func fakeLookPath(present ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, p := range present {
		set[p] = true
	}
	return func(bin string) (string, error) {
		if set[bin] {
			return "/usr/bin/" + bin, nil
		}
		return "", errors.New("executable file not found in $PATH")
	}
}

func TestUnmetReportsMissingBinariesInDeclarationOrder(t *testing.T) {
	a := &Adapter{Name: "vitest", Requires: []Requirement{
		{Bin: "node", Reason: "runs vitest and the importscan script"},
		{Bin: "npx", Reason: "resolves the vitest binary from the lockfile"},
	}}

	got := a.Unmet(fakeLookPath("node"))
	if len(got) != 1 {
		t.Fatalf("Unmet = %v, want exactly the npx entry", got)
	}
	if got[0].Bin != "npx" || got[0].Reason == "" {
		t.Errorf("Unmet[0] = %+v, want the npx requirement with its reason", got[0])
	}
}

func TestUnmetIsEmptyWhenEverythingIsPresent(t *testing.T) {
	a := &Adapter{Name: "vitest", Requires: []Requirement{{Bin: "node", Reason: "runs vitest"}}}
	if got := a.Unmet(fakeLookPath("node")); len(got) != 0 {
		t.Errorf("Unmet = %v, want none", got)
	}
}

// An adapter declaring nothing has nothing to be missing, and a nil adapter is the
// no-adapter repo.
func TestUnmetWithNoRequirements(t *testing.T) {
	if got := (&Adapter{Name: "python"}).Unmet(fakeLookPath()); len(got) != 0 {
		t.Errorf("Unmet = %v, want none", got)
	}
	var nilAdapter *Adapter
	if got := nilAdapter.Unmet(fakeLookPath()); len(got) != 0 {
		t.Errorf("nil adapter Unmet = %v, want none", got)
	}
}
