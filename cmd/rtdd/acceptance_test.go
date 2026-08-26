package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// buildPyFixture materialises the §F1 project and runs pytest under coverage with
// dynamic contexts. It returns the repo root and the parsed coverage result.
//
// Measured on 2026-08-26 (Python 3.13.5, coverage.py 7.15.4, pytest 9.0.3) the run
// produces exactly:
//
//	src/__init__.py   ctx=''                                  lines=[0]
//	src/constants.py  ctx=''                                  lines=[1,2,4,7,8,9,12,13,14,15]
//	src/logic.py      ctx=''                                  lines=[1,4,8]
//	src/logic.py      ctx='tests/test_it.py::test_logic|run'  lines=[5]
//
// It SKIPS, never fails, when the Python toolchain is absent — the same convention the
// M1b integration tests use, so a Go-only checkout still runs the whole suite.
func buildPyFixture(t *testing.T) (string, *coverage.Result) {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	if err := exec.Command(py, "-c", "import pytest, coverage, pytest_cov").Run(); err != nil {
		t.Skip("pytest / coverage / pytest-cov not importable")
	}

	root := t.TempDir()
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
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command(py, "-m", "pytest", "--cov=src", "--cov-context=test", "-q")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "COVERAGE_CORE=ctrace")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pytest failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "2 passed") {
		t.Fatalf("expected 2 passed, got:\n%s", out)
	}

	// Fatal, never skipped: a sysmon run silently drops ~90% of contexts (audit A7).
	if strings.Contains(string(out), "no-sysmon-context") {
		t.Fatalf("dynamic contexts were dropped; COVERAGE_CORE=ctrace was not honoured:\n%s", out)
	}

	cov, err := coverage.ReadSQLite(filepath.Join(root, ".coverage"), root)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}
	return root, cov
}

func TestAcceptanceImportTimeOnlyFileIsNeverUncovered(t *testing.T) {
	// GUARANTEE 1: a file whose changed lines are all import-time is correctly tested
	// and must produce a clean report. This is audit finding A1 and the reason RTDD
	// is usable on any repo with dataclasses, enums, config modules, Pydantic/Django
	// models, or __init__.py re-exports.
	_, cov := buildPyFixture(t)

	if lines, ok := cov.ImportTime["src/constants.py"]; !ok || len(lines) == 0 {
		t.Fatalf("fixture invariant broken: src/constants.py has no import-time lines: %#v", cov.ImportTime)
	}
	for _, tc := range cov.PerTest {
		if len(tc.Files["src/constants.py"]) != 0 {
			t.Fatalf("fixture invariant broken: %s attributes lines in src/constants.py; "+
				"finding A1 says import-time code is attributed to ZERO test contexts", tc.Test)
		}
	}

	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
	}
	reports := uncovered.Classify(changes, cov)
	if n := uncovered.Summarize(reports).UncoveredLines; n != 0 {
		t.Fatalf("uncovered_lines = %d, want 0 — a dataclass/enum/constants module asserted "+
			"on by two passing tests must produce a clean report\nreports: %#v", n, reports)
	}
	if s := RenderUncovered(reports); strings.Contains(s, "UNCOVERED") {
		t.Fatalf("text report must contain no UNCOVERED line:\n%s", s)
	}

	// The same guarantee in --json: `uncovered.files` is present and available (the run
	// really did produce fresh coverage), every range is import-time, and the summary's
	// uncovered_lines is 0. An agent reading the JSON must reach the same verdict a human
	// reading the text report does.
	out := BuildOutput(OutputInput{
		Command: "run", Base: "HEAD", Adapter: "python",
		Executed: true, Reports: reports, UncoveredOK: true,
		Instrumentable: map[string]bool{"src/constants.py": true},
		Changes:        changes,
	})
	if !out.Uncovered.Available {
		t.Fatal("json uncovered.available = false, want true after a real run")
	}
	if n := out.Uncovered.Summary.UncoveredLines; n != 0 {
		t.Fatalf("json uncovered_lines = %d, want 0", n)
	}
	for _, f := range out.Uncovered.Files {
		if f.UncoveredLines != 0 {
			t.Errorf("json files[%s].uncovered_lines = %d, want 0", f.Path, f.UncoveredLines)
		}
		for _, r := range f.Ranges {
			if r.Class == uncovered.Uncovered.String() {
				t.Errorf("json files[%s] carries an %q range %d-%d; import-time is never uncovered",
					f.Path, r.Class, r.Start, r.End)
			}
		}
	}
	blob, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(blob), `"class":"uncovered"`) {
		t.Fatalf("serialised --json must carry no uncovered range:\n%s", blob)
	}
}

