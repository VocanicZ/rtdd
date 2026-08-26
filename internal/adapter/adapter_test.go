package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T) *Adapter {
	t.Helper()
	a, err := Load("testdata/python.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return a
}

func TestLoadParsesEveryField(t *testing.T) {
	a := loadFixture(t)
	if a.Name != "python" {
		t.Errorf("Name = %q, want python", a.Name)
	}
	if a.Env["COVERAGE_CORE"] != "ctrace" {
		t.Errorf("Env[COVERAGE_CORE] = %q, want ctrace", a.Env["COVERAGE_CORE"])
	}
	if a.Coverage != "sqlite" || a.Report != "pytest-reportlog" {
		t.Errorf("Coverage/Report = %q/%q, want sqlite/pytest-reportlog", a.Coverage, a.Report)
	}
	if a.FailFastFlag != "-x" {
		t.Errorf("FailFastFlag = %q, want -x", a.FailFastFlag)
	}
	if a.ExitCodes[4] != "bad-selector" || a.ExitCodes[5] != "no-tests-collected" {
		t.Errorf("ExitCodes = %#v, want 4=bad-selector 5=no-tests-collected", a.ExitCodes)
	}
	if len(a.Detect) != 3 || len(a.TestGlobs) != 2 || len(a.FullEscalate) != 3 {
		t.Errorf("Detect/TestGlobs/FullEscalate lengths = %d/%d/%d, want 3/2/3",
			len(a.Detect), len(a.TestGlobs), len(a.FullEscalate))
	}
}

func TestLoadRejectsBadYAML(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"not yaml", "\tthis: is: not: yaml\n"},
		{"unknown field", "name: python\nnot_a_field: 1\n"},
		{"missing name", "detect: [\"pyproject.toml\"]\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "a.yaml")
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(p); err == nil {
				t.Errorf("Load accepted %q; a bad adapter is a configuration error (exit 2)", tc.content)
			}
		})
	}
}

func TestLoadMissingFileIsAnError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("Load of a missing adapter returned nil error")
	}
}

// The shipped declaration must parse through the same KnownFields(true) decoder as any
// host-repo adapter, including the seed/subset/list/exit_codes fields that M1a never
// executes. M1b inherits this file rather than rewriting it.
func TestShippedPythonAdapterParses(t *testing.T) {
	a, err := Load(filepath.Join("..", "..", "adapters", "python.yaml"))
	if err != nil {
		t.Fatalf("Load(adapters/python.yaml): %v", err)
	}
	if a.Name != "python" {
		t.Errorf("Name = %q, want python", a.Name)
	}
	if !strings.HasPrefix(a.Seed, "pytest ") || !strings.Contains(a.Seed, "--cov-context=test") {
		t.Errorf("Seed = %q, want a pytest command with --cov-context=test", a.Seed)
	}
	if !strings.Contains(a.Subset, "{tests}") {
		t.Errorf("Subset = %q, want it to carry the {tests} placeholder", a.Subset)
	}
	// M1a amendment: {src} is dropped from Seed/Subset; bare --cov honours the host's
	// own [run] source and omit settings, so seed and subset agree on scope.
	if strings.Contains(a.Seed, "{src}") || strings.Contains(a.Subset, "{src}") {
		t.Errorf("Seed/Subset still reference {src}: %q / %q", a.Seed, a.Subset)
	}
	if a.List == "" {
		t.Error("List is empty; M1b needs the collect-only command")
	}
	if a.ExitCodes[4] != "bad-selector" || a.ExitCodes[5] != "no-tests-collected" {
		t.Errorf("ExitCodes = %#v, want 4=bad-selector 5=no-tests-collected", a.ExitCodes)
	}
}

