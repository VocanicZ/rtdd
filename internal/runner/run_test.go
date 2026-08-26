package runner

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// stubScript stands in for pytest so the runner's control flow — chunking, env
// forcing, exit-code mapping, sysmon detection, per-chunk coverage merging — is
// tested deterministically and without a Python interpreter.
const stubScript = `#!/bin/sh
log=""
for a in "$@"; do
  case "$a" in
    --report-log=*) log="${a#--report-log=}" ;;
  esac
done
if [ -n "$RTDD_STUB_ARGS" ]; then
  printf -- '--- invocation ---\n' >> "$RTDD_STUB_ARGS"
  for a in "$@"; do printf '%s\n' "$a" >> "$RTDD_STUB_ARGS"; done
fi
if [ -n "$RTDD_STUB_STDOUT" ]; then
  printf '%s\n' "$RTDD_STUB_STDOUT"
fi
if [ -n "$RTDD_STUB_STDERR" ]; then
  printf '%s\n' "$RTDD_STUB_STDERR" >&2
fi
if [ -n "$log" ] && [ -n "$RTDD_STUB_REPORT" ] && [ -f "$RTDD_STUB_REPORT" ]; then
  cat "$RTDD_STUB_REPORT" > "$log"
fi
if [ -n "$RTDD_STUB_COVERAGE" ] && [ -f "$RTDD_STUB_COVERAGE" ]; then
  cp "$RTDD_STUB_COVERAGE" .coverage
fi
exit ${RTDD_STUB_EXIT:-0}
`

const stubDDL = `
CREATE TABLE meta (key text, value text, unique (key));
CREATE TABLE file (id integer primary key, path text, unique (path));
CREATE TABLE context (id integer primary key, context text, unique (context));
CREATE TABLE line_bits (file_id integer, context_id integer, numbits blob,
    unique (file_id, context_id));
CREATE TABLE arc (file_id integer, context_id integer, fromno integer, tono integer,
    unique (file_id, context_id, fromno, tono));
`

