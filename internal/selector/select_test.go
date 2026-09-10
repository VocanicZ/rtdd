package selector

import (
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// fixtureAdapter mirrors adapters/python.yaml without touching the filesystem.
func fixtureAdapter() *adapter.Adapter {
	return &adapter.Adapter{
		Name:         "python",
		TestGlobs:    []string{"tests/**/*.py", "**/test_*.py"},
		SourceGlobs:  []string{"src/**/*.py"},
		Opaque:       []string{"**/*.yaml", "**/*.html", "**/fixtures/**"},
		FullEscalate: []string{"requirements.txt", "pyproject.toml", "**/conftest.py"},
	}
}

// fixtureSelectMap is the hand-written map that drives M1a.
func fixtureSelectMap() *mapstore.Map {
	return mapOf(
		mapstore.Row{T: "tests/test_auth.py::test_login", F: []string{"src/auth.py", "src/db.py"}, C: "aaa1111", D: 412, S: "pass"},
		mapstore.Row{T: "tests/test_auth.py::test_logout", F: []string{"src/auth.py"}, C: "aaa1111", D: 90, S: "fail"},
		mapstore.Row{T: "tests/test_db.py::test_query", F: []string{"src/db.py"}, C: "aaa1111", D: 15, S: "pass"},
		mapstore.Row{T: "tests/test_render.py::test_page", F: []string{"src/render.py", "templates/page.html"}, C: "aaa1111", D: 230, S: "pass"},
	)
}

func mod(p string) gitctx.Change {
	return gitctx.Change{Path: p, Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 1}}}
}

func added(p string) gitctx.Change {
	return gitctx.Change{Path: p, Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 3}}}
}

func deleted(p string) gitctx.Change {
	return gitctx.Change{Path: p, Status: gitctx.Deleted}
}

// fresh reports every recorded commit as zero commits old.
func fresh(string) int { return 0 }

func baseInputs() Inputs {
	return Inputs{
		Map:      fixtureSelectMap(),
		Adapter:  fixtureAdapter(),
		Cfg:      DefaultConfig(),
		Distance: fresh,
	}
}

// The regression that made v1's contribution #2 unreachable: a test the agent just
// wrote has no map row, so it belonged to no tier and never ran.
func TestSelectPutsANewTestFileInTheDirectTierFirst(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{added("tests/test_brand_new.py")}

	got := Select(in)

	if !reflect.DeepEqual(got.Direct, []string{"tests/test_brand_new.py"}) {
		t.Fatalf("Direct = %#v, want the newly written test file", got.Direct)
	}
	if len(got.Tests) == 0 || got.Tests[0] != "tests/test_brand_new.py" {
		t.Fatalf("Tests = %#v, want the direct test FIRST", got.Tests)
	}
	if got.Tier != TierDirect {
		t.Errorf("Tier = %v, want TierDirect", got.Tier)
	}
	if got.Reason == "" {
		t.Error("Reason is empty; every Selection must explain itself")
	}
}

func TestSelectDirectTestsPrecedeMappedTests(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{
		added("tests/test_brand_new.py"),
		mod("src/auth.py"),
	}

	got := Select(in)

	want := []string{
		"tests/test_brand_new.py",         // direct, first, despite having no map row
		"tests/test_auth.py::test_logout", // ratio 1.0, last-failed
		"tests/test_auth.py::test_login",  // ratio 0.5
	}
	if !reflect.DeepEqual(got.Tests, want) {
		t.Errorf("Tests = %#v, want %#v", got.Tests, want)
	}
	if got.Tier != TierT0 {
		t.Errorf("Tier = %v, want TierT0", got.Tier)
	}
}

func TestSelectAChangedTestFileIsDirectEvenWhenItHasAMapRow(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("tests/test_auth.py")}

	got := Select(in)

	if !reflect.DeepEqual(got.Direct, []string{"tests/test_auth.py"}) {
		t.Errorf("Direct = %#v, want [tests/test_auth.py]", got.Direct)
	}
	if got.Tests[0] != "tests/test_auth.py" {
		t.Errorf("Tests[0] = %q, want the changed test file", got.Tests[0])
	}
}

