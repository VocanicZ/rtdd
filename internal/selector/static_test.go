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
