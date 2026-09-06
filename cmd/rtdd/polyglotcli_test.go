package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// polyglotRepo is spec §4.4's ordinary repository: a Python package beside a TypeScript
// service. Detection resolves both since #309, so every command must answer twice. Vitest
// arrives as a host adapter (spec §4.5), which is how a toolchain the binary has not
// shipped yet is supported at all.
func polyglotRepo(t *testing.T) string {
	t.Helper()
	dir := newPolyglotRepo(t)
	writeVitestAdapter(t, dir, "")
	return dir
}

// PRD #232 AC6 on the human surface: the reader must be able to tell which toolchain
// produced which ids. Two lists under one heading is one list with a blank line in it,
// and an agent that hands the wrong half to the wrong runner gets a green report from a
// suite that ran nothing.
func TestWhichNamesEachAdapterInAPolyglotRepository(t *testing.T) {
	dir := polyglotRepo(t)
	writeFile(t, dir, "src/logic.py", "def add(a, b):\n    return a + b + 0\n")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"adapter: python", "adapter: vitest"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("which output does not name %q:\n%s", want, stdout)
		}
	}
}

// The --json document is the whole of what an agent front-end reads, so the per-adapter
// split has to survive into it. A single merged `selection.tests` would be a list of ids
// from two runners with nothing saying where each came from.
func TestWhichJSONCarriesOneSelectionBlockPerAdapter(t *testing.T) {
	dir := polyglotRepo(t)
	writeFile(t, dir, "src/logic.py", "def add(a, b):\n    return a + b + 0\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if len(got.Selections) != 2 {
		t.Fatalf("selections = %+v, want one block per detected adapter", got.Selections)
	}
	names := []string{got.Selections[0].Adapter, got.Selections[1].Adapter}
	if names[0] != "python" || names[1] != "vitest" {
		t.Errorf("selections name %v, want [python vitest]", names)
	}
	if !strings.Contains(got.Adapter, "python") || !strings.Contains(got.Adapter, "vitest") {
		t.Errorf("adapter = %q, want both detected adapters named", got.Adapter)
	}
}

// A single-adapter repository's document is unchanged: `selections` is omitted entirely
// rather than emitted as a one-element array, so no existing consumer sees a new key.
func TestWhichJSONOmitsSelectionsForASingleAdapter(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if _, present := rawObject(t, stdout)["selections"]; present {
		t.Errorf("a single-adapter document carries a selections key:\n%s", stdout)
	}
	var out Output
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unparseable: %v", err)
	}
	if out.Adapter != "python" {
		t.Errorf("adapter = %q, want python", out.Adapter)
	}
}

// `rtdd run` reports which adapter produced each selection for the same reason `which`
// does: it is the command an agent actually calls, and its tier line is the only place
// the split is visible before the tests run.
func TestRunNamesEachAdapterInAPolyglotRepository(t *testing.T) {
	dir := polyglotRepo(t)
	writeFile(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b + 0;\n")
	chdir(t, dir)

	// The exit code is not the subject here — neither toolchain is installed in the
	// fixture — and the tier lines are printed before either runner is invoked.
	out := captureStdout(t, func() { cmdRun(nil) })

	for _, want := range []string{"adapter: python", "adapter: vitest"} {
		if !strings.Contains(out, want) {
			t.Errorf("run output does not name %q:\n%s", want, out)
		}
	}
}
