package adapter

import (
	"strings"
	"testing"
)

// staticYAML is the shape spec §4.2 describes: a toolchain RTDD cannot instrument, so it
// declares selection: static and records no coverage. `report` stays pytest-reportlog
// here because the junit-xml report value and its companion keys (report_path,
// id_template) are a sibling slice of contract v2; this slice owns selection fidelity
// only.
const staticYAML = `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
list: "npx vitest list"
selection: static
coverage: none
report: pytest-reportlog
test_globs: ["**/*.test.ts"]
source_globs: ["src/**/*.ts"]
`

// An adapter that declares no selection keeps the meaning it has today. This is what
// makes contract v2 additive rather than a migration: spec §4.1 requires a seeded Python
// repo's selection to stay byte-identical, and no shipped adapter is edited to get it.
func TestSelectionDefaultsToCoverage(t *testing.T) {
	a, err := Load(writeAdapter(t, t.TempDir(), "demo.yaml", validYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Selection != SelectionCoverage {
		t.Errorf("Selection = %q, want the default %q", a.Selection, SelectionCoverage)
	}
}

// The shipped Python adapter declares no selection key and must not need one.
func TestBuiltinPythonReportsCoverageSelection(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	py := byName(all, "python")
	if py == nil {
		t.Fatalf("Builtin has no python adapter")
	}
	if py.Selection != SelectionCoverage {
		t.Errorf("python Selection = %q, want the default %q", py.Selection, SelectionCoverage)
	}
	if py.Coverage != "sqlite" {
		t.Errorf("python Coverage = %q, want %q", py.Coverage, "sqlite")
	}
}

// selection: static with coverage: none and no seed is the whole point of the pair: an
// adapter that says outright it cannot derive selection from execution.
func TestLoadParsesStaticSelection(t *testing.T) {
	a, err := Load(writeAdapter(t, t.TempDir(), "vitest.yaml", staticYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Selection != SelectionStatic {
		t.Errorf("Selection = %q, want %q", a.Selection, SelectionStatic)
	}
	if a.Coverage != CoverageNone {
		t.Errorf("Coverage = %q, want %q", a.Coverage, CoverageNone)
	}
}

// selection takes exactly two values. Anything else is a typo, and exit 2 must name both
// the field and what was written so an agent can fix it without reading the source.
func TestLoadRejectsAnUnknownSelectionValue(t *testing.T) {
	p := writeAdapter(t, t.TempDir(), "a.yaml", strings.Replace(staticYAML, "selection: static", "selection: guesswork", 1))
	_, err := Load(p)
	if err == nil {
		t.Fatalf("Load accepted selection: guesswork; an unknown selection is a configuration error (exit 2)")
	}
	for _, want := range []string{"selection", `"guesswork"`, p} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err.Error(), want)
		}
	}
}

// The two combinations that cannot mean anything, with the wording the M6a plan pins.
// Each message names BOTH offending keys, so an agent reading exit 2 knows which one to
// change and what the other must become. The third case closes the remaining direction,
// so no combination of the three keys can express a static tier that also reads coverage.
func TestValidateRejectsTheIllegalKeyCombinations(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "coverage none with defaulted selection",
			yaml: `name: vitest
detect: ["package.json"]
seed: "npx vitest run"
subset: "npx vitest run {tests}"
coverage: none
report: pytest-reportlog
`,
			want: `coverage: none requires selection: static, got selection "coverage"`,
		},
		{
			name: "static with a seed command",
			yaml: `name: vitest
detect: ["package.json"]
seed: "pytest --cov"
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: pytest-reportlog
`,
			want: `selection: static forbids seed, got seed "pytest --cov"`,
		},
		{
			name: "static with coverage sqlite",
			yaml: `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: sqlite
report: pytest-reportlog
`,
			want: `selection: static requires coverage: none, got coverage "sqlite"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeAdapter(t, t.TempDir(), "a.yaml", tc.yaml)
			_, err := Load(p)
			if err == nil {
				t.Fatalf("Load accepted an illegal selection/coverage/seed combination")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
			}
			// Every adapter rejection names the file it came from, so a bad adapter in a
			// directory of adapters is identifiable.
			if !strings.Contains(err.Error(), p) {
				t.Errorf("error = %q, want it to name the source file %q", err.Error(), p)
			}
		})
	}
}

// seed instruments a suite. There is nothing to instrument under selection: static, and a
// coverage adapter without one cannot build a map at all.
func TestSeedIsForbiddenUnderStaticAndRequiredUnderCoverage(t *testing.T) {
	if _, err := Load(writeAdapter(t, t.TempDir(), "vitest.yaml", staticYAML)); err != nil {
		t.Errorf("static adapter without seed: Load = %v, want nil", err)
	}

	noSeed := `name: python2
detect: ["pyproject.toml"]
subset: "pytest {tests} --cov"
coverage: sqlite
report: pytest-reportlog
`
	_, err := Load(writeAdapter(t, t.TempDir(), "b.yaml", noSeed))
	if err == nil {
		t.Fatalf("Load accepted a coverage adapter with no seed command")
	}
	if !strings.Contains(err.Error(), "seed is required") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "seed is required")
	}
}

// The shipped adapter must survive every new rule unchanged. adapters/python.yaml is
// byte-frozen for this PRD; this is the behavioural half of that freeze.
func TestBuiltinAdaptersStillValidate(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	if len(all) == 0 {
		t.Fatalf("Builtin returned no adapters")
	}
}
