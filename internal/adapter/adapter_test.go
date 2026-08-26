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

// loadNoPanic calls Load and turns a panic into a test failure.
//
// paths.MatchGlob panics by design on a pattern ValidateGlob would reject: the engine has
// exactly one place that is allowed to see a malformed glob, and it is load time. That
// invariant only holds if Load rejects every globbed field, so a panic escaping here is
// the failure being tested for, not a crash to be reported as one.
func loadNoPanic(t *testing.T, p string) (a *Adapter, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Load panicked instead of returning an error: %v", r)
		}
	}()
	return Load(p)
}

// A typo'd glob must fail the load with a configuration error (exit 2) rather than
// classify nothing and let `rtdd which` report "no test file changed".
//
// detect is in the table for the same reason the other four are: it is globbed against
// every file in the repo by Detect, so a malformed pattern that survives Load reaches
// paths.MatchGlob and crashes the process with a stack trace instead of exiting 2.
func TestLoadRejectsAMalformedGlobInEveryGlobField(t *testing.T) {
	const badGlob = `tests/[a-*.py`
	for _, field := range []string{"detect", "test_globs", "source_globs", "opaque", "full_escalate"} {
		t.Run(field, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "a.yaml")
			content := "name: python\n" + field + ": [\"" + badGlob + "\"]\n"
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := loadNoPanic(t, p)
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

// --- M1b Task 1: full contract — validation, LoadFS/LoadAll/Builtin ------------------

const validYAML = `name: demo
detect: ["pyproject.toml"]
env:
  COVERAGE_CORE: ctrace
  COVERAGE_FILE: .coverage
seed: "pytest --cov --cov-context=test --cov-report= --report-log={log}"
subset: "pytest {tests} --cov --cov-context=test --cov-report= --report-log={log}"
list: "pytest --collect-only -q"
coverage: sqlite
report: pytest-reportlog
failfast_flag: "-x"
test_globs: ["tests/**/*.py"]
source_globs: ["src/**/*.py"]
exit_codes:
  4: bad-selector
  5: no-tests-collected
opaque: ["**/*.yaml"]
full_escalate: ["**/conftest.py"]
`

func writeAdapter(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// Each rejection carries its own message: an agent reading exit 2 must be told which
// field is wrong, not merely that the adapter is invalid.
func TestLoadRejectsEachInvalidFieldWithADistinctMessage(t *testing.T) {
	base := "name: x\ndetect: [\"a\"]\nseed: s\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n"
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"no name", "detect: [\"a\"]\nseed: s\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n", "name is required"},
		{"no detect", "name: x\nseed: s\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n", "detect is required"},
		{"no seed", "name: x\ndetect: [\"a\"]\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n", "seed is required"},
		{"subset without {tests}", "name: x\ndetect: [\"a\"]\nseed: s\nsubset: \"pytest --cov\"\ncoverage: sqlite\nreport: pytest-reportlog\n", "{tests}"},
		{"bad coverage", "name: x\ndetect: [\"a\"]\nseed: s\nsubset: \"{tests}\"\ncoverage: lcov\nreport: pytest-reportlog\n", "unsupported coverage"},
		{"bad report", "name: x\ndetect: [\"a\"]\nseed: s\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: junit\n", "unsupported report"},
		{"malformed glob", base + "test_globs: [\"tests/[a-*.py\"]\n", "tests/[a-*.py"},
	}
	seen := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeAdapter(t, t.TempDir(), "a.yaml", tc.yaml)
			_, err := Load(p)
			if err == nil {
				t.Fatalf("Load accepted %s; a bad adapter is a configuration error (exit 2)", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.want)
			}
			if prev, dup := seen[err.Error()]; dup {
				t.Fatalf("error for %s is byte-identical to the one for %s: %q", tc.name, prev, err)
			}
			seen[err.Error()] = tc.name
		})
	}
}

// An empty subset is missing {tests} too, but it deserves the "required" message rather
// than the placeholder one.
func TestLoadRejectsAnEmptySubsetAsMissing(t *testing.T) {
	p := writeAdapter(t, t.TempDir(), "a.yaml",
		"name: x\ndetect: [\"a\"]\nseed: s\ncoverage: sqlite\nreport: pytest-reportlog\n")
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "subset is required") {
		t.Fatalf("Load error = %v, want %q", err, "subset is required")
	}
}

func TestLoadAcceptsAValidAdapter(t *testing.T) {
	p := writeAdapter(t, t.TempDir(), "demo.yaml", validYAML)
	a, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Name != "demo" || a.List == "" || a.Env["COVERAGE_FILE"] != ".coverage" {
		t.Errorf("Load returned %#v, want the whole declaration parsed", a)
	}
}

