package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixLookPath replaces the process-wide prerequisite resolver for one test. Injection is
// the point: a test that asked the machine whether `npx` exists would pass on a laptop
// with node installed and fail on a minimal CI image, which is the opposite of a guard.
func fixLookPath(t *testing.T, present ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, p := range present {
		set[p] = true
	}
	prev := lookPath
	lookPath = func(bin string) (string, error) {
		if set[bin] {
			return "/usr/bin/" + bin, nil
		}
		return "", errors.New("executable file not found in $PATH")
	}
	t.Cleanup(func() { lookPath = prev })
}

// hostAdapterYAML is a host-authored vitest adapter with the given extra keys appended.
func writeVitestAdapter(t *testing.T, dir, extra string) {
	t.Helper()
	adir := filepath.Join(dir, ".rtdd", "adapters")
	if err := os.MkdirAll(adir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := "name: vitest\ndetect: [\"package.json\"]\nsubset: \"npx vitest run {tests}\"\n" +
		"selection: static\ncoverage: none\nreport: junit-xml\nreport_path: \".rtdd/junit.xml\"\n" +
		"id_template: \"{file}::{name}\"\ntest_for: [\"{dir}/{name}.test.ts\"]\n" +
		"test_globs: [\"**/*.test.ts\"]\nsource_globs: [\"src/**/*.ts\"]\n" + extra
	if err := os.WriteFile(filepath.Join(adir, "vitest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// Spec §4.3 / PRD #229 AC9: an unmet prerequisite surfaces at doctor AND init time, never
// as a mid-run parse failure against a report file that was never written. `init` is the
// command everyone runs, so it is the one that has to say so.
//
// It does NOT refuse: the repo has an adapter, so the §5 gate is satisfied, and a binary
// missing from this machine is no reason to decline to install — CI may install it later.
func TestInitReportsAnUnmetPrerequisiteAndStillExitsZero(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "requires:\n  - bin: npx\n    reason: \"vitest emits no JUnit XML without it\"\n")
	fixLookPath(t /* nothing installed */)

	code, _, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, want 0 — an unmet prerequisite is not a refusal (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"npx", "vitest", "vitest emits no JUnit XML without it"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not name %q:\n%s", want, stderr)
		}
	}
	// Still a real install: the report is a warning beside the work, not instead of it.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "rtdd", "SKILL.md")); err != nil {
		t.Errorf("init reported the prerequisite and then installed nothing: %v", err)
	}
}

// Nothing missing prints nothing: a prerequisites heading over an empty list reads as a
// problem the reader then goes looking for.
func TestInitIsSilentWhenEveryPrerequisiteIsMet(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "requires:\n  - bin: npx\n    reason: \"runs vitest\"\n")
	fixLookPath(t, "npx")

	code, _, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stderr, "prerequisite") || strings.Contains(stderr, "npx") {
		t.Errorf("stderr mentions a prerequisite that is installed:\n%s", stderr)
	}
}

// Scoped to the DETECTED adapters, matching #249. The built-in python adapter resolves in
// every repo; naming its prerequisites in a TypeScript repo sends an agent to install
// something nothing here runs.
func TestInitReportsNoPrerequisiteOfAnUndetectedAdapter(t *testing.T) {
	dir := newDetectableRepo(t)
	fixLookPath(t /* nothing installed */)

	code, _, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stderr, "vitest") {
		t.Errorf("stderr names an adapter this repository does not use:\n%s", stderr)
	}
}