func TestSelectADeletedTestFileIsNotRunDirectly(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{deleted("tests/test_auth.py")}

	got := Select(in)

	if len(got.Direct) != 0 {
		t.Errorf("Direct = %#v, want empty: a deleted test file cannot be executed", got.Direct)
	}
}

func TestSelectT0(t *testing.T) {
	tests := []struct {
		name      string
		changes   []gitctx.Change
		wantTier  Tier
		wantTests []string
	}{
		{
			name:      "one source file selects its two tests, ranked",
			changes:   []gitctx.Change{mod("src/auth.py")},
			wantTier:  TierT0,
			wantTests: []string{"tests/test_auth.py::test_logout", "tests/test_auth.py::test_login"},
		},
		{
			name:      "two source files union their tests",
			changes:   []gitctx.Change{mod("src/auth.py"), mod("src/db.py")},
			wantTier:  TierT0,
			wantTests: []string{"tests/test_auth.py::test_logout", "tests/test_db.py::test_query", "tests/test_auth.py::test_login"},
		},
		{
			name:      "a deleted source file still selects the tests whose F contains it",
			changes:   []gitctx.Change{deleted("src/render.py")},
			wantTier:  TierT0,
			wantTests: []string{"tests/test_render.py::test_page"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInputs()
			in.Changes = tc.changes
			got := Select(in)
			if got.Tier != tc.wantTier {
				t.Errorf("Tier = %v (%s), want %v", got.Tier, got.Reason, tc.wantTier)
			}
			if !reflect.DeepEqual(got.Tests, tc.wantTests) {
				t.Errorf("Tests = %#v, want %#v", got.Tests, tc.wantTests)
			}
		})
	}
}

func TestSelectRenameSelectsViaTheOldPathToo(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{{
		Path:    "src/authentication.py",
		OldPath: "src/auth.py",
		Status:  gitctx.Renamed,
		Lines:   []gitctx.LineRange{{Start: 1, End: 10}},
	}}

	got := Select(in)

	want := []string{"tests/test_auth.py::test_logout", "tests/test_auth.py::test_login"}
	if !reflect.DeepEqual(got.Tests, want) {
		t.Errorf("Tests = %#v, want %#v (the OLD path must still select its tests)", got.Tests, want)
	}
}

// TierEmpty is a distinct outcome with its own Reason. It must never be reported as a
// tier that ran and passed.
func TestSelectTierEmptyIsExplicit(t *testing.T) {
	tests := []struct {
		name    string
		changes []gitctx.Change
	}{
		{"a source file no test covers", []gitctx.Change{mod("src/orphan.py")}},
		{"nothing changed at all", nil},
		{"a file outside every glob", []gitctx.Change{mod("README.md")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInputs()
			in.Changes = tc.changes
			got := Select(in)
			if got.Tier != TierEmpty {
				t.Fatalf("Tier = %v, want TierEmpty", got.Tier)
			}
			if len(got.Tests) != 0 {
				t.Errorf("Tests = %#v, want empty", got.Tests)
			}
			if got.Reason == "" {
				t.Error("TierEmpty with an empty Reason: an empty selection must say why")
			}
		})
	}
}

func TestSelectToleratesNilMapAndNilAdapter(t *testing.T) {
	got := Select(Inputs{Changes: []gitctx.Change{mod("src/auth.py")}, Cfg: DefaultConfig()})
	if got.Tier != TierT2 {
		t.Errorf("Tier = %v, want TierT2 (a nil/empty map is unseeded)", got.Tier)
	}
	if got.Reason == "" {
		t.Error("Reason is empty")
	}
}

func TestSelectEscalatesToT2(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Inputs)
		reasonHas string
	}{
		{
			name: "unseeded map",
			mutate: func(in *Inputs) {
				in.Map = mapstore.New()
				in.Changes = []gitctx.Change{mod("src/auth.py")}
			},
			reasonHas: "unseeded",
		},
		{
			name: "dependency manifest changed",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py"), mod("requirements.txt")}
			},
			reasonHas: "requirements.txt",
		},
		{
			name: "test-harness config changed",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("tests/conftest.py")}
			},
			reasonHas: "conftest.py",
		},
		{
			name: "drift guard reached",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py")}
				in.Cycles = 100
			},
			reasonHas: "drift guard",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInputs()
			in.AllTests = []string{"tests/test_auth.py::test_login", "tests/test_zebra.py::test_z"}
			tc.mutate(&in)
			got := Select(in)
			if got.Tier != TierT2 {
				t.Fatalf("Tier = %v (%s), want TierT2", got.Tier, got.Reason)
			}
			if !contains(got.Reason, tc.reasonHas) {
				t.Errorf("Reason = %q, want it to mention %q", got.Reason, tc.reasonHas)
			}
			if len(got.Tests) != len(in.AllTests) {
				t.Errorf("Tests = %#v, want the full suite %#v", got.Tests, in.AllTests)
			}
		})
	}
}

