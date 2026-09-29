package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/runner"
)

// Decision 5's table, asserted. Collapsing 3 or 2 into 1 tells an agent a test failed
// when in fact nothing ran, which is the most expensive possible way to be wrong.
func TestFoldExitCodesTakesTheWorstAcrossAdapters(t *testing.T) {
	cases := []struct {
		name  string
		codes []int
		want  int
	}{
		{"all green", []int{0, 0}, 0},
		{"one test failed", []int{0, 1}, 1},
		{"a config error outranks a test failure", []int{1, 2}, 2},
		{"a fatal environment error outranks everything", []int{1, 2, 3}, 3},
		{"order does not matter", []int{3, 0}, 3},
		{"nothing ran", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runs := make([]AdapterRun, 0, len(tc.codes))
			for i, c := range tc.codes {
				runs = append(runs, AdapterRun{Adapter: string(rune('a' + i)), Code: c})
			}
			if got := FoldExitCodes(runs); got != tc.want {
				t.Errorf("FoldExitCodes(%v) = %d, want %d", tc.codes, got, tc.want)
			}
		})
	}
}

// A code outside the frozen 0-3 table is still a failure, and it is folded as the worst
// one: `rtdd run` may only ever exit 0, 1, 2 or 3 (00-interfaces.md), so passing an
// unmapped runner code through would invent a fifth code, and treating it as 0 would
// report a broken toolchain as a pass.
func TestFoldExitCodesClampsACodeOutsideTheTable(t *testing.T) {
	runs := []AdapterRun{{Adapter: "maven", Code: 137}, {Adapter: "vitest", Code: 0}}
	if got := FoldExitCodes(runs); got != 3 {
		t.Errorf("FoldExitCodes = %d, want 3 for an out-of-table code", got)
	}
}

// PRD #232 AC7: the failure is reported PER ADAPTER, and the other adapter still ran.
func TestOneAdaptersFailureDoesNotVoidAnothersRun(t *testing.T) {
	runs := []AdapterRun{
		{Adapter: "maven", Err: errors.New("mvn: command not found"), Code: 3},
		{Adapter: "vitest", Result: &runner.RunResult{ExitCode: 0}, Code: 0},
	}
	out := renderAdapterRuns(runs)
	for _, want := range []string{"maven", "mvn: command not found", "vitest"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered runs %q do not name %q; AC7 requires the failure reported per adapter", out, want)
		}
	}
	if got := FoldExitCodes(runs); got != 3 {
		t.Errorf("FoldExitCodes = %d, want 3: one adapter's environment is broken", got)
	}
}

// The rendered block says what each adapter's own run produced, so a reader never has to
// infer which toolchain the failing test belongs to from a flat list of ids.
func TestRenderAdapterRunsNamesEachAdaptersOwnOutcome(t *testing.T) {
	runs := []AdapterRun{
		{Adapter: "python", Result: &runner.RunResult{
			Outcomes: []runner.Outcome{{Test: "tests/test_calc.py::test_add", Status: "fail"}},
			Failed:   []string{"tests/test_calc.py::test_add"},
			ExitCode: 1,
		}, Code: 1},
		{Adapter: "vitest", Result: &runner.RunResult{
			Outcomes: []runner.Outcome{{Test: "src/calc.test.ts", Status: "pass"}},
		}, Code: 0},
	}
	out := renderAdapterRuns(runs)
	for _, want := range []string{"python", "tests/test_calc.py::test_add", "vitest"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered runs %q do not name %q", out, want)
		}
	}
}

// captureStderr is captureStdout for the other stream: the per-adapter execution report
// is written to stderr in both output modes, because under --json stdout is one document.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	func() {
		defer func() {
			os.Stderr = saved
			_ = w.Close()
		}()
		f()
	}()
	out := <-done
	_ = r.Close()
	return out
}

