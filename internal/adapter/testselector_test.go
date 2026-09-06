package adapter

import (
	"reflect"
	"strings"
	"testing"
)

// `test_selector` is decision 13 (plan 06-m6d): the TS tier names test FILE PATHS, and six
// of the nine shipped adapters select by test NAME. The key is the declared translation
// between the two — one template, rendered once per selected test file, producing the ONE
// token `subset` splices.
//
// Its vocabulary is the test file's own path and nothing else: {file}, {dir} and {name}.
// A JUnit attribute has no meaning here — no report has been written when a selection is
// made — so id_template's spellings are rejected rather than silently left as literals.
func TestSelectorsRendersEachTestFileThroughTheDeclaredTemplate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tmpl  string
		tests []string
		want  []string
	}{
		{
			name:  "an omitted template is the identity, which is every v1 adapter unchanged",
			tmpl:  "",
			tests: []string{"tests/test_a.py::test_one", "tests/test_b.py::test_two"},
			want:  []string{"tests/test_a.py::test_one", "tests/test_b.py::test_two"},
		},
		{
			name:  "{file} is the identity written down",
			tmpl:  "{file}",
			tests: []string{"src/calc.test.ts"},
			want:  []string{"src/calc.test.ts"},
		},
		{
			name:  "{dir} is the go package the test file lives in",
			tmpl:  "./{dir}",
			tests: []string{"calc/calc_test.go"},
			want:  []string{"./calc"},
		},
		{
			name:  "a test file at the repo root has {dir} \".\", and ./. is a package pattern",
			tmpl:  "./{dir}",
			tests: []string{"main_test.go"},
			want:  []string{"./."},
		},
		{
			name:  "{name} is the base name without its extension: maven's simple class name",
			tmpl:  "{name}",
			tests: []string{"src/test/java/calc/CalcTest.java"},
			want:  []string{"CalcTest"},
		},
		{
			name:  "a template may name a placeholder more than once",
			tmpl:  "binary({name}) | test(/{name}::/)",
			tests: []string{"tests/calc.rs"},
			want:  []string{"binary(calc) | test(/calc::/)"},
		},
		{
			name:  "two test files in one go package render one selector, not two",
			tmpl:  "./{dir}",
			tests: []string{"calc/calc_test.go", "calc/mul_test.go"},
			want:  []string{"./calc"},
		},
		{
			name:  "de-duplication keeps the ranked order the selector produced",
			tmpl:  "./{dir}",
			tests: []string{"b/x_test.go", "a/y_test.go", "b/z_test.go"},
			want:  []string{"./b", "./a"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &Adapter{Name: "probe", TestSelector: tc.tmpl}
			got, err := a.Selectors(tc.tests)
			if err != nil {
				t.Fatalf("Selectors: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Selectors(%v) = %v, want %v", tc.tests, got, tc.want)
			}
		})
	}
}

// The vocabulary is enforced at LOAD time, exit 2, for the reason every other template
// vocabulary is: an unsubstitutable {classname} would survive into the runner's argv as a
// literal brace, match nothing, and report a green run over zero executed tests — which is
// the defect #334 filed.
func TestLoadRejectsATestSelectorOutsideItsVocabulary(t *testing.T) {
	base := `name: probe
detect: ["go.mod"]
subset: "go test {tests}"
list: "go test ./..."
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{name}"
`
	for _, tc := range []struct{ tmpl, want string }{
		{"{classname}", "{classname}"},
		{"{subdir}", "{subdir}"},
		{"{tests}", "{tests}"},
	} {
		t.Run(tc.tmpl, func(t *testing.T) {
			_, err := Load(writeAdapter(t, t.TempDir(), "a.yaml", base+"test_selector: \""+tc.tmpl+"\"\n"))
			if err == nil {
				t.Fatalf("Load accepted test_selector %q", tc.tmpl)
			}
			for _, want := range []string{"test_selector", tc.want} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

// A test_selector naming no placeholder renders the SAME token for every selected file:
// every selection collapses to one runner invocation over whatever that constant matches,
// and the tests the tier actually chose never run. Rejected at load, alongside the
// id_template rule it mirrors.
func TestLoadRejectsATestSelectorThatNamesNoPlaceholder(t *testing.T) {
	base := `name: probe
detect: ["go.mod"]
subset: "go test {tests}"
list: "go test ./..."
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{name}"
`
	_, err := Load(writeAdapter(t, t.TempDir(), "a.yaml", base+"test_selector: \"./...\"\n"))
	if err == nil {
		t.Fatal("Load accepted a constant test_selector; every selected file would render the same token")
	}
	for _, want := range []string{"test_selector", "placeholder"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
}

// Under `selection: coverage` the ids reaching `subset` are MAP ids — a pytest node id,
// not a path — so rendering {dir} over one produces nonsense. The key belongs to the tier
// whose selections are file paths, and declaring it anywhere else is a configuration
// error rather than a surprise at run time.
func TestLoadRejectsATestSelectorOnACoverageAdapter(t *testing.T) {
	yaml := `name: probe
detect: ["pytest.ini"]
seed: "pytest --cov"
subset: "pytest {tests}"
coverage: sqlite
report: pytest-reportlog
test_selector: "{file}"
`
	_, err := Load(writeAdapter(t, t.TempDir(), "a.yaml", yaml))
	if err == nil {
		t.Fatal("Load accepted test_selector under selection: coverage")
	}
	for _, want := range []string{"test_selector", "selection: static"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
}
