package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
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

func TestBuildSignalFiltersToInstrumentableFiles(t *testing.T) {
	// A changed test file and a changed YAML asset are not in coverage at all.
	// Passing them to Classify would report them wholly Uncovered, which is wrong.
	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
		{Path: "tests/test_it.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 12}}},
		{Path: "config/app.yaml", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 3}}},
	}
	cov := &coverage.Result{
		PerTest: []coverage.TestCoverage{
			{Test: "tests/test_it.py::test_logic", Files: map[string][]int{"src/logic.py": {5}}},
		},
		ImportTime: map[string][]int{"src/logic.py": {1, 4, 8}},
	}
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/test_it.py::test_logic", F: []string{"src/logic.py"}, C: "aaa", D: 3, S: "pass"},
		func(a, b string) string { return a })

	got := BuildSignal(SignalInput{
		Changes: changes,
		Cov:     cov,
		Map:     m,
		IsInstrumentable: func(rel string) bool {
			return rel == "src/logic.py" || rel == "src/constants.py"
		},
	})

	if len(got.Reports) != 1 || got.Reports[0].Path != "src/logic.py" {
		t.Fatalf("Reports = %#v, want only src/logic.py", got.Reports)
	}
	if got.Reports[0].UncoveredLines() != 1 {
		t.Fatalf("UncoveredLines() = %d, want 1 (line 9 only; line 8 is a def, import-time)",
			got.Reports[0].UncoveredLines())
	}
	if !got.Instrumentable["src/logic.py"] || got.Instrumentable["tests/test_it.py"] {
		t.Fatalf("Instrumentable = %#v", got.Instrumentable)
	}
	if len(got.UnmappedFiles) != 0 {
		t.Fatalf("UnmappedFiles = %#v, want empty; src/logic.py IS covered by a map row", got.UnmappedFiles)
	}
}

func TestBuildSignalReportsUnmappedInstrumentableFiles(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
	}
	cov := &coverage.Result{
		PerTest:    []coverage.TestCoverage{},
		ImportTime: map[string][]int{"src/constants.py": {1, 2, 4, 7, 8, 9, 12, 13, 14, 15}},
	}
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/test_it.py::test_logic", F: []string{"src/logic.py"}, C: "aaa", D: 3, S: "pass"},
		func(a, b string) string { return a })

	got := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              cov,
		Map:              m,
		IsInstrumentable: func(rel string) bool { return true },
	})

	want := []string{"src/constants.py"}
	if len(got.UnmappedFiles) != 1 || got.UnmappedFiles[0] != want[0] {
		t.Fatalf("UnmappedFiles = %#v, want %#v — an import-time-only file is in no row's f",
			got.UnmappedFiles, want)
	}
	if got.Reports[0].UncoveredLines() != 0 {
		t.Fatalf("UncoveredLines() = %d, want 0 — every changed line is import-time",
			got.Reports[0].UncoveredLines())
	}
}

func TestBuildSignalWithNoCoverageIsAllUncovered(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/new.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 3}}},
	}
	got := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              &coverage.Result{ImportTime: map[string][]int{}},
		Map:              mapstore.New(),
		IsInstrumentable: func(rel string) bool { return true },
	})
	if len(got.Reports) != 1 || got.Reports[0].UncoveredLines() != 3 {
		t.Fatalf("Reports = %#v, want 3 uncovered lines", got.Reports)
	}
}

// UnmappedFiles is consumed as the static-import fallback's trigger set, so it must be a
// list a caller can range over unconditionally — never nil.
func TestBuildSignalUnmappedFilesIsNeverNil(t *testing.T) {
	got := BuildSignal(SignalInput{
		Changes:          nil,
		Cov:              nil,
		Map:              mapstore.New(),
		IsInstrumentable: func(rel string) bool { return true },
	})
	if got.UnmappedFiles == nil {
		t.Fatal("UnmappedFiles = nil, want an empty slice")
	}
	if got.Instrumentable == nil {
		t.Fatal("Instrumentable = nil, want an empty map")
	}
}

// A deleted file has no new-side lines to classify and cannot be uncovered, so it must
// not reach Classify and must not be reported as unmapped either.
func TestBuildSignalSkipsDeletedFiles(t *testing.T) {
	got := BuildSignal(SignalInput{
		Changes:          []gitctx.Change{{Path: "src/gone.py", Status: gitctx.Deleted}},
		Cov:              &coverage.Result{ImportTime: map[string][]int{}},
		Map:              mapstore.New(),
		IsInstrumentable: func(rel string) bool { return true },
	})
	if len(got.Reports) != 0 {
		t.Fatalf("Reports = %#v, want none for a deleted file", got.Reports)
	}
	if len(got.UnmappedFiles) != 0 {
		t.Fatalf("UnmappedFiles = %#v, want none for a deleted file", got.UnmappedFiles)
	}
}
