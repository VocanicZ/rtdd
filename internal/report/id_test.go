package report

import (
	"errors"
	"strings"
	"testing"
)

// Decision 1 (plan 06-m6c): {classname} is the spelling. The PRD's prose says {class};
// the shipped vocabulary in internal/adapter is {classname}, validateTemplates rejects
// everything else at load time, and this package renders exactly what that loader admits.
// {class} is NOT an alias — two spellings for one key is worse than one rejection with a
// message. internal/adapter/id_vocabulary_test.go asserts the two agree.
func TestRenderIDExpandsTheShippedPlaceholderVocabulary(t *testing.T) {
	c := JUnitCase{
		Suite:     "tests/math.test.ts",
		Classname: "math > add",
		Name:      "adds two numbers",
		File:      "tests/math.test.ts",
	}
	cases := []struct {
		tmpl string
		want string
	}{
		{"{file}::{name}", "tests/math.test.ts::adds two numbers"},
		{"{classname}#{name}", "math > add#adds two numbers"},
		{"{file}", "tests/math.test.ts"},
		{"{name}", "adds two numbers"},
	}
	for _, tc := range cases {
		got, err := RenderID(tc.tmpl, c)
		if err != nil {
			t.Fatalf("RenderID(%q): %v", tc.tmpl, err)
		}
		if got != tc.want {
			t.Errorf("RenderID(%q) = %q, want %q", tc.tmpl, got, tc.want)
		}
	}
}

// Decision 2: a runner that does not emit file= gets a named error, never a derived path.
// A derived path that is wrong renders an id the runner does not recognise, the subset
// selects nothing, and the run reports green having executed no tests.
func TestRenderIDRefusesToInventAMissingFileAttribute(t *testing.T) {
	surefire := JUnitCase{
		Suite:     "com.example.CalculatorTest",
		Classname: "com.example.CalculatorTest",
		Name:      "addsTwoNumbers",
	}
	_, err := RenderID("{file}::{name}", surefire)
	if err == nil {
		t.Fatalf("RenderID = nil error; a {file} the runner never wrote must not be derived")
	}
	if !errors.Is(err, ErrNoFileAttr) {
		t.Fatalf("error = %v, want errors.Is(_, ErrNoFileAttr)", err)
	}
	for _, want := range []string{"com.example.CalculatorTest", "addsTwoNumbers", "{file}"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q — the fix is the adapter's id_template, so the message must name the case and the placeholder", err, want)
		}
	}
	// The same case renders fine under a template that does not name {file}: this is an
	// adapter-authoring error, not an unreadable report.
	got, err := RenderID("{classname}#{name}", surefire)
	if err != nil {
		t.Fatalf("RenderID without {file}: %v", err)
	}
	if got != "com.example.CalculatorTest#addsTwoNumbers" {
		t.Errorf("RenderID = %q, want %q", got, "com.example.CalculatorTest#addsTwoNumbers")
	}
}

// parse → render → parse must be stable, which requires the template to be invertible.
func TestParseIDInvertsRenderID(t *testing.T) {
	c := JUnitCase{Classname: "com.example.CalculatorTest", Name: "addsTwoNumbers", File: "src/calc_test.go"}
	for _, tmpl := range []string{"{file}::{name}", "{classname}#{name}", "{file}::{classname}::{name}", "{name}"} {
		id, err := RenderID(tmpl, c)
		if err != nil {
			t.Fatalf("RenderID(%q): %v", tmpl, err)
		}
		back, err := ParseID(tmpl, id)
		if err != nil {
			t.Fatalf("ParseID(%q, %q): %v", tmpl, id, err)
		}
		again, err := RenderID(tmpl, back)
		if err != nil {
			t.Fatalf("RenderID after ParseID (%q): %v", tmpl, err)
		}
		if again != id {
			t.Errorf("round trip through %q: %q -> %q", tmpl, id, again)
		}
	}
}

