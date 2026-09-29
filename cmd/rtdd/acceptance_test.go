package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// buildPyFixture materialises the §F1 project in a git repo and runs its one unit
// (tests/test_it.py) through the real one-pipeline runner with the shipped python adapter.
// It returns the repo root and the parsed coverage result.
//
// It SKIPS, never fails, when the Python toolchain is absent, so a Go-only checkout still
// runs the whole suite.
func buildPyFixture(t *testing.T) (string, *coverage.Result) {
	t.Helper()
	if _, err := exec.LookPath("pytest"); err != nil {
		t.Skip("pytest not on PATH")
	}
	if err := exec.Command("python3", "-c", "import pytest_cov").Run(); err != nil {
		t.Skip("pytest-cov not importable")
	}

	root := gittest.Init(t)
	files := map[string]string{
		"src/__init__.py":   "",
		"tests/__init__.py": "",
		"pyproject.toml":    "[tool.pytest.ini_options]\npythonpath = [\".\"]\n",
		"src/constants.py": "from dataclasses import dataclass\nfrom enum import Enum\n\n" +
			"MAX_RETRIES = 3\n\n\n" +
			"class Colour(Enum):\n    RED = \"red\"\n    GREEN = \"green\"\n\n\n" +
			"@dataclass\nclass Limits:\n    soft: int = 10\n    hard: int = 20\n",
		"src/logic.py": "from src.constants import MAX_RETRIES\n\n\n" +
			"def retries_left(used):\n    return MAX_RETRIES - used\n\n\n" +
			"def unused_helper(x):\n    return x * 2\n",
		"tests/test_it.py": "from src.constants import MAX_RETRIES, Colour, Limits\n" +
			"from src.logic import retries_left\n\n\n" +
			"def test_constants():\n    assert MAX_RETRIES == 3\n" +
			"    assert Colour.RED.value == \"red\"\n    assert Limits().soft == 10\n\n\n" +
			"def test_logic():\n    assert retries_left(1) == 2\n",
	}
	for rel, content := range files {
		gittest.Write(t, root, rel, content)
	}
	gittest.Commit(t, root, "fixture")

	all, err := adapter.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	var py *adapter.Adapter
	for _, a := range all {
		if a.Name == "python" {
			py = a
		}
	}
	res, err := runner.Seed(py, root)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Status != "pass" {
		t.Fatalf("outcomes = %#v, want one passing unit\n%v", res.Outcomes, res.Output)
	}
	return root, res.Coverage
}

func TestAcceptanceImportExecutedLinesAreCovered(t *testing.T) {
	// GUARANTEE 1 (one-pipeline spec §8): a constants/enum/dataclass module's lines run on
	// import, inside the unit's own process, so they are Covered — never Uncovered.
	_, cov := buildPyFixture(t)

	// The module's executable lines; 3, 5, 6, 10 and 11 are blank.
	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{
			{Start: 1, End: 2}, {Start: 4, End: 4}, {Start: 7, End: 9}, {Start: 12, End: 15}}},
	}
	reports := uncovered.Classify(changes, cov)
	if n := uncovered.Summarize(reports).UncoveredLines; n != 0 {
		t.Fatalf("uncovered_lines = %d, want 0 — a module asserted on by a passing unit "+
			"must produce a clean report\nreports: %#v", n, reports)
	}
	if s := RenderUncovered(reports); s != "" {
		t.Fatalf("text report must be empty:\n%s", s)
	}

	out := BuildOutput(OutputInput{
		Command: "run", Base: "HEAD", Adapter: "python",
		Executed: true, Reports: reports, UncoveredOK: true,
		Instrumentable: map[string]bool{"src/constants.py": true},
		Changes:        changes,
	})
	blob, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !out.Uncovered.Available || strings.Contains(string(blob), `"class":"uncovered"`) {
		t.Fatalf("serialised --json must be available and carry no uncovered range:\n%s", blob)
	}
}

