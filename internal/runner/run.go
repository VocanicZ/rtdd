package runner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/report"
)

// RunResult is one complete execution: what ran, what it covered, and what failed.
type RunResult struct {
	Outcomes []report.Outcome
	Coverage *coverage.Result
	Failed   []string
	ExitCode int
}

// Run executes the adapter's subset command over tests.
//
// It sets Adapter.Env (which is how COVERAGE_CORE=ctrace is forced), chunks test
// ids across multiple invocations when argv would exceed MaxArgvBytes, and merges
// the per-chunk results. A chunk that exits with a code mapped in
// Adapter.ExitCodes (4=bad-selector, 5=no-tests-collected) is a fatal error, not
// a test failure. A chunk exiting 1 is a test failure and sets ExitCode.
//
// An empty selection runs nothing at all: `pytest --cov` with no selectors
// collects the whole suite, which is the most expensive possible way to be wrong
// about having nothing to run.
func Run(a *adapter.Adapter, repoRoot string, tests []string, failFast bool) (*RunResult, error) {
	if len(tests) == 0 {
		return &RunResult{Coverage: &coverage.Result{ImportTime: map[string][]int{}}}, nil
	}
	return execute(a, repoRoot, a.Subset, Chunk(tests, MaxArgvBytes), failFast)
}

// execute runs one command template. chunks == nil means a single invocation with
// no {tests} placeholder (the seed and list path).
func execute(a *adapter.Adapter, repoRoot, tmpl string, chunks [][]string, failFast bool) (*RunResult, error) {
	tmpDir, err := os.MkdirTemp("", "rtdd-run-")
	if err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	covPath := filepath.Join(repoRoot, ".coverage")

	// The adapter's report_path is resolved ONCE, before the loop: it is a fixed
	// declaration, not a per-chunk temp file, and resolving it per chunk would report a
	// bad declaration N times instead of once.
	var rp report.ReportPath
	if a.ReportPath != "" {
		rp, err = report.NewReportPathFor(a.Name, repoRoot, a.ReportPath)
		if err != nil {
			return nil, err
		}
	}

	res := &RunResult{Coverage: &coverage.Result{ImportTime: map[string][]int{}}}
	byTest := map[string]int{} // test id -> index in res.Outcomes, for dedupe

	n := len(chunks)
	if n == 0 {
		n = 1
	}
	for i := 0; i < n; i++ {
		logPath := filepath.Join(tmpDir, fmt.Sprintf("report-%d.jsonl", i))
		// {out} and {log} only. {src} was dropped by the M1b amendment: a bare
		// --cov honours the host's own [run] source, and an RTDD-guessed {src}
		// makes seed and subset disagree on scope. Expand has no closed variable
		// set, so this map IS the enforcement — an adapter naming {src} fails here
		// as an unresolved placeholder rather than silently narrowing coverage.
		//
		// {log} is this chunk's scratch file, and which side writes it is the adapter's
		// business: pytest writes its report log there, and a report_cmd adapter reads
		// the captured output the engine writes there. One name, one file, one lifetime.
		vars := map[string]string{"log": logPath, "out": tmpDir}
		// {report} joins them, and ONLY when the adapter declares report_path. Expand's
		// vocabulary is the caller's map, so an adapter naming {report} without
		// report_path still fails as an unresolved placeholder rather than receiving an
		// empty string and writing its report to "".
		if a.ReportPath != "" {
			vars["report"] = rp.Abs
		}

		var argv []string
		if len(chunks) == 0 {
			argv, err = a.Expand(tmpl, vars)
		} else {
			argv, err = a.ExpandTests(tmpl, vars, chunks[i])
		}
		if err != nil {
			return nil, err
		}
		if failFast && a.FailFastFlag != "" {
			argv = append(argv, a.FailFastFlag)
		}

		// A .coverage left by an earlier, unrelated run must never be mistaken for
		// this chunk's output. pytest erases it anyway unless --cov-append is
		// passed; removing it first makes that guarantee ours, not pytest's.
		if err := os.Remove(covPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("runner: removing stale %s: %w", covPath, err)
		}

		// The report is cleared per CHUNK, for the same reason: report_path is a fixed,
		// adapter-declared path, so clearing it once per Run would let chunk i's cases be
		// re-read as chunk i+1's if chunk i+1 crashed before writing.
		if a.ReportPath != "" {
			if err := rp.Clear(); err != nil {
				return nil, fmt.Errorf("runner: chunk %d: %w", i, err)
			}
		}

		var combined bytes.Buffer
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = repoRoot // .coverage is written to the process CWD, not the rootdir
		cmd.Env = mergeEnv(os.Environ(), a.Env)
		cmd.Stdout = &combined
		cmd.Stderr = &combined
		runErr := cmd.Run()

		code := 0
		if runErr != nil {
			var ee *exec.ExitError
			if !errors.As(runErr, &ee) {
				return nil, fmt.Errorf("runner: executing %s: %w", argv[0], runErr)
			}
			code = ee.ExitCode()
		}

		// Checked BEFORE the exit code, because the warning is emitted on a run
		// that exits 0 with a ~90%-empty map (audit A7). Measured: pytest prints
		// it on STDOUT; stderr is empty. This scans the combined stream.
		if hasSysmonWarning(combined.Bytes()) {
			return nil, ErrSysmonContext
		}

		if label, ok := a.ExitCodes[code]; ok {
			return nil, &FatalExitError{Chunk: i, Code: code, Label: label}
		}
		if code != 0 && code != 1 {
			return nil, fmt.Errorf("runner: chunk %d: %s exited %d\n%s",
				i, argv[0], code, tail(combined.Bytes(), 4000))
		}
		if code == 1 {
			res.ExitCode = 1
		}

		// report_cmd, when declared, is what PRODUCES the file the next statement reads.
		// It runs AFTER the invocation and BEFORE the read, once per chunk, so chunk i's
		// report is converted from chunk i's capture and from nothing else — the same
		// reason the report path is cleared per chunk just above.
		//
		// It runs on a chunk that exited 1 too: a failing subset is exactly the run whose
		// report matters most, and skipping the conversion there would read the report
		// the clear just emptied and call a failing suite green.
		if a.ReportCmd != "" {
			if err := runReportCmd(a, repoRoot, logPath, combined.Bytes(), vars); err != nil {
				return nil, err
			}
		}

		outs, err := readOutcomes(a, logPath, rp)
		if err != nil {
			return nil, fmt.Errorf("runner: chunk %d: %w", i, err)
		}
		for _, o := range outs {
			if j, seen := byTest[o.Test]; seen {
				// NOT last-invocation-wins: a junit chunk reports every case in the
				// files it loaded, so a later chunk re-reports an earlier chunk's
				// failure as <skipped/>. report.FoldOutcome keeps the worse status.
				res.Outcomes[j] = report.FoldOutcome(res.Outcomes[j], o)
				continue
			}
			byTest[o.Test] = len(res.Outcomes)
			res.Outcomes = append(res.Outcomes, o)
		}

		// Read and merge NOW, before the next chunk runs: pytest erases .coverage
		// at the start of every invocation, so chunk i's contexts exist nowhere
		// else once chunk i+1 starts. --cov-append is not the alternative — it
		// would also absorb a stale store from an unrelated earlier run.
		//
		// coverage: none has no .coverage to read at all, and ReadSQLite against a file
		// that does not exist is an error rather than an empty result — so the read is
		// skipped, not attempted and forgiven.
		if a.Coverage != adapter.CoverageNone {
			cov, err := coverage.ReadSQLite(covPath, repoRoot)
			if err != nil {
				return nil, fmt.Errorf("runner: chunk %d: %w", i, err)
			}
			res.Coverage.Merge(cov)
		}
	}

	for _, o := range res.Outcomes {
		if o.Status == "fail" || o.Status == "error" {
			res.Failed = append(res.Failed, o.Test)
		}
	}
	sort.Strings(res.Failed)
	if len(res.Failed) > 0 {
		res.ExitCode = 1
	}
	return res, nil
}

