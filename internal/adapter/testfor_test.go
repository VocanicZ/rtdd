package adapter

import (
	"strings"
	"testing"
)

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

// {subdir} is {dir} or any trailing part of it, longest first. It is what lets ONE
// declared template mirror a test tree onto a source tree whose root is several segments
// deep: src/main/java/calc/Calc.java corresponds to src/test/java/calc/CalcTest.java, and
// the mirrored part is the package, not the whole directory.
func TestTestForCandidateSubdirMirrorsATrailingPartOfTheDirectory(t *testing.T) {
	a := &Adapter{
		Name:      "maven",
		Selection: SelectionStatic,
		Coverage:  CoverageNone,
		TestFor:   []string{"src/test/java/{subdir}/{name}Test.java"},
	}
	got, ok := a.TestForCandidate("src/main/java/calc/Calc.java",
		existsIn("src/test/java/calc/CalcTest.java"))
	if !ok {
		t.Fatal("TestForCandidate found nothing, want src/test/java/calc/CalcTest.java")
	}
	if got != "src/test/java/calc/CalcTest.java" {
		t.Errorf("TestForCandidate = %q, want %q", got, "src/test/java/calc/CalcTest.java")
	}
}

// The LONGEST trailing part that names an existing file wins, so the most specific
// correspondence the repository actually has is the one selected. A monorepo holding both
// a module-local and a root-level test tree must not silently prefer the distant one.
func TestTestForCandidateSubdirPrefersTheLongestExistingMatch(t *testing.T) {
	a := &Adapter{
		Name:      "gradle",
		Selection: SelectionStatic,
		Coverage:  CoverageNone,
		TestFor:   []string{"{subdir}/{name}Test.java"},
	}
	exists := existsIn(
		"services/api/src/main/java/calc/CalcTest.java",
		"calc/CalcTest.java",
	)
	got, ok := a.TestForCandidate("services/api/src/main/java/calc/Calc.java", exists)
	if !ok {
		t.Fatal("TestForCandidate found nothing")
	}
	if got != "services/api/src/main/java/calc/CalcTest.java" {
		t.Errorf("TestForCandidate = %q, want the longest trailing match %q",
			got, "services/api/src/main/java/calc/CalcTest.java")
	}
}

// The widening never becomes a guess: a trailing part that names nothing is skipped, and
// an adapter whose templates resolve to no existing file still reports no match.
func TestTestForCandidateSubdirStillRequiresTheFileToExist(t *testing.T) {
	a := &Adapter{Name: "maven", TestFor: []string{"src/test/java/{subdir}/{name}Test.java"}}
	if got, ok := a.TestForCandidate("src/main/java/calc/Calc.java", existsIn()); ok {
		t.Errorf("TestForCandidate = %q, true; want no match when no file exists", got)
	}
}

// {dir} keeps its meaning. It is the whole directory, and no suffix of it: an adapter that
// declared {dir} was declaring the full path, and quietly widening it would change every
// host adapter's selection at once.
func TestTestForCandidateDirIsStillTheWholeDirectory(t *testing.T) {
	a := &Adapter{Name: "maven", TestFor: []string{"src/test/java/{dir}/{name}Test.java"}}
	if got, ok := a.TestForCandidate("src/main/java/calc/Calc.java",
		existsIn("src/test/java/calc/CalcTest.java")); ok {
		t.Errorf("TestForCandidate = %q, true; {dir} names the whole directory, so it must not match", got)
	}
}

// A file at the repository root has "." for a directory, which is one trailing part and
// not a crash.
func TestTestForCandidateSubdirAtTheRepoRoot(t *testing.T) {
	a := &Adapter{Name: "probe", TestFor: []string{"tests/{subdir}/{name}_test.go"}}
	got, ok := a.TestForCandidate("main.go", existsIn("tests/main_test.go"))
	if !ok || got != "tests/main_test.go" {
		t.Errorf("TestForCandidate = %q, %v; want %q, true", got, ok, "tests/main_test.go")
	}
}

// An unknown placeholder is still refused at load time; adding {subdir} widened the
// vocabulary by exactly one spelling.
func TestValidateTemplatesStillRejectsAnUnknownTestForPlaceholder(t *testing.T) {
	a := &Adapter{Name: "probe", TestFor: []string{"src/test/{folder}/{name}Test.java"}}
	err := a.validateTemplates()
	if err == nil {
		t.Fatal("validateTemplates accepted {folder}; an unsubstituted placeholder selects nothing, silently")
	}
	if !strings.Contains(err.Error(), "{folder}") {
		t.Errorf("validateTemplates error %q does not name the placeholder", err)
	}
}
