package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/selector"
)

func TestRenderWhichRankedSelection(t *testing.T) {
	sel := selector.Selection{
		Tier:   selector.TierT0,
		Direct: []string{"tests/test_new.py"},
		Tests:  []string{"tests/test_new.py", "tests/test_it.py::test_logic"},
		Reason: "",
	}
	got := RenderWhich(sel, nil, nil)
	want := "" +
		"  tier: T0  (2 tests selected, ranked)\n" +
		"  direct: tests/test_new.py\n" +
		"    tests/test_new.py\n" +
		"    tests/test_it.py::test_logic\n"
	if got != want {
		t.Fatalf("RenderWhich()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderWhichEmptySelectionIsExplicit(t *testing.T) {
	sel := selector.Selection{Tier: selector.TierEmpty, Reason: "no map row intersects the changed set"}
	got := RenderWhich(sel, nil, nil)
	want := "" +
		"  tier: empty  (0 tests selected, ranked)\n" +
		"  reason: no map row intersects the changed set\n" +
		"  NOTHING SELECTED — this is not the same as \"all passed\".\n"
	if got != want {
		t.Fatalf("RenderWhich()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderWhichNamesUnmappedFiles(t *testing.T) {
	sel := selector.Selection{Tier: selector.TierT1, Tests: []string{"tests/test_it.py"},
		Reason: "import-time-only change"}
	got := RenderWhich(sel, []string{"src/constants.py"}, nil)
	want := "" +
		"  tier: T1  (1 test selected, ranked)\n" +
		"  reason: import-time-only change\n" +
		"    tests/test_it.py\n" +
		"  no map row covers: src/constants.py  (import-time-only or untested; " +
		"tests selected by static import scan)\n"
	if got != want {
		t.Fatalf("RenderWhich()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWhichJSONHasNoUncoveredReport(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "which", Base: "HEAD", Adapter: "python",
		Sel:           selector.Selection{Tier: selector.TierT1, Tests: []string{"tests/test_it.py"}},
		UnmappedFiles: []string{"src/constants.py"},
	})
	if out.Uncovered.Available {
		t.Fatal("which must never claim a fresh uncovered report")
	}
	if len(out.UnmappedFiles) != 1 {
		t.Fatalf("unmapped_files = %#v", out.UnmappedFiles)
	}
	if out.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0", out.ExitCode)
	}
}

// PRD #233 AC8, spec §6. A TS selection is not built from anything that ran, and the
// human tier line is where that has to be visible: without the parenthetical a static
// selection is presented in exactly the same voice as a T1 one, and a human reading the
// terminal has no signal that nothing was executed to produce it.
//
// Only the label changes. The count and the `ranked` suffix are the ones every other
// tier prints, because they are true of a static selection too — it IS ranked, by
// correspondence first and then by import distance.
func TestRenderWhichStatesTheStaticFidelityOnTheTierLine(t *testing.T) {
	sel := selector.Selection{
		Tier:   selector.TierTS,
		Tests:  []string{"src/logic.test.ts"},
		Reason: "static selection: tests whose declared test_for correspondence names the changed set",
	}
	got := RenderWhich(sel, nil, staticAdapter())
	want := "" +
		"  tier: TS (static)  (1 test selected, ranked)\n" +
		"  reason: static selection: tests whose declared test_for correspondence names the changed set\n" +
		"    src/logic.test.ts\n"
	if got != want {
		t.Fatalf("RenderWhich()\n got:\n%s\nwant:\n%s", got, want)
	}
}

// The other half of AC8: the execution-derived tiers keep the line they have. The
// fidelity parenthetical is a claim about TS alone, and a `direct`, `T0`, `T1`, `T2` or
// `empty` line that moved would be a change visible to every repository that has ever
// run rtdd — asserted byte for byte so it cannot drift in unnoticed.
func TestRenderWhichLeavesTheExecutionDerivedTierLinesByteIdentical(t *testing.T) {
	for _, tc := range []struct {
		tier selector.Tier
		want string
	}{
		{selector.TierDirect, "  tier: direct  (1 test selected, ranked)\n"},
		{selector.TierT0, "  tier: T0  (1 test selected, ranked)\n"},
		{selector.TierT1, "  tier: T1  (1 test selected, ranked)\n"},
		{selector.TierT2, "  tier: T2  (1 test selected, ranked)\n"},
		{selector.TierEmpty, "  tier: empty  (1 test selected, ranked)\n"},
	} {
		sel := selector.Selection{Tier: tc.tier, Tests: []string{"tests/test_it.py"}}
		want := tc.want + "    tests/test_it.py\n"
		if got := RenderWhich(sel, nil, nil); got != want {
			t.Errorf("RenderWhich(%s)\n got:\n%s\nwant:\n%s", tc.tier, got, want)
		}
	}
}