// runReportCmd hands one chunk's captured combined output to the converter the adapter
// declares, which is the only way the argv-only engine can express `go test -json |
// go-junit-report`: the runner writes machine-readable output to stdout and the converter
// reads stdin, and the engine never hands a shell a string (spec §4.3, plan 06-m6d
// decision 9).
//
// {log} is the same per-chunk scratch file the pytest path names — there, the file the
// RUNNER writes; here, the file the ENGINE writes. Both are "this chunk's scratch file",
// so a third name would be a third thing for an adapter author to learn about one path.
//
// A non-zero exit is FATAL and names the adapter and the command. Continuing would read a
// report that was never written — or, without the per-chunk clear, the previous chunk's —
// and an empty JUnit report parses as a run in which nothing failed (#294).
func runReportCmd(a *adapter.Adapter, repoRoot, logPath string, captured []byte, vars map[string]string) error {
	if err := os.WriteFile(logPath, captured, 0o600); err != nil {
		return fmt.Errorf("runner: adapter %s: report_cmd %q: writing {log}: %w", a.Name, a.ReportCmd, err)
	}
	// Expand, not ExpandTests: the ids are already spent on the invocation that produced
	// the capture, and a report_cmd naming {tests} is rejected there rather than receiving
	// a chunk it has no use for.
	argv, err := a.Expand(a.ReportCmd, vars)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = repoRoot // the same directory the subset ran in: {log} and {report} are both absolute, but the converter may read the repo's own config
	cmd.Env = mergeEnv(os.Environ(), a.Env)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("runner: adapter %s: report_cmd %q: %w\n%s",
			a.Name, a.ReportCmd, err, tail(out.Bytes(), 4000))
	}
	return nil
}

