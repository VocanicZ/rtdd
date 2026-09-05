package selector

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

func staticFixtureAdapter() *adapter.Adapter {
	return &adapter.Adapter{
		Name:      "typescript",
		Selection: adapter.SelectionStatic,
		Coverage:  adapter.CoverageNone,
		TestGlobs: []string{"**/*.test.ts"},
		TestFor:   []string{"{dir}/{name}.test.ts"},
	}
}

func existsIn(paths ...string) func(string) bool {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return func(p string) bool { return set[p] }
}

// The fixture repository:
//
//	src/auth/token.ts           changed
//	src/auth/token.test.ts      corresponds to it             -> level 1
//	src/auth/session.test.ts    imports token.ts, 1 hop       -> level 2
//	src/api/gateway.test.ts     imports token.ts, 3 hops      -> level 2
//	src/auth/unrelated.test.ts  same directory, neither
//	                            corresponds nor imports       -> NOT a candidate
func staticInputs() Inputs {
	return Inputs{
		Adapter: staticFixtureAdapter(),
		Cfg:     DefaultConfig(),
		Changes: []gitctx.Change{mod("src/auth/token.ts")},
		AllTests: []string{
			"src/api/gateway.test.ts",
			"src/auth/session.test.ts",
			"src/auth/token.test.ts",
			"src/auth/unrelated.test.ts",
		},
		Exists: existsIn(
			"src/api/gateway.test.ts",
			"src/auth/session.test.ts",
			"src/auth/token.test.ts",
			"src/auth/unrelated.test.ts",
		),
		ImportDistance: func(changed string) map[string]int {
			if changed != "src/auth/token.ts" {
				return nil
			}
			return map[string]int{
				"src/auth/session.test.ts": 1,
				"src/api/gateway.test.ts":  3,
			}
		},
	}
}

func levelOf(cands []staticCandidate, test string) (staticCandidate, bool) {
	for _, c := range cands {
		if c.Test == test {
			return c, true
		}
	}
	return staticCandidate{}, false
}

func TestStaticCandidatesFindsCorrespondenceAtLevelOne(t *testing.T) {
	got, ok := levelOf(staticCandidates(staticInputs()), "src/auth/token.test.ts")
	if !ok {
		t.Fatal("the corresponding test is not a candidate")
	}
	if got.Level != 1 {
		t.Errorf("Level = %d, want 1 (test_for correspondence)", got.Level)
	}
}

// Correspondence resolves against the repository, so the first template naming a file
// that exists wins and a template naming nothing contributes nothing.
func TestStaticCandidatesTriesTemplatesInDeclarationOrder(t *testing.T) {
	in := staticInputs()
	ad := staticFixtureAdapter()
	ad.TestFor = []string{"{dir}/{name}.test.ts", "tests/{name}.test.ts"}
	in.Adapter = ad
	in.ImportDistance = nil // level 1 alone is under test here
	// The co-located template resolves to nothing here; the central one does.
	in.Exists = existsIn("tests/token.test.ts")

	cands := staticCandidates(in)

	if len(cands) != 1 || cands[0].Test != "tests/token.test.ts" {
		t.Fatalf("candidates = %#v, want the later template's tests/token.test.ts", cands)
	}
}

// Decision 1 of docs/plans/06-m6b-static-tier.md. src/auth/unrelated.test.ts shares the
// changed file's directory — the longest possible shared prefix — and does not
// correspond to it. Path proximity ORDERS a selection; it never admits to one, because
// the level-3 signal alone is the `path` baseline spec §7 pre-registers this tier against.
func TestStaticCandidatesNeverAdmitsOnPathProximityAlone(t *testing.T) {
	if got, ok := levelOf(staticCandidates(staticInputs()), "src/auth/unrelated.test.ts"); ok {
		t.Errorf("path proximity admitted %q at level %d; proximity only orders",
			got.Test, got.Level)
	}
}

