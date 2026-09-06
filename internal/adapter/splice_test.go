package adapter

import (
	"reflect"
	"strings"
	"testing"
)

// Gradle needs the flag before EACH id: `gradle test --tests A B` makes B a task name,
// and gradle then fails with "Task 'B' not found", which reads as a broken repo rather
// than a broken adapter.
func TestExpandTestsRepeatsTestFlagBeforeEachID(t *testing.T) {
	a := &Adapter{Name: "gradle", Subset: "gradle test {tests}", TestFlag: "--tests"}
	got, err := a.ExpandTests(a.Subset, nil, []string{"calc.CalcTest.adds", "calc.CalcTest.subs"})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"gradle", "test", "--tests", "calc.CalcTest.adds", "--tests", "calc.CalcTest.subs"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandTests = %q, want %q", got, want)
	}
}

// Surefire takes ONE -Dtest argument holding every selector, so {tests} has to be
// substitutable INSIDE a token. Splicing bare ids here would hand mvn a list of goals.
func TestExpandTestsJoinsIDsInsideOneTokenWhenTestJoinIsDeclared(t *testing.T) {
	a := &Adapter{Name: "maven", Subset: "mvn -B test -Dtest={tests}", TestJoin: ","}
	got, err := a.ExpandTests(a.Subset, nil, []string{"calc.CalcTest#adds", "calc.CalcTest#subs"})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"mvn", "-B", "test", "-Dtest=calc.CalcTest#adds,calc.CalcTest#subs"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandTests = %q, want %q", got, want)
	}
}

// The default is unchanged: pytest, vitest, jest, rspec and nextest all take repeated
// bare positionals, and adapters/python.yaml declares neither key.
func TestExpandTestsStillSplicesBareIDsWhenNeitherKeyIsDeclared(t *testing.T) {
	a := &Adapter{Name: "python", Subset: "pytest {tests} --cov"}
	got, err := a.ExpandTests(a.Subset, nil, []string{"tests/test_a.py::test_one", "tests/test_b.py::test_two"})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"pytest", "tests/test_a.py::test_one", "tests/test_b.py::test_two", "--cov"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandTests = %q, want %q", got, want)
	}
}