// ParseID recovers the fields the template NAMES, and only those: a template that does not
// name {classname} cannot invent one, and the round trip above is what keeps that honest.
func TestParseIDRecoversTheFieldsTheTemplateNames(t *testing.T) {
	got, err := ParseID("{file}::{classname}::{name}", "src/calc_test.go::example.com/calc::TestAdds")
	if err != nil {
		t.Fatalf("ParseID: %v", err)
	}
	want := JUnitCase{File: "src/calc_test.go", Classname: "example.com/calc", Name: "TestAdds"}
	if got != want {
		t.Errorf("ParseID = %+v, want %+v", got, want)
	}

	got, err = ParseID("{classname}#{name}", "calc.CalcTest#addsTwoNumbers")
	if err != nil {
		t.Fatalf("ParseID: %v", err)
	}
	if got.File != "" {
		t.Errorf("ParseID filled File=%q from a template that never names {file}", got.File)
	}
	if got.Classname != "calc.CalcTest" || got.Name != "addsTwoNumbers" {
		t.Errorf("ParseID = %+v, want classname calc.CalcTest / name addsTwoNumbers", got)
	}
}

// A template whose placeholders touch renders something no reader can split again, so the
// round-trip AC3 asks for cannot hold. It is rejected rather than silently one-way.
func TestParseIDRejectsAdjacentPlaceholders(t *testing.T) {
	_, err := ParseID("{classname}{name}", "com.example.CalculatorTestaddsTwoNumbers")
	if !errors.Is(err, ErrAmbiguousTemplate) {
		t.Fatalf("ParseID error = %v, want errors.Is(_, ErrAmbiguousTemplate)", err)
	}
	if _, err := RenderID("{classname}{name}", JUnitCase{Classname: "a", Name: "b"}); !errors.Is(err, ErrAmbiguousTemplate) {
		t.Fatalf("RenderID error = %v, want errors.Is(_, ErrAmbiguousTemplate); a template that cannot round-trip must fail on the way out too", err)
	}
}

// An id that is not a rendering of this template is named, never partially assigned: a
// half-parsed id would be spliced back into a subset command as a selector the runner
// silently matches nothing against.
func TestParseIDRejectsAnIDThatDoesNotMatchTheTemplate(t *testing.T) {
	for _, tc := range []struct{ tmpl, id string }{
		{"{classname}#{name}", "calc.CalcTest::addsTwoNumbers"}, // the separator is not there
		{"{file}::{name}", "no-separator-at-all"},
		{"tests/{file}::{name}", "src/calc.ts::adds"}, // leading literal does not match
		{"{name}.rb", "calc_spec"},                    // trailing literal does not match
	} {
		_, err := ParseID(tc.tmpl, tc.id)
		if err == nil {
			t.Errorf("ParseID(%q, %q) = nil error; an id that is not a rendering of the template must be named", tc.tmpl, tc.id)
			continue
		}
		if !strings.Contains(err.Error(), tc.id) || !strings.Contains(err.Error(), tc.tmpl) {
			t.Errorf("error %q must name both the id and the id_template", err)
		}
	}
}

// The vocabulary is closed on both sides of the round trip: internal/adapter rejects an
// unknown placeholder at load time, and neither RenderID nor ParseID may quietly treat one
// as a literal if it ever reaches here.
func TestRenderIDAndParseIDRejectAnUnknownPlaceholder(t *testing.T) {
	for _, tmpl := range []string{"{class}#{name}", "{dir}/{name}"} {
		if _, err := RenderID(tmpl, JUnitCase{Classname: "c", Name: "n", File: "f"}); err == nil {
			t.Errorf("RenderID(%q) = nil error; the loader rejects this template and so must the renderer", tmpl)
		}
		if _, err := ParseID(tmpl, "c#n"); err == nil {
			t.Errorf("ParseID(%q) = nil error; the loader rejects this template and so must the reader", tmpl)
		}
	}
}

