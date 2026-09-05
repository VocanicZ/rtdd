package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// The three fixtures the seed-advice surfaces are asked about. They are adapters, not
// names: whether an import scan may be named at all depends on the DECLARATION, which is
// the #275 rule this issue extends onto the uncovered-line surface.
func staticAdapter() *adapter.Adapter {
	return &adapter.Adapter{
		Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone,
		TestFor: []string{"{dir}/{name}.test.ts"},
	}
}

func staticAdapterWithScan() *adapter.Adapter {
	a := staticAdapter()
	a.Importscan = &adapter.Importscan{}
	return a
}

func coverageAdapter() *adapter.Adapter {
	return &adapter.Adapter{Name: "python", Selection: adapter.SelectionCoverage, Coverage: "sqlite"}
}

// forbiddenSeedAdvice is the substring AC9c forbade on init/seed/reason and this issue
// forbids on doctor/explain/which. Both spellings, because the two surfaces capitalise
// differently and a check for one alone passes on the other.
func assertNoSeedAdvice(t *testing.T, surface, out string) {
	t.Helper()
	for _, bad := range []string{"run `rtdd seed`", "Run `rtdd seed`", "run rtdd seed", "Run rtdd seed"} {
		if strings.Contains(out, bad) {
			t.Errorf("%s told a selection: static repository to %q:\n%s", surface, bad, out)
		}
	}
}

// --- doctor -----------------------------------------------------------------

// The defect: doctor states `coverage: none — nothing is recorded` and then advises the
// command whose only job is to record. An empty map is the steady state for a static
// adapter, not a missing setup step.
func TestRenderDoctorEmptyMapInAStaticOnlyRepoDoesNotAdviseSeeding(t *testing.T) {
	got := RenderDoctor(nil, 0, 20, []*adapter.Adapter{staticAdapter()})
	assertNoSeedAdvice(t, "doctor", got)
	if !strings.Contains(got, "vitest") {
		t.Errorf("the fan-out line does not name the adapter that records nothing:\n%s", got)
	}
	if !strings.Contains(got, "fan-out:") {
		t.Errorf("doctor dropped the fan-out line entirely:\n%s", got)
	}
	if !strings.Contains(got, "rtdd which") {
		t.Errorf("the fan-out line names no command that CAN answer:\n%s", got)
	}
}

// A coverage repository is unchanged byte for byte — including the pre-existing callers
// that pass no adapters at all.
func TestRenderDoctorEmptyMapWithACoverageAdapterIsByteIdentical(t *testing.T) {
	const want = "fan-out: the map is empty. Run `rtdd seed` first.\n"
	for _, detected := range [][]*adapter.Adapter{nil, {coverageAdapter()}} {
		got := RenderDoctor(nil, 0, 20, detected)
		if !strings.HasPrefix(got, want) {
			t.Fatalf("RenderDoctor()\n got:\n%s\nwant prefix:\n%s", got, want)
		}
	}
}

// A polyglot repository keeps the guidance, scoped to the adapter it applies to — the
// same shape as RenderNextStep in init.go.
func TestRenderDoctorEmptyMapInAPolyglotRepoScopesTheSeedGuidance(t *testing.T) {
	got := RenderDoctor(nil, 0, 20, []*adapter.Adapter{coverageAdapter(), staticAdapter()})
	if !strings.Contains(got, "rtdd seed") {
		t.Errorf("doctor dropped the seed guidance a coverage adapter still needs:\n%s", got)
	}
	for _, want := range []string{"python", "vitest"} {
		if !strings.Contains(got, want) {
			t.Errorf("the fan-out line does not name %q:\n%s", want, got)
		}
	}
}

// --- explain ----------------------------------------------------------------

func TestRenderExplainEmptyMapInAStaticOnlyRepoNamesWhatAnswersInstead(t *testing.T) {
	got := RenderExplain(mapstore.New(), "src/logic.ts", []*adapter.Adapter{staticAdapter()})
	assertNoSeedAdvice(t, "explain", got)
	if !strings.Contains(got, "test_for") {
		t.Errorf("explain does not name the declared correspondence that answers for this file:\n%s", got)
	}
	if !strings.Contains(got, "rtdd which") {
		t.Errorf("explain names no command that CAN answer:\n%s", got)
	}
	// The #275 rule: a level the adapter does not declare was not consulted, so the
	// output may not describe an import scan that never ran.
	if strings.Contains(got, "import scan") || strings.Contains(got, "importscan") {
		t.Errorf("explain claims an import scan for an adapter that declares none:\n%s", got)
	}
}

