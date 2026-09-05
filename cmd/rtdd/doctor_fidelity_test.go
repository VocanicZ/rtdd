package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/doctor"
)

func TestRenderFidelityExecutionDerived(t *testing.T) {
	got := RenderFidelity([]FidelityRow{{
		Name: "python", Src: "python.yaml", Fidelity: adapter.FidelityExecution,
		Why: "selection: coverage with coverage: sqlite — per-test coverage recorded from a real run",
	}}, nil)

	for _, want := range []string{"selection fidelity", "python", "built-in", "execution-derived"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

// The decision the M6a plan pins: fidelity none is stated plainly, with the fix, and it is
// not dressed up as a static selection.
func TestRenderFidelityNoneSaysWhatIsMissingAndHowToFixIt(t *testing.T) {
	got := RenderFidelity([]FidelityRow{{
		Name: "elixir", Src: ".rtdd/adapters/elixir.yaml", Host: true, Fidelity: adapter.FidelityNone,
		Why: "declares selection: static but no test_for templates and no importscan command",
	}}, nil)

	for _, want := range []string{
		"elixir",
		".rtdd/adapters/elixir.yaml",
		"none",
		"no test_for templates and no importscan command",
		"full suite",
		"Fix:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "execution-derived selection would have caught") {
		// The caveat mentions the phrase legitimately; strip it before asserting the row
		// itself never claims execution-derived fidelity.
		got = strings.ReplaceAll(got, doctor.StaticCaveat, "")
	}
	if strings.Contains(got, "execution-derived") {
		t.Errorf("a fidelity: none row must not mention execution-derived:\n%s", got)
	}
}

func TestRenderFidelityMarksAHostOverride(t *testing.T) {
	got := RenderFidelity([]FidelityRow{{
		Name: "python", Src: ".rtdd/adapters/python.yaml", Host: true, Override: true,
		Fidelity: adapter.FidelityExecution, Why: "per-test coverage recorded from a real run",
	}}, nil)
	for _, want := range []string{"python", ".rtdd/adapters/python.yaml", "host-authored", "overrides built-in"} {
		if !strings.Contains(got, want) {
			t.Errorf("a host override must be named, never silent; missing %q:\n%s", want, got)
		}
	}
}

// A built-in that nothing overrode says so, and never claims to be host-authored.
func TestRenderFidelityMarksABuiltin(t *testing.T) {
	got := RenderFidelity([]FidelityRow{{
		Name: "python", Src: "python.yaml", Fidelity: adapter.FidelityExecution, Why: "recorded coverage",
	}}, nil)
	if !strings.Contains(got, "built-in") {
		t.Errorf("output does not name the built-in source:\n%s", got)
	}
	if strings.Contains(got, "host-authored") {
		t.Errorf("a built-in row must not claim to be host-authored:\n%s", got)
	}
	if strings.Contains(got, "overrides built-in") {
		t.Errorf("a built-in row must not claim to override anything:\n%s", got)
	}
}

// §4.5: doctor validates host adapters and names the failing field. It is the ONE lenient
// reader — a broken file is reported, not fatal, because doctor is the command you run to
// find out what is wrong.
func TestRenderFidelityNamesAnInvalidHostAdapter(t *testing.T) {
	got := RenderFidelity([]FidelityRow{{
		Name: "python", Src: "python.yaml", Fidelity: adapter.FidelityExecution, Why: "recorded coverage",
	}}, []adapter.Invalid{{
		Path: ".rtdd/adapters/broken.yaml",
		Err:  errString(`adapter: .rtdd/adapters/broken.yaml: subset "go test ./..." has no {tests} placeholder`),
	}})

	for _, want := range []string{"broken.yaml", "{tests}", "not loaded"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "python") {
		t.Errorf("a broken host adapter hid the adapters that did load:\n%s", got)
	}
}

// A repo with no adapter at all is the state --force leaves behind. It is still `none`,
// and spec §6 says the field is never null.
func TestRenderFidelityWithNoAdapters(t *testing.T) {
	got := RenderFidelity(nil, nil)
	if !strings.Contains(got, "none") {
		t.Errorf("no-adapter output must still state a fidelity:\n%s", got)
	}
}

// Every row carries a fidelity and a reason. A verdict with no cause attached is not an
// honesty surface: an agent cannot calibrate on it (PRD #240 AC2, AC7).
func TestRenderFidelityAlwaysPrintsAFidelityAndAReason(t *testing.T) {
	rows := []FidelityRow{
		{Name: "python", Src: "python.yaml", Fidelity: adapter.FidelityExecution, Why: "recorded per-test coverage"},
		{Name: "vitest", Src: ".rtdd/adapters/vitest.yaml", Host: true, Fidelity: adapter.FidelityStatic,
			Why: "declares selection: static with 1 test_for template"},
		{Name: "elixir", Src: ".rtdd/adapters/elixir.yaml", Host: true, Fidelity: adapter.FidelityNone,
			Why: "declares selection: static but no test_for templates and no importscan command"},
	}
	got := RenderFidelity(rows, nil)
	for _, r := range rows {
		if !strings.Contains(got, string(r.Fidelity)) {
			t.Errorf("row %s has no fidelity value:\n%s", r.Name, got)
		}
		if !strings.Contains(got, r.Why) {
			t.Errorf("row %s has no reason:\n%s", r.Name, got)
		}
	}
}

// PRD #240: a static or none row is weaker evidence than an execution-derived one, and
// saying so is the point of the surface.
func TestRenderFidelityCaveatsEveryStaticOrNoneRow(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []FidelityRow
		want bool
	}{
		{
			name: "static",
			rows: []FidelityRow{{Name: "vitest", Src: "v.yaml", Fidelity: adapter.FidelityStatic, Why: "test_for templates"}},
			want: true,
		},
		{
			name: "none",
			rows: []FidelityRow{{Name: "elixir", Src: "e.yaml", Fidelity: adapter.FidelityNone, Why: "nothing to select with"}},
			want: true,
		},
		{
			name: "no adapters at all",
			rows: nil,
			want: true,
		},
		{
			name: "execution-derived only",
			rows: []FidelityRow{{Name: "python", Src: "python.yaml", Fidelity: adapter.FidelityExecution, Why: "recorded coverage"}},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderFidelity(tc.rows, nil)
			if has := strings.Contains(got, doctor.StaticCaveat); has != tc.want {
				t.Errorf("caveat present = %v, want %v:\n%s", has, tc.want, got)
			}
		})
	}
}