// writeStubCoverage builds a .coverage template holding one test context.
func writeStubCoverage(t *testing.T, path, absSrc, ctx string, numbits []byte) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open stub coverage: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(stubDDL); err != nil {
		t.Fatalf("stub ddl: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO meta (key, value) VALUES ('version','7.15.4'),('has_arcs','0')`); err != nil {
		t.Fatalf("stub meta: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO file (id, path) VALUES (1, ?)`, absSrc); err != nil {
		t.Fatalf("stub file: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO context (id, context) VALUES (1, ?)`, ctx); err != nil {
		t.Fatalf("stub context: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO line_bits (file_id, context_id, numbits) VALUES (1, 1, ?)`, numbits); err != nil {
		t.Fatalf("stub line_bits: %v", err)
	}
}

// stubEnv builds an adapter whose subset/seed commands are the stub script.
func stubAdapter(t *testing.T, repo string, env map[string]string) *adapter.Adapter {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub runner script is POSIX sh")
	}
	stub := filepath.Join(repo, "stubpytest")
	if strings.ContainsAny(stub, " \t") {
		t.Skipf("temp dir %q contains whitespace; command templates are whitespace-split", stub)
	}
	if err := os.WriteFile(stub, []byte(stubScript), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	if env == nil {
		env = map[string]string{}
	}
	return &adapter.Adapter{
		Name:         "stub",
		Detect:       []string{"pyproject.toml"},
		Env:          env,
		Seed:         stub + " --report-log={log}",
		Subset:       stub + " {tests} --report-log={log}",
		List:         stub + " --collect-only",
		Coverage:     "sqlite",
		Report:       "pytest-reportlog",
		FailFastFlag: "-x",
		ExitCodes:    map[int]string{4: "bad-selector", 5: "no-tests-collected"},
	}
}

func writeStubReport(t *testing.T, path string, entries ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("{\"pytest_version\": \"9.0.3\", \"$report_type\": \"SessionStart\"}\n")
	for _, e := range entries {
		b.WriteString(e)
		b.WriteString("\n")
	}
	b.WriteString("{\"exitstatus\": 0, \"$report_type\": \"SessionFinish\"}\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write stub report: %v", err)
	}
}

func testReportEntry(nodeID, when, outcome string, durSec float64) string {
	return fmt.Sprintf(
		`{"$report_type": "TestReport", "nodeid": %q, "when": %q, "outcome": %q, "duration": %v, "longrepr": null}`,
		nodeID, when, outcome, durSec)
}

func invocations(t *testing.T, argsFile string) int {
	t.Helper()
	b, err := os.ReadFile(argsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read args file: %v", err)
	}
	return strings.Count(string(b), "--- invocation ---")
}

func TestRunEmptySelectionExecutesNothing(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_ARGS": argsFile})

	res, err := Run(a, repo, nil, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if len(res.Outcomes) != 0 {
		t.Errorf("Outcomes = %+v, want none", res.Outcomes)
	}
	if res.Coverage == nil || res.Coverage.ImportTime == nil {
		t.Error("Coverage must be a usable empty Result, not nil")
	}
	if n := invocations(t, argsFile); n != 0 {
		t.Errorf("%d subprocess invocations for an empty selection, want 0", n)
	}
}

func TestRunHappyPath(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
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
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	res, err := Run(a, repo, []string{"tests/test_a.py::test_add"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Test != "tests/test_a.py::test_add" {
		t.Fatalf("Outcomes = %+v, want one entry for tests/test_a.py::test_add", res.Outcomes)
	}
	if res.Outcomes[0].Status != "pass" || res.Outcomes[0].DurationMS != 412 {
		t.Errorf("Outcome = %+v, want {pass 412}", res.Outcomes[0])
	}
	if len(res.Coverage.PerTest) != 1 {
		t.Fatalf("Coverage.PerTest = %+v, want one entry", res.Coverage.PerTest)
	}
	if got := res.Coverage.PerTest[0].Files["src/logic.py"]; len(got) != 1 || got[0] != 5 {
		t.Errorf("Coverage for src/logic.py = %v, want [5]", got)
	}
	if len(res.Failed) != 0 {
		t.Errorf("Failed = %v, want none", res.Failed)
	}
}

func TestRunAdapterEnvReachesTheChild(t *testing.T) {
	// This is the same mechanism COVERAGE_CORE=ctrace depends on. If Adapter.Env
	// does not reach the subprocess, sysmon stays on and the map silently empties.
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
		"RTDD_STUB_STDOUT":   "env-marker-reached-the-child",
	})
	if _, err := Run(a, repo, []string{"t"}, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := invocations(t, argsFile); n != 1 {
		t.Fatalf("%d invocations, want 1 — RTDD_STUB_ARGS did not reach the child", n)
	}
}

func TestRunOverridesInheritedEnv(t *testing.T) {
	// A developer with COVERAGE_CORE=sysmon exported in their shell must still get
	// ctrace. The adapter's value wins over the inherited one.
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})

	t.Setenv("RTDD_STUB_STDOUT", "inherited-value")
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
		"RTDD_STUB_STDOUT":   "", // adapter clears it
	})
	if _, err := Run(a, repo, []string{"t"}, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The stub prints RTDD_STUB_STDOUT only when non-empty; nothing to assert on
	// stdout, but the run must have succeeded with the adapter's empty override
	// rather than inheriting "inherited-value" and continuing.
	if n := invocations(t, argsFile); n != 1 {
		t.Fatalf("%d invocations, want 1", n)
	}
}

// mergeEnv is where the override happens, and the empty-string override above can
// only be observed indirectly. Assert the shape directly: the inherited entry is
// REPLACED, not appended after, so a child that reads the first match still wins.
func TestMergeEnvReplacesRatherThanAppends(t *testing.T) {
	base := []string{"PATH=/bin", "COVERAGE_CORE=sysmon", "HOME=/home/x"}
	got := mergeEnv(base, map[string]string{"COVERAGE_CORE": "ctrace"})

	var cores []string
	for _, kv := range got {
		if strings.HasPrefix(kv, "COVERAGE_CORE=") {
			cores = append(cores, kv)
		}
	}
	if len(cores) != 1 || cores[0] != "COVERAGE_CORE=ctrace" {
		t.Fatalf("COVERAGE_CORE entries = %v, want exactly [COVERAGE_CORE=ctrace]", cores)
	}
	for _, want := range []string{"PATH=/bin", "HOME=/home/x"} {
		found := false
		for _, kv := range got {
			if kv == want {
				found = true
			}
		}
		if !found {
			t.Errorf("mergeEnv dropped inherited %q; got %v", want, got)
		}
	}
}

// AUDIT A7 REGRESSION TEST. The warning arrives on STDOUT and the process exits 0.
// Scanning stderr alone, or trusting the exit code, writes a ~90%-empty map.
func TestRunSysmonWarningOnStdoutIsFatal(t *testing.T) {
	repo := t.TempDir()
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
		"RTDD_STUB_EXIT":     "0",
		"RTDD_STUB_STDOUT": "  /lib/coverage/control.py:799: CoverageWarning: Dynamic contexts aren't supported " +
			"with core=sysmon; context data may be incomplete (no-sysmon-context); see " +
			"https://coverage.readthedocs.io/en/7.15.4/messages.html#warning-no-sysmon-context",
	})

	_, err := Run(a, repo, []string{"tests/test_a.py::test_add"}, false)
	if !errors.Is(err, ErrSysmonContext) {
		t.Fatalf("Run = %v, want ErrSysmonContext — the warning was on stdout and the exit code was 0", err)
	}
}

