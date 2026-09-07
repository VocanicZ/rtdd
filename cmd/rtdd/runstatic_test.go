package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// staticRunRepo is a `selection: static` / `coverage: none` repository `rtdd run` can be
// driven over end to end, on a machine with no JavaScript toolchain installed.
//
// The adapter's `subset` is `cp`, and what it copies into {report} is a committed JUnit
// XML. That is not a shortcut around the runner: the engine still builds argv, clears
// report_path, executes a child process, skips the .coverage read a `coverage: none`
// adapter has no store for, and parses the report through the shipped junit reader. What
// it removes is the ONE part this test is not about — a real runner's installation —
// which is also the part CI does not have: the `test` job installs Go and nothing else,
// and a test that skipped there would leave PRD #233 AC9 unguarded on every push.
//
// `test_selector` is what makes `cp` expressible: the TS tier names the test FILE
// src/calc.test.js, and the selector renders the report that stands in for running it.
func staticRunRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skipf("cp is not on PATH: %v", err)
	}
	dir := gittest.Init(t)
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Write(t, dir, "stub.toml", "# detect marker\n")
	gittest.Write(t, dir, "src/calc.js",
		"function add(a, b) {\n  return a + b;\n}\nmodule.exports = { add };\n")
	gittest.Write(t, dir, "src/calc.test.js", "test('adds', () => {});\n")
	gittest.Write(t, dir, "src/calc.test.junit.xml",
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+
			"<testsuites>\n"+
			"  <testsuite name=\"calc\" tests=\"1\" failures=\"0\" errors=\"0\">\n"+
			"    <testcase classname=\"calc\" name=\"adds\" file=\"src/calc.test.js\" time=\"0.01\"/>\n"+
			"  </testsuite>\n"+
			"</testsuites>\n")
	gittest.Commit(t, dir, "init")

	adir := filepath.Join(dir, ".rtdd", "adapters")
	if err := os.MkdirAll(adir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := "name: stub\ndetect: [\"stub.toml\"]\n" +
		"selection: static\ncoverage: none\n" +
		"subset: \"cp {tests} {report}\"\n" +
		"test_selector: \"{dir}/{name}.junit.xml\"\n" +
		"report: junit-xml\nreport_path: \".rtdd/junit.xml\"\n" +
		"id_template: \"{file}::{name}\"\n" +
		"test_for: [\"{dir}/{name}.test.js\"]\n" +
		"test_globs: [\"**/*.test.js\"]\nsource_globs: [\"src/**/*.js\"]\n"
	if err := os.WriteFile(filepath.Join(adir, "stub.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write adapter: %v", err)
	}
	return dir
}

// touchStaticSource adds a line to the one source file, so the changed set is exactly
// src/calc.js and the TS tier has a correspondence to resolve.
func touchStaticSource(t *testing.T, dir string) {
	t.Helper()
	gittest.Write(t, dir, "src/calc.js",
		"function add(a, b) {\n  return a + b;\n}\nfunction sub(a, b) {\n  return a - b;\n}\nmodule.exports = { add, sub };\n")
}

// PRD #233 AC9a and AC9b, driven end to end.
//
// Nothing was instrumented, so no changed line can be KNOWN to be uncovered, and the map
// the run refreshed holds no row for an adapter that records nothing. Both claims used to
// be printed anyway: every changed line reported UNCOVERED unconditionally, and the
// summary counted outcome rows as map rows.
func TestCmdRunOnACoverageNoneAdapterClaimsNeitherUncoveredLinesNorMapRows(t *testing.T) {
	dir := staticRunRepo(t)
	touchStaticSource(t, dir)
	chdir(t, dir)

	code := -1
	// captureStdout, not the rtdd() helper: cmdRun writes its report to os.Stdout, and
	// what this test is about is what an agent reading that stream is told.
	stdout := captureStdout(t, func() { code = cmdRun(nil) })
	if code != 0 {
		t.Fatalf("rtdd run = %d, want 0 over a green static fixture\nstdout:\n%s", code, stdout)
	}
	// The exact assertion PRD #233 AC9 requires.
	if strings.Contains(stdout, "UNCOVERED") {
		t.Errorf("a coverage: none run claims uncovered lines:\n%s", stdout)
	}
	if strings.Contains(stdout, "rows in the map") {
		t.Errorf("a coverage: none run claims map rows:\n%s", stdout)
	}
	// The run must still say what it DID do: the suppression is stated, never silent.
	var notes []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "coverage: none") {
			notes = append(notes, line)
		}
	}
	if len(notes) != 1 {
		t.Fatalf("want exactly one line saying why the uncovered report is absent, got %d:\n%s", len(notes), stdout)
	}
	if !strings.Contains(notes[0], "uncovered") {
		t.Errorf("the suppression line does not name the report it replaces: %q", notes[0])
	}
	if !strings.Contains(stdout, "1 ran") {
		t.Errorf("the run summary is gone with the map-row count:\n%s", stdout)
	}
}

