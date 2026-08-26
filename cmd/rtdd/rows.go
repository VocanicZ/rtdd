package main

import (
	"errors"
	"fmt"
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

// detectAdapter picks the one adapter that matches repoRoot, from the set embedded
// in the binary. Zero matches and more than one match are both configuration
// errors (exit 2): rtdd never guesses which language it is looking at.
func detectAdapter(repoRoot string) (*adapter.Adapter, error) {
	all, err := adapter.Builtin()
	if err != nil {
		return nil, err
	}
	return adapter.Detect(repoRoot, all)
}

// rowsFrom joins a run's coverage and its test report into map rows, one row per
// OUTCOME. The outcome list is the spine: `s` and `d` exist in no coverage report,
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
func rowsFrom(res *runner.RunResult, sha string) []mapstore.Row {
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
	if err == nil {
		return 0
	}
	if errors.Is(err, runner.ErrSysmonContext) {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		fmt.Fprintln(os.Stderr, "rtdd: coverage.py dropped dynamic contexts; the map would be ~90% empty on a run that exits 0.")
		fmt.Fprintln(os.Stderr, "rtdd: the python adapter forces COVERAGE_CORE=ctrace - check for a wrapper script or CI setting that overrides it.")
		return 3
	}
	var fe *runner.FatalExitError
	if errors.As(err, &fe) {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		if fe.Code == 4 {
			fmt.Fprintln(os.Stderr, "rtdd: the test ids rtdd produced were rejected by the runner; the map may be stale. Try: rtdd seed")
		}
		return 2
	}
	fmt.Fprintln(os.Stderr, "rtdd:", err)
	return 3
}
