package adapter

import "testing"

func testForAdapter() *Adapter {
	return &Adapter{
		Name:      "typescript",
		Selection: SelectionStatic,
		Coverage:  CoverageNone,
		TestFor: []string{
			"{dir}/{name}.test.ts",
			"{dir}/__tests__/{name}.test.ts",
			"tests/{name}.test.ts",
		},
	}
}

func existsIn(paths ...string) func(string) bool {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return func(p string) bool { return set[p] }
}

func TestTestForCandidateExpandsDirAndName(t *testing.T) {
	got, ok := testForAdapter().TestForCandidate(
		"src/auth/token.ts", existsIn("src/auth/token.test.ts"))
	if !ok {
		t.Fatal("TestForCandidate found nothing, want src/auth/token.test.ts")
	}
	if got != "src/auth/token.test.ts" {
		t.Errorf("TestForCandidate = %q, want %q", got, "src/auth/token.test.ts")
	}
}

// Declaration order is the adapter author's confidence order, so the first template that
// resolves wins even when a later one also does.
func TestTestForCandidateTriesTemplatesInDeclarationOrder(t *testing.T) {
	exists := existsIn("src/auth/__tests__/token.test.ts", "tests/token.test.ts")
	got, ok := testForAdapter().TestForCandidate("src/auth/token.ts", exists)
	if !ok {
		t.Fatal("TestForCandidate found nothing")
	}
	if got != "src/auth/__tests__/token.test.ts" {
		t.Errorf("TestForCandidate = %q, want the earlier template's %q",
			got, "src/auth/__tests__/token.test.ts")
	}
}

// "The first template resolving to an EXISTING file wins" — a template that expands
// cleanly but names nothing is not a candidate. Guessing here would select a path the
// runner then rejects.
func TestTestForCandidateReportsNoMatchRatherThanAGuess(t *testing.T) {
	got, ok := testForAdapter().TestForCandidate("src/auth/token.ts", existsIn())
	if ok {
		t.Errorf("TestForCandidate = %q, true; want no match when no file exists", got)
	}
}

// At the repository root path.Dir is ".", and "./index.test.ts" matches no repo-relative
// path in the engine, which keeps every path cleaned.
func TestTestForCandidateAtTheRepoRootEmitsNoDotSegment(t *testing.T) {
	got, ok := testForAdapter().TestForCandidate("index.ts", existsIn("index.test.ts"))
	if !ok || got != "index.test.ts" {
		t.Errorf("TestForCandidate = %q, %v; want %q, true", got, ok, "index.test.ts")
	}
}

// A nil exists — a caller with no filesystem to ask — skips level 1. It is not fatal
// and it is not an assumption that the file is there.
func TestTestForCandidateWithoutAnExistsFunctionSkips(t *testing.T) {
	if _, ok := testForAdapter().TestForCandidate("src/a.ts", nil); ok {
		t.Error("TestForCandidate resolved with a nil exists; want no match")
	}
}

// An adapter declaring no templates is the ordinary Python case, not an error.
func TestTestForCandidateWithNoTemplatesSkips(t *testing.T) {
	a := &Adapter{Name: "python"}
	if _, ok := a.TestForCandidate("src/a.py", existsIn("src/a.py")); ok {
		t.Error("TestForCandidate resolved without templates; want no match")
	}
}
