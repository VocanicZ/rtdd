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

// At least TWO requirements must be unmet, or "declaration order" is never exercised:
// a one-element slice is in every order at once. The declared order here is deliberately
// NOT alphabetical, so a sorted implementation fails this test.
func TestUnmetReportsMissingBinariesInDeclarationOrder(t *testing.T) {
	a := &Adapter{Name: "vitest", Requires: []Requirement{
		{Bin: "npx", Reason: "resolves the vitest binary from the lockfile"},
		{Bin: "node", Reason: "runs vitest and the importscan script"},
		{Bin: "jq", Reason: "reshapes the coverage json"},
	}}

	got := a.Unmet(fakeLookPath("node"))
	if len(got) != 2 {
		t.Fatalf("Unmet = %v, want the npx and jq entries", got)
	}
	want := []string{"npx", "jq"}
	for i, bin := range want {
		if got[i].Bin != bin {
			t.Errorf("Unmet[%d].Bin = %q, want %q (declaration order)", i, got[i].Bin, bin)
		}
		if got[i].Reason == "" {
			t.Errorf("Unmet[%d] = %+v, want its declared reason carried through", i, got[i])
		}
	}
}

// UnmetFindings is the doctor/init-shared collector: every unmet prerequisite of every
// adapter it is given, adapter by adapter, each in declaration order. Callers pass the
// DETECTED adapters — a binary only some other toolchain wants is not a finding about
// this repository.
func TestUnmetFindingsPairsEveryUnmetEntryWithItsAdapter(t *testing.T) {
	vitest := &Adapter{Name: "vitest", Requires: []Requirement{
		{Bin: "npx", Reason: "resolves the vitest binary"},
		{Bin: "node", Reason: "runs vitest"},
	}}
	python := &Adapter{Name: "python", Requires: []Requirement{
		{Bin: "pytest", Reason: "runs the suite"},
	}}

	got := UnmetFindings([]*Adapter{vitest, python}, fakeLookPath("node"))

	want := []UnmetFinding{
		{Adapter: "vitest", Req: Requirement{Bin: "npx", Reason: "resolves the vitest binary"}},
		{Adapter: "python", Req: Requirement{Bin: "pytest", Reason: "runs the suite"}},
	}
	if len(got) != len(want) {
		t.Fatalf("UnmetFindings = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("UnmetFindings[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Nothing missing is no findings at all, not an empty heading's worth of noise.
func TestUnmetFindingsIsEmptyWhenEveryPrerequisiteResolves(t *testing.T) {
	a := &Adapter{Name: "python", Requires: []Requirement{{Bin: "pytest", Reason: "runs the suite"}}}
	if got := UnmetFindings([]*Adapter{a}, fakeLookPath("pytest")); len(got) != 0 {
		t.Errorf("UnmetFindings = %+v, want none", got)
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
