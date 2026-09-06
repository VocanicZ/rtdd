package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
)

// findRepoRoot walks up from start looking for a .git entry.
//
// It tests for the entry's existence, not for it being a directory: in a git
// worktree — and in a submodule — .git is a FILE holding a `gitdir:` pointer, and
// a directory-only check reports "not inside a git repository" for every one of
// them.
func findRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", start, err)
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a git repository (searched upward from %s)", start)
		}
		dir = parent
	}
}

// detectAdapters returns every adapter that matches repoRoot, from the built-in set
// overlaid with the repo's own .rtdd/adapters/*.yaml. Zero matches is a configuration
// error (exit 2): rtdd has no toolchain to run. Two or more is ordinary since spec §4.4
// — a polyglot repository is served by both.
//
// A host adapter that failed to load is reported on warn and then skipped, never fatal:
// a repo where someone is halfway through authoring one still runs on the adapters that
// are valid. Skipping it silently would let a typo in a host override read as the
// built-in simply winning, so the file and the failing field are always named.
func detectAdapters(repoRoot string, warn io.Writer) ([]*adapter.Adapter, error) {
	all, invalid, err := adapter.AvailableReport(repoRoot)
	if err != nil {
		return nil, err
	}
	warnInvalidAdapters(warn, repoRoot, invalid)
	return adapter.Detect(repoRoot, all)
}

// warnInvalidAdapters prints one line per host adapter that did not load.
func warnInvalidAdapters(warn io.Writer, repoRoot string, invalid []adapter.Invalid) {
	if warn == nil {
		return
	}
	for _, bad := range invalid {
		fmt.Fprintf(warn, "rtdd: ignoring %s: %v\n", relToRoot(repoRoot, bad.Path), bad.Err)
	}
}

// relToRoot renders a path the way someone standing in the repo would type it.
func relToRoot(repoRoot, p string) string {
	rel, err := filepath.Rel(repoRoot, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}

// rowsFrom joins a run's coverage and its test report into map rows, one row per
// OUTCOME, each tagged with the adapter that produced it (PRD #232 AC6).
//
// The tag is what stops a row reaching a runner that cannot execute its id: a polyglot
// repository holds a pytest nodeid and a vitest file path in one map.jsonl, and
// Map.TestsCoveringFor serves each only to the adapter named here. The outcome list is the spine: `s` and `d` exist in no coverage report,
// and a test that ran without recording a single measurable line still needs its
// status refreshed.
//
// `f` keeps EVERY repo-relative path coverage reported for the test, including the
// test's own module and helpers under tests/. It is deliberately NOT filtered
// through adapter.IsInstrumentable: filtering would drop tests/helpers.py from
// every row, so editing a shared helper would select nothing. Over-selection is
// safe for selection; under-selection is not (spec §4, Task 17). Out-of-repo paths
// — site-packages, the stdlib — were already dropped by coverage.ReadSQLite.
//
// Import-time lines belong to no test and are not in PerTest, so they cannot leak
// into any `f`.
func rowsFrom(res *runner.RunResult, sha, adapterName string) []mapstore.Row {
	byTest := map[string][]string{}
	if res.Coverage != nil {
		for _, tc := range res.Coverage.PerTest {
			files := make([]string, 0, len(tc.Files))
			for f := range tc.Files {
				files = append(files, f)
			}
			sort.Strings(files)
			byTest[tc.Test] = files
		}
	}
	rows := make([]mapstore.Row, 0, len(res.Outcomes))
	for _, o := range res.Outcomes {
		rows = append(rows, mapstore.Row{
			T: o.Test,
			F: byTest[o.Test],
			C: sha,
			D: o.DurationMS,
			S: o.Status,
			A: adapterName,
		})
	}
	return rows
}

// reportRunErr prints a runner error and returns the process exit code for it.
// It returns 0 for a nil error so callers can write `if code := reportRunErr(err);
// code != 0` without a second nil check.
//
// The split is the one frozen in docs/plans/00-interfaces.md: a mapped exit code
// (4 bad-selector, 5 no-tests-collected) says rtdd's own configuration or map is
// wrong, which is exit 2; everything else that reaches here — the sysmon warning
// included — says the environment cannot produce a trustworthy map, which is
// exit 3. Neither is exit 1: a test did not fail.
func reportRunErr(err error) int {
	code, hints := runErrClass(err)
	if err == nil {
		return code
	}
	fmt.Fprintln(os.Stderr, "rtdd:", err)
	for _, h := range hints {
		fmt.Fprintln(os.Stderr, "rtdd:", h)
	}
	return code
}

// runErrClass is reportRunErr without the printing: the exit code a runner error maps to,
// and the operator hints that explain it.
//
// The split exists for the polyglot loop, which must not print as it goes — one adapter's
// failure is rendered in the block that names the adapter, beside the adapters that ran
// (PRD #232 AC7), and a line emitted mid-loop would arrive detached from both.
func runErrClass(err error) (int, []string) {
	if err == nil {
		return 0, nil
	}
	if errors.Is(err, runner.ErrSysmonContext) {
		return 3, []string{
			"coverage.py dropped dynamic contexts; the map would be ~90% empty on a run that exits 0.",
			"the python adapter forces COVERAGE_CORE=ctrace - check for a wrapper script or CI setting that overrides it.",
		}
	}
	var fe *runner.FatalExitError
	if errors.As(err, &fe) {
		if fe.Code == 4 {
			return 2, []string{"the test ids rtdd produced were rejected by the runner; the map may be stale. Try: rtdd seed"}
		}
		return 2, nil
	}
	return 3, nil
}
