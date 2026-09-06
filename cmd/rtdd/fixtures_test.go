package main

import (
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// The ten committed fixture repositories (PRD #232 AC9 and AC10). Each is the smallest
// tree that detects its adapter, holds one source file, and holds the test file that
// adapter's declared test_for correspondence resolves to — and nothing else.
//
// NO RUNNER IS EXECUTED. These fixtures prove detection and SELECTION, which is what
// `rtdd which` answers; proving a runner accepts the rendered id needs the runner
// installed and belongs to spec §7's evidence PRD (#233).

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

// PRD #232 AC9: one committed minimal repository per shipped adapter, and a non-empty TS
// selection naming the expected test file. A shipped adapter nobody ever pointed at a real
// tree is a YAML file that compiles, not a supported language.
//
// The tier is asserted as well as the id: a selection that named the right file from T2
// would be the full suite wearing the right answer's clothes, and one that named it from
// the map would mean the fixture had been seeded, which a `selection: static` adapter can
// never be.
func TestEachFixtureRepoDetectsItsAdapterAndSelectsItsTest(t *testing.T) {
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
			sel := whichSelectionForFixture(t, root, got, tc.changed)
			if sel.Tier != selector.TierTS {
				t.Fatalf("tier = %s, want TS; %s", sel.Tier, sel.Reason)
			}
			if len(sel.Tests) == 0 {
				t.Fatalf("rtdd which selected nothing for %s; a shipped adapter must narrow its own fixture", tc.changed)
			}
			if !containsString(sel.Tests, tc.wantTest) {
				t.Errorf("selection %v does not name %q", sel.Tests, tc.wantTest)
			}
		})
	}
}

// PRD #232 AC10: a package.json plus a pom.xml detects EXACTLY two adapters, and both
// contribute a selection. Never three — decision 1 keeps package.json out of every
// detect list, so jest does not join in.
func TestPolyglotFixtureDetectsExactlyTwoAdaptersAndSelectsFromBoth(t *testing.T) {
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

	ts := whichSelectionForFixture(t, root, got, "src/calc.ts")
	if ts.Tier != selector.TierTS {
		t.Errorf("vitest half tier = %s, want TS; %s", ts.Tier, ts.Reason)
	}
	if !containsString(ts.Tests, "src/calc.test.ts") {
		t.Errorf("vitest half selected %v, want src/calc.test.ts", ts.Tests)
	}

	jv := whichSelectionForFixture(t, root, got, "src/main/java/calc/Calc.java")
	if jv.Tier != selector.TierTS {
		t.Errorf("maven half tier = %s, want TS; %s", jv.Tier, jv.Reason)
	}
	if !containsString(jv.Tests, "src/test/java/calc/CalcTest.java") {
		t.Errorf("maven half selected %v, want src/test/java/calc/CalcTest.java", jv.Tests)
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

// whichSelectionForFixture is `rtdd which` over a fixture repository, reduced to the one
// block that answers for the changed file.
//
// It goes through selectPerAdapter rather than calling the selector directly, so what the
// fixtures prove is the CLI's own wiring — the resolvers `which` injects included — and
// not a second selection path that only the tests have.
//
// The block is chosen by classification: the adapter whose source_globs own the changed
// file is the one whose answer is about it. In the polyglot fixture that is the whole
// point — the Java file is maven's question and the TypeScript file is vitest's, and a
// helper that merged the two would erase the split the fixture exists to demonstrate.
func whichSelectionForFixture(t *testing.T, root string, ads []*adapter.Adapter, changed string) selector.Selection {
	t.Helper()
	blocks, err := selectPerAdapter(root, ads, mapstore.New(), mapstore.Meta{}, selectionContext{
		Changes: []gitctx.Change{{Path: changed, Status: gitctx.Modified}},
		Cfg:     selector.DefaultConfig(),
	})
	if err != nil {
		t.Fatalf("selectPerAdapter(%s): %v", root, err)
	}
	for _, blk := range blocks {
		if blk.Ad != nil && blk.Ad.IsInstrumentable(changed) {
			return blk.Selection
		}
	}
	t.Fatalf("no detected adapter in %s classifies %s as one of its source files", root, changed)
	return selector.Selection{}
}
