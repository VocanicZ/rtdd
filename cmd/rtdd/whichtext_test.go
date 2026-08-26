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
	got := RenderWhich(sel, nil)
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
	got := RenderWhich(sel, nil)
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
	got := RenderWhich(sel, []string{"src/constants.py"})
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