// Two answers to one question is a configuration error, not a precedence puzzle.
func TestBothSplicingKeysIsALoadTimeError(t *testing.T) {
	_, err := parse([]byte(`name: bad
detect: ["x.toml"]
selection: static
coverage: none
subset: "run {tests}"
list: "run --list"
report: junit-xml
report_path: "junit.xml"
id_template: "{classname}#{name}"
test_flag: "--tests"
test_join: ","
`), "bad.yaml")
	if err == nil {
		t.Fatal("parse = nil error, want a rejection naming both test_flag and test_join")
	}
	for _, want := range []string{"test_flag", "test_join"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// The shipped pytest expansion is frozen byte-for-byte. Splicing gained two shapes; the
// adapter that declares neither must come out of ExpandTests exactly as it did before,
// down to flag order, so the new keys cannot have moved the default by accident.
func TestBuiltinPythonSubsetExpansionIsByteForByteUnchanged(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	var py *Adapter
	for _, a := range all {
		if a.Name == "python" {
			py = a
		}
	}
	if py == nil {
		t.Fatal("Builtin has no python adapter")
	}
	if py.TestFlag != "" || py.TestJoin != "" {
		t.Fatalf("python declares splicing keys test_flag=%q test_join=%q; it must declare neither", py.TestFlag, py.TestJoin)
	}
	got, err := py.ExpandTests(py.Subset, map[string]string{"log": ".rtdd/run.jsonl"}, []string{
		"tests/test_a.py::test_one",
		"tests/test_b.py::test_param[1-one two]",
	})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{
		"pytest",
		"tests/test_a.py::test_one",
		"tests/test_b.py::test_param[1-one two]",
		"--cov", "--cov-context=test", "--cov-report=", "--report-log=.rtdd/run.jsonl",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandTests = %q, want %q", got, want)
	}
}

// An id holding the separator would split into two selectors inside the joined token —
// `dotnet test --filter A|B` with an id that already contains a '|' selects a test
// nobody asked for and drops the one that was asked for, and the run still exits 0.
// Name it instead.
func TestExpandTestsRefusesAnIDContainingTheJoinSeparator(t *testing.T) {
	a := &Adapter{Name: "dotnet", Subset: "dotnet test --filter {tests}", TestJoin: "|"}
	_, err := a.ExpandTests(a.Subset, nil, []string{"Calc.Adds", "Calc.Handles|Pipe"})
	if err == nil {
		t.Fatal("ExpandTests = nil error, want a rejection naming the id and the separator")
	}
	for _, want := range []string{"dotnet", "Calc.Handles|Pipe", "test_join"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// Losing the placeholder check under test_join would run the whole suite silently: the
// token is no longer exactly "{tests}", so the whole-token search cannot be the one that
// decides it.
func TestExpandTestsUnderTestJoinStillRefusesATemplateWithNoPlaceholder(t *testing.T) {
	a := &Adapter{Name: "maven", Subset: "mvn -B test", TestJoin: ","}
	_, err := a.ExpandTests(a.Subset, nil, []string{"calc.CalcTest#adds"})
	if err == nil {
		t.Fatal("ExpandTests = nil error, want a rejection naming the missing {tests} placeholder")
	}
	if !strings.Contains(err.Error(), "{tests}") {
		t.Errorf("error %q does not name %q", err, "{tests}")
	}
}

// test_flag places its ids at the {tests} token, so the token rule is today's rule: a
// template that never names {tests} would drop every selector.
func TestExpandTestsUnderTestFlagStillRefusesATemplateWithNoPlaceholder(t *testing.T) {
	a := &Adapter{Name: "gradle", Subset: "gradle test", TestFlag: "--tests"}
	_, err := a.ExpandTests(a.Subset, nil, []string{"calc.CalcTest.adds"})
	if err == nil {
		t.Fatal("ExpandTests = nil error, want a rejection naming the missing {tests} placeholder")
	}
	if !strings.Contains(err.Error(), "{tests}") {
		t.Errorf("error %q does not name %q", err, "{tests}")
	}
}

// Under test_join every other placeholder in the same token still substitutes, and the
// joined ids are never re-substituted: an id containing a brace group is data.
func TestExpandTestsUnderTestJoinLeavesIDsAloneAndStillSubstitutesTheRest(t *testing.T) {
	a := &Adapter{Name: "phpunit", Subset: "phpunit --filter {tests} --log-junit {report}", TestJoin: "|"}
	got, err := a.ExpandTests(a.Subset, map[string]string{"report": "junit.xml"}, []string{"CalcTest::testAdds", "CalcTest::test{name}"})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"phpunit", "--filter", "CalcTest::testAdds|CalcTest::test{name}", "--log-junit", "junit.xml"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandTests = %q, want %q", got, want)
	}
}

// A single id under test_join is one token with no separator in it — the joined shape is
// not a multi-id special case.
func TestExpandTestsUnderTestJoinWithOneID(t *testing.T) {
	a := &Adapter{Name: "go", Subset: "go test -run {tests} ./...", TestJoin: "|"}
	got, err := a.ExpandTests(a.Subset, nil, []string{"TestAdds"})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"go", "test", "-run", "TestAdds", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandTests = %q, want %q", got, want)
	}
}

// Either key alone loads. The mutual exclusion is the rejection, not the keys.
func TestEitherSplicingKeyAloneLoads(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"test_flag", `test_flag: "--tests"`},
		{"test_join", `test_join: ","`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := parse([]byte(`name: ok
detect: ["x.toml"]
selection: static
coverage: none
subset: "run {tests}"
list: "run --list"
report: junit-xml
report_path: "junit.xml"
id_template: "{classname}#{name}"
`+tc.key+"\n"), "ok.yaml")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if tc.name == "test_flag" && a.TestFlag != "--tests" {
				t.Errorf("TestFlag = %q, want %q", a.TestFlag, "--tests")
			}
			if tc.name == "test_join" && a.TestJoin != "," {
				t.Errorf("TestJoin = %q, want %q", a.TestJoin, ",")
			}
		})
	}
}
