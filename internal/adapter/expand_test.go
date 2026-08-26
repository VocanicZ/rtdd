package adapter

import (
	"reflect"
	"strings"
	"testing"
)

func TestExpandSubstitutesTheKnownPlaceholders(t *testing.T) {
	a := &Adapter{Name: "python"}
	got, err := a.Expand("pytest --cov --cov-context=test --cov-report= --report-log={log}",
		map[string]string{"log": "/tmp/x/report-0.jsonl"})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{"pytest", "--cov", "--cov-context=test", "--cov-report=", "--report-log=/tmp/x/report-0.jsonl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand =\n  %q\nwant\n  %q", got, want)
	}
}

func TestExpandSubstitutesSrcAndOut(t *testing.T) {
	a := &Adapter{Name: "python"}
	got, err := a.Expand("cov --source={src} --out={out} --log={log}",
		map[string]string{"src": "src", "out": ".rtdd/out", "log": "l.jsonl"})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{"cov", "--source=src", "--out=.rtdd/out", "--log=l.jsonl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand = %q, want %q", got, want)
	}
}

// A substituted value is never re-split: the template is tokenised BEFORE substitution,
// so a path with a space in it stays one argv element.
func TestExpandDoesNotResplitASubstitutedValue(t *testing.T) {
	a := &Adapter{Name: "python"}
	got, err := a.Expand("pytest --report-log={log}", map[string]string{"log": "/tmp/a b/report 0.jsonl"})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{"pytest", "--report-log=/tmp/a b/report 0.jsonl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand = %q, want %q", got, want)
	}
}

func TestExpandRejectsTestsPlaceholder(t *testing.T) {
	a := &Adapter{Name: "python"}
	_, err := a.Expand("pytest {tests}", map[string]string{})
	if err == nil {
		t.Fatal("Expand on a {tests} template = nil error, want error directing the caller to ExpandTests")
	}
	if !strings.Contains(err.Error(), "ExpandTests") {
		t.Fatalf("Expand error = %q, want it to mention ExpandTests", err.Error())
	}
}

// An unrecognised placeholder is a typo in the adapter, not a literal to pass through:
// `--junit={junit}` reaching the runner verbatim is a bad-selector exit nobody can read.
func TestExpandUnknownPlaceholderIsAnError(t *testing.T) {
	a := &Adapter{Name: "python"}
	for _, tmpl := range []string{
		"pytest --junit={junit}",
		"pytest --out={OUT}",
		"pytest --dir={out-dir}",
		"pytest {}",
	} {
		_, err := a.Expand(tmpl, map[string]string{"log": "l"})
		if err == nil {
			t.Errorf("Expand(%q) = nil error, want error; an unknown placeholder must never pass through as a literal", tmpl)
		}
	}
}

func TestExpandUnknownPlaceholderErrorNamesIt(t *testing.T) {
	a := &Adapter{Name: "python"}
	_, err := a.Expand("pytest --junit={junit}", map[string]string{"log": "l"})
	if err == nil {
		t.Fatal("Expand with an unknown placeholder = nil error, want error")
	}
	if !strings.Contains(err.Error(), "{junit}") {
		t.Fatalf("Expand error = %q, want it to name {junit}", err.Error())
	}
}

func TestExpandEmptyTemplate(t *testing.T) {
	a := &Adapter{Name: "python"}
	for _, tmpl := range []string{"", "   "} {
		if _, err := a.Expand(tmpl, map[string]string{}); err == nil {
			t.Errorf("Expand(%q) = nil error, want error", tmpl)
		}
	}
}

// The ids below are the real ones measured from coverage.py contexts: they contain a
// space, a '-', a '|' and '[' / ']'. Each must arrive as exactly ONE argv element, with
// no quoting and no escaping applied — pytest accepts them back as selectors verbatim
// only when they are passed individually.
func TestExpandTestsRoundTripsHostileIDs(t *testing.T) {
	a := &Adapter{Name: "python"}
	tests := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_pipe.py::test_pipe[a|b]",
		"tests/test_pipe.py::test_pipe[c d]",
	}
	got, err := a.ExpandTests("pytest {tests} --cov --report-log={log}",
		map[string]string{"log": "/tmp/r.jsonl"}, tests)
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{
		"pytest",
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_pipe.py::test_pipe[a|b]",
		"tests/test_pipe.py::test_pipe[c d]",
		"--cov",
		"--report-log=/tmp/r.jsonl",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExpandTests =\n  %q\nwant\n  %q", got, want)
	}
}