func TestSelectT2KeepsDirectTestsFirst(t *testing.T) {
	in := baseInputs()
	in.AllTests = []string{"tests/test_auth.py::test_login", "tests/test_db.py::test_query"}
	in.Changes = []gitctx.Change{mod("requirements.txt"), added("tests/test_brand_new.py")}

	got := Select(in)

	if got.Tier != TierT2 {
		t.Fatalf("Tier = %v, want TierT2", got.Tier)
	}
	if got.Tests[0] != "tests/test_brand_new.py" {
		t.Errorf("Tests[0] = %q, want the direct test even at T2", got.Tests[0])
	}
}

func TestSelectEscalatesToT1(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Inputs)
		reasonHas string
		wantHas   string
	}{
		{
			name: "opaque file selects its directory's tests",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("templates/page.html")}
			},
			reasonHas: "opaque",
			wantHas:   "tests/test_render.py::test_page",
		},
		{
			name: "HEAD is a merge commit",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py")}
				in.Merge = true
			},
			reasonHas: "merge commit",
			wantHas:   "tests/test_auth.py::test_logout",
		},
		{
			name: "a row is staler than stale_commits",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py")}
				in.Distance = func(string) int { return 51 }
			},
			reasonHas: "stale",
			wantHas:   "tests/test_auth.py::test_logout",
		},
		{
			name: "an unreachable commit is unknown, never fresh",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py")}
				in.Distance = func(string) int { return -1 }
			},
			reasonHas: "unreachable",
			wantHas:   "tests/test_auth.py::test_logout",
		},
		{
			name: "import-time-only fallback contributes tests",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/constants.py")}
				in.ImportOnly = func(rel string) []string {
					if rel == "src/constants.py" {
						return []string{"tests/test_db.py::test_query"}
					}
					return nil
				}
			},
			reasonHas: "import-time-only",
			wantHas:   "tests/test_db.py::test_query",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInputs()
			tc.mutate(&in)
			got := Select(in)
			if got.Tier != TierT1 {
				t.Fatalf("Tier = %v (%s), want TierT1", got.Tier, got.Reason)
			}
			if !contains(got.Reason, tc.reasonHas) {
				t.Errorf("Reason = %q, want it to mention %q", got.Reason, tc.reasonHas)
			}
			found := false
			for _, id := range got.Tests {
				if id == tc.wantHas {
					found = true
				}
			}
			if !found {
				t.Errorf("Tests = %#v, want it to contain %q", got.Tests, tc.wantHas)
			}
		})
	}
}

func TestSelectT1WithNothingToSelectIsStillEmpty(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("config/unmapped.yaml")}

	got := Select(in)

	if got.Tier != TierEmpty {
		t.Errorf("Tier = %v (%s), want TierEmpty: an escalation that selects nothing is still nothing",
			got.Tier, got.Reason)
	}
	if got.Reason == "" {
		t.Error("Reason is empty")
	}
}

// A fresh row must NOT escalate, or every selection becomes T1.
func TestSelectFreshRowsDoNotEscalate(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("src/auth.py")}
	in.Distance = func(string) int { return 50 } // exactly at the limit, not over it

	got := Select(in)

	if got.Tier != TierT0 {
		t.Errorf("Tier = %v (%s), want TierT0 at exactly stale_commits", got.Tier, got.Reason)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && strings.Contains(haystack, needle)
}