func TestRunSysmonWarningOnStderrIsAlsoFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_EXIT":   "0",
		"RTDD_STUB_STDERR": "CoverageWarning: ... (no-sysmon-context); see ...",
	})
	_, err := Run(a, repo, []string{"t"}, false)
	if !errors.Is(err, ErrSysmonContext) {
		t.Fatalf("Run = %v, want ErrSysmonContext", err)
	}
}

func TestRunExit4IsFatalNotATestFailure(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "4"})
	_, err := Run(a, repo, []string{"tests/test_a.py::test_gone"}, false)
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatalf("Run = %v, want *FatalExitError", err)
	}
	if fe.Code != 4 || fe.Label != "bad-selector" {
		t.Fatalf("FatalExitError = %+v, want code 4 bad-selector", fe)
	}
}

func TestRunExit5IsFatalNotATestFailure(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "5"})
	_, err := Run(a, repo, []string{"t"}, false)
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatalf("Run = %v, want *FatalExitError", err)
	}
	if fe.Code != 5 || fe.Label != "no-tests-collected" {
		t.Fatalf("FatalExitError = %+v, want code 5 no-tests-collected", fe)
	}
}

// A fatal chunk stops the run: the remaining chunks must never be executed, and
// the reported Chunk index must be the one that actually failed.
func TestRunFatalExitStopsTheRemainingChunks(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS": argsFile,
		"RTDD_STUB_EXIT": "4",
	})

	ids := make([]string, 8000)
	for i := range ids {
		ids[i] = fmt.Sprintf("tests/unit/test_module_%04d.py::test_case_name[param-%d]", i, i)
	}
	if want := len(Chunk(ids, MaxArgvBytes)); want < 2 {
		t.Fatalf("fixture produced %d chunks, want the run to be chunked", want)
	}

	_, err := Run(a, repo, ids, false)
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatalf("Run = %v, want *FatalExitError", err)
	}
	if fe.Chunk != 0 {
		t.Errorf("FatalExitError.Chunk = %d, want 0", fe.Chunk)
	}
	if n := invocations(t, argsFile); n != 1 {
		t.Errorf("%d invocations after a fatal chunk 0, want 1 — the run must stop", n)
	}
}

func TestRunExit1IsATestFailureNotAnError(t *testing.T) {
	repo := t.TempDir()
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile,
		testReportEntry("tests/test_b.py::test_fail", "setup", "passed", 0.001),
		testReportEntry("tests/test_b.py::test_fail", "call", "failed", 0.002),
		testReportEntry("tests/test_b.py::test_fail", "teardown", "passed", 0.001),
	)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"),
		"tests/test_b.py::test_fail|run", []byte{0x00, 0x02})

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
		"RTDD_STUB_EXIT":     "1",
	})
	res, err := Run(a, repo, []string{"tests/test_b.py::test_fail"}, false)
	if err != nil {
		t.Fatalf("Run = %v, want nil error (exit 1 is a test failure, not a fatal error)", err)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", res.ExitCode)
	}
	want := []string{"tests/test_b.py::test_fail"}
	if len(res.Failed) != 1 || res.Failed[0] != want[0] {
		t.Errorf("Failed = %v, want %v", res.Failed, want)
	}
}