// The caveat states both halves spec §6 asks for: what a static selection can miss, and
// what a passing static selection is worth.
func TestStaticCaveatStatesTheMissAndTheWeakerEvidence(t *testing.T) {
	for _, want := range []string{"static", "miss", "execution-derived", "weaker evidence"} {
		if !strings.Contains(doctor.StaticCaveat, want) {
			t.Errorf("StaticCaveat does not contain %q:\n%s", want, doctor.StaticCaveat)
		}
	}
}

// A host adapter that CAN select statically: correspondence templates, nothing recorded.
const hostStaticVitestYAML = `name: vitest
detect: ["vitest.config.ts"]
subset: "npx vitest run {tests}"
list: "npx vitest list"
selection: static
coverage: none
report: pytest-reportlog
test_for: ["{dir}/{name}.test.ts"]
test_globs: ["**/*.test.ts"]
source_globs: ["src/**/*.ts"]
`

// A host adapter that can select nothing: static, but neither a template nor a scanner.
const hostNothingElixirYAML = `name: elixir
detect: ["mix.exs"]
subset: "mix test {tests}"
selection: static
coverage: none
report: pytest-reportlog
test_globs: ["test/**/*_test.exs"]
source_globs: ["lib/**/*.ex"]
`

// A host adapter declaring a prerequisite that is not installed anywhere.
const hostRequiresYAML = `name: vitest
detect: ["vitest.config.ts"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: pytest-reportlog
test_for: ["{dir}/{name}.test.ts"]
requires:
  - bin: rtdd-no-such-binary
    reason: "resolves the vitest binary from the lockfile"
`

// Spec §6 / PRD #240: doctor states the fidelity this repository can actually achieve for
// every adapter it resolved, host-authored ones included, with the reason attached.
func TestDoctorReportsStaticFidelityWithItsReasonAndCaveat(t *testing.T) {
	dir := newHostAdapterRepo(t)
	writeHostAdapter(t, dir, "vitest.yaml", hostStaticVitestYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{
		"selection fidelity",
		"vitest", ".rtdd/adapters/vitest.yaml", "host-authored",
		"static",
		"test_for",
		"python", "execution-derived",
		doctor.StaticCaveat,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor output does not contain %q:\n%s", want, stdout)
		}
	}
}

// An adapter that can record nothing and select nothing is reported as `none`, with the
// consequence and the fix — never as a bare verdict, and never dressed up as `static`.
func TestDoctorReportsNoneWithTheConsequenceAndTheFix(t *testing.T) {
	dir := newHostAdapterRepo(t)
	writeHostAdapter(t, dir, "elixir.yaml", hostNothingElixirYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{
		"elixir", "none",
		"no test_for templates and no importscan command",
		"full suite", "Fix:",
		doctor.StaticCaveat,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor output does not contain %q:\n%s", want, stdout)
		}
	}
}

// Spec §4.3: an unmet prerequisite is a named, actionable doctor finding — the binary, the
// adapter that needs it and the declared reason — and doctor still exits 0.
func TestDoctorReportsAnUnmetPrerequisiteAndStillExitsZero(t *testing.T) {
	dir := newHostAdapterRepo(t)
	writeHostAdapter(t, dir, "vitest.yaml", hostRequiresYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{
		"prerequisites",
		"rtdd-no-such-binary", "not on PATH", "vitest",
		"resolves the vitest binary from the lockfile",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor output does not contain %q:\n%s", want, stdout)
		}
	}
}

// A repo whose adapters are all met says nothing about prerequisites: a heading over an
// empty list reads as a problem.
func TestDoctorIsSilentAboutPrerequisitesWhenNoneAreMissing(t *testing.T) {
	dir := newHostAdapterRepo(t)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "prerequisites") {
		t.Errorf("doctor reported prerequisites on a repo with none unmet:\n%s", stdout)
	}
}

// The fan-out table's own §9 caveat still ships alongside the fidelity block: they answer
// different questions and neither substitutes for the other.
func TestDoctorPrintsBothCaveats(t *testing.T) {
	dir := newHostAdapterRepo(t)
	writeHostAdapter(t, dir, "elixir.yaml", hostNothingElixirYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{doctor.StaticCaveat, doctor.Caveat} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor output does not contain:\n%s\ngot:\n%s", want, stdout)
		}
	}
}
