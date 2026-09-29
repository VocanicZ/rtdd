package adapter

import (
	"strings"
	"testing"
)

// Every shipped adapter must LOAD. The embedded set is read at startup by every command,
// so a typo in one of the nine is not a bad adapter, it is a binary that cannot run
// anywhere (PRD #232 AC1).
func TestBuiltinShipsTenAdaptersAndAllOfThemLoad(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	want := []string{
		"cargo", "dotnet", "go", "gradle", "jest",
		"maven", "phpunit", "python", "rspec", "vitest",
	}
	if len(all) != len(want) {
		t.Fatalf("Builtin returned %d adapters (%v), want %d", len(all), adapterNamesFor(all), len(want))
	}
	for i, name := range want {
		if all[i].Name != name {
			t.Errorf("Builtin()[%d].Name = %q, want %q (LoadFS sorts by name)", i, all[i].Name, name)
		}
	}
}

// Decision 1: package.json is NOT a marker. Listing it in both JS adapters makes every
// JavaScript repo detect two adapters and AC10's fixture detect three.
func TestNoShippedAdapterDetectsOnPackageJSON(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	for _, a := range all {
		for _, m := range a.Detect {
			if m == "package.json" {
				t.Errorf("adapter %s detects on package.json; both JS adapters would match every JS repo (decision 1)", a.Name)
			}
		}
	}
}

// PRD #232 AC8: full_escalate names the ecosystem's REAL lockfile and runner config. A
// dependency bump that escalates nothing selects a stale suite against new dependencies.
func TestFullEscalateNamesTheLockfileAndRunnerConfig(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	want := map[string][]string{
		"vitest":  {"package-lock.json", "pnpm-lock.yaml", "vitest.config.ts"},
		"jest":    {"package-lock.json", "jest.config.js"},
		"go":      {"go.sum"},
		"cargo":   {"Cargo.lock"},
		"maven":   {"pom.xml"},
		"gradle":  {"gradle/wrapper/gradle-wrapper.properties", "build.gradle"},
		"rspec":   {"Gemfile.lock", ".rspec"},
		"dotnet":  {"Directory.Packages.props"},
		"phpunit": {"composer.lock", "phpunit.xml"},
	}
	byName := map[string]*Adapter{}
	for _, a := range all {
		byName[a.Name] = a
	}
	for name, needles := range want {
		a := byName[name]
		if a == nil {
			t.Errorf("no adapter named %s", name)
			continue
		}
		joined := strings.Join(a.FullEscalate, " ")
		for _, n := range needles {
			if !strings.Contains(joined, n) {
				t.Errorf("adapter %s full_escalate does not name %q: %v", name, n, a.FullEscalate)
			}
		}
	}
}

func adapterNamesFor(as []*Adapter) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}