// Stated separately from the round-trip so the failure message says what broke: an id is
// one element, byte-identical to what was handed in.
func TestExpandTestsAppliesNoQuotingOrEscaping(t *testing.T) {
	a := &Adapter{Name: "python"}
	tests := []string{
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_pipe.py::test_pipe[a|b]",
		`tests/test_q.py::test_q["quoted" 'x']`,
		`tests/test_s.py::test_s[a\b]`,
	}
	got, err := a.ExpandTests("pytest {tests}", map[string]string{}, tests)
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	if len(got) != 1+len(tests) {
		t.Fatalf("ExpandTests produced %d argv elements (%q), want %d — every id must be exactly one element",
			len(got), got, 1+len(tests))
	}
	for i, id := range tests {
		if got[i+1] != id {
			t.Errorf("argv[%d] = %q, want %q byte-identical (no quoting, no escaping)", i+1, got[i+1], id)
		}
	}
}

// A brace inside a test id is data, not a placeholder: ids are spliced, never substituted.
func TestExpandTestsDoesNotSubstituteInsideATestID(t *testing.T) {
	a := &Adapter{Name: "python"}
	id := "tests/test_a.py::test_fmt[{log}]"
	got, err := a.ExpandTests("pytest {tests} --report-log={log}", map[string]string{"log": "r.jsonl"}, []string{id})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"pytest", id, "--report-log=r.jsonl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExpandTests = %q, want %q", got, want)
	}
}

func TestExpandTestsSplicesAtThePlaceholderPosition(t *testing.T) {
	a := &Adapter{Name: "python"}
	got, err := a.ExpandTests("pytest --cov {tests}", map[string]string{}, []string{"t1", "t2"})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"pytest", "--cov", "t1", "t2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExpandTests = %q, want %q", got, want)
	}
}

// AC: an empty selection must never become a whole-suite run. `pytest --cov` with the
// ids dropped collects everything, which is the most expensive possible way to be wrong.
func TestExpandTestsOnAnEmptyListIsAnError(t *testing.T) {
	a := &Adapter{Name: "python"}
	for _, tests := range [][]string{nil, {}} {
		_, err := a.ExpandTests("pytest {tests} --cov", map[string]string{}, tests)
		if err == nil {
			t.Fatalf("ExpandTests(%v) = nil error, want error; a command with no ids would run the whole suite", tests)
		}
	}
}

func TestExpandTestsRejectsAnEmptyTestID(t *testing.T) {
	a := &Adapter{Name: "python"}
	if _, err := a.ExpandTests("pytest {tests}", map[string]string{}, []string{"t1", ""}); err == nil {
		t.Fatal("ExpandTests with an empty id = nil error, want error; an empty selector argument makes pytest collect everything")
	}
}

func TestExpandTestsRequiresPlaceholder(t *testing.T) {
	a := &Adapter{Name: "python"}
	_, err := a.ExpandTests("pytest --cov", map[string]string{}, []string{"t"})
	if err == nil {
		t.Fatal("ExpandTests on a template with no {tests} = nil error, want error; test ids would be silently dropped")
	}
	if !strings.Contains(err.Error(), "{tests}") {
		t.Fatalf("ExpandTests error = %q, want it to name {tests}", err.Error())
	}
}

func TestExpandTestsPropagatesAnUnknownPlaceholder(t *testing.T) {
	a := &Adapter{Name: "python"}
	if _, err := a.ExpandTests("pytest {tests} --junit={junit}", map[string]string{}, []string{"t"}); err == nil {
		t.Fatal("ExpandTests with an unknown placeholder = nil error, want error")
	}
}

// The shipped adapter's own templates must expand: seed with Expand, subset with
// ExpandTests. This is the pairing cmd/rtdd depends on.
func TestBuiltinPythonTemplatesExpand(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	py := byName(all, "python")
	if py == nil {
		t.Fatal("Builtin has no python adapter")
	}
	vars := map[string]string{"src": "src", "out": ".rtdd/out", "log": ".rtdd/report-0.jsonl"}

	seed, err := py.Expand(py.Seed, vars)
	if err != nil {
		t.Fatalf("Expand(seed=%q): %v", py.Seed, err)
	}
	if len(seed) == 0 || seed[0] != "pytest" {
		t.Fatalf("seed argv = %q, want it to start with pytest", seed)
	}

	ids := []string{"tests/test_a.py::test_param[1-one two]"}
	subset, err := py.ExpandTests(py.Subset, vars, ids)
	if err != nil {
		t.Fatalf("ExpandTests(subset=%q): %v", py.Subset, err)
	}
	found := false
	for _, arg := range subset {
		if arg == ids[0] {
			found = true
		}
	}
	if !found {
		t.Fatalf("subset argv = %q, want it to carry %q as its own element", subset, ids[0])
	}
	if py.List != "" {
		if _, err := py.Expand(py.List, vars); err != nil {
			t.Fatalf("Expand(list=%q): %v", py.List, err)
		}
	}
}