func TestAcceptanceClassificationIsLineGranular(t *testing.T) {
	// GUARANTEE 2: adding a function to an already-covered file must report the new
	// function's lines as Uncovered. v1 was file-granular and reported green here.
	_, cov := buildPyFixture(t)

	// src/logic.py IS covered (line 5 runs in tests/test_it.py), yet lines 8-9 are the
	// never-called unused_helper. Line 8 is the `def`, executed on import; line 9 is its
	// body, which nothing executes.
	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
	}
	reports := uncovered.Classify(changes, cov)
	if len(reports) != 1 {
		t.Fatalf("reports = %#v, want 1 file", reports)
	}
	if n := reports[0].UncoveredLines(); n != 1 {
		t.Fatalf("UncoveredLines() = %d, want 1 (line 9 only)\nranges: %#v", n, reports[0].Ranges)
	}
	want := "  UNCOVERED: src/logic.py:9  (1 changed line, no executing test)\n"
	if got := RenderUncovered(reports); got != want {
		t.Fatalf("RenderUncovered()\n got:\n%s\nwant:\n%s", got, want)
	}

	// And the covered function body in the SAME file is attributed to its test.
	covChanges := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 5, End: 5}}},
	}
	covReports := uncovered.Classify(covChanges, cov)
	if len(covReports) != 1 || len(covReports[0].Ranges) != 1 ||
		covReports[0].Ranges[0].Class != uncovered.Covered {
		t.Fatalf("line 5 must be Covered; got %#v", covReports)
	}
}

func TestAcceptanceUncoveredReportNeverChangesTheExitCode(t *testing.T) {
	// GUARANTEE 3: `rtdd run` exits 1 only when a test fails. Uncovered is a signal.
	_, cov := buildPyFixture(t)

	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
	}
	reports := uncovered.Classify(changes, cov)
	if uncovered.Summarize(reports).UncoveredLines == 0 {
		t.Fatal("fixture invariant broken: the report must be non-empty for this test to mean anything")
	}

	outcomes := []runner.Outcome{{Test: "tests/test_it.py", Status: "pass", DurationMS: 3}}
	if code := ExitCodeFor(outcomes, reports); code != 0 {
		t.Fatalf("ExitCodeFor() = %d, want 0 with a non-empty uncovered report and no failing test", code)
	}

	out := BuildOutput(OutputInput{
		Command: "run", Base: "HEAD", Adapter: "python",
		Executed: true, Outcomes: outcomes,
		Reports: reports, UncoveredOK: true,
		Instrumentable: map[string]bool{"src/logic.py": true},
		Changes:        changes,
	})
	if out.ExitCode != 0 {
		t.Fatalf("json exit_code = %d, want 0", out.ExitCode)
	}
	if out.Uncovered.Summary.UncoveredLines != 1 {
		t.Fatalf("json uncovered_lines = %d, want 1", out.Uncovered.Summary.UncoveredLines)
	}
}

func TestAcceptanceImportedFileIsInItsUnitsRow(t *testing.T) {
	// GUARANTEE 4: a file a unit only imports is still in that unit's row, so a change to
	// it selects the unit and is not reported unmapped.
	_, cov := buildPyFixture(t)

	keep := func(a, b string) string { return a }
	m := mapstore.New()
	for _, tc := range cov.PerTest {
		var fs []string
		for path := range tc.Files {
			fs = append(fs, path)
		}
		m.Union(mapstore.Row{T: tc.Test, F: fs, C: "aaaaaaa", D: 1, S: "pass"}, keep)
	}
	if got := m.TestsCovering([]string{"src/constants.py"}); len(got) != 1 || got[0] != "tests/test_it.py" {
		t.Fatalf("TestsCovering(src/constants.py) = %#v, want [tests/test_it.py]", got)
	}
	sig := BuildSignal(SignalInput{
		Changes:          []gitctx.Change{{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 1}}}},
		Cov:              cov,
		Map:              m,
		IsInstrumentable: func(rel string) bool { return strings.HasPrefix(rel, "src/") },
	})
	if len(sig.UnmappedFiles) != 0 {
		t.Fatalf("UnmappedFiles = %#v, want none", sig.UnmappedFiles)
	}
}