// With no adapter, nothing can be classified as a test file. Saying "no test file
// changed" then states a fact the selector never checked — and one that is false here.
func TestSelectEmptyReasonDoesNotDenyATestFileItCouldNotClassify(t *testing.T) {
	in := baseInputs()
	in.Adapter = nil
	in.Changes = []gitctx.Change{added("tests/test_new.py")}

	got := Select(in)

	if got.Tier != TierEmpty {
		t.Fatalf("Tier = %v, want TierEmpty", got.Tier)
	}
	if strings.Contains(got.Reason, "no test file changed") {
		t.Errorf("Reason = %q: a test file did change; classification was disabled, not negative", got.Reason)
	}
	if !strings.Contains(got.Reason, "adapter") {
		t.Errorf("Reason = %q, want it to name the missing adapter as the cause", got.Reason)
	}
}

// A deleted test file did change. It cannot be executed, which is a different fact.
func TestSelectEmptyReasonNamesADeletedTestFile(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{deleted("tests/test_orphan.py")}

	got := Select(in)

	if got.Tier != TierEmpty {
		t.Fatalf("Tier = %v, want TierEmpty (reason %q)", got.Tier, got.Reason)
	}
	if strings.Contains(got.Reason, "no test file changed") {
		t.Errorf("Reason = %q: tests/test_orphan.py changed - it was deleted", got.Reason)
	}
	if !strings.Contains(got.Reason, "tests/test_orphan.py") {
		t.Errorf("Reason = %q, want it to name the deleted test file", got.Reason)
	}
}

// A Selection's REASON is part of its answer, so it has to be as reproducible as its
// test list. mapstore.TestsCovering walks a Go map, so the order T1's staleness scan sees
// its candidates in is randomised per run; naming "the first stale row" out of that order
// made the reason — and the seeded-repository golden that pins it — differ between runs
// on identical inputs. The row named is the lexicographically first stale one, every run.
func TestSelectStaleReasonNamesTheSameRowEveryRun(t *testing.T) {
	rows := []mapstore.Row{
		{T: "tests/test_a.py::test_one", F: []string{"src/auth.py"}, C: "aaa1111", D: 1, S: "pass"},
		{T: "tests/test_b.py::test_one", F: []string{"src/auth.py"}, C: "aaa1111", D: 2, S: "pass"},
		{T: "tests/test_c.py::test_one", F: []string{"src/auth.py"}, C: "aaa1111", D: 3, S: "pass"},
		{T: "tests/test_d.py::test_one", F: []string{"src/auth.py"}, C: "aaa1111", D: 4, S: "pass"},
		{T: "tests/test_e.py::test_one", F: []string{"src/auth.py"}, C: "aaa1111", D: 5, S: "pass"},
		{T: "tests/test_f.py::test_one", F: []string{"src/auth.py"}, C: "aaa1111", D: 6, S: "pass"},
	}
	const want = "row tests/test_a.py::test_one is 999 commits stale (limit 50)"

	for i := 0; i < 20; i++ {
		in := baseInputs()
		in.Map = mapOf(rows...)
		in.Changes = []gitctx.Change{mod("src/auth.py")}
		in.Distance = func(string) int { return 999 }

		if got := Select(in).Reason; got != want {
			t.Fatalf("run %d: Reason = %q, want %q: map iteration order reached the reason",
				i, got, want)
		}
	}
}

// The static tier answers where the coverage relation cannot: no map, an adapter that
// declares selection: static, and a test_for template naming a file that exists.
// Both admitting levels reach the selection, and the fourth test — near the change but
// vouched for by neither level — does not.
func TestSelectResolvesTS(t *testing.T) {
	in := staticInputs()

	got := Select(in)

	if got.Tier != TierTS {
		t.Fatalf("Tier = %v (%s), want TierTS", got.Tier, got.Reason)
	}
	want := []string{
		"src/auth/token.test.ts",   // level 1, declared correspondence
		"src/auth/session.test.ts", // level 2, 1 hop
		"src/api/gateway.test.ts",  // level 2, 3 hops
	}
	if len(got.Tests) != len(want) {
		t.Fatalf("Tests = %#v, want the three related tests and not the fourth", got.Tests)
	}
	for i := range want {
		if got.Tests[i] != want[i] {
			t.Fatalf("Tests = %#v, want %#v: correspondence first, then shortest import path",
				got.Tests, want)
		}
	}
}