// An adapter with no test_for templates has no correspondence to offer. That is a
// skipped level, not a failure: the mirror of a missing importscan, and level 2 still
// answers on its own.
func TestStaticCandidatesWithoutTestForStillFindsImporters(t *testing.T) {
	in := staticInputs()
	ad := staticFixtureAdapter()
	ad.TestFor = nil
	in.Adapter = ad

	cands := staticCandidates(in)

	if len(cands) != 2 {
		t.Fatalf("candidates = %#v, want the two importers", cands)
	}
	for _, c := range cands {
		if c.Level != 2 {
			t.Errorf("%s Level = %d, want 2", c.Test, c.Level)
		}
	}
}

// An adapter with neither declaration answers nothing at all — still not an error:
// Select turns an empty candidate set into an honest T2.
func TestStaticCandidatesWithNeitherDeclarationFindsNothing(t *testing.T) {
	in := staticInputs()
	ad := staticFixtureAdapter()
	ad.TestFor = nil
	in.Adapter = ad
	in.ImportDistance = nil
	if cands := staticCandidates(in); len(cands) != 0 {
		t.Errorf("candidates = %#v, want none without test_for or importscan", cands)
	}
}

// A nil Exists — a caller with no filesystem to ask — skips level 1 rather than
// assuming every expanded template names a real file.
func TestStaticCandidatesWithoutExistsFindsNothing(t *testing.T) {
	in := staticInputs()
	in.Exists = nil
	in.ImportDistance = nil // level 1 alone is under test here
	if cands := staticCandidates(in); len(cands) != 0 {
		t.Errorf("candidates = %#v, want none without an Exists resolver", cands)
	}
}

// A deleted file cannot correspond to anything runnable, and a changed TEST file is the
// direct tier's business, not the static tier's.
func TestStaticCandidatesIgnoresDeletedAndTestFileChanges(t *testing.T) {
	in := staticInputs()
	in.Changes = []gitctx.Change{
		deleted("src/auth/token.ts"),
		mod("src/auth/token.test.ts"),
	}
	if cands := staticCandidates(in); len(cands) != 0 {
		t.Errorf("candidates = %#v, want none", cands)
	}
}

// Level 2 of spec §4.1: a test that transitively imports a changed file is a candidate,
// and it carries the number of hops it was reached in so the ranking can order by it.
func TestStaticCandidatesFindsImportersAtLevelTwoWithTheirDistance(t *testing.T) {
	cands := staticCandidates(staticInputs())
	for _, tc := range []struct {
		test string
		dist int
	}{
		{"src/auth/session.test.ts", 1},
		{"src/api/gateway.test.ts", 3},
	} {
		got, ok := levelOf(cands, tc.test)
		if !ok {
			t.Errorf("%s is not a candidate", tc.test)
			continue
		}
		if got.Level != 2 {
			t.Errorf("%s Level = %d, want 2 (import distance)", tc.test, got.Level)
		}
		if got.Distance != tc.dist {
			t.Errorf("%s Distance = %d, want %d", tc.test, got.Distance, tc.dist)
		}
	}
}

// PRD #230 AC7: an adapter declaring no importscan is not a broken adapter. Level 2 is
// skipped, level 1 still answers, and nothing about it is an error.
func TestStaticCandidatesWithoutImportDistanceStillFindsCorrespondence(t *testing.T) {
	in := staticInputs()
	in.ImportDistance = nil

	cands := staticCandidates(in)

	if len(cands) != 1 || cands[0].Test != "src/auth/token.test.ts" {
		t.Fatalf("candidates = %#v, want only the corresponding test", cands)
	}
	if cands[0].Level != 1 {
		t.Errorf("Level = %d, want 1", cands[0].Level)
	}
}