func TestLoadFSReadsADirectorySortedByName(t *testing.T) {
	dir := t.TempDir()
	writeAdapter(t, dir, "zeta.yaml", strings.Replace(validYAML, "name: demo", "name: zeta", 1))
	writeAdapter(t, dir, "demo.yaml", validYAML)
	writeAdapter(t, dir, "ignored.txt", "not yaml at all")

	all, err := LoadFS(os.DirFS(dir), ".")
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len = %d, want 2 (non-.yaml entries are skipped)", len(all))
	}
	if all[0].Name != "demo" || all[1].Name != "zeta" {
		t.Fatalf("names = [%s %s], want [demo zeta]", all[0].Name, all[1].Name)
	}
}

func TestLoadFSPropagatesAnInvalidAdapter(t *testing.T) {
	dir := t.TempDir()
	writeAdapter(t, dir, "broken.yaml", "name: x\n")
	if _, err := LoadFS(os.DirFS(dir), "."); err == nil {
		t.Fatal("LoadFS accepted a directory containing an invalid adapter")
	}
}

func TestLoadAllReadsADirectoryOnDisk(t *testing.T) {
	dir := t.TempDir()
	writeAdapter(t, dir, "demo.yaml", validYAML)
	all, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 1 || all[0].Name != "demo" {
		t.Fatalf("LoadAll = %#v, want one adapter named demo", all)
	}
}

// Builtin reads through the embedded FS: the shipped binary carries its adapters and
// needs no files on disk.
func TestBuiltinShipsPythonWithoutTheFilesystem(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	py := byName(all, "python")
	if py == nil {
		t.Fatalf("Builtin() has no adapter named python; got %d adapters", len(all))
	}
	// Audit A7: sysmon records ~1 context in 4 and still exits 0, so this is asserted
	// rather than left to review.
	if py.Env["COVERAGE_CORE"] != "ctrace" {
		t.Errorf("Env[COVERAGE_CORE] = %q, want ctrace", py.Env["COVERAGE_CORE"])
	}
	// The env var beats a host `[run] data_file` setting, so the runner always knows
	// which database to read.
	if py.Env["COVERAGE_FILE"] != ".coverage" {
		t.Errorf("Env[COVERAGE_FILE] = %q, want .coverage", py.Env["COVERAGE_FILE"])
	}
	if py.ExitCodes[4] != "bad-selector" || py.ExitCodes[5] != "no-tests-collected" {
		t.Errorf("ExitCodes = %#v, want 4=bad-selector 5=no-tests-collected", py.ExitCodes)
	}
}

func byName(all []*Adapter, name string) *Adapter {
	for _, a := range all {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// Measured: `--cov=` with an empty value makes pytest exit 1 and record nothing, and any
// guessed {src} makes seed and subset disagree on scope. Bare --cov defers to the host's
// own [run] source/omit for both commands.
func TestBuiltinPythonTemplatesUseBareCov(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	py := byName(all, "python")
	if py == nil {
		t.Fatal("Builtin() has no adapter named python")
	}
	for _, tc := range []struct{ field, tmpl string }{{"seed", py.Seed}, {"subset", py.Subset}} {
		if !hasBareFlag(tc.tmpl, "--cov") {
			t.Errorf("%s = %q, want a bare --cov argument", tc.field, tc.tmpl)
		}
		if strings.Contains(tc.tmpl, "--cov=") {
			t.Errorf("%s = %q, must never use --cov=<value>", tc.field, tc.tmpl)
		}
		if strings.Contains(tc.tmpl, "{src}") {
			t.Errorf("%s = %q, still references {src}", tc.field, tc.tmpl)
		}
	}
}

// hasBareFlag reports whether flag appears in tmpl as its own whitespace-delimited
// argument, so "--cov-report=" does not count as "--cov".
func hasBareFlag(tmpl, flag string) bool {
	for _, f := range strings.Fields(tmpl) {
		if f == flag {
			return true
		}
	}
	return false
}

// Regression for #68: an adapter file with a malformed detect glob loaded with a nil
// error, so the pattern was not caught until Detect globbed the repo with it —
// detect.go -> glob.go -> paths.MatchGlob, which panics by design on a pattern
// ValidateGlob rejects. A hand-written adapter must never crash rtdd; a bad detect glob
// is a configuration error (exit 2) like every other bad glob.
func TestLoadRejectsAMalformedDetectGlobRatherThanPanickingInDetect(t *testing.T) {
	p := writeAdapter(t, t.TempDir(), "a.yaml", "name: python\ndetect: [\"[bad\"]\nseed: s\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n")

	a, err := loadNoPanic(t, p)
	if err == nil {
		t.Fatalf("Load accepted detect: [\"[bad\"]; a malformed glob is a configuration error (exit 2)")
	}
	if !strings.Contains(err.Error(), "detect") {
		t.Errorf("error = %q, want it to name the offending field %q", err, "detect")
	}
	if !strings.Contains(err.Error(), "[bad") {
		t.Errorf("error = %q, want it to name the offending pattern %q", err, "[bad")
	}
	if a != nil {
		t.Fatalf("Load returned an adapter alongside an error; Detect would glob with the bad pattern")
	}
}