// The shipped declaration and the test fixture must not drift apart: the fixture is what
// the adapter, selector and CLI tests classify against.
func TestFixtureMatchesShippedAdapter(t *testing.T) {
	shipped, err := os.ReadFile(filepath.Join("..", "..", "adapters", "python.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "python.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(shipped) != string(fixture) {
		t.Error("testdata/python.yaml has drifted from adapters/python.yaml")
	}
}

func TestClassification(t *testing.T) {
	a := loadFixture(t)
	tests := []struct {
		name           string
		rel            string
		test           bool
		opaque         bool
		fullEscalate   bool
		instrumentable bool
	}{
		{"test under tests/", "tests/test_auth.py", true, false, false, false},
		{"nested test under tests/", "tests/unit/api/test_auth.py", true, false, false, false},
		{"test_ prefixed anywhere", "src/pkg/test_helpers.py", true, false, false, false},
		{"plain source", "src/auth.py", false, false, false, true},
		{"nested source", "src/pkg/deep/auth.py", false, false, false, true},
		{"source outside source_globs", "scripts/tool.py", false, false, false, false},
		{"template is opaque", "templates/page.html", false, true, false, false},
		{"yaml is opaque", "config/settings.yaml", false, true, false, false},
		{"fixtures directory is opaque", "tests/fixtures/data/users.json", false, true, false, false},
		{"conftest escalates to full", "tests/conftest.py", false, false, true, false},
		{"pyproject escalates to full", "pyproject.toml", false, false, true, false},
		{"requirements escalates to full", "requirements.txt", false, false, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := a.IsTestFile(tc.rel); got != tc.test {
				t.Errorf("IsTestFile(%q) = %v, want %v", tc.rel, got, tc.test)
			}
			if got := a.IsOpaque(tc.rel); got != tc.opaque {
				t.Errorf("IsOpaque(%q) = %v, want %v", tc.rel, got, tc.opaque)
			}
			if got := a.IsFullEscalate(tc.rel); got != tc.fullEscalate {
				t.Errorf("IsFullEscalate(%q) = %v, want %v", tc.rel, got, tc.fullEscalate)
			}
			if got := a.IsInstrumentable(tc.rel); got != tc.instrumentable {
				t.Errorf("IsInstrumentable(%q) = %v, want %v", tc.rel, got, tc.instrumentable)
			}
		})
	}
}

// A source file that is also opaque or a test must not be instrumented: the AND in
// IsInstrumentable is what keeps a YAML fixture under src/ out of the coverage scope.
func TestIsInstrumentableExcludesTestsAndOpaque(t *testing.T) {
	a := &Adapter{
		Name:        "x",
		SourceGlobs: []string{"src/**/*.py", "src/**/*.yaml"},
		TestGlobs:   []string{"**/test_*.py"},
		Opaque:      []string{"**/*.yaml"},
	}
	if a.IsInstrumentable("src/pkg/test_thing.py") {
		t.Error("IsInstrumentable: a test file matching source_globs must not be instrumentable")
	}
	if a.IsInstrumentable("src/pkg/data.yaml") {
		t.Error("IsInstrumentable: an opaque file matching source_globs must not be instrumentable")
	}
	if !a.IsInstrumentable("src/pkg/thing.py") {
		t.Error("IsInstrumentable: a plain source file must be instrumentable")
	}
}

func TestNilAdapterClassifiesNothing(t *testing.T) {
	var a *Adapter
	if a.IsTestFile("tests/test_a.py") || a.IsOpaque("a.yaml") || a.IsFullEscalate("pyproject.toml") || a.IsInstrumentable("src/a.py") {
		t.Error("a nil *Adapter must classify nothing rather than panic")
	}
}

// M1a deliberately runs nothing. A stray exec import here would mean the classifier stub
// had grown a runner half.
func TestPackageExecutesNoSubprocess(t *testing.T) {
	src, err := os.ReadFile("adapter.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"os/exec", "syscall.Exec"} {
		if strings.Contains(string(src), banned) {
			t.Errorf("adapter.go references %q; subprocess execution is M1b", banned)
		}
	}
}

// A fixture module matching a broad test glob collects no tests; naming it as a runner
// selector is exit 5 (no-tests-collected), which is fatal. FullEscalate wins.
func TestFullEscalateBeatsTestGlobs(t *testing.T) {
	a := &Adapter{
		Name:         "x",
		TestGlobs:    []string{"tests/**/*.py"},
		FullEscalate: []string{"**/conftest.py"},
	}
	if a.IsTestFile("tests/conftest.py") {
		t.Error("IsTestFile: a full-escalate file must never be offered as a test selector")
	}
	if !a.IsFullEscalate("tests/conftest.py") {
		t.Error("IsFullEscalate(tests/conftest.py) = false, want true")
	}
	if !a.IsTestFile("tests/test_auth.py") {
		t.Error("IsTestFile(tests/test_auth.py) = false, want true")
	}
}

// A typo'd glob must fail the load with a configuration error (exit 2) rather than
// classify nothing and let `rtdd which` report "no test file changed".
func TestLoadRejectsAMalformedGlobInEveryGlobField(t *testing.T) {
	const badGlob = `tests/[a-*.py`
	for _, field := range []string{"test_globs", "source_globs", "opaque", "full_escalate"} {
		t.Run(field, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "a.yaml")
			content := "name: python\n" + field + ": [\"" + badGlob + "\"]\n"
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(p)
			if err == nil {
				t.Fatalf("Load accepted a malformed glob in %s; that is a configuration error", field)
			}
			if !strings.Contains(err.Error(), badGlob) {
				t.Errorf("error = %q, want it to name the offending pattern %q", err, badGlob)
			}
			if !strings.Contains(err.Error(), field) {
				t.Errorf("error = %q, want it to name the offending field %q", err, field)
			}
		})
	}
}
