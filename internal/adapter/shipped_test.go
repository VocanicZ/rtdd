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
		"cargo-nextest", "dotnet", "go", "gradle", "jest",
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

// PRD #232 AC3, and decision 11: the five declare a requires whose bin is a PATH binary,
// and whose reason names the package a human has to install.
func TestTheFiveRequiresAdaptersNameACheckableBinAndTheirPackage(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	want := map[string]struct{ bin, pkg string }{
		"go":            {"go-junit-report", "go-junit-report"},
		"cargo-nextest": {"cargo-nextest", "nextest"},
		"jest":          {"npx", "jest-junit"},
		"rspec":         {"rspec", "rspec_junit_formatter"},
		"dotnet":        {"dotnet", "JUnitTestLogger"},
	}
	for _, a := range all {
		w, needs := want[a.Name]
		if !needs {
			if len(a.Requires) != 0 {
				t.Errorf("adapter %s declares requires; its runner emits JUnit XML unaided (AC3)", a.Name)
			}
			continue
		}
		if len(a.Requires) == 0 {
			t.Errorf("adapter %s declares no requires (AC3)", a.Name)
			continue
		}
		if a.Requires[0].Bin != w.bin {
			t.Errorf("adapter %s requires[0].Bin = %q, want the PATH-checkable %q (decision 11)", a.Name, a.Requires[0].Bin, w.bin)
		}
		if !strings.Contains(a.Requires[0].Reason, w.pkg) {
			t.Errorf("adapter %s requires[0].Reason %q does not name the package %q", a.Name, a.Requires[0].Reason, w.pkg)
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
		"vitest":        {"package-lock.json", "pnpm-lock.yaml", "vitest.config.ts"},
		"jest":          {"package-lock.json", "jest.config.js"},
		"go":            {"go.sum"},
		"cargo-nextest": {"Cargo.lock", ".config/nextest.toml"},
		"maven":         {"pom.xml"},
		"gradle":        {"gradle/wrapper/gradle-wrapper.properties", "build.gradle"},
		"rspec":         {"Gemfile.lock", ".rspec"},
		"dotnet":        {"Directory.Packages.props"},
		"phpunit":       {"composer.lock", "phpunit.xml"},
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

// PRD #232 AC1: the nine are the STATIC tier, uniformly. An adapter that shipped as
// `selection: coverage` would want a seed command it has no way to run, and one that
// declared another report would parse against a file its runner never writes.
func TestTheNineShippedStaticAdaptersDeclareTheStaticJUnitTriple(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	nine := map[string]bool{
		"vitest": true, "jest": true, "go": true, "cargo-nextest": true, "maven": true,
		"gradle": true, "rspec": true, "dotnet": true, "phpunit": true,
	}
	seen := 0
	for _, a := range all {
		if !nine[a.Name] {
			continue
		}
		seen++
		if a.Selection != SelectionStatic {
			t.Errorf("adapter %s selection = %q, want %q", a.Name, a.Selection, SelectionStatic)
		}
		if a.Coverage != CoverageNone {
			t.Errorf("adapter %s coverage = %q, want %q", a.Name, a.Coverage, CoverageNone)
		}
		if a.Report != "junit-xml" {
			t.Errorf("adapter %s report = %q, want junit-xml", a.Name, a.Report)
		}
		if a.Seed != "" {
			t.Errorf("adapter %s declares seed %q; selection: static has nothing to seed", a.Name, a.Seed)
		}
	}
	if seen != len(nine) {
		t.Errorf("found %d of the nine static adapters in Builtin (%v)", seen, adapterNamesFor(all))
	}
}

// Spec §4.3 fixes the id_template vocabulary at {file}, {classname} and {name}. {class}
// is NOT one of them: it renders as a literal brace run in the id, the round trip back
// into `subset` selects nothing, and the run reports green. Load rejects it, and this
// pins the shipped set against the exact near-miss spelling.
func TestShippedIDTemplatesUseOnlyTheThreePlaceholders(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	for _, a := range all {
		if a.IDTemplate == "" {
			continue
		}
		if strings.Contains(a.IDTemplate, "{class}") {
			t.Errorf("adapter %s id_template %q names {class}, which is not a placeholder (spec §4.3)", a.Name, a.IDTemplate)
		}
		if bad := unknownPlaceholder(a.IDTemplate, idTemplatePlaceholders); bad != "" {
			t.Errorf("adapter %s id_template %q names %s, outside {file}/{classname}/{name}", a.Name, a.IDTemplate, bad)
		}
	}
}

// Nextest has no --junit flag: the report exists because [profile.rtdd.junit] in
// .config/nextest.toml says so, and --profile selects that block. An adapter that reached
// for a CLI flag would run a command nextest rejects, so both halves are pinned — the
// profile is passed, and no junit flag is.
func TestCargoNextestConfiguresJUnitThroughItsConfigFileAndNotAFlag(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	a := byName(all, "cargo-nextest")
	if a == nil {
		t.Fatal("Builtin has no cargo-nextest adapter")
	}
	for _, cmd := range []string{a.Subset, a.List} {
		if !strings.Contains(cmd, "--profile rtdd") {
			t.Errorf("cargo-nextest command %q does not select the rtdd profile; nothing would write JUnit XML", cmd)
		}
		if strings.Contains(cmd, "junit") {
			t.Errorf("cargo-nextest command %q names a junit flag; nextest has none, the profile block is the configuration", cmd)
		}
	}
	if !contains(a.Detect, ".config/nextest.toml") {
		t.Errorf("cargo-nextest detect = %v, want it to name .config/nextest.toml", a.Detect)
	}
	if !contains(a.FullEscalate, ".config/nextest.toml") {
		t.Errorf("cargo-nextest full_escalate = %v, want it to name .config/nextest.toml", a.FullEscalate)
	}
}

// Surefire and Gradle write one report per test class into a directory, and `dotnet test`
// takes --results-directory rather than a file, so those three declare a report_path
// ending in "/"; the other six name one file. The runner clears report_path before every
// chunk and reads it after, and it tells the two shapes apart by that trailing "/" — a
// directory declared as a file is read as one report that never appears.
//
// Issue #314 lists only maven and gradle as directory-shaped. dotnet is the third because
// its CLI has no file-valued option to declare: --results-directory names a store, the
// JUnitTestLogger writes into it, and the plan's own dotnet declaration
// (docs/plans/06-m6d-shipped-adapters.md, Task 4) is "TestResults/" for that reason.
func TestReportPathIsADirectoryForMavenAndGradleAndAFileForTheRest(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	dirs := map[string]bool{"maven": true, "gradle": true, "dotnet": true}
	for _, a := range all {
		if a.Report != "junit-xml" {
			continue
		}
		isDir := strings.HasSuffix(a.ReportPath, "/")
		if dirs[a.Name] && !isDir {
			t.Errorf("adapter %s report_path = %q, want a directory ending in %q", a.Name, a.ReportPath, "/")
		}
		if !dirs[a.Name] && isDir {
			t.Errorf("adapter %s report_path = %q, want a single file", a.Name, a.ReportPath)
		}
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func adapterNamesFor(as []*Adapter) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}