// A test reachable both ways keeps the more confident level and the import distance it
// was also found at, so the level-2 tiebreak still has a value to use.
func TestStaticCandidatesKeepsTheMoreConfidentLevel(t *testing.T) {
	in := staticInputs()
	in.ImportDistance = func(string) map[string]int {
		return map[string]int{"src/auth/token.test.ts": 4}
	}

	got, ok := levelOf(staticCandidates(in), "src/auth/token.test.ts")

	if !ok {
		t.Fatal("the corresponding test is not a candidate")
	}
	if got.Level != 1 {
		t.Errorf("Level = %d, want 1: correspondence outranks an import path", got.Level)
	}
	if got.Distance != 4 {
		t.Errorf("Distance = %d, want the import distance 4 to be retained", got.Distance)
	}
}

// PRD #230 AC5, and this issue's two levels asserted together against ONE fixture: the
// test fails if import distance is applied out of turn, not only if it is missing.
//
//	src/auth/token.test.ts    level 1                -> first
//	src/auth/session.test.ts  level 2, 1 hop         -> second
//	src/api/gateway.test.ts   level 2, 3 hops        -> third
func TestRankStaticOrdersByLevelThenImportDistance(t *testing.T) {
	cands := []staticCandidate{
		{Test: "src/api/gateway.test.ts", Level: 2, Distance: 3},
		{Test: "src/auth/session.test.ts", Level: 2, Distance: 1},
		{Test: "src/auth/token.test.ts", Level: 1},
	}
	want := []string{
		"src/auth/token.test.ts",
		"src/auth/session.test.ts",
		"src/api/gateway.test.ts",
	}

	got := rankStatic(cands, []string{"src/auth/token.ts"})

	if len(got) != len(want) {
		t.Fatalf("rankStatic = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rankStatic = %#v, want %#v", got, want)
		}
	}
}

// A level-1 candidate that also carries an import distance must not be reordered by it:
// correspondence is the more confident level and settles the comparison first.
func TestRankStaticNeverLetsImportDistanceOutrankCorrespondence(t *testing.T) {
	cands := []staticCandidate{
		{Test: "src/auth/session.test.ts", Level: 2, Distance: 1},
		{Test: "src/auth/token.test.ts", Level: 1, Distance: 9},
	}

	got := rankStatic(cands, []string{"src/auth/token.ts"})

	if got[0] != "src/auth/token.test.ts" {
		t.Errorf("rankStatic = %#v, want the corresponding test first despite its 9 hops", got)
	}
}

// Two candidates alike on every key still have to come out in one order, every run:
// a selection an agent cannot reproduce is a selection it cannot bisect.
func TestRankStaticIsDeterministicOnAFullTie(t *testing.T) {
	cands := []staticCandidate{
		{Test: "src/auth/b.test.ts", Level: 1},
		{Test: "src/auth/a.test.ts", Level: 1},
	}
	for i := 0; i < 5; i++ {
		got := rankStatic(cands, []string{"src/auth/token.ts"})
		if len(got) != 2 || got[0] != "src/auth/a.test.ts" || got[1] != "src/auth/b.test.ts" {
			t.Fatalf("rankStatic = %#v, want lexicographic on a full tie", got)
		}
	}
}

// PRD #230 AC5 whole: ONE fixture that exercises all three of spec §4.1's ranking
// levels, run through the real candidate set rather than a hand-built one, so the test
// fails if any level is applied out of turn — not only if a level is missing.
//
//	src/auth/token.test.ts      level 1                        -> first
//	src/auth/session.test.ts    level 2, 1 hop                 -> second
//	src/auth/far.test.ts        level 2, 3 hops, shares src/auth -> third
//	src/api/gateway.test.ts     level 2, 3 hops, shares src     -> fourth
//	src/auth/unrelated.test.ts  proximity only                 -> not selected at all
//
// far and gateway tie on level AND on distance, and lexicographic order alone would put
// gateway first: only the level-3 proximity key separates them, so the assertion is a
// live one rather than a restatement of the fallback.
func TestRankStaticOrdersByCorrespondenceThenImportsThenProximity(t *testing.T) {
	in := staticInputs()
	in.AllTests = append(in.AllTests, "src/auth/far.test.ts")
	in.Exists = existsIn(
		"src/api/gateway.test.ts",
		"src/auth/far.test.ts",
		"src/auth/session.test.ts",
		"src/auth/token.test.ts",
		"src/auth/unrelated.test.ts",
	)
	in.ImportDistance = func(changed string) map[string]int {
		if changed != "src/auth/token.ts" {
			return nil
		}
		return map[string]int{
			"src/auth/session.test.ts": 1,
			"src/auth/far.test.ts":     3,
			"src/api/gateway.test.ts":  3,
		}
	}

	got := rankStatic(staticCandidates(in), []string{"src/auth/token.ts"})

	want := []string{
		"src/auth/token.test.ts",
		"src/auth/session.test.ts",
		"src/auth/far.test.ts",
		"src/api/gateway.test.ts",
	}
	if len(got) != len(want) {
		t.Fatalf("rankStatic = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rankStatic = %#v, want %#v", got, want)
		}
	}
}

