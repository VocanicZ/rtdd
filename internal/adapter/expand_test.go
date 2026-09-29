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

// An unrecognised placeholder is a typo in the adapter, not a literal to pass through:
// `--junit={junit}` reaching the runner verbatim is a bad-selector exit nobody can read.
func TestExpandUnknownPlaceholderIsAnError(t *testing.T) {
	a := &Adapter{Name: "python"}
	for _, tmpl := range []string{
		"pytest --junit={junit}",
		"pytest --out={OUT}",
		"pytest --dir={out-dir}",
	} { // `{}` is a literal: see TestEmptyBracesAreALiteral
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

// An empty brace pair names nothing, so it is a literal: jest's --coverageThreshold={}
// (the one spelling that clears a project threshold) must reach the runner verbatim and
// pass validation.
func TestEmptyBracesAreALiteral(t *testing.T) {
	a := &Adapter{Name: "jest"}
	got, err := a.Expand("jest --coverageThreshold={} {unit}", map[string]string{"unit": "a.test.js"})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if want := []string{"jest", "--coverageThreshold={}", "a.test.js"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand = %q, want %q", got, want)
	}
	if bad := unknownPlaceholder("jest --coverageThreshold={} {unit}", unitPlaceholders); bad != "" {
		t.Fatalf("unknownPlaceholder = %q, want none", bad)
	}
}