// The whole point of spec §2's two-axis split: a static selection must never describe
// itself with the words an execution-derived one uses.
func TestSelectTSReasonNamesItsEvidenceAndNotCoverage(t *testing.T) {
	got := Select(staticInputs())
	if contains(got.Reason, "recorded coverage") {
		t.Errorf("Reason = %q, must not contain %q", got.Reason, "recorded coverage")
	}
	if !contains(got.Reason, "correspondence") && !contains(got.Reason, "import") {
		t.Errorf("Reason = %q, want it to name correspondence or imports", got.Reason)
	}
}

// PRD #230 AC7. An adapter's importscan key is optional by contract, so an adapter that
// omits it is not a broken adapter: level 2 is skipped, the other levels still produce a
// TS selection, and no error, no warning and no escalation follows from the absence.
func TestSelectWithoutImportscanIsSkippedNotFatal(t *testing.T) {
	in := staticInputs()
	in.ImportDistance = nil // the adapter declares no importscan

	got := Select(in)

	if got.Tier != TierTS {
		t.Fatalf("Tier = %v (%s), want TierTS: a missing importscan skips a level, "+
			"it does not escalate", got.Tier, got.Reason)
	}
	if len(got.Tests) != 1 || got.Tests[0] != "src/auth/token.test.ts" {
		t.Errorf("Tests = %#v, want only the corresponding test", got.Tests)
	}
	for _, unwanted := range []string{"importscan", "error", "recorded coverage"} {
		if contains(got.Reason, unwanted) {
			t.Errorf("Reason = %q, must not contain %q: the absence is not a fault",
				got.Reason, unwanted)
		}
	}
}

// A selection resting on imports alone says so, and still never borrows the words an
// execution-derived selection uses.
func TestSelectTSImportDerivedReasonNamesImports(t *testing.T) {
	in := staticInputs()
	ad := staticFixtureAdapter()
	ad.TestFor = nil // imports are the only evidence there is
	ad.Importscan = &adapter.Importscan{Command: "node {script}", Script: "scan.js"}
	in.Adapter = ad

	got := Select(in)

	if got.Tier != TierTS {
		t.Fatalf("Tier = %v (%s), want TierTS", got.Tier, got.Reason)
	}
	if !contains(got.Reason, "import") {
		t.Errorf("Reason = %q, want it to name imports", got.Reason)
	}
	if contains(got.Reason, "recorded coverage") {
		t.Errorf("Reason = %q, must not contain %q", got.Reason, "recorded coverage")
	}
	want := []string{"src/auth/session.test.ts", "src/api/gateway.test.ts"}
	for i := range want {
		if got.Tests[i] != want[i] {
			t.Fatalf("Tests = %#v, want %#v: shortest import path first", got.Tests, want)
		}
	}
}

// Decision 2 of docs/plans/06-m6b-static-tier.md: fidelity none declares selection:
// static, so the gate opens, but it can produce no candidate and path proximity may not
// fill the gap. The honest answer is the full suite at T2, with a reason naming what the
// adapter is missing — the same fact its rtdd doctor line reports.
func TestSelectFidelityNoneIsAFullSuiteThatSaysWhy(t *testing.T) {
	in := staticInputs()
	ad := staticFixtureAdapter()
	ad.TestFor = nil
	in.Adapter = ad
	in.ImportDistance = nil // no test_for and no importscan: fidelity none

	got := Select(in)

	if got.Tier != TierT2 {
		t.Fatalf("Tier = %v (%s), want TierT2", got.Tier, got.Reason)
	}
	if len(got.Tests) != len(in.AllTests) {
		t.Errorf("Tests = %#v, want the full suite", got.Tests)
	}
	for _, want := range []string{"typescript", "test_for", "importscan"} {
		if !contains(got.Reason, want) {
			t.Errorf("Reason = %q, want it to mention %q", got.Reason, want)
		}
	}
	if contains(got.Reason, "recorded coverage") {
		t.Errorf("Reason = %q, must not contain %q", got.Reason, "recorded coverage")
	}
}

