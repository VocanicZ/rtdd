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

// f1Coverage is two isolated units: each unit's own run recorded every line it
// executed, import-time lines included (spec §8).
func f1Coverage() *coverage.Result {
	return &coverage.Result{
		PerTest: []coverage.TestCoverage{
			{Test: "tests/test_logic.py", Files: map[string][]int{"src/logic.py": {1, 4, 5, 8}}},
			{Test: "tests/test_constants.py", Files: map[string][]int{"src/constants.py": {1, 2, 4, 7, 8, 9, 12, 13, 14, 15}}},
		},
	}
}

func TestClassifyBothClassesInOneFile(t *testing.T) {
	// src/logic.py: 1, 4, 5, 8 covered; 2-3, 6-7, 9 uncovered.
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
				{Range: gitctx.LineRange{Start: 1, End: 1}, Class: Covered},
				{Range: gitctx.LineRange{Start: 2, End: 3}, Class: Uncovered},
				{Range: gitctx.LineRange{Start: 4, End: 5}, Class: Covered},
				{Range: gitctx.LineRange{Start: 6, End: 7}, Class: Uncovered},
				{Range: gitctx.LineRange{Start: 8, End: 8}, Class: Covered},
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
				{Range: gitctx.LineRange{Start: 8, End: 8}, Class: Covered},
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
				{Range: gitctx.LineRange{Start: 4, End: 5}, Class: Covered},
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
				{Range: gitctx.LineRange{Start: 1, End: 1}, Class: Covered},
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

func TestClassifyFileAbsentFromCoverageIsUncovered(t *testing.T) {
	// A file coverage never saw at all is a brand-new source file: wholly Uncovered.
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
			{Range: gitctx.LineRange{Start: 5, End: 9}, Class: Covered},
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
				{Range: gitctx.LineRange{Start: 1, End: 4}, Class: Covered},
				{Range: gitctx.LineRange{Start: 5, End: 5}, Class: Uncovered},
			},
		},
	}
	got := Summarize(reports)
	want := Summary{Files: 2, CoveredLines: 6, UncoveredLines: 4}
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
