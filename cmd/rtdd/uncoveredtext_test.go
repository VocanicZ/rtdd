package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

func TestRenderUncoveredMatchesSpecExample(t *testing.T) {
	reports := []uncovered.FileReport{
		{Path: "src/auth.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 40, End: 51}, Class: uncovered.Covered},
			{Range: gitctx.LineRange{Start: 52, End: 58}, Class: uncovered.Uncovered},
		}},
		{Path: "src/constants.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 12}, Class: uncovered.ImportTime},
		}},
	}
	got := RenderUncovered(reports)
	want := "" +
		"  UNCOVERED: src/auth.py:52-58  (7 changed lines, no executing test)\n" +
		"  import-time: src/constants.py:1-12  (executed during collection, not attributed)\n"
	if got != want {
		t.Fatalf("RenderUncovered()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderUncoveredCleanWhenOnlyImportTimeAndCovered(t *testing.T) {
	// A file whose changed lines are ALL import-time is correctly tested and must
	// produce no UNCOVERED line at all.
	reports := []uncovered.FileReport{
		{Path: "src/constants.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 15}, Class: uncovered.ImportTime},
		}},
	}
	got := RenderUncovered(reports)
	want := "  import-time: src/constants.py:1-15  (executed during collection, not attributed)\n"
	if got != want {
		t.Fatalf("RenderUncovered()\n got:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "UNCOVERED") {
		t.Fatal("an all-import-time file must never render an UNCOVERED line")
	}
}

func TestRenderUncoveredSingleLineRange(t *testing.T) {
	reports := []uncovered.FileReport{
		{Path: "src/logic.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 9, End: 9}, Class: uncovered.Uncovered},
		}},
	}
	got := RenderUncovered(reports)
	want := "  UNCOVERED: src/logic.py:9  (1 changed line, no executing test)\n"
	if got != want {
		t.Fatalf("RenderUncovered()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderUncoveredAllCoveredIsEmpty(t *testing.T) {
	reports := []uncovered.FileReport{
		{Path: "src/logic.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 5, End: 5}, Class: uncovered.Covered},
		}},
	}
	if got := RenderUncovered(reports); got != "" {
		t.Fatalf("RenderUncovered() = %q, want empty", got)
	}
}

func TestRenderUncoveredNoReports(t *testing.T) {
	if got := RenderUncovered(nil); got != "" {
		t.Fatalf("RenderUncovered(nil) = %q, want empty", got)
	}
}