// `error` is a distinct pytest outcome from `fail` — a fixture that raises never
// reaches the call phase. Both belong in Failed, or a broken fixture reads as a pass.
func TestRunErrorOutcomeCountsAsFailed(t *testing.T) {
	repo := t.TempDir()
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile,
		testReportEntry("tests/test_c.py::test_broken_fixture", "setup", "failed", 0.003),
		testReportEntry("tests/test_c.py::test_broken_fixture", "teardown", "passed", 0.001),
	)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
		"RTDD_STUB_EXIT":     "1",
	})
	res, err := Run(a, repo, []string{"tests/test_c.py::test_broken_fixture"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Status != "error" {
		t.Fatalf("Outcomes = %+v, want one entry with status error", res.Outcomes)
	}
	if len(res.Failed) != 1 || res.Failed[0] != "tests/test_c.py::test_broken_fixture" {
		t.Errorf("Failed = %v, want the errored test", res.Failed)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", res.ExitCode)
	}
}

func TestRunUnexpectedExitCodeIsAnError(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "3"})
	if _, err := Run(a, repo, []string{"t"}, false); err == nil {
		t.Fatal("Run with an unmapped nonzero exit = nil error, want error")
	}
}

func TestRunFailFastAppendsTheFlag(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	if _, err := Run(a, repo, []string{"t"}, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, _ := os.ReadFile(argsFile)
	if strings.Contains(string(b), "\n-x\n") {
		t.Fatal("-x was passed without --fail-fast; fail-fast is opt-in only (spec §5, decision D6)")
	}

	os.Remove(argsFile)
	if _, err := Run(a, repo, []string{"t"}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, _ = os.ReadFile(argsFile)
	if !strings.Contains(string(b), "\n-x\n") {
		t.Fatal("--fail-fast did not add the adapter's failfast_flag")
	}
}

// The ids below are the real ones measured from coverage.py contexts. Each must
// arrive at the child as exactly one argv element.
func TestRunPassesHostileIDsAsSeparateArgvElements(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	ids := []string{
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_pipe.py::test_pipe[a|b]",
	}
	if _, err := Run(a, repo, ids, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	lines := strings.Split(string(b), "\n")
	for _, id := range ids {
		found := false
		for _, l := range lines {
			if l == id {
				found = true
			}
		}
		if !found {
			t.Errorf("id %q did not arrive as its own argv element; got lines %q", id, lines)
		}
	}
}

func TestRunChunksAndMergesAtRealisticScale(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
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
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	ids := make([]string, 8000)
	for i := range ids {
		ids[i] = fmt.Sprintf("tests/unit/test_module_%04d.py::test_case_name[param-%d]", i, i)
	}
	res, err := Run(a, repo, ids, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := invocations(t, argsFile); n < 2 {
		t.Fatalf("%d invocations for 8000 ids, want the run to be chunked", n)
	}
	// Every chunk wrote the same stub report; Run must dedupe by test id rather
	// than emitting one Outcome per chunk.
	if len(res.Outcomes) != 1 {
		t.Fatalf("Outcomes = %+v, want one deduped entry", res.Outcomes)
	}
	// Every chunk erased and rewrote .coverage; the merged result must still hold it.
	if len(res.Coverage.PerTest) != 1 {
		t.Fatalf("Coverage.PerTest = %+v, want the merged single entry", res.Coverage.PerTest)
	}
	if got := res.Coverage.PerTest[0].Files["src/logic.py"]; len(got) != 1 || got[0] != 5 {
		t.Fatalf("merged coverage for src/logic.py = %v, want [5]", got)
	}
}

// The load-bearing claim of the per-chunk read: pytest ERASES .coverage every run,
// so chunk 1's contexts exist only in the accumulator by the time chunk 2 finishes.
// A single read after the last chunk loses chunk 1 entirely.
func TestRunMergesEachChunkBeforeTheNextOneErasesCoverage(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	writeStubReport(t, reportFile)

	// Two distinct .coverage templates. The wrapper hands out the first on the first
	// invocation and the second afterwards — exactly as pytest overwrites the store
	// between chunks, keeping nothing of the previous one.
	covA := filepath.Join(repo, "cov-a.db")
	covB := filepath.Join(repo, "cov-b.db")
	writeStubCoverage(t, covA, filepath.Join(repo, "src", "logic.py"),
		"tests/chunk1.py::test_one|run", []byte{0x20})
	writeStubCoverage(t, covB, filepath.Join(repo, "src", "logic.py"),
		"tests/chunk2.py::test_two|run", []byte{0x20})

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":   argsFile,
		"RTDD_STUB_REPORT": reportFile,
	})
	marker := filepath.Join(repo, "seen-first-chunk")
	wrapper := filepath.Join(repo, "rotating-stub")
	wrapperScript := "#!/bin/sh\n" +
		"if [ -f " + marker + " ]; then RTDD_STUB_COVERAGE=" + covB + "; " +
		"else : > " + marker + "; RTDD_STUB_COVERAGE=" + covA + "; fi\n" +
		"export RTDD_STUB_COVERAGE\n" +
		"exec " + filepath.Join(repo, "stubpytest") + " \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(wrapperScript), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}
	a.Subset = wrapper + " {tests} --report-log={log}"

	ids := make([]string, 8000)
	for i := range ids {
		ids[i] = fmt.Sprintf("tests/unit/test_module_%04d.py::test_case_name[param-%d]", i, i)
	}
	res, err := Run(a, repo, ids, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := invocations(t, argsFile); n < 2 {
		t.Fatalf("%d invocations for 8000 ids, want the run to be chunked", n)
	}

	seen := map[string]bool{}
	for _, tc := range res.Coverage.PerTest {
		seen[tc.Test] = true
	}
	// chunk 1's context survives ONLY if it was read and merged before chunk 2 ran.
	if !seen["tests/chunk1.py::test_one"] {
		t.Errorf("chunk 1's context is gone; a late single read of .coverage loses it. PerTest = %+v",
			res.Coverage.PerTest)
	}
	if !seen["tests/chunk2.py::test_two"] {
		t.Errorf("chunk 2's context is missing; PerTest = %+v", res.Coverage.PerTest)
	}
}

func TestRunRemovesAStaleCoverageBeforeExecuting(t *testing.T) {
	// A .coverage left by an earlier, unrelated run must never be mistaken for
	// this run's output.
	repo := t.TempDir()
	stale := filepath.Join(repo, ".coverage")
	if err := os.WriteFile(stale, []byte("not a sqlite file"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})
	res, err := Run(a, repo, []string{"t"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.Coverage.ImportTime["src/logic.py"]; len(got) != 2 {
		t.Fatalf("ImportTime[src/logic.py] = %v, want [1 4] from the stub's fresh .coverage", got)
	}
}

func TestRunMissingCoverageAfterAChunkIsAnError(t *testing.T) {
	repo := t.TempDir()
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	writeStubReport(t, reportFile)
	// No RTDD_STUB_COVERAGE: the stub writes no .coverage at all.
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_REPORT": reportFile})
	if _, err := Run(a, repo, []string{"t"}, false); err == nil {
		t.Fatal("Run with no .coverage produced = nil error; a missing store must be fatal, never an empty map")
	}
}

// The M1b amendment dropped {src} from the runner's variable set: a bare --cov
// honours the host's own [run] source, and an RTDD-guessed {src} makes seed and
// subset disagree on scope. Nothing in internal/adapter enforces that — the var
// map Run supplies is the enforcement, so an adapter naming {src} must FAIL.
func TestRunRejectsAnAdapterThatNamesSrc(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, nil)
	a.Subset = filepath.Join(repo, "stubpytest") + " --cov={src} {tests} --report-log={log}"

	_, err := Run(a, repo, []string{"t"}, false)
	if err == nil {
		t.Fatal("Run with an adapter naming {src} = nil error; the placeholder must be unresolvable")
	}
	if !strings.Contains(err.Error(), "{src}") {
		t.Errorf("error = %v, want it to name the unresolved {src} placeholder", err)
	}
}
