package main

import (
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// The committed fixture repositories. Each is the smallest tree that detects its adapter.
// Tasks 8-12 of the one-pipeline plan run each through seed/which/run; this file proves
// detection only.

// fixtureCase is one shipped adapter's fixture repository.
type fixtureCase struct {
	fixture     string
	wantAdapter string
	changed     string
	wantTest    string
}

var shippedFixtures = []fixtureCase{
	{"vitest", "vitest", "src/calc.ts", "src/calc.test.ts"},
	{"jest", "jest", "src/calc.js", "src/calc.test.js"},
	{"go", "go", "calc/calc.go", "calc/calc_test.go"},
	{"cargo-nextest", "cargo-nextest", "src/calc.rs", "tests/calc.rs"},
	{"maven", "maven", "src/main/java/calc/Calc.java", "src/test/java/calc/CalcTest.java"},
	{"gradle", "gradle", "src/main/java/calc/Calc.java", "src/test/java/calc/CalcTest.java"},
	{"rspec", "rspec", "lib/calc.rb", "spec/calc_spec.rb"},
	{"dotnet", "dotnet", "Calc/Calc.cs", "tests/Calc.Tests/CalcTests.cs"},
	{"phpunit", "phpunit", "src/Calc.php", "tests/CalcTest.php"},
}

// One committed minimal repository per shipped adapter, and each detects exactly its own
// adapter and classifies its test and source files as that adapter's.
func TestEachFixtureRepoDetectsItsAdapter(t *testing.T) {
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	for _, tc := range shippedFixtures {
		t.Run(tc.fixture, func(t *testing.T) {
			root := fixtureRoot(t, tc.fixture)
			got, err := adapter.Detect(root, all)
			if err != nil {
				t.Fatalf("Detect(%s): %v", root, err)
			}
			if len(got) != 1 || got[0].Name != tc.wantAdapter {
				t.Fatalf("Detect(%s) = %v, want exactly [%s]", root, adapterNames(got), tc.wantAdapter)
			}
			if !got[0].IsInstrumentable(tc.changed) {
				t.Errorf("%s is not instrumentable under %s", tc.changed, tc.wantAdapter)
			}
			if !got[0].IsTestFile(tc.wantTest) {
				t.Errorf("%s is not a test file under %s", tc.wantTest, tc.wantAdapter)
			}
		})
	}
}

// PRD #232 AC10: a package.json plus a pom.xml detects EXACTLY two adapters. Never three
// — decision 1 keeps package.json out of every detect list, so jest does not join in.
func TestPolyglotFixtureDetectsExactlyTwoAdapters(t *testing.T) {
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	root := fixtureRoot(t, "polyglot")
	got, err := adapter.Detect(root, all)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Detect = %v, want exactly two adapters", adapterNames(got))
	}
	if !containsString(adapterNames(got), "vitest") || !containsString(adapterNames(got), "maven") {
		t.Fatalf("Detect = %v, want vitest and maven", adapterNames(got))
	}

}

// fixtureRoot is one fixture repository's ABSOLUTE path.
//
// Absolute rather than "testdata/fixtures/<name>": internal/paths.Normalize resolves a
// relative path against the repo root it is given, so a relative root would have every
// walked path joined onto itself and nothing would ever match a detect marker. The CLI
// reaches Detect through findRepoRoot, which is always absolute, so this is the wiring
// the fixtures are meant to prove and not a convenience.
func fixtureRoot(t *testing.T, name string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("resolving fixture %s: %v", name, err)
	}
	return root
}

func adapterNames(as []*adapter.Adapter) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}
