package coverage

import (
	"reflect"
	"testing"
)

func TestResultMergeUnionsChunks(t *testing.T) {
	// Chunk 1 ran tests/test_a.py; chunk 2 ran tests/test_b.py. pytest erased
	// .coverage between them, so each Result holds only its own chunk.
	c1 := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
		},
		ImportTime: map[string][]int{
			"src/logic.py":     {1, 4, 8, 12},
			"src/constants.py": {1, 3, 5, 6, 7},
		},
	}
	c2 := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_b.py::test_mul", Files: map[string][]int{"src/logic.py": {9}}},
		},
		ImportTime: map[string][]int{
			"src/logic.py": {1, 8},
			"src/other.py": {2},
		},
	}

	c1.Merge(c2)

	wantPerTest := []TestCoverage{
		{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
		{Test: "tests/test_b.py::test_mul", Files: map[string][]int{"src/logic.py": {9}}},
	}
	if !reflect.DeepEqual(c1.PerTest, wantPerTest) {
		t.Fatalf("PerTest =\n  %+v\nwant\n  %+v", c1.PerTest, wantPerTest)
	}
	wantImport := map[string][]int{
		"src/logic.py":     {1, 4, 8, 12},
		"src/constants.py": {1, 3, 5, 6, 7},
		"src/other.py":     {2},
	}
	if !reflect.DeepEqual(c1.ImportTime, wantImport) {
		t.Fatalf("ImportTime =\n  %+v\nwant\n  %+v", c1.ImportTime, wantImport)
	}
}

func TestResultMergeSameTestAcrossChunks(t *testing.T) {
	// The same test id can appear in two Results when its |setup and |run phases
	// land in different reads, or when a repeated id shows up twice.
	a := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_c.py::test_fx", Files: map[string][]int{"src/fixt.py": {2}}},
		},
		ImportTime: map[string][]int{},
	}
	b := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_c.py::test_fx", Files: map[string][]int{
				"src/fixt.py":  {6, 10, 2},
				"src/other.py": {3},
			}},
		},
		ImportTime: map[string][]int{},
	}
	a.Merge(b)
	if len(a.PerTest) != 1 {
		t.Fatalf("len(PerTest) = %d, want 1", len(a.PerTest))
	}
	want := map[string][]int{"src/fixt.py": {2, 6, 10}, "src/other.py": {3}}
	if !reflect.DeepEqual(a.PerTest[0].Files, want) {
		t.Fatalf("Files = %+v, want %+v (sorted and deduped)", a.PerTest[0].Files, want)
	}
}

func TestResultMergeDoesNotAliasOther(t *testing.T) {
	a := &Result{ImportTime: map[string][]int{}}
	b := &Result{
		PerTest:    []TestCoverage{{Test: "t", Files: map[string][]int{"f.py": {1}}}},
		ImportTime: map[string][]int{"g.py": {2}},
	}
	a.Merge(b)
	b.PerTest[0].Files["f.py"] = append(b.PerTest[0].Files["f.py"], 99)
	b.ImportTime["g.py"] = append(b.ImportTime["g.py"], 99)
	if got := a.PerTest[0].Files["f.py"]; !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("Merge aliased other's Files: got %v, want [1]", got)
	}
	if got := a.ImportTime["g.py"]; !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("Merge aliased other's ImportTime: got %v, want [2]", got)
	}
}

func TestResultMergeNilIsANoop(t *testing.T) {
	a := &Result{
		PerTest:    []TestCoverage{{Test: "t", Files: map[string][]int{"f.py": {1}}}},
		ImportTime: map[string][]int{},
	}
	a.Merge(nil)
	if len(a.PerTest) != 1 {
		t.Fatalf("len(PerTest) after Merge(nil) = %d, want 1", len(a.PerTest))
	}
}

func TestResultMergeIntoZeroValue(t *testing.T) {
	var a Result
	a.Merge(&Result{
		PerTest:    []TestCoverage{{Test: "t", Files: map[string][]int{"f.py": {1}}}},
		ImportTime: map[string][]int{"g.py": {2}},
	})
	if a.ImportTime == nil {
		t.Fatal("Merge into a zero Result left ImportTime nil")
	}
	if len(a.PerTest) != 1 || a.PerTest[0].Test != "t" {
		t.Fatalf("PerTest = %+v, want one entry for t", a.PerTest)
	}
}

// Chunk 1's contexts must survive the merge of chunk 2 — the exact property that
// running without --cov-append would otherwise destroy, since pytest erases
// .coverage at the start of every run.
func TestResultMergeKeepsChunk1ContextsAfterChunk2(t *testing.T) {
	chunk1 := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_a.py::test_one", Files: map[string][]int{"src/a.py": {1, 2}}},
			{Test: "tests/test_a.py::test_two", Files: map[string][]int{"src/a.py": {3}}},
		},
		ImportTime: map[string][]int{"src/a.py": {1}},
	}
	// What the second pytest run leaves behind: only chunk 2's contexts.
	chunk2 := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_b.py::test_three", Files: map[string][]int{"src/b.py": {7}}},
		},
		ImportTime: map[string][]int{"src/b.py": {1}},
	}

	chunk1.Merge(chunk2)

	got := make([]string, 0, len(chunk1.PerTest))
	for _, tc := range chunk1.PerTest {
		got = append(got, tc.Test)
	}
	want := []string{
		"tests/test_a.py::test_one",
		"tests/test_a.py::test_two",
		"tests/test_b.py::test_three",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tests after merging chunk 2 = %v, want %v", got, want)
	}
	if lines := chunk1.PerTest[0].Files["src/a.py"]; !reflect.DeepEqual(lines, []int{1, 2}) {
		t.Fatalf("chunk 1 lines lost: %v, want [1 2]", lines)
	}
	if lines := chunk1.ImportTime["src/a.py"]; !reflect.DeepEqual(lines, []int{1}) {
		t.Fatalf("chunk 1 ImportTime lost: %v, want [1]", lines)
	}
}

// mergeLines is the shared sorted-union helper ReadSQLite reuses when the same
// (file, context) pair yields more than one numbits row.
func TestMergeLinesSortsDedupesAndNeverAliases(t *testing.T) {
	b := []int{9, 3, 3}
	got := mergeLines([]int{5, 3}, b)
	if want := []int{3, 5, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeLines = %v, want %v", got, want)
	}
	b[0] = 99
	if !reflect.DeepEqual(got, []int{3, 5, 9}) {
		t.Fatalf("mergeLines aliased b: %v", got)
	}
	if got := mergeLines(nil, nil); !reflect.DeepEqual(got, []int{}) {
		t.Fatalf("mergeLines(nil, nil) = %v, want empty non-nil slice", got)
	}
}
