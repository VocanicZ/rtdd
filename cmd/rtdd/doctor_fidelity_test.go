package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/doctor"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
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
	// The marker, not just the file: doctor reports the fidelity a repository can
	// achieve, so an adapter has to be DETECTED here before its row is that claim.
	gittest.Write(t, dir, "vitest.config.ts", "export default {}\n")
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
	gittest.Write(t, dir, "mix.exs", "defmodule Demo.MixProject do\nend\n")
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
	gittest.Write(t, dir, "vitest.config.ts", "export default {}\n")
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
	gittest.Write(t, dir, "mix.exs", "defmodule Demo.MixProject do\nend\n")
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

// newVitestRepo is a TypeScript repo the built-in python adapter does NOT detect: the
// bug this fixture exists for is a TS repo being told `python … execution-derived`.
func newVitestRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "package.json", "{\"name\":\"demo\"}\n")
	gittest.Write(t, dir, "vitest.config.ts", "export default {}\n")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b\n")
	gittest.Write(t, dir, "src/logic.test.ts", "test('add', () => {})\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// fidelityBlock is the part of doctor's output that answers "what can THIS repository
// achieve" — everything above the separately headed undetected section, with the caveat
// stripped so its legitimate mention of execution-derived is not read as a row's claim.
func fidelityBlock(t *testing.T, stdout string) string {
	t.Helper()
	block := stdout
	if i := strings.Index(block, undetectedHeading); i >= 0 {
		block = block[:i]
	}
	return strings.ReplaceAll(block, doctor.StaticCaveat, "")
}

// Spec §6 / PRD #229 AC8: the fidelity report is per DETECTED adapter. A TypeScript repo
// must never be told `python … execution-derived` — under a heading that reads `selection
// fidelity`, an undetected row is a fidelity claim this repository cannot cash.
func TestDoctorScopesFidelityToTheDetectedAdapters(t *testing.T) {
	dir := newVitestRepo(t)
	writeHostAdapter(t, dir, "vitest.yaml", hostStaticVitestYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	block := fidelityBlock(t, stdout)
	if !strings.Contains(block, "vitest") || !strings.Contains(block, "static") {
		t.Errorf("the detected adapter is missing from the fidelity block:\n%s", block)
	}
	if strings.Contains(block, "python") {
		t.Errorf("doctor presented the undetected built-in python as this repository's fidelity:\n%s", block)
	}
	if strings.Contains(block, "execution-derived") {
		t.Errorf("doctor claimed a fidelity this repository cannot achieve:\n%s", block)
	}
}

// AC5: an adapter that resolved but detects nothing here is still visible — "I wrote
// .rtdd/adapters/vitest.yaml and doctor says nothing" is a worse diagnostic than the bug —
// under a heading that cannot be read as this repository's fidelity, with its markers.
func TestDoctorListsAResolvedButUndetectedAdapterUnderItsOwnHeading(t *testing.T) {
	dir := newVitestRepo(t)
	writeHostAdapter(t, dir, "vitest.yaml", hostStaticVitestYAML)
	writeHostAdapter(t, dir, "elixir.yaml", hostNothingElixirYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	i := strings.Index(stdout, undetectedHeading)
	if i < 0 {
		t.Fatalf("doctor dropped the resolved-but-undetected adapters entirely:\n%s", stdout)
	}
	tail := stdout[i:]
	for _, want := range []string{"elixir", "mix.exs", "python", "pyproject.toml"} {
		if !strings.Contains(tail, want) {
			t.Errorf("the undetected section does not name %q:\n%s", want, tail)
		}
	}
	if strings.Contains(tail, string(adapter.FidelityExecution)) {
		t.Errorf("the undetected section stated a fidelity, which is exactly what it must not do:\n%s", tail)
	}
}

// AC2: the `none` verdict is reachable end-to-end, through cmdDoctor and not only through
// RenderFidelity — this is the state `rtdd init --force` leaves behind.
func TestDoctorPrintsTheNoneVerdictWhenNoAdapterDetectsTheRepository(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "README.md", "# demo\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"selection fidelity", "none", "full suite", "Fix:", doctor.StaticCaveat} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor output does not contain %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(fidelityBlock(t, stdout), "execution-derived") {
		t.Errorf("a repo no adapter detects was told it can reach execution-derived selection:\n%s", stdout)
	}
}

// AC3: the caveat describes the DETECTED selection. An undetected static adapter is not
// this repository's weakness, so it must not drag the caveat in.
func TestDoctorOmitsTheStaticCaveatWhenTheDetectedAdapterIsExecutionDerived(t *testing.T) {
	dir := newHostAdapterRepo(t)
	writeHostAdapter(t, dir, "vitest.yaml", hostStaticVitestYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, doctor.StaticCaveat) {
		t.Errorf("the static caveat was printed for an adapter this repository does not use:\n%s", stdout)
	}
}

// AC4: prerequisites are the detected adapters' prerequisites. Naming a binary nothing
// here needs sends an agent to install a toolchain this repository never runs.
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
	if strings.Contains(stdout, "prerequisites") {
		t.Errorf("doctor printed a prerequisites heading with nothing detected to need one:\n%s", stdout)
	}
}

func TestRenderUndetectedIsSilentWhenEverythingIsDetected(t *testing.T) {
	if got := RenderUndetected(nil); got != "" {
		t.Errorf("RenderUndetected(nil) = %q, want the empty string", got)
	}
}

// The section names the adapter, where it came from, and the markers that would have
// matched — and states no fidelity, because an adapter this repository does not use has
// no fidelity here to state.
func TestRenderUndetectedNamesTheMarkersAndStatesNoFidelity(t *testing.T) {
	got := RenderUndetected([]FidelityRow{{
		Name: "python", Src: "python.yaml", Fidelity: adapter.FidelityExecution,
		Why:     "selection: coverage with coverage: sqlite",
		Markers: []string{"pytest.ini", "pyproject.toml"},
	}})
	for _, want := range []string{undetectedHeading, "python", "python.yaml", "pytest.ini", "pyproject.toml"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{string(adapter.FidelityExecution), "selection: coverage"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("the undetected section stated %q, which cannot be read as this repo's fidelity:\n%s", unwanted, got)
		}
	}
}

// An adapter with no detect: globs can never be detected; saying "no file matches its
// markers" about an empty list would be a riddle.
func TestRenderUndetectedExplainsAnAdapterWithNoMarkers(t *testing.T) {
	got := RenderUndetected([]FidelityRow{{
		Name: "nomarkers", Src: ".rtdd/adapters/nomarkers.yaml", Host: true, Fidelity: adapter.FidelityNone,
	}})
	if !strings.Contains(got, "no detect: markers") {
		t.Errorf("output does not explain the empty marker list:\n%s", got)
	}
}
