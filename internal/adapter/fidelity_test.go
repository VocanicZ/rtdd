package adapter

import "testing"

// The truth table. `none` is the decision the M6a plan pins: path proximity alone is the
// `path` baseline the static tier must beat (spec §7), so an adapter offering only that
// selects nothing and says so.
func TestFidelity(t *testing.T) {
	cases := []struct {
		name string
		a    *Adapter
		want Fidelity
	}{
		{
			name: "coverage adapter",
			a:    &Adapter{Selection: SelectionCoverage, Coverage: "sqlite"},
			want: FidelityExecution,
		},
		{
			name: "static with correspondence templates",
			a:    &Adapter{Selection: SelectionStatic, Coverage: CoverageNone, TestFor: []string{"{dir}/{name}.test.ts"}},
			want: FidelityStatic,
		},
		{
			name: "static with an import scanner only",
			a: &Adapter{Selection: SelectionStatic, Coverage: CoverageNone,
				Importscan: &Importscan{Command: "node {script}", Script: "scan-imports.mjs"}},
			want: FidelityStatic,
		},
		{
			name: "static with neither",
			a:    &Adapter{Selection: SelectionStatic, Coverage: CoverageNone},
			want: FidelityNone,
		},
		{
			name: "static with an empty test_for list",
			a:    &Adapter{Selection: SelectionStatic, Coverage: CoverageNone, TestFor: []string{}},
			want: FidelityNone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Fidelity(); got != tc.want {
				t.Errorf("Fidelity() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Fidelity is DERIVED from selection and coverage, never asserted beside them: a repo
// cannot be told it has execution-derived selection by an adapter declaring it records
// nothing (PRD #240 AC3).
func TestFidelityIsDerivedFromTheLoadedDeclaration(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
		want Fidelity
	}{
		{
			name: "selection: coverage",
			yaml: "name: python\ndetect: [\"pyproject.toml\"]\nseed: \"pytest --cov\"\n" +
				"subset: \"pytest {tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n",
			want: FidelityExecution,
		},
		{
			name: "selection: static with test_for",
			yaml: "name: vitest\ndetect: [\"vitest.config.ts\"]\nsubset: \"npx vitest run {tests}\"\n" +
				"selection: static\ncoverage: none\nreport: pytest-reportlog\n" +
				"test_for: [\"{dir}/{name}.test.ts\"]\n",
			want: FidelityStatic,
		},
		{
			name: "selection: static with neither",
			yaml: "name: elixir\ndetect: [\"mix.exs\"]\nsubset: \"mix test {tests}\"\n" +
				"selection: static\ncoverage: none\nreport: pytest-reportlog\n",
			want: FidelityNone,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := parse([]byte(tc.yaml), tc.name+".yaml")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := a.Fidelity(); got != tc.want {
				t.Errorf("Fidelity() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The wire values are a contract with --json consumers (spec §6). Renaming one is a
// breaking change to the agent front-ends, so it fails here first.
func TestFidelityWireValues(t *testing.T) {
	for got, want := range map[Fidelity]string{
		FidelityExecution: "execution-derived",
		FidelityStatic:    "static",
		FidelityNone:      "none",
	} {
		if string(got) != want {
			t.Errorf("fidelity constant = %q, want %q", string(got), want)
		}
	}
}

// A nil adapter is the no-adapter repo, and spec §6 says the field is never null.
func TestNilAdapterIsFidelityNone(t *testing.T) {
	var a *Adapter
	if got := a.Fidelity(); got != FidelityNone {
		t.Errorf("(*Adapter)(nil).Fidelity() = %q, want %q", got, FidelityNone)
	}
}
