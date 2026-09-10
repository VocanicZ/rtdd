package runner

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/report"
)

// stubJUnitScript stands in for a JUnit-emitting runner. It writes the XML it is told to
// write to the path it is given, records its argv, and exits with the code it is told to,
// so the runner's control flow — clearing, chunking, exit-code mapping, per-chunk merging
// — is tested without node, maven or ruby on PATH.
const stubJUnitScript = `#!/bin/sh
out=""
for a in "$@"; do
  case "$a" in
    --outputFile=*) out="${a#--outputFile=}" ;;
  esac
done
if [ -n "$RTDD_STUB_ARGS" ]; then
  printf -- '--- invocation ---\n' >> "$RTDD_STUB_ARGS"
  for a in "$@"; do printf '%s\n' "$a" >> "$RTDD_STUB_ARGS"; done
fi
if [ -n "$out" ] && [ -n "$RTDD_STUB_XML" ] && [ -f "$RTDD_STUB_XML" ]; then
  cat "$RTDD_STUB_XML" > "$out"
fi
exit ${RTDD_STUB_EXIT:-0}
`

// junitStubAdapter builds a static adapter whose subset command is the stub script.
func junitStubAdapter(t *testing.T, repo string, env map[string]string) *adapter.Adapter {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub runner script is POSIX sh")
	}
	stub := filepath.Join(repo, "stubjunit")
	if strings.ContainsAny(stub, " \t") {
		t.Skipf("temp dir %q contains whitespace; command templates are whitespace-split", stub)
	}
	if err := os.WriteFile(stub, []byte(stubJUnitScript), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	if env == nil {
		env = map[string]string{}
	}
	return &adapter.Adapter{
		Name:       "stubjunit",
		Detect:     []string{"package.json"},
		Env:        env,
		Selection:  adapter.SelectionStatic,
		Coverage:   adapter.CoverageNone,
		Subset:     stub + " {tests} --outputFile={report}",
		Report:     "junit-xml",
		ReportPath: ".rtdd/junit.xml",
		IDTemplate: "{classname}#{name}",
		ExitCodes:  map[int]string{4: "bad-selector", 5: "no-tests-collected"},
	}
}