func TestAcceptanceClassificationIsLineGranular(t *testing.T) {
	// GUARANTEE 2: adding a function to an already-covered file must report the new
	// function's lines as Uncovered. v1 was file-granular and reported green here.
	_, cov := buildPyFixture(t)

	// src/logic.py IS covered (line 5 belongs to tests/test_it.py::test_logic), yet
	// lines 8-9 are the never-called unused_helper. Line 8 is the `def` (import-time);
	// line 9 is its body, which nothing executes.
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
	want := "  UNCOVERED: src/logic.py:9  (1 changed line, no executing test)\n" +
		"  import-time: src/logic.py:8  (executed during collection, not attributed)\n"
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

	outcomes := []report.Outcome{
		{Test: "tests/test_it.py::test_constants", Status: "pass", DurationMS: 2},
		{Test: "tests/test_it.py::test_logic", Status: "pass", DurationMS: 1},
	}
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

func TestAcceptanceImportOnlyFileIsInNoMapRow(t *testing.T) {
	// GUARANTEE 4: an import-time-only file is in NO map row's f, which is exactly the
	// static-import fallback's trigger condition (spec §6, D14).
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

	if got := m.TestsCovering([]string{"src/constants.py"}); len(got) != 0 {
		t.Fatalf("TestsCovering(src/constants.py) = %#v, want empty — import-time lines "+
			"are attributed to no test and therefore enter no row's f", got)
	}

	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
	}
	sig := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              cov,
		Map:              m,
		IsInstrumentable: func(rel string) bool { return strings.HasPrefix(rel, "src/") },
	})
	if len(sig.UnmappedFiles) != 1 || sig.UnmappedFiles[0] != "src/constants.py" {
		t.Fatalf("UnmappedFiles = %#v, want [src/constants.py]", sig.UnmappedFiles)
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
	// tests/test_it.py::test_logic really does execute src/logic.py, so the map says the
	// file is covered. Line 9 is still uncovered, and only the coverage knows that.
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
	// two ranges skip lines 6-7, which are blank: coverage stores only executed lines, so
	// a blank line inside a file some test DID touch is indistinguishable from a dead one
	// and classifies Uncovered. That is settled behaviour, and not what this test is about.
	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified,
			Lines: []gitctx.LineRange{{Start: 5, End: 5}, {Start: 8, End: 9}}},
	}
	instrumentable := func(rel string) bool { return strings.HasPrefix(rel, "src/") }

	fresh := BuildSignal(SignalInput{Changes: changes, Cov: cov, Map: m, IsInstrumentable: instrumentable})
	if n := uncovered.Summarize(fresh.Reports).CoveredLines; n != 1 {
		t.Fatalf("with fresh coverage covered_lines = %d, want 1 (line 5)\nreports: %#v",
			n, fresh.Reports)
	}
	if n := uncovered.Summarize(fresh.Reports).UncoveredLines; n != 1 {
		t.Fatalf("with fresh coverage uncovered_lines = %d, want 1 (line 9)\nreports: %#v",
			n, fresh.Reports)
	}
	if n := uncovered.Summarize(fresh.Reports).ImportTimeLines; n != 1 {
		t.Fatalf("with fresh coverage import_time_lines = %d, want 1 (line 8)\nreports: %#v",
			n, fresh.Reports)
	}

	// Same map, same changes, no fresh coverage: every changed line is Uncovered. If any
	// line survived as Covered here it could only have come from the map.
	stale := BuildSignal(SignalInput{Changes: changes, Cov: nil, Map: m, IsInstrumentable: instrumentable})
	s := uncovered.Summarize(stale.Reports)
	if s.CoveredLines != 0 || s.ImportTimeLines != 0 {
		t.Fatalf("without fresh coverage the map must contribute nothing; got %#v\nreports: %#v",
			s, stale.Reports)
	}
	if s.UncoveredLines != 3 {
		t.Fatalf("without fresh coverage uncovered_lines = %d, want 3 (lines 5, 8 and 9)\nreports: %#v",
			s.UncoveredLines, stale.Reports)
	}
}