// map.jsonl is file-level by design (spec §4): it records which files a test executed,
// never which lines. Classification therefore cannot come from it, and this test proves
// that twice over on the real fixture — once structurally, by reading the serialised map
// back and finding no line data in it, and once behaviourally, by feeding the SAME fully
// populated map to BuildSignal with and without the fresh coverage and watching the
// verdict change only with the coverage.
func TestAcceptanceClassificationConsumesOnlyFreshCoverageNeverTheMap(t *testing.T) {
	root, cov := buildPyFixture(t)

	keep := func(a, b string) string { return a }
	m := mapstore.New()
	for _, tc := range cov.PerTest {
		var fs []string
		for path := range tc.Files {
			fs = append(fs, path)
		}
		m.Union(mapstore.Row{T: tc.Test, F: fs, C: "aaaaaaa", D: 1, S: "pass"}, keep)
	}
	// tests/test_it.py really does execute src/logic.py, so the map says the file is
	// covered. Line 9 is still uncovered, and only the coverage knows that.
	if got := m.TestsCovering([]string{"src/logic.py"}); len(got) == 0 {
		t.Fatalf("fixture invariant broken: no map row covers src/logic.py")
	}

	mapPath := filepath.Join(root, ".rtdd", "map.jsonl")
	if err := os.MkdirAll(filepath.Dir(mapPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := m.Save(mapPath); err != nil {
		t.Fatalf("Save: %v", err)
	}
	f, err := os.Open(mapPath)
	if err != nil {
		t.Fatalf("open map.jsonl: %v", err)
	}
	defer f.Close()
	allowed := map[string]bool{"t": true, "f": true, "c": true, "d": true, "s": true}
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		var row map[string]json.RawMessage
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			t.Fatalf("map.jsonl:%d is not JSON: %v", line, err)
		}
		for k := range row {
			if !allowed[k] {
				t.Errorf("map.jsonl:%d carries key %q; the map is file-level and must hold "+
					"no line data whatsoever", line, k)
			}
		}
		var fs []string
		if err := json.Unmarshal(row["f"], &fs); err != nil {
			t.Fatalf("map.jsonl:%d field f is not a list of paths: %v", line, err)
		}
		for _, p := range fs {
			if strings.ContainsAny(p, ":") {
				t.Errorf("map.jsonl:%d f entry %q looks line-qualified; f holds bare paths", line, p)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan map.jsonl: %v", err)
	}

	// Line 5 is a covered body, 8 is a `def` executed at import, 9 is a dead body. The
	// two ranges skip lines 6-7, which are blank and would classify Uncovered.
	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified,
			Lines: []gitctx.LineRange{{Start: 5, End: 5}, {Start: 8, End: 9}}},
	}
	instrumentable := func(rel string) bool { return strings.HasPrefix(rel, "src/") }

	fresh := BuildSignal(SignalInput{Changes: changes, Cov: cov, Map: m, IsInstrumentable: instrumentable})
	if n := uncovered.Summarize(fresh.Reports).CoveredLines; n != 2 {
		t.Fatalf("with fresh coverage covered_lines = %d, want 2 (lines 5 and 8)\nreports: %#v",
			n, fresh.Reports)
	}
	if n := uncovered.Summarize(fresh.Reports).UncoveredLines; n != 1 {
		t.Fatalf("with fresh coverage uncovered_lines = %d, want 1 (line 9)\nreports: %#v",
			n, fresh.Reports)
	}

	// Same map, same changes, no fresh coverage: every changed line is Uncovered. If any
	// line survived as Covered here it could only have come from the map.
	stale := BuildSignal(SignalInput{Changes: changes, Cov: nil, Map: m, IsInstrumentable: instrumentable})
	s := uncovered.Summarize(stale.Reports)
	if s.CoveredLines != 0 {
		t.Fatalf("without fresh coverage the map must contribute nothing; got %#v\nreports: %#v",
			s, stale.Reports)
	}
	if s.UncoveredLines != 3 {
		t.Fatalf("without fresh coverage uncovered_lines = %d, want 3 (lines 5, 8 and 9)\nreports: %#v",
			s.UncoveredLines, stale.Reports)
	}
}