// AC5: classname and name routinely carry the characters a selector treats as structural.
// The renderer does not escape them — the result is ONE argv token, which ExpandTests
// splices as one argument, so what the runner receives is exactly this string.
func TestRenderIDCarriesSelectorStructuralCharactersUnescapedAndStillRoundTrips(t *testing.T) {
	cases := []struct {
		what string
		tmpl string
		c    JUnitCase
		want string
	}{
		{
			// Maven Surefire: -Dtest=calc.CalcTest$Nested#adds(int)[1] — a nested class, a
			// method signature and a parameterised case's brackets.
			what: "surefire nested class and a parameterised case",
			tmpl: "{classname}#{name}",
			c:    JUnitCase{Suite: "calc.CalcTest", Classname: "calc.CalcTest$Nested", Name: "adds(int)[1]"},
			want: "calc.CalcTest$Nested#adds(int)[1]",
		},
		{
			// PHPUnit: --filter 'CalcTest::testAdds with data set #2' — the runner's own
			// separator appears inside the name too.
			what: "phpunit data set carrying :: and # inside the name",
			tmpl: "{classname}::{name}",
			c:    JUnitCase{Suite: "CalcTest", Classname: "CalcTest", Name: "testAdds::inner with data set #2"},
			want: "CalcTest::testAdds::inner with data set #2",
		},
		{
			// Vitest/Jest: a describe chain full of spaces, dots and brackets, against a
			// path that itself contains a space.
			what: "a spaced path and a describe chain with dots, spaces and brackets",
			tmpl: "{file}::{name}",
			c:    JUnitCase{Suite: "test/my tests/calc.test.ts", File: "test/my tests/calc.test.ts", Name: "calculator > adds [1, 2] to 3.0"},
			want: "test/my tests/calc.test.ts::calculator > adds [1, 2] to 3.0",
		},
		{
			// go-junit-report: the package path is the classname, and Go's subtest
			// separator is a slash inside the name.
			what: "a go subtest whose name carries a slash",
			tmpl: "{classname}#{name}",
			c:    JUnitCase{Suite: "example.com/calc", Classname: "example.com/calc", Name: "TestAdds/two_numbers"},
			want: "example.com/calc#TestAdds/two_numbers",
		},
	}
	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			got, err := RenderID(tc.tmpl, tc.c)
			if err != nil {
				t.Fatalf("RenderID(%q): %v", tc.tmpl, err)
			}
			if got != tc.want {
				t.Fatalf("RenderID = %q, want %q — the id is one argv token and is never escaped or quoted here", got, tc.want)
			}
			back, err := ParseID(tc.tmpl, got)
			if err != nil {
				t.Fatalf("ParseID(%q, %q): %v", tc.tmpl, got, err)
			}
			again, err := RenderID(tc.tmpl, back)
			if err != nil {
				t.Fatalf("RenderID after ParseID: %v", err)
			}
			if again != got {
				t.Errorf("round trip: %q -> %q", got, again)
			}
		})
	}
}

// Rendering reads a struct and a template and nothing else — no map iteration, no clock,
// no environment — so the same case renders the same id on every call. A subset command
// that varied between runs would make the map's ids unjoinable with the report's.
func TestRenderIDIsDeterministic(t *testing.T) {
	c := JUnitCase{Suite: "s", Classname: "calc.CalcTest", Name: "adds", File: "src/Calc.java"}
	const tmpl = "{file}::{classname}::{name}"
	first, err := RenderID(tmpl, c)
	if err != nil {
		t.Fatalf("RenderID: %v", err)
	}
	for i := 0; i < 100; i++ {
		got, err := RenderID(tmpl, c)
		if err != nil {
			t.Fatalf("RenderID call %d: %v", i, err)
		}
		if got != first {
			t.Fatalf("RenderID call %d = %q, want %q on every call", i, got, first)
		}
	}
}