// Level 3 orders; it never admits. The same fixture that pins the three-level order also
// pins the decision behind it (docs/plans/06-m6b-static-tier.md, decision 1): the test
// sharing the changed file's own directory, with neither correspondence nor an import,
// is not in the ranked selection at any position.
func TestRankStaticNeverRanksAProximityOnlyTest(t *testing.T) {
	in := staticInputs()

	for _, test := range rankStatic(staticCandidates(in), []string{"src/auth/token.ts"}) {
		if test == "src/auth/unrelated.test.ts" {
			t.Errorf("proximity alone put %q into the selection; it may only order one", test)
		}
	}
}

// Proximity is the longest shared DIRECTORY prefix, counted in path segments. Counted in
// bytes, "src/authz" and "src/auth" would share eight of them and two unrelated packages
// would rank as neighbours.
func TestRankStaticProximityCountsSegmentsNotBytes(t *testing.T) {
	cands := []staticCandidate{
		{Test: "src/authz/a.test.ts", Level: 2, Distance: 2},
		{Test: "src/b.test.ts", Level: 2, Distance: 2},
	}

	got := rankStatic(cands, []string{"src/auth/token.ts"})

	// Both share exactly one segment ("src") with the changed file, so neither wins on
	// proximity and lexicographic order settles it. A byte-wise prefix would have ranked
	// src/authz first on eight shared characters.
	if got[0] != "src/authz/a.test.ts" || got[1] != "src/b.test.ts" {
		t.Fatalf("rankStatic = %#v, want lexicographic: src/authz shares one segment, not eight bytes", got)
	}
}

// A test in the changed file's own directory outranks a more distant one when the level
// and the import distance are equal — this is the level-3 key doing the only work it is
// allowed to do.
func TestRankStaticProximityBreaksATieOnLevelAndDistance(t *testing.T) {
	cands := []staticCandidate{
		{Test: "src/api/a.test.ts", Level: 2, Distance: 2},
		{Test: "src/auth/z.test.ts", Level: 2, Distance: 2},
	}

	got := rankStatic(cands, []string{"src/auth/token.ts"})

	if got[0] != "src/auth/z.test.ts" {
		t.Fatalf("rankStatic = %#v, want the co-located test first on proximity", got)
	}
}

// Proximity is a TIEBREAK, never a promotion: a level-1 candidate stays ahead of a
// level-2 one that sits in the changed file's directory, and a shorter import path stays
// ahead of a nearer file within level 2.
func TestRankStaticProximityNeverOutranksLevelOrDistance(t *testing.T) {
	cands := []staticCandidate{
		{Test: "src/auth/near.test.ts", Level: 2, Distance: 5},
		{Test: "far/away/corresponds.test.ts", Level: 1},
		{Test: "src/auth/nearer.test.ts", Level: 2, Distance: 9},
		{Test: "far/away/imports.test.ts", Level: 2, Distance: 1},
	}

	got := rankStatic(cands, []string{"src/auth/token.ts"})

	want := []string{
		"far/away/corresponds.test.ts",
		"far/away/imports.test.ts",
		"src/auth/near.test.ts",
		"src/auth/nearer.test.ts",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rankStatic = %#v, want %#v", got, want)
		}
	}
}