// An adapter that DOES declare an importscan may have it named — the level was supplied.
func TestRenderExplainEmptyMapNamesTheImportScanOnlyWhenDeclared(t *testing.T) {
	got := RenderExplain(mapstore.New(), "src/logic.ts", []*adapter.Adapter{staticAdapterWithScan()})
	assertNoSeedAdvice(t, "explain", got)
	if !strings.Contains(got, "import") {
		t.Errorf("explain omits the import evidence the adapter declares:\n%s", got)
	}
}

func TestRenderExplainEmptyMapWithACoverageAdapterIsByteIdentical(t *testing.T) {
	want := "src/hub.py is covered by 0 tests.\n  The map is empty. Run `rtdd seed` first.\n"
	for _, detected := range [][]*adapter.Adapter{nil, {coverageAdapter()}} {
		got := RenderExplain(mapstore.New(), "src/hub.py", detected)
		if got != want {
			t.Fatalf("RenderExplain()\n got:\n%s\nwant:\n%s", got, want)
		}
	}
}

func TestRenderExplainEmptyMapInAPolyglotRepoKeepsTheSeedGuidance(t *testing.T) {
	got := RenderExplain(mapstore.New(), "src/hub.py", []*adapter.Adapter{coverageAdapter(), staticAdapter()})
	if !strings.Contains(got, "rtdd seed") {
		t.Errorf("explain dropped the seed guidance a coverage adapter still needs:\n%s", got)
	}
	if !strings.Contains(got, "vitest") {
		t.Errorf("explain does not scope the guidance away from the static adapter:\n%s", got)
	}
}

// --- which ------------------------------------------------------------------

// Three claims that do not hold for a coverage: none adapter — there is no map for a row
// to be absent from, `import-time-only` is a coverage-attribution concept, and no import
// scan ran. The line carries no information a static selection can act on, so it goes.
func TestRenderWhichSuppressesTheUnmappedNoticeForAStaticAdapter(t *testing.T) {
	sel := selector.Selection{Tier: selector.TierTS, Tests: []string{"src/logic.test.ts"},
		Reason: "static selection: tests whose declared test_for correspondence names the changed set"}
	got := RenderWhich(sel, []string{"src/logic.ts"}, staticAdapter())
	if strings.Contains(got, "no map row covers") {
		t.Errorf("which describes a map row for an adapter that builds no map:\n%s", got)
	}
	if strings.Contains(got, "import-time-only") {
		t.Errorf("which borrows coverage-attribution vocabulary for a static selection:\n%s", got)
	}
	if strings.Contains(got, "import scan") {
		t.Errorf("which claims an import scan the adapter never declared:\n%s", got)
	}
	assertNoSeedAdvice(t, "which", got)
}

// The coverage path is unchanged byte for byte, with an adapter and without one.
func TestRenderWhichKeepsTheUnmappedNoticeForACoverageAdapter(t *testing.T) {
	sel := selector.Selection{Tier: selector.TierT1, Tests: []string{"tests/test_a.py::test_one"}}
	want := "  tier: T1  (1 test selected, ranked)\n" +
		"    tests/test_a.py::test_one\n" +
		"  no map row covers: src/constants.py  (import-time-only or untested; " +
		"tests selected by static import scan)\n"
	for _, ad := range []*adapter.Adapter{nil, coverageAdapter()} {
		got := RenderWhich(sel, []string{"src/constants.py"}, ad)
		if got != want {
			t.Fatalf("RenderWhich()\n got:\n%s\nwant:\n%s", got, want)
		}
	}
}

// --- end to end -------------------------------------------------------------

// AC9c's shape, on the surfaces AC9 did not enumerate: a real static repository, the
// three commands, no seed advice anywhere in what they print.
func TestStaticRepoIsNeverToldToSeedByDoctorExplainOrWhich(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "")
	fixLookPath(t, "npx")

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"doctor", []string{"doctor"}},
		{"explain", []string{"explain", "src/logic.ts"}},
		{"which", []string{"which"}},
	} {
		code, stdout, stderr := rtdd(t, dir, tc.args...)
		if code != 0 {
			t.Fatalf("rtdd %s = %d, want 0 (stderr: %s)", tc.name, code, stderr)
		}
		assertNoSeedAdvice(t, tc.name, stdout)
		assertNoSeedAdvice(t, tc.name, stderr)
	}
}

// The uncovered/no-map-row line, end to end: the reported command output carried three
// claims a coverage: none adapter cannot support.
func TestWhichInAStaticRepoDoesNotDescribeAMapRowOrAnImportScan(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("rtdd which = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, bad := range []string{"no map row covers", "import-time-only", "static import scan"} {
		if strings.Contains(stdout, bad) {
			t.Errorf("which output contains %q for a selection: static adapter:\n%s", bad, stdout)
		}
	}
}
