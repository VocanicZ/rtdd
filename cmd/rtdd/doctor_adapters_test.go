package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// A host adapter for a toolchain no built-in serves, declaring a prerequisite that is not
// installed anywhere.
const hostRequiresYAML = `name: vitest
detect: ["vitest.config.ts"]
unit_cmd: "npx vitest run {unit}"
coverage_file: "{tmp}/lcov.info"
coverage_format: lcov
test_globs: ["**/*.test.ts"]
source_globs: ["src/**/*.ts"]
requires:
  - bin: rtdd-no-such-binary
    reason: "resolves the vitest binary from the lockfile"
`

func TestRenderAdaptersNamesSourceOriginAndUnmetRequires(t *testing.T) {
	got := RenderAdapters([]AdapterRow{
		{Name: "python", Src: "python.yaml"},
		{Name: "vitest", Src: ".rtdd/adapters/vitest.yaml", Host: true, Override: true,
			Unmet: []adapter.Requirement{{Bin: "npx", Reason: "runs vitest"}}},
	}, []adapter.Invalid{{Path: ".rtdd/adapters/bad.yaml", Err: errors.New("unit_cmd is required")}})
	for _, want := range []string{
		"adapters",
		"python", "python.yaml", "(built-in)",
		".rtdd/adapters/vitest.yaml", "(host-authored, overrides built-in)",
		"npx is not on PATH: runs vitest",
		"not loaded: .rtdd/adapters/bad.yaml", "unit_cmd is required",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

func TestRenderAdaptersWithNoAdaptersSaysHowToFixIt(t *testing.T) {
	got := RenderAdapters(nil, nil)
	for _, want := range []string{"none detected", "Fix:"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

// Spec §4.3: an unmet prerequisite of a DETECTED adapter is named in its row, and doctor
// still exits 0.
func TestDoctorReportsAnUnmetPrerequisiteAndStillExitsZero(t *testing.T) {
	dir := newHostAdapterRepo(t)
	gittest.Write(t, dir, "vitest.config.ts", "export default {}\n")
	writeHostAdapter(t, dir, "vitest.yaml", hostRequiresYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"rtdd-no-such-binary is not on PATH", "resolves the vitest binary from the lockfile"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor output does not contain %q:\n%s", want, stdout)
		}
	}
}

// Prerequisites are the detected adapters' prerequisites only.
func TestDoctorReportsPrerequisitesOnlyForDetectedAdapters(t *testing.T) {
	dir := newHostAdapterRepo(t)
	writeHostAdapter(t, dir, "vitest.yaml", hostRequiresYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "rtdd-no-such-binary") {
		t.Errorf("doctor demanded a binary only an undetected adapter needs:\n%s", stdout)
	}
	// The undetected adapter is still named, with the markers that would have matched.
	i := strings.Index(stdout, undetectedHeading)
	if i < 0 || !strings.Contains(stdout[i:], "vitest.config.ts") {
		t.Errorf("the undetected section does not name vitest's markers:\n%s", stdout)
	}
}

func TestRenderUndetectedIsSilentWhenEverythingIsDetected(t *testing.T) {
	if got := RenderUndetected(nil); got != "" {
		t.Errorf("RenderUndetected(nil) = %q, want the empty string", got)
	}
}

// An adapter with no detect: globs can never be detected; saying "no file matches its
// markers" about an empty list would be a riddle.
func TestRenderUndetectedExplainsAnAdapterWithNoMarkers(t *testing.T) {
	got := RenderUndetected([]AdapterRow{{Name: "nomarkers", Src: ".rtdd/adapters/nomarkers.yaml", Host: true}})
	if !strings.Contains(got, "no detect: markers") {
		t.Errorf("output does not explain the empty marker list:\n%s", got)
	}
}