// A static adapter that HAS declarations but finds nothing for this particular change is
// also the full suite, with its own reason — never an empty TS, which would report a tier
// that narrowed nothing.
func TestSelectAStaticAdapterThatFindsNothingIsAFullSuite(t *testing.T) {
	in := staticInputs()
	in.Exists = existsIn() // the templates expand, and name nothing that is there
	in.ImportDistance = func(string) map[string]int { return nil }

	got := Select(in)

	if got.Tier != TierT2 {
		t.Fatalf("Tier = %v (%s), want TierT2", got.Tier, got.Reason)
	}
	if len(got.Tests) != len(in.AllTests) {
		t.Errorf("Tests = %#v, want the full suite", got.Tests)
	}
	if contains(got.Reason, "recorded coverage") {
		t.Errorf("Reason = %q, must not contain %q", got.Reason, "recorded coverage")
	}
}

// A seeded map that covers nothing in the changed set is an honest empty. Turning it into
// a speculative static selection would replace a true "nothing is related" with a guess:
// TS is for a map that CANNOT answer, not for one that answered zero.
func TestSelectASeededMapSelectingNothingStaysEmpty(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("src/untouched.py")}
	in.Exists = existsIn("tests/test_untouched.py")
	in.Adapter = fixtureAdapter()
	in.Adapter.TestFor = []string{"tests/test_{name}.py"}

	got := Select(in)

	if got.Tier != TierEmpty {
		t.Fatalf("Tier = %v (%s), want TierEmpty: a seeded map answered", got.Tier, got.Reason)
	}
	if len(got.Tests) != 0 {
		t.Errorf("Tests = %#v, want empty", got.Tests)
	}
}

// An unseeded COVERAGE adapter keeps today's answer word for word — seeding really is
// the fix for it, and adapters/python.yaml declares no static capability.
func TestSelectAnUnseededCoverageRepoIsUnchanged(t *testing.T) {
	in := baseInputs()
	in.Map = mapstore.New()
	in.Changes = []gitctx.Change{mod("src/auth.py")}
	in.AllTests = []string{"tests/test_auth.py::test_login"}

	got := Select(in)

	if got.Tier != TierT2 {
		t.Fatalf("Tier = %v, want TierT2", got.Tier)
	}
	const want = "the map is unseeded, so no selection is trustworthy: run rtdd seed"
	if got.Reason != want {
		t.Errorf("Reason = %q, want %q", got.Reason, want)
	}
}

// --- T2 escalation must not stick for a whole session (the drift finding) --------
//
// `full_escalate` was evaluated against the working diff alone, and in an agent's
// inner loop that diff only ever grows: one `conftest.py` edit entered it and every
// later cycle re-escalated to T2 on the same edit. Measured on the replay corpus,
// that was 24 of 25 flask cycles and 16 of 16 httpie cycles pinned to the full
// suite — RTDD telling the agent to run everything, which is the behaviour it
// exists to replace.
//
// The escalation is about whether a full run has happened SINCE the config reached
// its current state, not about whether the diff mentions it. `EscalateDigest` is
// that state now; `EscalateDigestAtLastFull` is what meta.json recorded when the
// last full run completed. Equal means the full run already covered this config.

func TestAFullEscalateFileEscalatesWhenNoFullRunHasCoveredIt(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("src/auth.py"), mod("tests/conftest.py")}
	in.EscalateDigest = "sha256:cfg-after-the-edit"
	in.EscalateDigestAtLastFull = "sha256:cfg-before-the-edit"
	in.AllTests = []string{"tests/test_auth.py::test_login", "tests/test_db.py::test_query"}

	got := Select(in)
	if got.Tier != TierT2 {
		t.Fatalf("Tier = %v, want T2: the config changed since the last full run", got.Tier)
	}
	if !strings.Contains(got.Reason, "conftest.py") {
		t.Errorf("Reason = %q, want it to name the file that forced the full run", got.Reason)
	}
}

func TestTheSameFullEscalateEditDoesNotEscalateTwice(t *testing.T) {
	// The full run has happened; the conftest edit is still in the working diff,
	// because nothing has been committed. A second full suite buys nothing.
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("src/auth.py"), mod("tests/conftest.py")}
	in.EscalateDigest = "sha256:cfg-after-the-edit"
	in.EscalateDigestAtLastFull = "sha256:cfg-after-the-edit"
	in.AllTests = []string{"tests/test_auth.py::test_login", "tests/test_db.py::test_query"}

	got := Select(in)
	if got.Tier == TierT2 {
		t.Fatalf("Tier = T2 (%s); a full run already covered this config state", got.Reason)
	}
	if len(got.Tests) == 0 {
		t.Fatal("selection is empty; the map should still answer for src/auth.py")
	}
}

