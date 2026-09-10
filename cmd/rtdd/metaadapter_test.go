package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// legacyRepoAdapters is the polyglot repository of issue #336: a Java module that records
// nothing beside a Python one that does. `maven` sorts — and detects — FIRST, which is the
// whole point: the singular `adapter` may not be decided by name order.
func legacyRepoAdapters() []*adapter.Adapter {
	return []*adapter.Adapter{
		{
			Name: "maven", Detect: []string{"pom.xml"},
			Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone,
			TestGlobs: []string{"src/test/java/**/*.java"}, SourceGlobs: []string{"src/main/java/**/*.java"},
			TestFor: []string{"src/test/java/{name}Test.java"},
		},
		{
			Name: "python", Detect: []string{"pyproject.toml"},
			Selection: adapter.SelectionCoverage, Coverage: "sqlite",
			TestGlobs: []string{"tests/**/*.py"}, SourceGlobs: []string{"src/**/*.py"},
		},
	}
}

// Decision 4 of docs/plans/06-m6d-shipped-adapters.md: meta.json's singular `adapter` is
// the COVERAGE adapter that produced the map. `run` used to write whichever adapter sorted
// first, which in this repository is a static one that produced no map at all.
func TestRunNamesTheCoverageAdapterInMetaNotWhicheverSortsFirst(t *testing.T) {
	got := metaAfterRun(meta{V: 1, Adapters: []string{"maven", "python"}}, legacyRepoAdapters(), selector.TierT0, "")
	if got.Adapter != "python" {
		t.Errorf("meta.adapter after run = %q, want %q — the singular field names the coverage adapter, "+
			"never whichever name sorts first", got.Adapter, "python")
	}
}

// AC1: in a repository whose adapters are ALL static the field stays empty. There is no
// map for it to speak for, and a name here would claim every untagged row for a runner
// that never recorded one.
func TestMetaAdapterStaysEmptyWhenEveryAdapterIsStatic(t *testing.T) {
	ads := []*adapter.Adapter{
		{Name: "maven", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone},
		{Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone},
	}
	if got := metaAfterRun(meta{V: 1}, ads, selector.TierT0, "").Adapter; got != "" {
		t.Errorf("meta.adapter after run = %q, want \"\" — no adapter here records coverage", got)
	}
}

// AC2: the two writers of the field derive it through one helper, so they cannot drift.
// seed's value is plan[0].Name over the coverage half; run's must be the same string.
func TestRunAndSeedAgreeOnTheMapsAdapter(t *testing.T) {
	detected := legacyRepoAdapters()
	plan, _ := seedPlan(detected)
	if len(plan) == 0 {
		t.Fatal("seedPlan produced nothing; the comparison below would be vacuous")
	}
	if got, want := metaAfterRun(meta{V: 1}, detected, selector.TierT0, "").Adapter, plan[0].Name; got != want {
		t.Errorf("run writes adapter %q, seed writes %q; the two must agree", got, want)
	}
	if got := coverageAdapterName(detected); got != plan[0].Name {
		t.Errorf("coverageAdapterName = %q, seed's own value is %q", got, plan[0].Name)
	}
}

// An adapter meta ALREADY names is never rewritten: the field says whose the untagged rows
// of an existing map are, and a repository that gains a second toolchain must not have
// that answer changed underneath it.
func TestRunLeavesAnAlreadyNamedAdapterAlone(t *testing.T) {
	got := metaAfterRun(meta{V: 1, Adapter: "python"}, legacyRepoAdapters(), selector.TierT0, "")
	if got.Adapter != "python" {
		t.Errorf("meta.adapter = %q, want the name already recorded", got.Adapter)
	}
}

// A meta written before `v` existed still gets one; the run must not write a versionless
// document back.
func TestRunStampsTheSchemaVersionOnAVersionlessMeta(t *testing.T) {
	if got := metaAfterRun(meta{}, legacyRepoAdapters(), selector.TierT0, "").V; got != 1 {
		t.Errorf("meta.v after run = %d, want 1", got)
	}
}

// PRD #232 AC6, end to end over the read path: untagged legacy rows, a meta.json naming no
// adapter, and a static adapter that sorts first. After the run's meta write, the pytest
// nodeids must still reach python and must NEVER reach maven — `mvn -Dtest=tests/test_a.py`
// matches nothing and exits 0, a false pass wearing a real id.
func TestUntaggedLegacyRowsAreNotServedToAStaticAdapterAfterARun(t *testing.T) {
	ads := legacyRepoAdapters()
	m := mapstore.New()
	m.Replace(mapstore.Row{T: "tests/test_calc.py::test_add", F: []string{"src/calc.py"}, S: "pass"})

	// Exactly what `rtdd run` persists: a meta that named no adapter, plus this run's fix-up.
	mt := metaAfterRun(meta{V: 1, Adapters: []string{"maven", "python"}}, ads, selector.TierT0, "")

	if got := rowsVisibleTo(ads[0], ads, m, mt); got.Len() != 0 {
		t.Errorf("maven is served %d untagged row(s); a static adapter recorded none of them", got.Len())
	}
	if got := rowsVisibleTo(ads[1], ads, m, mt); got.Len() != 1 {
		t.Errorf("python is served %d rows, want its own 1 — the legacy map is python's", got.Len())
	}

	blocks, err := selectPerAdapter(t.TempDir(), ads, m, mt, selectionContext{
		Changes: []gitctx.Change{{Path: "src/calc.py", Status: gitctx.Modified}},
	})
	if err != nil {
		t.Fatalf("selectPerAdapter: %v", err)
	}
	selected := map[string][]string{}
	for _, blk := range blocks {
		selected[blk.Adapter] = blk.Selection.Tests
	}
	for _, id := range selected["maven"] {
		if id == "tests/test_calc.py::test_add" {
			t.Errorf("maven's selection contains the untagged pytest id %q", id)
		}
	}
	if len(selected["python"]) == 0 {
		t.Error("python selected nothing; the legacy map is its own and the assertion above would be vacuous")
	}
}
