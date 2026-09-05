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

// The static tier answers where the coverage relation cannot: no map, an adapter that
// declares selection: static, and a test_for template naming a file that exists.
func TestSelectResolvesTS(t *testing.T) {
	in := staticInputs()

	got := Select(in)

	if got.Tier != TierTS {
		t.Fatalf("Tier = %v (%s), want TierTS", got.Tier, got.Reason)
	}
	if len(got.Tests) != 1 || got.Tests[0] != "src/auth/token.test.ts" {
		t.Errorf("Tests = %#v, want only the corresponding test", got.Tests)
	}
}

// The whole point of spec §2's two-axis split: a static selection must never describe
// itself with the words an execution-derived one uses.
func TestSelectTSReasonNamesItsEvidenceAndNotCoverage(t *testing.T) {
	got := Select(staticInputs())
	if contains(got.Reason, "recorded coverage") {
		t.Errorf("Reason = %q, must not contain %q", got.Reason, "recorded coverage")
	}
	if !contains(got.Reason, "correspondence") {
		t.Errorf("Reason = %q, want it to name correspondence", got.Reason)
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