func TestASecondFullEscalateEditEscalatesAgain(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("tests/conftest.py")}
	in.EscalateDigest = "sha256:cfg-edited-again"
	in.EscalateDigestAtLastFull = "sha256:cfg-after-the-first-edit"
	in.AllTests = []string{"tests/test_auth.py::test_login"}

	if got := Select(in); got.Tier != TierT2 {
		t.Fatalf("Tier = %v, want T2: the config moved again since the last full run", got.Tier)
	}
}

func TestAMapSeededBeforeDigestsWereRecordedStillEscalates(t *testing.T) {
	// Back-compat: meta.json written by an older rtdd carries no digest. Treating
	// "unknown" as "already covered" would silently stop escalating on every
	// repository that upgraded, so an absent record escalates exactly as before.
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("pyproject.toml")}
	in.EscalateDigest = "sha256:cfg-now"
	in.EscalateDigestAtLastFull = ""
	in.AllTests = []string{"tests/test_auth.py::test_login"}

	if got := Select(in); got.Tier != TierT2 {
		t.Fatalf("Tier = %v, want T2: no full run is on record for this config", got.Tier)
	}
}

func TestTheDriftGuardStillEscalatesWhateverTheDigestSays(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("src/auth.py")}
	in.Cycles = in.Cfg.DriftGuard
	in.EscalateDigest = "sha256:cfg"
	in.EscalateDigestAtLastFull = "sha256:cfg"
	in.AllTests = []string{"tests/test_auth.py::test_login"}

	got := Select(in)
	if got.Tier != TierT2 {
		t.Fatalf("Tier = %v, want T2: the drift guard is a separate rule", got.Tier)
	}
	if !strings.Contains(got.Reason, "drift guard") {
		t.Errorf("Reason = %q, want the drift-guard wording", got.Reason)
	}
}

// TestAnUncommittedSessionStopsPinningTheFullSuite replays the shape of the drift
// session that exposed this: cycles accumulate, nothing is committed, so the changed
// set only grows and the conftest.py edit from cycle 2 is still in it at cycle 25.
//
// Before the fix this asserted-on loop produced T2 on every cycle from 2 onward,
// matching the measured curve (flask 24/25, httpie 16/16). The rule is not "never
// escalate" — cycle 2 still runs the full suite, because the config really did change
// and nothing had covered it. It is "escalate once".
func TestAnUncommittedSessionStopsPinningTheFullSuite(t *testing.T) {
	const cycles = 25
	changes := []gitctx.Change{mod("src/auth.py")}
	lastFull := "" // meta.json: no full run on record yet

	var tiers []Tier
	for cycle := 1; cycle <= cycles; cycle++ {
		if cycle == 2 {
			// The agent edits conftest.py once. It stays in the diff forever after,
			// because nothing in this session is ever committed.
			changes = append(changes, mod("tests/conftest.py"))
		}
		if cycle >= 5 {
			// And keeps touching ordinary source, as an agent does.
			changes = append(changes, mod("src/db.py"))
		}

		in := baseInputs()
		in.Changes = changes
		in.AllTests = []string{"tests/test_auth.py::test_login", "tests/test_db.py::test_query"}
		in.EscalateDigest = "sha256:conftest-v2" // unchanged: one edit, never re-edited
		if cycle == 1 {
			in.EscalateDigest = "" // nothing full-escalate is changed yet
		}
		in.EscalateDigestAtLastFull = lastFull

		got := Select(in)
		tiers = append(tiers, got.Tier)
		if got.Tier == TierT2 {
			lastFull = in.EscalateDigest // the full run covered this config
		}
	}

	full := 0
	for _, tr := range tiers {
		if tr == TierT2 {
			full++
		}
	}
	if full != 1 {
		t.Fatalf("ran the full suite on %d of %d cycles, want exactly 1; tiers = %v", full, cycles, tiers)
	}
	if tiers[1] != TierT2 {
		t.Errorf("cycle 2 tier = %v, want T2: the config changed and nothing had covered it", tiers[1])
	}
	if tiers[len(tiers)-1] == TierT2 {
		t.Error("the last cycle is still T2; the escalation is still sticky")
	}
}
