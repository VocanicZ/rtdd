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
//	src/auth/token.test.ts      corresponds to it            -> level 1
//	src/auth/unrelated.test.ts  same directory, no declared
//	                            correspondence               -> NOT a candidate
//	src/auth/session.test.ts    neither                       -> NOT a candidate
//	src/api/gateway.test.ts     neither                       -> NOT a candidate
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
// skipped level, not a failure: Select turns an empty candidate set into an honest T2.
func TestStaticCandidatesWithoutTestForFindsNothing(t *testing.T) {
	in := staticInputs()
	ad := staticFixtureAdapter()
	ad.TestFor = nil
	in.Adapter = ad
	if cands := staticCandidates(in); len(cands) != 0 {
		t.Errorf("candidates = %#v, want none without test_for templates", cands)
	}
}

// A nil Exists — a caller with no filesystem to ask — skips level 1 rather than
// assuming every expanded template names a real file.
func TestStaticCandidatesWithoutExistsFindsNothing(t *testing.T) {
	in := staticInputs()
	in.Exists = nil
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
