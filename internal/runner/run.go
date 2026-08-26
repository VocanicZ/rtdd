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
		vars := map[string]string{"log": logPath, "out": tmpDir}

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

		outs, err := report.ReadReportLog(logPath)
		if err != nil {
			return nil, fmt.Errorf("runner: chunk %d: %w", i, err)
		}
		for _, o := range outs {
			if j, seen := byTest[o.Test]; seen {
				res.Outcomes[j] = o // last invocation wins
				continue
			}
			byTest[o.Test] = len(res.Outcomes)
			res.Outcomes = append(res.Outcomes, o)
		}

		// Read and merge NOW, before the next chunk runs: pytest erases .coverage
		// at the start of every invocation, so chunk i's contexts exist nowhere
		// else once chunk i+1 starts. --cov-append is not the alternative — it
		// would also absorb a stale store from an unrelated earlier run.
		cov, err := coverage.ReadSQLite(covPath, repoRoot)
		if err != nil {
			return nil, fmt.Errorf("runner: chunk %d: %w", i, err)
		}
		res.Coverage.Merge(cov)
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