// brokenVitestAdapter is a host adapter whose runner is not installed — the ordinary
// polyglot failure, where one half of the repository has its toolchain and the other does
// not. The binary name cannot resolve on any machine, so the failure is the same
// everywhere rather than dependent on whether node happens to be present.
func brokenVitestAdapter(t *testing.T, dir string) {
	t.Helper()
	adir := filepath.Join(dir, ".rtdd", "adapters")
	if err := os.MkdirAll(adir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := "name: vitest\ndetect: [\"package.json\"]\n" +
		"unit_cmd: \"rtdd-no-such-runner-binary {unit}\"\n" +
		"coverage_file: \"{tmp}/lcov.info\"\ncoverage_format: lcov\n" +
		"test_globs: [\"**/*.test.ts\"]\nsource_globs: [\"src/**/*.ts\"]\n"
	if err := os.WriteFile(filepath.Join(adir, "vitest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// PRD #232 AC7, end to end against real pytest: a repository whose TypeScript half has no
// runner installed still runs its Python half, keeps what that produced, and exits the
// worst code any adapter produced.
//
// The defect this closes is the one the TODO in run.go named: the first adapter error
// returned from the loop, so a missing `npx` threw away a completed pytest run — results
// the user had already paid for — and reported an environment failure as the whole story.
func TestRunKeepsOneAdaptersResultsWhenAnotherAdapterCannotStart(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	makeSuiteGreen(t, repo)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 0 {
		t.Fatalf("cmdSeed = %d, want 0 once the suite is green", code)
	}

	// The TypeScript half arrives as a host adapter (spec §4.5), with a source file and
	// its corresponding test so the TS tier has something to select.
	brokenVitestAdapter(t, repo)
	writeRepoFile(t, repo, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	writeRepoFile(t, repo, "src/logic.ts", "export const add = (a: number, b: number) => a + b;\n")
	writeRepoFile(t, repo, "src/logic.test.ts", "it('adds', () => {});\n")
	touchLogic(t, repo)

	var (
		code   int
		stdout string
	)
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() { code = cmdRun(nil) })
	})

	if code != 3 {
		t.Fatalf("cmdRun = %d, want 3: vitest's runner is not installed\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "vitest") {
		t.Errorf("stderr does not name the adapter that failed:\n%s", stderr)
	}
	// The load-bearing half: python's run happened and was kept. A count of 0 means the
	// broken adapter discarded it, which is exactly what AC7 forbids.
	if strings.Contains(stdout, "0 ran,") || !strings.Contains(stdout, " ran,") {
		t.Errorf("python's completed run was discarded by vitest's failure:\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stderr, "python") {
		t.Errorf("the per-adapter report does not name the adapter that DID run:\n%s", stderr)
	}
}

// writeRepoFile writes a file into an already-initialised fixture repository, leaving it
// untracked — which is what makes it a changed file for the selection under test.
func writeRepoFile(t *testing.T, repo, rel, body string) {
	t.Helper()
	p := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// brokenEnumerationAdapter is brokenVitestAdapter with a `full_escalate` entry: changing
// package.json forces T2, which is the one tier that enumerates, and enumerating is what
// this adapter cannot do because its runner binary does not resolve.
func brokenEnumerationAdapter(t *testing.T, dir string) {
	t.Helper()
	adir := filepath.Join(dir, ".rtdd", "adapters")
	if err := os.MkdirAll(adir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := "name: vitest\ndetect: [\"package.json\"]\n" +
		"unit_cmd: \"rtdd-no-such-runner-binary {unit}\"\n" +
		"coverage_file: \"{tmp}/lcov.info\"\ncoverage_format: lcov\n" +
		"test_globs: [\"**/*.test.ts\"]\nsource_globs: [\"src/**/*.ts\"]\n" +
		"full_escalate: [\"package.json\"]\n"
	if err := os.WriteFile(filepath.Join(adir, "vitest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// brokenEnumerationRepo is the single-adapter repository in which the AC7 gap reproduces:
// the only adapter's enumeration fails, and because a T2 block whose enumeration returned
// no list has nothing to run, the folded selection is EMPTY. The map is empty too, so no
// stale row rescues the selection.
func brokenEnumerationRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Write(t, dir, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b;\n")
	gittest.Write(t, dir, "src/logic.test.ts", "it('adds', () => {});\n")
	gittest.Commit(t, dir, "init")
	brokenEnumerationAdapter(t, dir)
	// The change that escalates to T2. It is the LAST thing written, so the run under test
	// sees exactly one changed file and that file is the escalating one.
	gittest.Write(t, dir, "package.json", "{\n  \"name\": \"demo\",\n  \"version\": \"0.0.2\"\n}\n")
	return dir
}

// PRD #232 AC7 on the path that produced no results at all. An adapter whose enumeration
// failed is the REASON the selection is empty, so exiting 0 with "nothing ran" reports a
// broken toolchain as a repository with nothing to do — indistinguishable, to an agent,
// from a green loop iteration.
//
// This drives cmdRun, not selectPerAdapter: the defect lived in the early return that
// cmdRun takes before the loop that reports EnumErr ever runs.
func TestCmdRunReportsAnEnumerationFailureWhenTheSelectionIsEmpty(t *testing.T) {
	repo := brokenEnumerationRepo(t)
	chdir(t, repo)

	var (
		code   int
		stdout string
	)
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() { code = cmdRun(nil) })
	})

	if code == 0 {
		t.Fatalf("cmdRun = 0 on an empty selection caused by a failed enumeration; "+
			"the failure must move the exit code\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if code != 3 {
		t.Errorf("cmdRun = %d, want 3: the adapter's runner is not installed\nstdout:\n%s\nstderr:\n%s",
			code, stdout, stderr)
	}
	if !strings.Contains(stderr, "vitest") {
		t.Errorf("the per-adapter report does not name the adapter that failed:\n%s", stderr)
	}
	if !strings.Contains(stderr, "rtdd-no-such-runner-binary") {
		t.Errorf("the enumeration's own error text is discarded:\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
}

// The same failure on the --json path. A consumer discards stderr, so an error that
// reaches only stderr is one the agent front-end never learns about: it sees a generic
// empty-selection warning next to exit 0 and cannot tell it from a quiet repository.
func TestCmdRunJSONReportsAnEnumerationFailureWhenTheSelectionIsEmpty(t *testing.T) {
	repo := brokenEnumerationRepo(t)
	chdir(t, repo)

	var (
		code int
		raw  string
	)
	_ = captureStderr(t, func() {
		raw = captureStdout(t, func() { code = cmdRun([]string{"--json"}) })
	})

	if code == 0 {
		t.Fatalf("cmdRun --json = 0 on an empty selection caused by a failed enumeration\n%s", raw)
	}
	var got Output
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("--json stdout is not a single JSON document: %v\n%s", err, raw)
	}
	if got.ExitCode != code {
		t.Errorf("document exit_code = %d, process exit = %d; the two must agree\n%s", got.ExitCode, code, raw)
	}
	if !anyWarningContains(got.Warnings, "rtdd-no-such-runner-binary") {
		t.Errorf("warnings do not carry the enumeration's error text, got %#v\n%s", got.Warnings, raw)
	}
	if !anyWarningContains(got.Warnings, "vitest") {
		t.Errorf("warnings do not name the adapter that failed, got %#v", got.Warnings)
	}
}

// PRD #232 AC7 end to end, one stage earlier than
// TestRunKeepsOneAdaptersResultsWhenAnotherAdapterCannotStart: the TypeScript half's
// ENUMERATION is what cannot start, and the Python half's completed run is still executed,
// kept and reported beside the failure.
func TestRunKeepsOneAdaptersResultsWhenAnotherAdaptersEnumerationCannotStart(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	makeSuiteGreen(t, repo)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 0 {
		t.Fatalf("cmdSeed = %d, want 0 once the suite is green", code)
	}

	brokenEnumerationAdapter(t, repo)
	// package.json is the TypeScript adapter's full_escalate file, so it escalates to T2 —
	// the one tier that enumerates — and the enumeration is what fails.
	writeRepoFile(t, repo, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	writeRepoFile(t, repo, "src/logic.ts", "export const add = (a: number, b: number) => a + b;\n")
	writeRepoFile(t, repo, "src/logic.test.ts", "it('adds', () => {});\n")
	touchLogic(t, repo)

	var (
		code   int
		stdout string
	)
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() { code = cmdRun(nil) })
	})

	if code != 3 {
		t.Fatalf("cmdRun = %d, want 3: vitest's runner is not installed\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "vitest") || !strings.Contains(stderr, "rtdd-no-such-runner-binary") {
		t.Errorf("the enumeration failure is not reported per adapter:\n%s", stderr)
	}
	if strings.Contains(stdout, "0 ran,") || !strings.Contains(stdout, " ran,") {
		t.Errorf("python's completed run was discarded by vitest's enumeration failure:\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stderr, "python") {
		t.Errorf("the per-adapter report does not name the adapter that DID run:\n%s", stderr)
	}
}
