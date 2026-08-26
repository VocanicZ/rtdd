package uncovered

import (
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// f1Coverage is fixture F1, measured on 2026-08-26 with coverage.py 7.15.4 /
// pytest 9.0.3 / Python 3.13.5 on a project whose two tests both pass:
//
//	src/__init__.py   ctx=''                                  lines=[0]
//	src/constants.py  ctx=''                                  lines=[1,2,4,7,8,9,12,13,14,15]
//	src/logic.py      ctx=''                                  lines=[1,4,8]
//	src/logic.py      ctx='tests/test_it.py::test_logic|run'  lines=[5]
//
// src/constants.py holds a module constant, an Enum and a @dataclass, is imported and
// asserted on by BOTH passing tests, and is attributed to ZERO test contexts.
func f1Coverage() *coverage.Result {
	return &coverage.Result{
		PerTest: []coverage.TestCoverage{
			{
				Test:  "tests/test_it.py::test_logic",
				Files: map[string][]int{"src/logic.py": {5}},
			},
			{
				Test:  "tests/test_it.py::test_constants",
				Files: map[string][]int{},
			},
		},
		ImportTime: map[string][]int{
			"src/__init__.py":  {0},
			"src/constants.py": {1, 2, 4, 7, 8, 9, 12, 13, 14, 15},
			"src/logic.py":     {1, 4, 8},
		},
	}
}

func TestClassifyImportTimeOnlyFileIsClean(t *testing.T) {
	// The whole point of ImportTime: a dataclass/enum/constants module changed
	// end to end, asserted on by two passing tests, must produce ZERO uncovered lines.
	changes := []gitctx.Change{
		{
			Path:   "src/constants.py",
			Status: gitctx.Modified,
			Lines:  []gitctx.LineRange{{Start: 1, End: 15}},
		},
	}
	got := Classify(changes, f1Coverage())
	if len(got) != 1 {
		t.Fatalf("Classify() len = %d, want 1", len(got))
	}
	if n := got[0].UncoveredLines(); n != 0 {
		t.Fatalf("UncoveredLines() = %d, want 0 — import-time lines must NEVER be Uncovered\n got: %#v", n, got[0].Ranges)
	}
	for _, r := range got[0].Ranges {
		if r.Class == Uncovered {
			t.Fatalf("range %d-%d classified Uncovered; import-time lines must never be", r.Range.Start, r.Range.End)
		}
	}
}

func TestClassifyThreeWayInOneFile(t *testing.T) {
	// src/logic.py exhibits all three classes at once:
	//   1 import-time, 4 import-time, 5 covered, 8 import-time, 9 uncovered.
	changes := []gitctx.Change{
		{
			Path:   "src/logic.py",
			Status: gitctx.Modified,
			Lines:  []gitctx.LineRange{{Start: 1, End: 9}},
		},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path: "src/logic.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 1, End: 1}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 2, End: 3}, Class: Uncovered},
				{Range: gitctx.LineRange{Start: 4, End: 4}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 5, End: 5}, Class: Covered},
				{Range: gitctx.LineRange{Start: 6, End: 7}, Class: Uncovered},
				{Range: gitctx.LineRange{Start: 8, End: 8}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 9, End: 9}, Class: Uncovered},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifyIsLineGranularNotFileGranular(t *testing.T) {
	// Audit A2: adding a function to an ALREADY-COVERED file must report the new
	// function's lines as Uncovered. v1 was file-granular and reported green here.
	changes := []gitctx.Change{
		{
			Path:   "src/logic.py",
			Status: gitctx.Modified,
			Lines:  []gitctx.LineRange{{Start: 8, End: 9}},
		},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path: "src/logic.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 8, End: 8}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 9, End: 9}, Class: Uncovered},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
	if n := got[0].UncoveredLines(); n != 1 {
		t.Fatalf("UncoveredLines() = %d, want 1", n)
	}
}

func TestClassifyBrandNewFileIsAllUncovered(t *testing.T) {
	changes := []gitctx.Change{
		{
			Path:   "src/brand_new.py",
			Status: gitctx.Added,
			Lines:  []gitctx.LineRange{{Start: 1, End: 4}},
		},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path: "src/brand_new.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 1, End: 4}, Class: Uncovered},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifyTestAttributionBeatsImportTime(t *testing.T) {
	cov := &coverage.Result{
		PerTest: []coverage.TestCoverage{
			{Test: "tests/test_x.py::test_a", Files: map[string][]int{"src/dual.py": {4}}},
		},
		ImportTime: map[string][]int{"src/dual.py": {4}},
	}
	changes := []gitctx.Change{
		{Path: "src/dual.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 4, End: 4}}},
	}
	got := Classify(changes, cov)
	want := []FileReport{
		{
			Path:   "src/dual.py",
			Ranges: []ClassifiedRange{{Range: gitctx.LineRange{Start: 4, End: 4}, Class: Covered}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifySkipsDeletedAndEmpty(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/gone.py", Status: gitctx.Deleted},
		{Path: "src/empty.py", Status: gitctx.Added, Lines: nil},
	}
	got := Classify(changes, f1Coverage())
	if len(got) != 0 {
		t.Fatalf("Classify() = %#v, want empty", got)
	}
}

func TestClassifyOverlappingRangesAreDeduped(t *testing.T) {
	changes := []gitctx.Change{
		{
			Path:   "src/logic.py",
			Status: gitctx.Modified,
			Lines: []gitctx.LineRange{
				{Start: 4, End: 5},
				{Start: 5, End: 5},
				{Start: 5, End: 6},
			},
		},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path: "src/logic.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 4, End: 4}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 5, End: 5}, Class: Covered},
				{Range: gitctx.LineRange{Start: 6, End: 6}, Class: Uncovered},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifySortsByPath(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/z.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 1}}},
		{Path: "src/a.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 1}}},
	}
	got := Classify(changes, f1Coverage())
	if len(got) != 2 || got[0].Path != "src/a.py" || got[1].Path != "src/z.py" {
		t.Fatalf("Classify() paths not sorted: %#v", got)
	}
}

func TestClassifyRangesSortedAscendingWithinFile(t *testing.T) {
	// The caller may hand ranges in any order; each file's ranges come back ascending.
	changes := []gitctx.Change{
		{
			Path:   "src/logic.py",
			Status: gitctx.Modified,
			Lines: []gitctx.LineRange{
				{Start: 9, End: 9},
				{Start: 1, End: 1},
				{Start: 5, End: 5},
			},
		},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path: "src/logic.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 1, End: 1}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 5, End: 5}, Class: Covered},
				{Range: gitctx.LineRange{Start: 9, End: 9}, Class: Uncovered},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifyNilCoverageIsAllUncovered(t *testing.T) {
	// A run that produced no coverage at all must not panic: everything is Uncovered.
	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 3}}},
	}
	got := Classify(changes, nil)
	want := []FileReport{
		{
			Path:   "src/logic.py",
			Ranges: []ClassifiedRange{{Range: gitctx.LineRange{Start: 1, End: 3}, Class: Uncovered}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifyImportTimeOnlyFileHasNoUncoveredGaps(t *testing.T) {
	// Fixture F1's src/constants.py is measured but attributed to ZERO test contexts:
	// its import-time lines are 1,2,4,7,8,9,12-15 and the gaps (3,5,6,10,11) are blank
	// lines coverage never records. Coverage cannot tell a blank line from a dead
	// statement, so an import-time-only file classifies wholly ImportTime rather than
	// reporting blank lines as Uncovered (spec §6, audit A1).
	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path:   "src/constants.py",
			Ranges: []ClassifiedRange{{Range: gitctx.LineRange{Start: 1, End: 15}, Class: ImportTime}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifyImportTimeOnlyRuleNeedsZeroTestAttribution(t *testing.T) {
	// The blanket rule applies ONLY to a file no test touches. src/logic.py has one
	// test-attributed line, so it stays line-granular and its unexecuted line 9 is
	// still Uncovered — audit A2 must not be traded away for A1.
	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 9, End: 9}}},
	}
	got := Classify(changes, f1Coverage())
	if len(got) != 1 || len(got[0].Ranges) != 1 || got[0].Ranges[0].Class != Uncovered {
		t.Fatalf("Classify() = %#v, want line 9 Uncovered", got)
	}
}

func TestClassifyEmptyTestAttributionIsNotAttribution(t *testing.T) {
	// A PerTest entry carrying an EMPTY line slice for a file attributes nothing —
	// F1's test_constants is exactly that shape — so the file is still import-time-only.
	cov := &coverage.Result{
		PerTest: []coverage.TestCoverage{
			{Test: "tests/test_it.py::test_constants", Files: map[string][]int{"src/constants.py": {}}},
		},
		ImportTime: map[string][]int{"src/constants.py": {1, 2, 4}},
	}
	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 5}}},
	}
	got := Classify(changes, cov)
	if n := Summarize(got).UncoveredLines; n != 0 {
		t.Fatalf("UncoveredLines = %d, want 0; an empty line slice is not test attribution\n got: %#v", n, got)
	}
}

func TestClassifyFileAbsentFromCoverageIsUncoveredNotImportTime(t *testing.T) {
	// The import-time-only rule needs the file to be MEASURED. A file coverage never
	// saw at all is a brand-new source file: wholly Uncovered.
	changes := []gitctx.Change{
		{Path: "src/never_seen.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 3}}},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path:   "src/never_seen.py",
			Ranges: []ClassifiedRange{{Range: gitctx.LineRange{Start: 1, End: 3}, Class: Uncovered}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassString(t *testing.T) {
	tests := []struct {
		c    Class
		want string
	}{
		{Covered, "covered"},
		{Uncovered, "uncovered"},
		{ImportTime, "import-time"},
		{Class(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.c.String(); got != tc.want {
			t.Fatalf("Class(%d).String() = %q, want %q", int(tc.c), got, tc.want)
		}
	}
}

func TestUncoveredLinesCountsWholeRanges(t *testing.T) {
	r := FileReport{
		Path: "src/logic.py",
		Ranges: []ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 3}, Class: Uncovered},
			{Range: gitctx.LineRange{Start: 4, End: 4}, Class: Covered},
			{Range: gitctx.LineRange{Start: 5, End: 9}, Class: ImportTime},
			{Range: gitctx.LineRange{Start: 10, End: 11}, Class: Uncovered},
		},
	}
	if got := r.UncoveredLines(); got != 5 {
		t.Fatalf("UncoveredLines() = %d, want 5", got)
	}
	if got := (FileReport{Path: "src/none.py"}).UncoveredLines(); got != 0 {
		t.Fatalf("UncoveredLines() on empty report = %d, want 0", got)
	}
}

func TestSummarize(t *testing.T) {
	reports := []FileReport{
		{
			Path: "src/a.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 1, End: 2}, Class: Covered},
				{Range: gitctx.LineRange{Start: 3, End: 5}, Class: Uncovered},
			},
		},
		{
			Path: "src/b.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 1, End: 4}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 5, End: 5}, Class: Uncovered},
			},
		},
	}
	got := Summarize(reports)
	want := Summary{Files: 2, CoveredLines: 2, UncoveredLines: 4, ImportTimeLines: 4}
	if got != want {
		t.Fatalf("Summarize() = %#v, want %#v", got, want)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	if got := Summarize(nil); got != (Summary{}) {
		t.Fatalf("Summarize(nil) = %#v, want zero Summary", got)
	}
}

func TestSummarizeMatchesUncoveredLines(t *testing.T) {
	// Summarize's UncoveredLines total is the sum of the per-file counts, so the
	// --json header and the per-file rows can never disagree.
	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 9}}},
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
	}
	reports := Classify(changes, f1Coverage())
	sum := 0
	for _, r := range reports {
		sum += r.UncoveredLines()
	}
	if got := Summarize(reports); got.UncoveredLines != sum {
		t.Fatalf("Summarize().UncoveredLines = %d, want %d", got.UncoveredLines, sum)
	}
}

// TestUncoveredNeverReadsMapJSONL guards the acceptance criterion "no code path in
// internal/uncovered reads line data from map.jsonl": map.jsonl carries no line data at
// all, so classification must consume only the fresh *coverage.Result. Importing
// mapstore, or naming the file in code, is the symptom to catch. Comments are stripped
// first — the prose explaining WHY the file is never read is not a read of it.
func TestUncoveredNeverReadsMapJSONL(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, nil, 0) // 0: comments discarded
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			if strings.Contains(imp.Path.Value, "mapstore") {
				t.Fatalf("%s imports %s; classification must use only the fresh *coverage.Result",
					name, imp.Path.Value)
			}
		}
		var b strings.Builder
		if err := printer.Fprint(&b, fset, f); err != nil {
			t.Fatalf("print %s: %v", name, err)
		}
		if strings.Contains(b.String(), "map.jsonl") {
			t.Fatalf("%s names map.jsonl in code; it carries no line data at all", name)
		}
	}
}
