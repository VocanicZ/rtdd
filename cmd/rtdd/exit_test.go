package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

func uncoveredReports() []uncovered.FileReport {
	return []uncovered.FileReport{
		{Path: "src/auth.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 52, End: 58}, Class: uncovered.Uncovered},
		}},
	}
}

func TestExitCodeZeroWithNonEmptyUncoveredReport(t *testing.T) {
	// The load-bearing guarantee: `rtdd run` exits 1 ONLY when a test fails.
	// Uncovered is a signal, not a verdict.
	outcomes := []report.Outcome{
		{Test: "tests/test_auth.py::test_login", Status: "pass", DurationMS: 12},
		{Test: "tests/test_auth.py::test_logout", Status: "pass", DurationMS: 9},
	}
	reports := uncoveredReports()
	if n := uncovered.Summarize(reports).UncoveredLines; n != 7 {
		t.Fatalf("fixture is wrong: uncovered_lines = %d, want 7", n)
	}
	if got := ExitCodeFor(outcomes, reports); got != 0 {
		t.Fatalf("ExitCodeFor() = %d, want 0 with 7 uncovered lines and no failing test", got)
	}
}

func TestExitCodeZeroWithImportTimeOnlyReport(t *testing.T) {
	outcomes := []report.Outcome{{Test: "tests/t.py::a", Status: "pass", DurationMS: 1}}
	reports := []uncovered.FileReport{
		{Path: "src/constants.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 15}, Class: uncovered.ImportTime},
		}},
	}
	if got := ExitCodeFor(outcomes, reports); got != 0 {
		t.Fatalf("ExitCodeFor() = %d, want 0", got)
	}
}

func TestExitCodeOneOnFailure(t *testing.T) {
	tests := []struct {
		name     string
		outcomes []report.Outcome
		want     int
	}{
		{"all pass", []report.Outcome{{Test: "a", Status: "pass"}}, 0},
		{"one fail", []report.Outcome{{Test: "a", Status: "pass"}, {Test: "b", Status: "fail"}}, 1},
		{"one error", []report.Outcome{{Test: "a", Status: "error"}}, 1},
		{"skips are not failures", []report.Outcome{{Test: "a", Status: "skip"}}, 0},
		{"empty selection", nil, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCodeFor(tc.outcomes, uncoveredReports()); got != tc.want {
				t.Fatalf("ExitCodeFor() = %d, want %d", got, tc.want)
			}
		})
	}
}