func writeStubXML(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// runWithBudget is Run with an explicit argv budget, so the chunking path is exercised
// without a 100,000-byte argv.
func runWithBudget(a *adapter.Adapter, repoRoot string, tests []string, budget int) (*RunResult, error) {
	return execute(a, repoRoot, a.Subset, Chunk(tests, budget), false, true)
}

// The junit path end to end: {report} expands, the stub writes there, the parser reads it,
// and the ids come back rendered through id_template — in the same namespace as the ids
// that were spliced into the command.
func TestRunReadsOutcomesFromTheAdaptersReportPath(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	xml := writeStubXML(t, repo, "src.xml", `<testsuite name="s" time="0.04">
  <testcase classname="s.A" name="one" time="0.01"/>
  <testcase classname="s.B" name="two" time="0.03"><failure message="nope"/></testcase>
</testsuite>`)
	a.Env["RTDD_STUB_XML"] = xml

	res, err := Run(a, repo, []string{"s.A#one", "s.B#two"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Outcomes) != 2 {
		t.Fatalf("Outcomes = %+v, want two", res.Outcomes)
	}
	if res.Outcomes[0].Test != "s.A#one" || res.Outcomes[0].Status != "pass" || res.Outcomes[0].DurationMS != 10 {
		t.Errorf("Outcomes[0] = %+v, want s.A#one pass 10ms", res.Outcomes[0])
	}
	if len(res.Failed) != 1 || res.Failed[0] != "s.B#two" {
		t.Errorf("Failed = %v, want [s.B#two]", res.Failed)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1 for a run with a failing test", res.ExitCode)
	}
	// coverage: none means nothing reads .coverage, and the empty Result is not nil.
	if res.Coverage == nil || len(res.Coverage.ImportTime) != 0 {
		t.Errorf("Coverage = %+v, want the empty result under coverage: none", res.Coverage)
	}
}

// PRD #231 AC7, in the runner: a report left by a previous run must be cleared before the
// invocation, so a runner that writes nothing fails loudly instead of replaying history.
func TestRunClearsAStaleReportBeforeInvoking(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	if err := os.MkdirAll(filepath.Join(repo, ".rtdd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := filepath.Join(repo, ".rtdd", "junit.xml")
	if err := os.WriteFile(stale, []byte(`<testsuite name="old"><testcase classname="old.X" name="ghost"/></testsuite>`), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	// No RTDD_STUB_XML: the stub runs and writes nothing at all.
	_, err := Run(a, repo, []string{"s.A#one"}, false)
	if err == nil {
		t.Fatalf("Run = nil error; a runner that wrote no report must not return the previous run's outcomes")
	}
	if !errors.Is(err, report.ErrNoReport) {
		t.Fatalf("error = %v, want errors.Is(_, report.ErrNoReport)", err)
	}
	if strings.Contains(err.Error(), "ghost") {
		t.Errorf("the stale report reached the caller: %v", err)
	}
}

// Decision 4: report_path is fixed, so chunk i+1 overwrites chunk i's report. The read
// therefore happens per chunk, and the cross-chunk de-duplication carries over unchanged.
func TestRunMergesEveryChunksReportBeforeTheNextOverwritesIt(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	// One id per chunk: MaxArgvBytes is a byte budget, so a tiny budget forces the split.
	first := writeStubXML(t, repo, "first.xml", `<testsuite name="s"><testcase classname="s.A" name="one" time="0.01"/></testsuite>`)
	a.Env["RTDD_STUB_XML"] = first

	res, err := runWithBudget(a, repo, []string{"s.A#one", "s.B#two"}, 12)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Both chunks wrote the same single-case report, so the merged result is that one
	// outcome recorded once — not two, and not the second chunk's report alone.
	if len(res.Outcomes) != 1 || res.Outcomes[0].Test != "s.A#one" {
		t.Fatalf("Outcomes = %+v, want exactly one s.A#one after the per-chunk merge", res.Outcomes)
	}
}

// Two chunks, each writing its OWN report: every test from both appears exactly once, so
// the merge is a union rather than the last chunk's report alone.
func TestRunMergesTheOutcomesOfEveryChunkExactlyOnce(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	// The stub picks its XML from RTDD_STUB_XML, which is fixed for the whole run, so
	// each chunk is made to write its own report by keying the source file off the ids
	// the chunk was handed: the wrapper script copies the file named after its first
	// selector argument.
	writeStubXML(t, repo, "s.A#one.xml", `<testsuite name="s"><testcase classname="s.A" name="one" time="0.01"/></testsuite>`)
	writeStubXML(t, repo, "s.B#two.xml", `<testsuite name="s"><testcase classname="s.B" name="two" time="0.02"/></testsuite>`)
	perChunk := filepath.Join(repo, "stubperchunk")
	script := `#!/bin/sh
out=""
first=""
for a in "$@"; do
  case "$a" in
    --outputFile=*) out="${a#--outputFile=}" ;;
    *) if [ -z "$first" ]; then first="$a"; fi ;;
  esac
done
cat "` + repo + `/$first.xml" > "$out"
`
	if err := os.WriteFile(perChunk, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	a.Subset = perChunk + " {tests} --outputFile={report}"

	res, err := runWithBudget(a, repo, []string{"s.A#one", "s.B#two"}, 12)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	seen := map[string]int{}
	for _, o := range res.Outcomes {
		seen[o.Test]++
	}
	if len(res.Outcomes) != 2 || seen["s.A#one"] != 1 || seen["s.B#two"] != 1 {
		t.Fatalf("Outcomes = %+v, want each of s.A#one and s.B#two exactly once", res.Outcomes)
	}
}

// A chunk whose report the parser cannot read fails the run with an error naming the
// chunk, never a silent partial result carrying only the chunks that did parse.
func TestAnUnparseableChunkReportFailsTheRunNamingTheChunk(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	broken := writeStubXML(t, repo, "broken.xml", `<testsuite name="s"><testcase `)
	a.Env["RTDD_STUB_XML"] = broken

	_, err := runWithBudget(a, repo, []string{"s.A#one", "s.B#two"}, 12)
	if err == nil {
		t.Fatalf("Run = nil error; an unparseable report must fail the run")
	}
	if !errors.Is(err, report.ErrMalformedReport) {
		t.Fatalf("error = %v, want errors.Is(_, report.ErrMalformedReport)", err)
	}
	if !strings.Contains(err.Error(), "chunk 0") {
		t.Errorf("error = %v, want it to name the failing chunk", err)
	}
}

// PRD #231 AC8: the adapter's exit_codes mapping applies on this path exactly as it does
// on the pytest one — and it is read BEFORE the report, so a bad-selector exit is a named
// fatal error and never a parse failure against a file the runner declined to write.
func TestExitCodeMappingAppliesOnTheJUnitPathBeforeTheReportIsRead(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "4"})

	_, err := Run(a, repo, []string{"s.A#one"}, false)
	if err == nil {
		t.Fatalf("Run = nil error for an exit mapped to bad-selector")
	}
	var fatal *FatalExitError
	if !errors.As(err, &fatal) {
		t.Fatalf("error = %v (%T), want *FatalExitError", err, err)
	}
	if fatal.Code != 4 || fatal.Label != "bad-selector" {
		t.Errorf("FatalExitError = %+v, want code 4 bad-selector", fatal)
	}
	if errors.Is(err, report.ErrNoReport) {
		t.Errorf("the missing report shadowed the exit-code mapping: %v", err)
	}
}

// The pytest-reportlog path is unchanged by the dispatch: the same stub adapter that
// produced these outcomes before produces them now, through readOutcomes rather than an
// unconditional report.ReadReportLog.
func TestReportLogPathIsUnchangedByTheDispatch(t *testing.T) {
	repo := t.TempDir()
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile,
		testReportEntry("tests/test_a.py::test_add", "setup", "passed", 0.001),
		testReportEntry("tests/test_a.py::test_add", "call", "passed", 0.412),
		testReportEntry("tests/test_a.py::test_add", "teardown", "passed", 0.001),
	)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"),
		"tests/test_a.py::test_add|run", []byte{0x20})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	res, err := Run(a, repo, []string{"tests/test_a.py::test_add"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Test != "tests/test_a.py::test_add" {
		t.Fatalf("Outcomes = %+v, want the one outcome the reportlog path produced before", res.Outcomes)
	}
	if res.Outcomes[0].Status != "pass" || res.Outcomes[0].DurationMS != 412 {
		t.Errorf("Outcome = %+v, want {pass 412}", res.Outcomes[0])
	}
	if len(res.Coverage.PerTest) != 1 {
		t.Errorf("Coverage.PerTest = %+v, want the coverage read to be unchanged", res.Coverage.PerTest)
	}
}

// An adapter declaring a report format the runner cannot read fails by name rather than
// falling through to whichever parser happens to be first.
func TestAnUnsupportedReportFormatIsNamed(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	a.Report = "tap"

	_, err := Run(a, repo, []string{"s.A#one"}, false)
	if err == nil {
		t.Fatalf("Run = nil error for an unsupported report format")
	}
	if !strings.Contains(err.Error(), `unsupported report "tap"`) {
		t.Errorf("error = %v, want it to name the unsupported report", err)
	}
}

// #285: two cases of one report file sharing a rendered id must not collapse into the
// LAST one — a failure followed by a pass under a file-granular id_template would exit 0
// with an empty Failed list. The parser folds them worst-status-wins before the runner's
// cross-chunk rule ever sees them, so the run is red.
func TestRunDoesNotLoseAFailureToACaseSharingItsID(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	a.IDTemplate = "{classname}"
	xml := writeStubXML(t, repo, "src.xml", `<testsuite name="s">
  <testcase classname="test/calc.test.js" name="a" time="0.01"><failure message="boom"/></testcase>
  <testcase classname="test/calc.test.js" name="b" time="0.01"/>
</testsuite>`)
	a.Env["RTDD_STUB_XML"] = xml

	res, err := Run(a, repo, []string{"test/calc.test.js"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Status != "fail" {
		t.Fatalf("Outcomes = %+v, want one test/calc.test.js fail", res.Outcomes)
	}
	if len(res.Failed) != 1 || res.Failed[0] != "test/calc.test.js" {
		t.Errorf("Failed = %v, want [test/calc.test.js]", res.Failed)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1: a failing case is in the report", res.ExitCode)
	}
}

// PRD #231 AC6 at the runner: a runner that exits 0 having run nothing must not produce a
// green RTDD run. `go test ./... -run TestDoesNotExist` is exactly this — exit 0 and a
// report naming no test — so a non-empty selection whose ids the runner did not match
// would otherwise print "0 ran, 0 failed" and exit 0, which is the false green the whole
// PRD exists to prevent. The stub exits 0 deliberately: the guard is the report's content,
// not the process's exit code, which exit_codes cannot substitute for.
func TestRunRefusesAReportThatNamesNoTest(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	xml := writeStubXML(t, repo, "none.xml", `<?xml version="1.0"?><testsuites><testsuite name="pkg" tests="0"></testsuite></testsuites>`)
	a.Env["RTDD_STUB_XML"] = xml

	res, err := Run(a, repo, []string{"s.A#one"}, false)
	if err == nil {
		t.Fatalf("Run = %+v, nil error; a run whose report names no test must not be a success", res)
	}
	if !errors.Is(err, report.ErrNoTestcases) {
		t.Fatalf("error = %v, want errors.Is(_, report.ErrNoTestcases)", err)
	}
}

// The same guard for a report whose ROOT carries the failure: the runner blew up before it
// named a suite, and reading only <testsuite> children turned that into a run of nothing.
func TestRunSurfacesARootLevelReportFailure(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	xml := writeStubXML(t, repo, "rootfail.xml", `<testsuites name="run"><failure message="config blew up"/></testsuites>`)
	a.Env["RTDD_STUB_XML"] = xml

	res, err := Run(a, repo, []string{"s.A#one"}, false)
	if err == nil {
		t.Fatalf("Run = %+v, nil error for a report whose root carries a <failure>", res)
	}
	if !errors.Is(err, report.ErrSuiteFailure) {
		t.Fatalf("error = %v, want errors.Is(_, report.ErrSuiteFailure)", err)
	}
	if !strings.Contains(err.Error(), "config blew up") {
		t.Errorf("error %q does not carry the runner's own message", err)
	}
}

// perChunkStubAdapter is junitStubAdapter with a wrapper that writes a DIFFERENT report
// per chunk: the file named after the chunk's first selector argument. A real junit runner
// reports every case in the files it loads, not just the ones its -t filter selected, so
// two chunks' reports routinely overlap.
func perChunkStubAdapter(t *testing.T, repo string) *adapter.Adapter {
	t.Helper()
	a := junitStubAdapter(t, repo, nil)
	perChunk := filepath.Join(repo, "stubperchunk")
	script := `#!/bin/sh
out=""
first=""
for a in "$@"; do
  case "$a" in
    --outputFile=*) out="${a#--outputFile=}" ;;
    *) if [ -z "$first" ]; then first="$a"; fi ;;
  esac
done
cat "` + repo + `/$first.xml" > "$out"
`
	if err := os.WriteFile(perChunk, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	a.Subset = perChunk + " {tests} --outputFile={report}"
	return a
}

// #300: on the junit path a chunk reports cases it did not select — jest, vitest and RSpec
// spell the filtered-out cases of every file they load as <skipped/>. Folding chunks with
// "last invocation wins" therefore lets a later chunk's skip erase an earlier chunk's
// genuine failure, putting a green row in the map for a red test. Chunks fold
// worst-status-wins, exactly as the cases of one report already do.
func TestALaterChunksSkipDoesNotEraseAnEarlierChunksFailure(t *testing.T) {
	repo := t.TempDir()
	a := perChunkStubAdapter(t, repo)
	writeStubXML(t, repo, "s.B#two.xml", `<testsuite name="s"><testcase classname="s.B" name="two" time="0.01"><failure message="nope"/></testcase></testsuite>`)
	writeStubXML(t, repo, "s.C#three.xml", `<testsuite name="s">
  <testcase classname="s.B" name="two" time="0.01"><skipped/></testcase>
  <testcase classname="s.C" name="three" time="0.01"/>
</testsuite>`)

	res, err := runWithBudget(a, repo, []string{"s.B#two", "s.C#three"}, 12)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	byTest := map[string]string{}
	for _, o := range res.Outcomes {
		byTest[o.Test] = o.Status
	}
	if byTest["s.B#two"] != "fail" {
		t.Errorf("Outcomes = %+v, want s.B#two to stay fail after chunk 1 reported it skipped", res.Outcomes)
	}
	if byTest["s.C#three"] != "pass" {
		t.Errorf("Outcomes = %+v, want s.C#three pass", res.Outcomes)
	}
	if len(res.Failed) != 1 || res.Failed[0] != "s.B#two" {
		t.Errorf("Failed = %v, want [s.B#two]", res.Failed)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1 — a failure a later chunk reported as skipped is still a failure", res.ExitCode)
	}
}

// The symmetric direction: the failure arrives in the LATER chunk. Worst-status-wins is not
// "the first chunk wins" — the red result survives whichever chunk reported it.
func TestALaterChunksFailureSurvivesAnEarlierChunksSkip(t *testing.T) {
	repo := t.TempDir()
	a := perChunkStubAdapter(t, repo)
	writeStubXML(t, repo, "s.B#two.xml", `<testsuite name="s"><testcase classname="s.B" name="two" time="0.01"><skipped/></testcase></testsuite>`)
	writeStubXML(t, repo, "s.C#three.xml", `<testsuite name="s">
  <testcase classname="s.B" name="two" time="0.01"><failure message="nope"/></testcase>
  <testcase classname="s.C" name="three" time="0.01"/>
</testsuite>`)

	res, err := runWithBudget(a, repo, []string{"s.B#two", "s.C#three"}, 12)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	byTest := map[string]string{}
	for _, o := range res.Outcomes {
		byTest[o.Test] = o.Status
	}
	if byTest["s.B#two"] != "fail" {
		t.Errorf("Outcomes = %+v, want s.B#two fail", res.Outcomes)
	}
	if len(res.Failed) != 1 || res.Failed[0] != "s.B#two" {
		t.Errorf("Failed = %v, want [s.B#two]", res.Failed)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", res.ExitCode)
	}
}