// AC5: the machine-readable document must not fabricate what the text no longer prints.
// `available: false` with a reason is the shape schema v1 already has for a report no
// fresh coverage backs; an empty `files` list would read as "nothing uncovered".
func TestCmdRunJSONOnACoverageNoneAdapterFabricatesNoUncoveredRanges(t *testing.T) {
	dir := staticRunRepo(t)
	touchStaticSource(t, dir)
	chdir(t, dir)

	code := -1
	// captureStdout, not the rtdd() helper: emitJSON writes the document to os.Stdout
	// itself, which is exactly the guarantee "--json is the WHOLE of stdout" is about.
	stdout := captureStdout(t, func() { code = cmdRun([]string{"--json"}) })
	if code != 0 {
		t.Fatalf("rtdd run --json = %d, want 0\nstdout:\n%s", code, stdout)
	}
	var got Output
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout)
	}
	if got.Uncovered.Available {
		t.Fatalf("uncovered.available = true for an adapter that instrumented nothing:\n%s", stdout)
	}
	if len(got.Uncovered.Files) != 0 {
		t.Errorf("uncovered.files has %d entries for a coverage: none adapter:\n%s", len(got.Uncovered.Files), stdout)
	}
	if got.Uncovered.Summary.UncoveredLines != 0 {
		t.Errorf("uncovered.summary.uncovered_lines = %d, want 0", got.Uncovered.Summary.UncoveredLines)
	}
	if !strings.Contains(got.Uncovered.Reason, "coverage: none") {
		t.Errorf("uncovered.reason does not say why the report is absent: %q", got.Uncovered.Reason)
	}
	if !got.Run.Executed || got.Run.Passed != 1 {
		t.Errorf("run = %+v, want one executed passing test", got.Run)
	}
}

// AC3's other half: the map itself. An adapter that records nothing must leave no row
// behind, so the count the summary line used to print was of outcome rows that should
// never have existed.
func TestACoverageNoneRunWritesNoMapRow(t *testing.T) {
	dir := staticRunRepo(t)
	touchStaticSource(t, dir)
	chdir(t, dir)

	code := -1
	stdout := captureStdout(t, func() { code = cmdRun(nil) })
	if code != 0 {
		t.Fatalf("rtdd run = %d, want 0\nstdout:\n%s", code, stdout)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".rtdd", "map.jsonl"))
	if err != nil {
		t.Fatalf("read map.jsonl: %v", err)
	}
	if strings.TrimSpace(string(raw)) != "" {
		t.Errorf("a coverage: none run wrote map rows:\n%s", raw)
	}
}

// AC9b as a unit. The clause is dropped, not zeroed: "0 rows in the map" is a true
// sentence about a map that should not be mentioned at all, and it still puts the
// coverage vocabulary on the output of a run that recorded none.
func TestRenderRunSummaryDropsTheMapRowClauseWhenTheMapIsEmpty(t *testing.T) {
	if got, want := renderRunSummary(1, 0, 0, false), "1 ran, 0 failed\n"; got != want {
		t.Errorf("renderRunSummary(1, 0, 0, false) = %q, want %q", got, want)
	}
	// An execution-derived run's summary is byte-identical to what it has always been —
	// an empty map included, where the zero says the repository is unseeded.
	if got, want := renderRunSummary(3, 1, 12, true), "3 ran, 1 failed, 12 rows in the map\n"; got != want {
		t.Errorf("renderRunSummary(3, 1, 12, true) = %q, want %q", got, want)
	}
	if got, want := renderRunSummary(3, 1, 0, true), "3 ran, 1 failed, 0 rows in the map\n"; got != want {
		t.Errorf("renderRunSummary(3, 1, 0, true) = %q, want %q", got, want)
	}
}
