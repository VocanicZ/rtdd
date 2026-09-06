package main

import (
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// The gate issue #334 says fixtures_test.go was missing.
//
// TestEachFixtureRepoDetectsItsAdapterAndSelectsItsTest asserts a TS selection NAMES the
// expected test file, which is PRD #232 AC9 exactly — and it passed while `rtdd run`
// executed nothing at all, because nothing asserted that the named id is one the adapter's
// own `subset` selector can consume. A test file path spliced into `-run`, `-Dtest=` or
// `--filter` matches no test, the runner exits 0, and the loop reports a pass over zero
// executed tests.
//
// So this table closes the round trip the other end: for every one of the nine shipped
// adapters, the TS selection produced against that adapter's committed fixture is rendered
// through `test_selector`, spliced through `ExpandTests`, and the resulting token is
// matched against the identity that adapter's runner would give the fixture's one test,
// under that runner's OWN selector semantics.
//
// No runner is executed here — the toolchains are not installed, and the go regression in
// selectorexec_test.go is where a real invocation happens. What is asserted is the
// contract between the two vocabularies, which is where #334 actually broke.

// runnerTest is the identity a runner gives the single test in a fixture repository.
// Whichever fields that ecosystem's selector reads are filled; the rest stay empty.
type runnerTest struct {
	File   string // the test file, repo-relative — every path-selecting runner reads this
	Class  string // the fully qualified class, for the JVM and .NET runners
	Method string // the test method or function name
	Binary string // the cargo-nextest test binary the file compiles into
}

// selectorMatch is one adapter's selector semantics, written down. It answers the only
// question that matters: would this runner, given this token, run that test?
type selectorMatch func(sel string, rt runnerTest) bool

// matchPath is a runner whose selector IS a path: vitest, jest and rspec all take the test
// file positionally.
func matchPath(sel string, rt runnerTest) bool { return sel == rt.File }

// matchGoPackage is `go test`'s package pattern. The selector names a directory, and it
// selects the test file when that directory is the file's own package directory.
func matchGoPackage(sel string, rt runnerTest) bool {
	return strings.TrimPrefix(sel, "./") == path.Dir(rt.File)
}

// matchNextest is a nextest filterset: a union of clauses, of which `binary(NAME)` names a
// test binary and `test(/REGEX/)` matches a test name. One clause matching is enough,
// because the union runs the union.
func matchNextest(sel string, rt runnerTest) bool {
	for _, clause := range strings.Split(sel, "|") {
		clause = strings.TrimSpace(clause)
		if inner, ok := unwrap(clause, "binary(", ")"); ok && inner == rt.Binary {
			return true
		}
		if inner, ok := unwrap(clause, "test(/", "/)"); ok {
			if re, err := regexp.Compile(inner); err == nil && re.MatchString(rt.Binary+"::"+rt.Method) {
				return true
			}
		}
	}
	return false
}

// matchSurefire is Maven's `-Dtest=`: a simple class name, a fully qualified one, or
// either followed by `#method`.
func matchSurefire(sel string, rt runnerTest) bool {
	cls, method := sel, ""
	if i := strings.Index(sel, "#"); i >= 0 {
		cls, method = sel[:i], sel[i+1:]
	}
	if cls != rt.Class && cls != simpleName(rt.Class) {
		return false
	}
	return method == "" || method == rt.Method
}

// matchGradleTests is Gradle's `--tests`: a pattern over the fully qualified name, with
// `*` as the only wildcard, matching the class or the class-plus-method.
func matchGradleTests(sel string, rt runnerTest) bool {
	re, err := regexp.Compile("^" + strings.ReplaceAll(regexp.QuoteMeta(sel), `\*`, ".*") + "$")
	if err != nil {
		return false
	}
	return re.MatchString(rt.Class) || re.MatchString(rt.Class+"."+rt.Method)
}

// matchDotnetFilter is `dotnet test --filter`: `FullyQualifiedName~value` is a substring
// test against the fully qualified test name, and a bare value means the same thing.
func matchDotnetFilter(sel string, rt runnerTest) bool {
	for _, clause := range strings.Split(sel, "|") {
		v := strings.TrimPrefix(strings.TrimSpace(clause), "FullyQualifiedName~")
		if v != "" && strings.Contains(rt.Class+"."+rt.Method, v) {
			return true
		}
	}
	return false
}

// matchPHPUnitFilter is PHPUnit's `--filter`: a regular expression over `Class::method`.
func matchPHPUnitFilter(sel string, rt runnerTest) bool {
	re, err := regexp.Compile(sel)
	return err == nil && re.MatchString(rt.Class+"::"+rt.Method)
}

func unwrap(s, open, close string) (string, bool) {
	if strings.HasPrefix(s, open) && strings.HasSuffix(s, close) {
		return s[len(open) : len(s)-len(close)], true
	}
	return "", false
}

func simpleName(fqcn string) string {
	if i := strings.LastIndex(fqcn, "."); i >= 0 {
		return fqcn[i+1:]
	}
	return fqcn
}

// selectorCase is one shipped adapter's row: the fixture, the test its runner would report,
// how that runner reads a selector, and the argv the whole subset command becomes.
type selectorCase struct {
	fixture  string
	test     runnerTest
	match    selectorMatch
	wantArgv []string
}

var shippedSelectorCases = []selectorCase{
	{
		fixture:  "vitest",
		test:     runnerTest{File: "src/calc.test.ts"},
		match:    matchPath,
		wantArgv: []string{"npx", "vitest", "run", "src/calc.test.ts", "--reporter=junit", "--outputFile=REPORT"},
	},
	{
		fixture:  "jest",
		test:     runnerTest{File: "src/calc.test.js"},
		match:    matchPath,
		wantArgv: []string{"npx", "jest", "--reporters=jest-junit", "src/calc.test.js"},
	},
	{
		fixture:  "rspec",
		test:     runnerTest{File: "spec/calc_spec.rb"},
		match:    matchPath,
		wantArgv: []string{"bundle", "exec", "rspec", "--format", "RspecJunitFormatter", "--out", "REPORT", "spec/calc_spec.rb"},
	},
	{
		fixture:  "go",
		test:     runnerTest{File: "calc/calc_test.go", Method: "TestAdd"},
		match:    matchGoPackage,
		wantArgv: []string{"go", "test", "-json", "./calc"},
	},
	{
		fixture:  "cargo-nextest",
		test:     runnerTest{File: "tests/calc.rs", Method: "adds", Binary: "calc"},
		match:    matchNextest,
		wantArgv: []string{"cargo", "nextest", "run", "--profile", "rtdd", "-E", "binary(calc) | test(/calc::/)"},
	},
	{
		fixture:  "maven",
		test:     runnerTest{File: "src/test/java/calc/CalcTest.java", Class: "calc.CalcTest", Method: "adds"},
		match:    matchSurefire,
		wantArgv: []string{"mvn", "-B", "test", "-Dtest=CalcTest", "-DfailIfNoSpecifiedTests=false"},
	},
	{
		fixture:  "gradle",
		test:     runnerTest{File: "src/test/java/calc/CalcTest.java", Class: "calc.CalcTest", Method: "adds"},
		match:    matchGradleTests,
		wantArgv: []string{"./gradlew", "test", "--tests", "*CalcTest"},
	},
	{
		fixture:  "dotnet",
		test:     runnerTest{File: "tests/Calc.Tests/CalcTests.cs", Class: "Calc.Tests.CalcTests", Method: "Adds"},
		match:    matchDotnetFilter,
		wantArgv: []string{"dotnet", "test", "--logger", "junit", "--results-directory", "REPORT", "--filter", "FullyQualifiedName~CalcTests"},
	},
	{
		fixture:  "phpunit",
		test:     runnerTest{File: "tests/CalcTest.php", Class: "CalcTest", Method: "testAdds"},
		match:    matchPHPUnitFilter,
		wantArgv: []string{"vendor/bin/phpunit", "--log-junit", "REPORT", "--filter", "CalcTest"},
	},
}

// PRD #334 AC1 and AC2. Every shipped adapter's TS selection, over its own fixture, is a
// value its own `subset` selector matches.
func TestEveryShippedAdapterSelectionMatchesItsSubsetSelector(t *testing.T) {
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	byFixture := map[string]selectorCase{}
	for _, sc := range shippedSelectorCases {
		byFixture[sc.fixture] = sc
	}

	for _, fc := range shippedFixtures {
		sc, ok := byFixture[fc.fixture]
		if !ok {
			t.Errorf("no selector case for fixture %s; a shipped adapter whose selection nobody matched against its own selector is the #334 defect", fc.fixture)
			continue
		}
		t.Run(fc.fixture, func(t *testing.T) {
			root := fixtureRoot(t, fc.fixture)
			ads, err := adapter.Detect(root, all)
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			sel := whichSelectionForFixture(t, root, ads, fc.changed)
			if sel.Tier != selector.TierTS {
				t.Fatalf("tier = %s, want TS; %s", sel.Tier, sel.Reason)
			}
			if !containsString(sel.Tests, sc.test.File) {
				t.Fatalf("selection %v does not name %q", sel.Tests, sc.test.File)
			}

			ad := adapterNamed(t, ads, fc.wantAdapter)
			selectors, err := ad.Selectors(sel.Tests)
			if err != nil {
				t.Fatalf("Selectors(%v): %v", sel.Tests, err)
			}
			matched := false
			for _, s := range selectors {
				if sc.match(s, sc.test) {
					matched = true
				}
			}
			if !matched {
				t.Errorf("adapter %s: selection %v renders selectors %v, and none of them makes %s run the test in %s;\n"+
					"the runner would match nothing and exit 0 over zero executed tests (#334)",
					ad.Name, sel.Tests, selectors, ad.Name, sc.test.File)
			}

			// And the whole command, so the shape of the invocation is visible here and
			// a template edit that changes it has to be stated rather than discovered.
			argv, err := ad.ExpandTests(ad.Subset, map[string]string{"report": "REPORT", "log": "LOG"}, selectors)
			if err != nil {
				t.Fatalf("ExpandTests: %v", err)
			}
			if !equalArgv(argv, sc.wantArgv) {
				t.Errorf("subset argv =\n  %v\nwant\n  %v", argv, sc.wantArgv)
			}
		})
	}
}

// A tenth junit-xml adapter dropped into adapters/ with no row here would ship with the
// #334 mismatch unchecked, which is the hole this gate exists to close. The enumeration is
// over the EMBEDDED set, never a hand-written list.
func TestEveryStaticShippedAdapterHasASelectorCase(t *testing.T) {
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	covered := map[string]bool{}
	for _, fc := range shippedFixtures {
		covered[fc.wantAdapter] = true
	}
	rows := map[string]bool{}
	for _, sc := range shippedSelectorCases {
		rows[sc.fixture] = true
	}
	for _, a := range all {
		if a.Report != "junit-xml" {
			continue
		}
		if !covered[a.Name] {
			t.Errorf("adapter %s declares report: junit-xml but has no fixture in shippedFixtures", a.Name)
		}
		if !rows[a.Name] {
			t.Errorf("adapter %s declares report: junit-xml but has no row in shippedSelectorCases; its TS selection is unchecked against its own selector (#334)", a.Name)
		}
	}
}

// Every static adapter declares test_selector out loud, the three file-granular ones
// included. The identity is a real answer — "a test file path IS my selector" — and
// writing it down is what makes its absence a missing declaration rather than a default
// nobody chose.
func TestEveryStaticShippedAdapterDeclaresATestSelector(t *testing.T) {
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	for _, a := range all {
		if a.Selection != adapter.SelectionStatic {
			continue
		}
		if a.TestSelector == "" {
			t.Errorf("adapter %s (%s): selection: static but no test_selector; the TS tier hands subset a test FILE path and nothing says what this runner does with one (#334)", a.Name, a.Src)
		}
	}
}

func adapterNamed(t *testing.T, ads []*adapter.Adapter, name string) *adapter.Adapter {
	t.Helper()
	for _, a := range ads {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("adapter %s not among %v", name, adapterNames(ads))
	return nil
}

func equalArgv(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
