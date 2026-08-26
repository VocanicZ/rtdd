package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/runner"
)

func TestRowsFrom(t *testing.T) {
	res := &runner.RunResult{
		Outcomes: []report.Outcome{
			{Test: "tests/test_a.py::test_add", Status: "pass", DurationMS: 412},
			{Test: "tests/test_b.py::test_fail", Status: "fail", DurationMS: 2},
			// A test that ran but recorded no coverage rows at all: it executed
			// nothing measurable outside already-recorded import-time lines.
			{Test: "tests/test_b.py::test_skipped", Status: "skip", DurationMS: 1},
		},
		Coverage: &coverage.Result{
			PerTest: []coverage.TestCoverage{
				{Test: "tests/test_a.py::test_add", Files: map[string][]int{
					"src/logic.py":     {5},
					"tests/test_a.py":  {6, 7},
					"tests/helpers.py": {2},
				}},
				{Test: "tests/test_b.py::test_fail", Files: map[string][]int{
					"src/logic.py": {9},
				}},
			},
			ImportTime: map[string][]int{"src/constants.py": {1, 3, 5, 6, 7}},
		},
	}

	got := rowsFrom(res, "a3f21e0")
	want := []mapRow{
		{T: "tests/test_a.py::test_add", F: []string{"src/logic.py", "tests/helpers.py", "tests/test_a.py"}, C: "a3f21e0", D: 412, S: "pass"},
		{T: "tests/test_b.py::test_fail", F: []string{"src/logic.py"}, C: "a3f21e0", D: 2, S: "fail"},
		{T: "tests/test_b.py::test_skipped", F: nil, C: "a3f21e0", D: 1, S: "skip"},
	}
	if len(got) != len(want) {
		t.Fatalf("len(rowsFrom) = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].T != want[i].T || got[i].C != want[i].C || got[i].D != want[i].D || got[i].S != want[i].S {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
		if len(got[i].F) == 0 && len(want[i].F) == 0 {
			continue
		}
		if !reflect.DeepEqual(got[i].F, want[i].F) {
			t.Errorf("row %d F = %v, want %v (sorted)", i, got[i].F, want[i].F)
		}
	}
	// Import-time files belong to no test and must not leak into any f.
	for _, r := range got {
		for _, f := range r.F {
			if f == "src/constants.py" {
				t.Errorf("row %q has import-time file src/constants.py in f", r.T)
			}
		}
	}
}

// mapRow mirrors mapstore.Row's field set so the assertions above read clearly.
type mapRow struct {
	T string
	F []string
	C string
	D int
	S string
}

// TestRowsFromKeepsTestOwnedFiles pins the decision recorded in Task 17: rowsFrom does
// NOT filter f through adapter.IsInstrumentable. Filtering would drop tests/helpers.py
// from every row, so editing a shared test helper would select nothing. Over-selection
// is safe for selection; under-selection is not.
func TestRowsFromKeepsTestOwnedFiles(t *testing.T) {
	res := &runner.RunResult{
		Outcomes: []report.Outcome{{Test: "tests/test_a.py::test_add", Status: "pass", DurationMS: 9}},
		Coverage: &coverage.Result{PerTest: []coverage.TestCoverage{
			{Test: "tests/test_a.py::test_add", Files: map[string][]int{
				"src/logic.py":     {5},
				"tests/helpers.py": {2},
				"tests/test_a.py":  {6},
				"conftest.py":      {1},
			}},
		}},
	}
	got := rowsFrom(res, "a3f21e0")
	if len(got) != 1 {
		t.Fatalf("len(rowsFrom) = %d, want 1", len(got))
	}
	for _, want := range []string{"conftest.py", "src/logic.py", "tests/helpers.py", "tests/test_a.py"} {
		found := false
		for _, f := range got[0].F {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("f = %v, missing %q; a filtered f makes editing a shared test helper select nothing", got[0].F, want)
		}
	}
}

// TestRowsFromEveryCoveredRowIsComplete asserts the join actually happened: a row whose
// test recorded coverage carries a non-empty f, the passed sha in c, the report's d, and
// a status the map's schema recognises. A row missing any of these is a row the selector
// cannot act on.
func TestRowsFromEveryCoveredRowIsComplete(t *testing.T) {
	res := &runner.RunResult{
		Outcomes: []report.Outcome{
			{Test: "tests/test_a.py::test_add", Status: "pass", DurationMS: 412},
			{Test: "tests/test_b.py::test_fail", Status: "fail", DurationMS: 2},
		},
		Coverage: &coverage.Result{PerTest: []coverage.TestCoverage{
			{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
			{Test: "tests/test_b.py::test_fail", Files: map[string][]int{"src/logic.py": {9}}},
		}},
	}
	valid := map[string]bool{"pass": true, "fail": true, "skip": true, "error": true}
	for _, r := range rowsFrom(res, "a3f21e0") {
		if len(r.F) == 0 {
			t.Errorf("row %q has an empty f but its test recorded coverage", r.T)
		}
		if r.C != "a3f21e0" {
			t.Errorf("row %q c = %q, want the passed sha %q", r.T, r.C, "a3f21e0")
		}
		if r.D == 0 {
			t.Errorf("row %q d = 0; the duration comes from the report, not from coverage", r.T)
		}
		if !valid[r.S] {
			t.Errorf("row %q s = %q, want one of pass|fail|skip|error", r.T, r.S)
		}
	}
}

// TestRowsFromWithoutCoverage: a run that produced no coverage at all still yields one
// row per outcome, so s and d refresh even when the coverage store was unreadable.
func TestRowsFromWithoutCoverage(t *testing.T) {
	res := &runner.RunResult{
		Outcomes: []report.Outcome{{Test: "tests/test_a.py::test_add", Status: "pass", DurationMS: 4}},
	}
	got := rowsFrom(res, "beef")
	if len(got) != 1 || got[0].T != "tests/test_a.py::test_add" || got[0].C != "beef" {
		t.Fatalf("rowsFrom with a nil Coverage = %+v", got)
	}
}

func TestMetaRoundTrip(t *testing.T) {
	repo := t.TempDir()

	got, err := readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta on a fresh repo: %v", err)
	}
	if (got != meta{}) {
		t.Fatalf("readMeta on a fresh repo = %+v, want the zero value", got)
	}

	want := meta{V: 1, Adapter: "python", SeededAt: "a3f21e0", Cycles: 7}
	if err := writeMeta(repo, want); err != nil {
		t.Fatalf("writeMeta: %v", err)
	}
	got, err = readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if got != want {
		t.Fatalf("readMeta = %+v, want %+v", got, want)
	}

	b, err := os.ReadFile(metaPath(repo))
	if err != nil {
		t.Fatalf("read meta file: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("meta.json is not valid JSON: %v", err)
	}
	for _, k := range []string{"v", "adapter", "seeded_at", "cycles"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("meta.json has no %q key; got %v", k, raw)
		}
	}
}

func TestMetaMalformedIsAnError(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(metaPath(repo)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(metaPath(repo), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readMeta(repo); err == nil {
		t.Fatal("readMeta on malformed meta.json = nil error, want error")
	}
}

// TestMapPathIsUnderRtdd pins the two paths every command resolves the same way.
func TestMapPathIsUnderRtdd(t *testing.T) {
	repo := filepath.FromSlash("/repo")
	if got, want := mapPath(repo), filepath.Join(repo, ".rtdd", "map.jsonl"); got != want {
		t.Errorf("mapPath = %q, want %q", got, want)
	}
	if got, want := metaPath(repo), filepath.Join(repo, ".rtdd", "meta.json"); got != want {
		t.Errorf("metaPath = %q, want %q", got, want)
	}
}

func TestFindRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}
	got, err := findRepoRoot(deep)
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}
	gotEval, _ := filepath.EvalSymlinks(got)
	rootEval, _ := filepath.EvalSymlinks(root)
	if gotEval != rootEval {
		t.Fatalf("findRepoRoot = %q, want %q", gotEval, rootEval)
	}
}

func TestFindRepoRootOutsideAnyRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := findRepoRoot(dir); err == nil {
		t.Fatal("findRepoRoot outside a git repo = nil error, want error")
	}
}

// TestFindRepoRootWorktreeFile: in a git worktree .git is a FILE, not a directory.
// A check that only accepts a directory reports "not inside a git repository" for
// every worktree, which is exactly where the fleet runs.
func TestFindRepoRootWorktreeFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}
	got, err := findRepoRoot(root)
	if err != nil {
		t.Fatalf("findRepoRoot in a worktree: %v", err)
	}
	gotEval, _ := filepath.EvalSymlinks(got)
	rootEval, _ := filepath.EvalSymlinks(root)
	if gotEval != rootEval {
		t.Fatalf("findRepoRoot = %q, want %q", gotEval, rootEval)
	}
}

func TestReportRunErrExitCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil is not an error", nil, 0},
		{"sysmon is a fatal environment error", runner.ErrSysmonContext, 3},
		{"wrapped sysmon", runner.ErrSysmonContext, 3},
		{"bad selector is a configuration error", &runner.FatalExitError{Chunk: 0, Code: 4, Label: "bad-selector"}, 2},
		{"no tests collected is a configuration error", &runner.FatalExitError{Chunk: 1, Code: 5, Label: "no-tests-collected"}, 2},
		{"wrapped fatal exit", &runner.FatalExitError{Chunk: 2, Code: 4, Label: "bad-selector"}, 2},
		{"anything else is a fatal environment error", errors.New("boom"), 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.err
			if strings.HasPrefix(tc.name, "wrapped ") {
				err = wrapErr(err)
			}
			if got := reportRunErr(err); got != tc.want {
				t.Fatalf("reportRunErr(%v) = %d, want %d", err, got, tc.want)
			}
		})
	}
}

func wrapErr(e error) error { return errWrap{e} }

type errWrap struct{ e error }

func (w errWrap) Error() string { return "wrapped: " + w.e.Error() }
func (w errWrap) Unwrap() error { return w.e }

// STRUCTURAL GUARD (spec §4, decision D11, audit A4): mapstore.Replace may be
// called from exactly one place in the tree — cmd/rtdd/seed.go. Every other path
// must union, because a subset run legitimately records LESS coverage than the
// seed and a failing test records a truncated prefix.
func TestOnlySeedCallsMapstoreReplace(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	var offenders []string
	err := filepath.Walk(repoRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "testdata", "bench":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".go" || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(p)
		if strings.HasSuffix(rel, "cmd/rtdd/seed.go") || strings.Contains(rel, "internal/mapstore/") {
			return nil
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(b), ".Replace(") {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("mapstore.Replace is called outside cmd/rtdd/seed.go: %v\n"+
			"Only rtdd seed may shrink a row (spec §4, D11). Everything else must Union.", offenders)
	}
}