// readOutcomes reads one chunk's outcomes through the parser the adapter's `report:`
// field names. It is the ONE dispatch this path has, and it sits exactly where
// report.ReadReportLog was called unconditionally, so the two parsers stay side by side
// rather than one acquiring a caller it was never tested under.
//
// The junit path reads the adapter's report_path rather than the per-chunk {log}: an
// adapter-declared path is fixed, which is why the caller clears it and reads it per
// chunk. Both parsers produce report.Outcome in the same vocabulary, whose Test is in the
// same namespace as the ids spliced into the command — so the caller's cross-chunk
// de-duplication keeps meaning what it means on the pytest path. Which outcome that
// de-duplication keeps is report.FoldOutcome's decision, not this dispatch's.
func readOutcomes(a *adapter.Adapter, logPath string, rp report.ReportPath) ([]report.Outcome, error) {
	switch a.Report {
	case "pytest-reportlog":
		return report.ReadReportLog(logPath)
	case "junit-xml":
		return report.ReadJUnitReport(rp, a.IDTemplate)
	default:
		return nil, fmt.Errorf("runner: adapter %s: unsupported report %q", a.Name, a.Report)
	}
}

// mergeEnv returns base with overrides applied, replacing rather than appending
// so an inherited COVERAGE_CORE=sysmon cannot survive.
func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		k, _, ok := strings.Cut(kv, "=")
		if ok {
			if _, hit := overrides[k]; hit {
				continue
			}
		}
		out = append(out, kv)
	}
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+overrides[k])
	}
	return out
}

// tail keeps a failing chunk's diagnostics readable: the end of the stream is
// where the traceback and the summary line are.
func tail(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return "..." + string(b[len(b)-n:])
}

// Seed runs the adapter's full instrumented seed command once, over the whole
// suite. It is the only operation that may shrink a map row, so a partial or
// corrupted seed is worse than no seed: every condition Run treats as fatal is
// fatal here too, exit 5 (an empty suite) included.
//
// chunks == nil is what makes it one invocation with no {tests} placeholder — a
// seed template that names {tests} fails in Expand rather than running the suite
// with the ids dropped.
func Seed(a *adapter.Adapter, repoRoot string) (*RunResult, error) {
	return execute(a, repoRoot, a.Seed, nil, false)
}

// emptySuiteLabel is the exit_codes label whose meaning differs between List and
// Run. For List it is a legitimately empty suite; for a subset Run it means the
// ids RTDD produced selected nothing, which is fatal.
const emptySuiteLabel = "no-tests-collected"

// List returns every test id the adapter's list command reports, in COLLECTION
// ORDER — pytest runs the suite in that order, so sorting here would silently
// reorder every run driven off the result.
//
// Measured `pytest --collect-only -q` output is one nodeid per line, terminated by
// a blank line and a `N tests collected in Xs` summary. The blank line is the
// terminator; the summary must never become a test id.
//
// It does not go through execute: there is no coverage store and no report log to
// read, and — the asymmetry that matters — an exit mapped to "no-tests-collected"
// is an empty suite here, not the fatal "the ids I produced selected nothing" it
// means for a subset run.
func List(a *adapter.Adapter, repoRoot string) ([]string, error) {
	if a.List == "" {
		return nil, fmt.Errorf("runner: adapter %s has no list command", a.Name)
	}

	tmpDir, err := os.MkdirTemp("", "rtdd-list-")
	if err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// {out} and {log} only, exactly as in execute. {src} is absent by the M1b
	// amendment and this map is the enforcement. Real paths rather than empty
	// strings: a list template naming {log} must not receive `--report-log=`.
	vars := map[string]string{
		"log": filepath.Join(tmpDir, "list-report.jsonl"),
		"out": tmpDir,
	}
	argv, err := a.Expand(a.List, vars)
	if err != nil {
		return nil, err
	}

	var combined bytes.Buffer
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = repoRoot
	cmd.Env = mergeEnv(os.Environ(), a.Env)
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	runErr := cmd.Run()

	code := 0
	if runErr != nil {
		var ee *exec.ExitError
		if !errors.As(runErr, &ee) {
			return nil, fmt.Errorf("runner: executing %s: %w", argv[0], runErr)
		}
		code = ee.ExitCode()
	}

	// Before the exit code, as in execute: the warning rides a run that exits 0.
	if hasSysmonWarning(combined.Bytes()) {
		return nil, ErrSysmonContext
	}

	if label, ok := a.ExitCodes[code]; ok {
		if label == emptySuiteLabel {
			return nil, nil // an empty suite is empty, not an error
		}
		return nil, &FatalExitError{Chunk: 0, Code: code, Label: label}
	}
	if code != 0 {
		return nil, fmt.Errorf("runner: listing tests: %s exited %d\n%s",
			argv[0], code, tail(combined.Bytes(), 4000))
	}
	return parseTestIDs(combined.String()), nil
}

// parseTestIDs reads collect-only output: one id per line up to the blank line
// that precedes the summary.
//
// Leading blank lines are skipped rather than treated as the terminator — the
// terminator is the blank line AFTER the ids, and a stray one before them would
// otherwise turn a populated suite into an empty list. Lines without "::" are
// dropped so a summary reached by any other route can never become a selector.
func parseTestIDs(out string) []string {
	var ids []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			if len(ids) > 0 {
				break
			}
			continue
		}
		if !strings.Contains(line, "::") {
			continue
		}
		ids = append(ids, line)
	}
	return ids
}
