package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// Decision 6: seeding a mixed repo seeds the coverage half and NAMES the static half. The
// existing refusal is for a repo whose coverage half is empty; refusing here would strand
// the map the Python half genuinely needs.
func TestSeedInAMixedRepoSeedsTheCoverageAdaptersAndNamesTheStaticOnes(t *testing.T) {
	detected := []*adapter.Adapter{
		{Name: "python", Selection: adapter.SelectionCoverage, Coverage: "sqlite", Seed: "pytest --cov"},
		{Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone},
	}
	static, coverage := selectionSplit(detected)
	if len(static) != 1 || len(coverage) != 1 {
		t.Fatalf("selectionSplit = static %v, coverage %v, want one each", adapterNames(static), adapterNames(coverage))
	}

	plan, msg := seedPlan(detected)
	if len(plan) != 1 || plan[0].Name != "python" {
		t.Fatalf("seedPlan = %v, want only the coverage adapter", adapterNames(plan))
	}
	for _, want := range []string{"vitest", "no map is ever built", "python"} {
		if !strings.Contains(msg, want) {
			t.Errorf("seed message %q does not name %q", msg, want)
		}
	}
}

// The all-static repo keeps the existing exit-2 refusal: there is genuinely nothing to do.
func TestSeedInAnAllStaticRepoStillRefuses(t *testing.T) {
	detected := []*adapter.Adapter{
		{Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone},
		{Name: "maven", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone},
	}
	plan, msg := seedPlan(detected)
	if len(plan) != 0 {
		t.Fatalf("seedPlan = %v, want nothing to seed", adapterNames(plan))
	}
	if !strings.Contains(msg, "vitest") || !strings.Contains(msg, "maven") {
		t.Errorf("refusal %q must name both static adapters", msg)
	}
}

// A repository with no static half is seeded with no extra sentence at all: the mixed-case
// line exists to scope guidance away from an adapter that records nothing, and there is
// no such adapter here. Printing it anyway would change every single-toolchain repo's
// output for no reader.
func TestSeedInACoverageOnlyRepoSaysNothingAboutStaticAdapters(t *testing.T) {
	detected := []*adapter.Adapter{
		{Name: "python", Selection: adapter.SelectionCoverage, Coverage: "sqlite"},
	}
	plan, msg := seedPlan(detected)
	if len(plan) != 1 || plan[0].Name != "python" {
		t.Fatalf("seedPlan = %v, want the one coverage adapter", adapterNames(plan))
	}
	if msg != "" {
		t.Errorf("seedPlan message = %q, want \"\" — no static adapter to scope away from", msg)
	}
}

// The all-static refusal for ONE adapter is the string #257 shipped, byte for byte: the
// plural rendering may not churn the message every existing static repository already
// reads.
func TestSeedRefusalForASingleStaticAdapterIsByteIdentical(t *testing.T) {
	ad := &adapter.Adapter{Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone}
	_, got := seedPlan([]*adapter.Adapter{ad})
	if want := staticSeedRefusal(ad); got != want {
		t.Errorf("seedPlan refusal =\n%q\nwant\n%q", got, want)
	}
}

// End to end, on a real mixed repository: a pytest project with a host vitest adapter
// beside it. `seed` runs the Python half and says plainly that the vitest half records
// nothing — it neither refuses outright nor pretends to have seeded both (PRD #230 AC9d).
func TestCmdSeedInAMixedRepositorySeedsPythonAndNamesVitest(t *testing.T) {
	repo := realRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte("{\n  \"name\": \"demo\"\n}\n"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	writeVitestAdapter(t, repo, "")
	chdir(t, repo)

	var out, errBuf strings.Builder
	// Exit 1, not 2: the fixture has one deliberate failure, and a mixed repository is
	// not a configuration error.
	if code := cmdSeed(nil, &out, &errBuf); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1 (stdout: %s, stderr: %s)", code, out.String(), errBuf.String())
	}
	stdout := out.String()
	if !strings.Contains(stdout, "seeding with the python adapter") {
		t.Errorf("seed did not seed the coverage half:\n%s", stdout)
	}
	for _, want := range []string{"vitest", "no map is ever built"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("seed output does not say %q of the static half:\n%s", want, stdout)
		}
	}

	// The map is the Python half's, and every row says so.
	rows := readMapJSONL(t, repo)
	if len(rows) == 0 {
		t.Fatal("seed wrote no map in a mixed repository")
	}
	mt, err := readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if mt.Adapter != "python" {
		t.Errorf("meta.adapter = %q, want the coverage adapter that produced the map", mt.Adapter)
	}
	if strings.Join(mt.Adapters, ",") != "python,vitest" {
		t.Errorf("meta.adapters = %v, want the whole detected set", mt.Adapters)
	}
}

// A wholly-static repository still refuses with exit 2 and writes nothing, as #257
// established — and the refusal names every adapter it is talking about.
func TestCmdSeedInAnAllStaticRepositoryStillExitsTwo(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "")
	chdir(t, dir)

	var out, errBuf strings.Builder
	if code := cmdSeed(nil, &out, &errBuf); code != 2 {
		t.Fatalf("cmdSeed = %d, want 2 (stdout: %s)", code, out.String())
	}
	if !strings.Contains(errBuf.String(), "vitest") {
		t.Errorf("refusal does not name the adapter:\n%s", errBuf.String())
	}
	if strings.Contains(out.String(), "seeding with the") {
		t.Errorf("seed announced a run it must never start:\n%s", out.String())
	}
	if _, err := os.Stat(mapPath(dir)); err == nil {
		t.Error(".rtdd/map.jsonl was written by a refused seed")
	}
}
