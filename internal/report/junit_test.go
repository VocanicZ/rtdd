package report

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeXML materialises one report file and returns its path.
func writeXML(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "junit.xml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// nestedXML is the shape Maven Surefire, Gradle and jest-junit all produce: a
// <testsuites> root wrapping one or more <testsuite>, and — the case a flat reader gets
// wrong — a <testsuite> nested inside another <testsuite>.
const nestedXML = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="all" tests="4" failures="1" errors="1" skipped="1" time="0.75">
  <testsuite name="outer" tests="2" time="0.5">
    <testcase classname="outer.Alpha" name="passes" file="tests/alpha.ts" time="0.25"/>
    <testsuite name="inner" tests="1" time="0.25">
      <testcase classname="inner.Beta" name="fails" file="tests/beta.ts" time="0.25">
        <failure message="expected 1 to be 2" type="AssertionError">at beta.ts:7</failure>
      </testcase>
    </testsuite>
  </testsuite>
  <testsuite name="second" tests="2" time="0.25">
    <testcase classname="second.Gamma" name="errors" time="0.2">
      <error message="boom" type="RuntimeError">stack</error>
    </testcase>
    <testcase classname="second.Delta" name="skipped">
      <skipped message="not on this platform"/>
    </testcase>
  </testsuite>
</testsuites>
`

func TestReadJUnitFileFlattensNestedSuitesInDocumentOrder(t *testing.T) {
	cases, err := ReadJUnitFile(writeXML(t, nestedXML))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	want := []JUnitCase{
		{Suite: "outer", Classname: "outer.Alpha", Name: "passes", File: "tests/alpha.ts", Status: "pass", DurationMS: 250},
		{Suite: "inner", Classname: "inner.Beta", Name: "fails", File: "tests/beta.ts", Status: "fail", DurationMS: 250},
		{Suite: "second", Classname: "second.Gamma", Name: "errors", Status: "error", DurationMS: 200},
		{Suite: "second", Classname: "second.Delta", Name: "skipped", Status: "skip"},
	}
	if len(cases) != len(want) {
		t.Fatalf("got %d cases, want %d: %+v", len(cases), len(want), cases)
	}
	for i := range want {
		if cases[i] != want[i] {
			t.Errorf("case %d = %+v, want %+v", i, cases[i], want[i])
		}
	}
}

// PRD #231 AC2: an <error> is a failure, not a skip. Both statuses already mean "this test
// did not pass" to internal/runner, which is why they must never collapse into "skip".
func TestReadJUnitFileMapsErrorToErrorAndNotSkip(t *testing.T) {
	cases, err := ReadJUnitFile(writeXML(t, nestedXML))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	for _, c := range cases {
		if c.Name == "errors" && c.Status != "error" {
			t.Errorf("<error> case mapped to %q, want %q", c.Status, "error")
		}
		if c.Name == "skipped" && c.Status != "skip" {
			t.Errorf("<skipped> case mapped to %q, want %q", c.Status, "skip")
		}
	}
}

// A single <testsuite> root with no <testsuites> wrapper is what pytest, PHPUnit and
// RSpec write. It is not a special case in the XML; it must not be one in the reader.
func TestReadJUnitFileAcceptsABareTestsuiteRoot(t *testing.T) {
	const bare = `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="only" tests="1" time="0.01">
  <testcase classname="only.One" name="works" time="0.01"/>
</testsuite>
`
	cases, err := ReadJUnitFile(writeXML(t, bare))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 1 || cases[0].Status != "pass" || cases[0].DurationMS != 10 {
		t.Fatalf("got %+v, want one passing case of 10ms", cases)
	}
}

// A testcase with no time= is not a zero-length test that ran; it is a runner that did not
// say. Both render as 0, and the map has always carried 0 for "unknown" — but a MISSING
// attribute must never become a parse error, because three of the six shipped fixtures
// omit it on skipped cases.
func TestReadJUnitFileTreatsAMissingTimeAsZero(t *testing.T) {
	const noTime = `<testsuite name="s"><testcase classname="s.C" name="n"/></testsuite>`
	cases, err := ReadJUnitFile(writeXML(t, noTime))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 1 || cases[0].DurationMS != 0 {
		t.Fatalf("got %+v, want one case with DurationMS 0", cases)
	}
}

// Flattening is DOCUMENT order, not "every case then every nested suite". Gradle writes a
// nested <testsuite> ahead of the enclosing suite's own <testcase> children, and a reader
// that drains the two child kinds separately reorders the run. The map's golden comparison
// is a positional one, so a reordering is a diff on every line below the first move.
func TestReadJUnitFileKeepsDocumentOrderWhenASuitePrecedesACase(t *testing.T) {
	const interleaved = `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="outer">
  <testcase classname="outer.First" name="first" time="0.01"/>
  <testsuite name="inner">
    <testcase classname="inner.Second" name="second" time="0.01"/>
  </testsuite>
  <testcase classname="outer.Third" name="third" time="0.01"/>
</testsuite>
`
	cases, err := ReadJUnitFile(writeXML(t, interleaved))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	var got []string
	for _, c := range cases {
		got = append(got, c.Name)
	}
	want := []string{"first", "second", "third"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v — the third case is written after the nested suite and must be read after it", got, want)
		}
	}
}

// Each of the three input failures — a report that is not there, one that is there and
// empty, and one that is there and is not XML — is a DISTINCT sentinel, because a caller
// diagnoses them differently: the first usually means the adapter's `requires` was unmet,
// the second that the runner died before it wrote, the third that it wrote garbage. None
// of them may return zero outcomes and a nil error: a silent zero-test success is
// indistinguishable from a passing run of an empty suite.
func TestReadJUnitFileNamesEachWayAReportCanBeUnreadable(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.xml")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	blank := filepath.Join(dir, "blank.xml")
	if err := os.WriteFile(blank, []byte("   \n\t\n"), 0o644); err != nil {
		t.Fatalf("write blank: %v", err)
	}
	torn := filepath.Join(dir, "torn.xml")
	if err := os.WriteFile(torn, []byte(`<testsuite name="s"><testcase name="a"`), 0o644); err != nil {
		t.Fatalf("write torn: %v", err)
	}
	wrongRoot := filepath.Join(dir, "wrong.xml")
	if err := os.WriteFile(wrongRoot, []byte(`<?xml version="1.0"?><results><ok/></results>`), 0o644); err != nil {
		t.Fatalf("write wrongRoot: %v", err)
	}

	cases := []struct {
		name string
		path string
		want error
	}{
		{"missing file", filepath.Join(dir, "absent.xml"), ErrNoReport},
		{"zero bytes", empty, ErrEmptyReport},
		{"whitespace only", blank, ErrEmptyReport},
		{"truncated xml", torn, ErrMalformedReport},
		{"root is not a junit root", wrongRoot, ErrMalformedReport},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadJUnitFile(tc.path)
			if err == nil {
				t.Fatalf("ReadJUnitFile(%s) = %v, nil error; a report that is not a report must never parse as zero tests", tc.path, got)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("ReadJUnitFile(%s) error = %v, want errors.Is(_, %v)", tc.path, err, tc.want)
			}
			if !strings.Contains(err.Error(), filepath.Base(tc.path)) {
				t.Errorf("error %q does not name the offending file", err)
			}
		})
	}
}

// The three input failures must also be distinguishable from EACH OTHER: a caller that
// only knows "something went wrong" cannot tell an uninstalled reporter from a crashed
// runner, so each sentinel matches its own case and no other.
func TestReadJUnitFileInputFailuresAreDistinctFromEachOther(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.xml")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	torn := filepath.Join(dir, "torn.xml")
	if err := os.WriteFile(torn, []byte(`<testsuite`), 0o644); err != nil {
		t.Fatalf("write torn: %v", err)
	}

	byPath := map[string]error{
		filepath.Join(dir, "absent.xml"): ErrNoReport,
		empty:                            ErrEmptyReport,
		torn:                             ErrMalformedReport,
	}
	all := []error{ErrNoReport, ErrEmptyReport, ErrMalformedReport}
	for path, want := range byPath {
		_, err := ReadJUnitFile(path)
		if err == nil {
			t.Fatalf("ReadJUnitFile(%s) = nil error", path)
		}
		for _, other := range all {
			if errors.Is(err, other) != errors.Is(want, other) {
				t.Errorf("ReadJUnitFile(%s) error %v matches %v; the three input failures must not collapse into one", path, err, other)
			}
		}
	}
}

// PRD #231 AC5: a suite-level <failure> with no <testcase> is the "the whole file blew up
// at import time" report. Zero cases plus exit 0 would be a false green.
func TestReadJUnitFileSurfacesASuiteLevelFailureWithNoTestcase(t *testing.T) {
	const suiteFailed = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="tests/broken.ts" tests="0" failures="1">
    <failure message="Cannot find module &#39;./missing&#39;" type="Error">at broken.ts:1</failure>
  </testsuite>
</testsuites>
`
	got, err := ReadJUnitFile(writeXML(t, suiteFailed))
	if err == nil {
		t.Fatalf("ReadJUnitFile = %v, nil error for a suite that failed before any test ran", got)
	}
	if !errors.Is(err, ErrSuiteFailure) {
		t.Fatalf("error = %v, want errors.Is(_, ErrSuiteFailure)", err)
	}
	for _, want := range []string{"tests/broken.ts", "Cannot find module"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q — the suite name and the runner's own message are the only diagnosis available", err, want)
		}
	}
}

// A suite-level <error> with no cases is the same catastrophe as a suite-level <failure>:
// Surefire writes <error> where jest-junit writes <failure> for a file that would not
// load, and reading only one of the two drops half the collection crashes.
func TestReadJUnitFileSurfacesASuiteLevelErrorWithNoTestcase(t *testing.T) {
	const suiteErrored = `<testsuite name="broken/Suite" tests="0" errors="1">
  <error message="ClassNotFoundException" type="Error">at Suite.java:1</error>
</testsuite>`
	got, err := ReadJUnitFile(writeXML(t, suiteErrored))
	if err == nil {
		t.Fatalf("ReadJUnitFile = %v, nil error for a suite that errored before any test ran", got)
	}
	if !errors.Is(err, ErrSuiteFailure) {
		t.Fatalf("error = %v, want errors.Is(_, ErrSuiteFailure)", err)
	}
	if !strings.Contains(err.Error(), "broken/Suite") {
		t.Errorf("error %q does not name the suite", err)
	}
}

// A suite-level <failure> ALONGSIDE testcases is a summary, not a catastrophe: Surefire
// writes one when a test failed, and the cases carry the detail. It must not error.
func TestReadJUnitFileKeepsASuiteFailureThatAlsoHasTestcases(t *testing.T) {
	const both = `<testsuite name="s" tests="1" failures="1">
  <failure message="1 test failed"/>
  <testcase classname="s.C" name="n" time="0.01"><failure message="nope"/></testcase>
</testsuite>`
	cases, err := ReadJUnitFile(writeXML(t, both))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 1 || cases[0].Status != "fail" {
		t.Fatalf("got %+v, want one failing case", cases)
	}
}

// A suite that carries a suite-level <failure> and no <testcase> of its own is the
// collection crash whether or not it also nests further suites. PHPUnit and Surefire both
// write <testsuite> inside <testsuite>, so a parent that blew up while a sibling child
// suite still reported is a real shape — and gating the guard on "no children at all"
// dropped the parent's failure the moment one nested suite was present, reporting the
// child's passing cases and exit 0 for a run in which a whole suite never ran.
func TestReadJUnitFileSurfacesASuiteFailureWhenTheSuiteOnlyHasNestedSuites(t *testing.T) {
	const nested = `<testsuite name="outer">
  <failure message="outer blew up at import time"/>
  <testsuite name="inner"><testcase classname="I" name="ok"/></testsuite>
</testsuite>`
	got, err := ReadJUnitFile(writeXML(t, nested))
	if err == nil {
		t.Fatalf("ReadJUnitFile = %v, nil error for a suite that failed before any of its tests ran", got)
	}
	if !errors.Is(err, ErrSuiteFailure) {
		t.Fatalf("error = %v, want errors.Is(_, ErrSuiteFailure)", err)
	}
	if !strings.Contains(err.Error(), "outer") {
		t.Errorf("error %q does not name the failing suite", err)
	}
}

// <error> in place of <failure> is the same catastrophe read through the other half of the
// vocabulary: Surefire writes <error> where jest-junit writes <failure>, and a nested-suite
// parent must surface both.
func TestReadJUnitFileSurfacesASuiteErrorWhenTheSuiteOnlyHasNestedSuites(t *testing.T) {
	const nested = `<testsuites>
  <testsuite name="outer">
    <error message="ClassNotFoundException" type="Error">at Outer.java:1</error>
    <testsuite name="inner"><testcase classname="I" name="ok"/></testsuite>
  </testsuite>
</testsuites>`
	got, err := ReadJUnitFile(writeXML(t, nested))
	if err == nil {
		t.Fatalf("ReadJUnitFile = %v, nil error for a suite that errored before any of its tests ran", got)
	}
	if !errors.Is(err, ErrSuiteFailure) {
		t.Fatalf("error = %v, want errors.Is(_, ErrSuiteFailure)", err)
	}
	if !strings.Contains(err.Error(), "outer") {
		t.Errorf("error %q does not name the failing suite", err)
	}
}

// The mirror of the guard: a parent suite that nests suites and did NOT fail is ordinary
// PHPUnit output, and its children's cases must come through untouched.
func TestReadJUnitFileKeepsNestedSuitesWhenTheParentDidNotFail(t *testing.T) {
	const nested = `<testsuite name="outer">
  <testsuite name="inner"><testcase classname="I" name="ok" time="0.01"/></testsuite>
</testsuite>`
	cases, err := ReadJUnitFile(writeXML(t, nested))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 1 || cases[0].Suite != "inner" || cases[0].Status != "pass" {
		t.Fatalf("got %+v, want one passing case in suite inner", cases)
	}
}

// A suite-level <failure> alongside the suite's OWN testcases stays a summary even when the
// suite also nests further suites: the cases carry the detail, so erroring here would fail
// whole runs that reported every one of their tests.
func TestReadJUnitFileKeepsASuiteFailureThatHasItsOwnTestcasesAndNestedSuites(t *testing.T) {
	const both = `<testsuite name="outer" tests="2" failures="1">
  <failure message="1 test failed"/>
  <testcase classname="O" name="failing"><failure message="nope"/></testcase>
  <testsuite name="inner"><testcase classname="I" name="ok"/></testsuite>
</testsuite>`
	cases, err := ReadJUnitFile(writeXML(t, both))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 2 || cases[0].Status != "fail" || cases[1].Suite != "inner" || cases[1].Status != "pass" {
		t.Fatalf("got %+v, want the outer failing case then the inner passing one", cases)
	}
}

// An empty <testsuite> with no failure of its own is not an error: a runner writes one for
// a file it collected and skipped entirely, and erroring on it would fail whole runs that
// went fine.
func TestReadJUnitFileAcceptsAnEmptySuiteThatDidNotFail(t *testing.T) {
	const emptySuite = `<testsuites>
  <testsuite name="nothing/here" tests="0"/>
  <testsuite name="s" tests="1"><testcase classname="s.C" name="n" time="0.01"/></testsuite>
</testsuites>`
	cases, err := ReadJUnitFile(writeXML(t, emptySuite))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 1 || cases[0].Status != "pass" {
		t.Fatalf("got %+v, want one passing case", cases)
	}
}

// time= is telemetry that arrives from a foreign runner, and strconv.ParseFloat accepts
// far more than a duration: "NaN", "Inf", and magnitudes past int64 all parse, so the
// unparseable-is-zero fallback never fires for them and int(math.Round(sec*1000)) on an
// out-of-range float is implementation-defined — in practice the int64 minimum. A negative
// DurationMS is not a fast test; it is a number that silently poisons every ordering and
// budgeting sum the ranker builds on top of it. So: non-finite is 0, negative is 0, a
// padded value keeps its telemetry, and an absurd magnitude clamps instead of wrapping.
func TestReadJUnitFileNeverTurnsATimeAttributeIntoANegativeDuration(t *testing.T) {
	cases := []struct {
		attr string
		want int
	}{
		{attr: "", want: 0},
		{attr: "abc", want: 0},
		{attr: "NaN", want: 0},
		{attr: "Inf", want: 0},
		{attr: "+Inf", want: 0},
		{attr: "-Inf", want: 0},
		{attr: "1e30", want: MaxDurationMS},
		{attr: "-1e30", want: 0},
		{attr: "-1", want: 0},
		{attr: "-0.25", want: 0},
		{attr: " 0.5 ", want: 500},
		{attr: "0.25", want: 250},
		{attr: "0", want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.attr, func(t *testing.T) {
			xmlDoc := fmt.Sprintf(
				`<testsuite name="s"><testcase classname="s.C" name="n" time=%q/></testsuite>`, tc.attr)
			got, err := ReadJUnitFile(writeXML(t, xmlDoc))
			if err != nil {
				t.Fatalf("ReadJUnitFile: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d cases, want 1", len(got))
			}
			if got[0].DurationMS < 0 {
				t.Fatalf("time=%q produced a negative DurationMS %d", tc.attr, got[0].DurationMS)
			}
			if got[0].DurationMS != tc.want {
				t.Fatalf("time=%q gave DurationMS %d, want %d", tc.attr, got[0].DurationMS, tc.want)
			}
		})
	}
}

// XML permits exactly ONE root element. A document with trailing garbage, or with a second
// <testsuite> root concatenated after the first, is not a well-formed document at all — but
// a reader that stops the moment the first root closes never looks, so it reports the first
// suite's cases and a nil error. The second shape is what a report written twice, or opened
// in append mode rather than truncated, looks like on disk: every suite after the first is
// dropped SILENTLY, so a failure in suite two is not merely unreported, it is invisible
// behind a full-looking report. That is the false green ErrMalformedReport exists to name.
//
// A comment or a processing instruction after the root IS legal XML and must still parse:
// only content that cannot follow a root is refused.
func TestReadJUnitFileRefusesContentAfterTheRootElement(t *testing.T) {
	refused := []struct {
		name string
		body string
	}{
		{
			"trailing garbage",
			`<?xml version="1.0"?><testsuite name="s"><testcase classname="a" name="b"/></testsuite>GARBAGE <<<`,
		},
		{
			"trailing character data",
			`<?xml version="1.0"?><testsuite name="s"><testcase classname="a" name="b"/></testsuite>oops`,
		},
		{
			"second root element",
			`<?xml version="1.0"?><testsuite name="one"><testcase classname="a" name="b"/></testsuite><testsuite name="two"><testcase classname="c" name="d"/></testsuite>`,
		},
		{
			"second root element under testsuites",
			`<?xml version="1.0"?><testsuites><testsuite name="one"><testcase classname="a" name="b"/></testsuite></testsuites><testsuites><testsuite name="two"><testcase classname="c" name="d"/></testsuite></testsuites>`,
		},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			p := writeXML(t, tc.body)
			got, err := ReadJUnitFile(p)
			if err == nil {
				t.Fatalf("ReadJUnitFile = %v, nil error; content after the root element must never parse as a partial success", got)
			}
			if !errors.Is(err, ErrMalformedReport) {
				t.Fatalf("ReadJUnitFile error = %v, want errors.Is(_, ErrMalformedReport)", err)
			}
			if !strings.Contains(err.Error(), filepath.Base(p)) {
				t.Errorf("error %q does not name the offending file", err)
			}
		})
	}

	accepted := []struct {
		name string
		body string
	}{
		{
			"trailing newline",
			"<?xml version=\"1.0\"?><testsuite name=\"s\"><testcase classname=\"a\" name=\"b\"/></testsuite>\n",
		},
		{
			"trailing comment",
			`<?xml version="1.0"?><testsuite name="s"><testcase classname="a" name="b"/></testsuite><!-- written by a runner -->`,
		},
		{
			"trailing processing instruction",
			`<?xml version="1.0"?><testsuite name="s"><testcase classname="a" name="b"/></testsuite><?some-pi value?>`,
		},
		{
			"trailing comment then whitespace",
			"<?xml version=\"1.0\"?><testsuite name=\"s\"><testcase classname=\"a\" name=\"b\"/></testsuite>\n<!-- ok -->\n",
		},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			cases, err := ReadJUnitFile(writeXML(t, tc.body))
			if err != nil {
				t.Fatalf("ReadJUnitFile: %v; a comment, a processing instruction and whitespace are legal after the root", err)
			}
			if len(cases) != 1 || cases[0].Name != "b" {
				t.Fatalf("ReadJUnitFile = %v, want the one case b", cases)
			}
		})
	}
}

// A <failure> that is a direct child of the <testsuites> ROOT is the same catastrophe AC5
// already refuses one element lower, and the root wrapper decoded only <testsuite>
// children — so a report saying the run itself blew up parsed as a run with nothing in it.
// There is no <testcase> a root can own, so unlike a suite there is no summary reading of
// it: a root-level failure is always the run failing before any test was named.
func TestReadJUnitFileSurfacesARootLevelFailure(t *testing.T) {
	const rootFailed = `<?xml version="1.0"?>
<testsuites name="vitest run">
  <failure message="root blew up" type="Error">at config.ts:1</failure>
</testsuites>
`
	got, err := ReadJUnitFile(writeXML(t, rootFailed))
	if err == nil {
		t.Fatalf("ReadJUnitFile = %v, nil error for a report whose root carries a <failure>", got)
	}
	if !errors.Is(err, ErrSuiteFailure) {
		t.Fatalf("error = %v, want errors.Is(_, ErrSuiteFailure)", err)
	}
	for _, want := range []string{"vitest run", "root blew up"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q — the root's name and the runner's own message are the only diagnosis available", err, want)
		}
	}
}

// A root-level <error> is the same shape as a root-level <failure>, for the same reason
// Surefire's <error> and jest-junit's <failure> are both read one level down. An unnamed
// root still has to say something a human can act on, so the message falls back to the
// element itself rather than printing an empty name.
func TestReadJUnitFileSurfacesARootLevelErrorWithNoName(t *testing.T) {
	const rootErrored = `<testsuites><error message="ClassNotFoundException"/></testsuites>`
	got, err := ReadJUnitFile(writeXML(t, rootErrored))
	if err == nil {
		t.Fatalf("ReadJUnitFile = %v, nil error for a report whose root carries an <error>", got)
	}
	if !errors.Is(err, ErrSuiteFailure) {
		t.Fatalf("error = %v, want errors.Is(_, ErrSuiteFailure)", err)
	}
	for _, want := range []string{"testsuites", "ClassNotFoundException"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
}

// A root-level failure is decided BEFORE the suites are walked, so the root's own message
// wins over a nested suite's — the outer one is the cause and the inner one the symptom.
// The nested suite's cases must not be reported either: they are the part of the run that
// happened to survive whatever killed the root.
func TestReadJUnitFileRootLevelFailureWinsOverTheSuitesBelowIt(t *testing.T) {
	const both = `<testsuites name="run"><failure message="root blew up"/>
  <testsuite name="ok"><testcase classname="ok.A" name="one"/></testsuite>
</testsuites>`
	got, err := ReadJUnitFile(writeXML(t, both))
	if err == nil {
		t.Fatalf("ReadJUnitFile = %v, nil error", got)
	}
	if !errors.Is(err, ErrSuiteFailure) || !strings.Contains(err.Error(), "root blew up") {
		t.Fatalf("error = %v, want the ROOT's own message under ErrSuiteFailure", err)
	}
}

// A <failure> nested inside a <testsuite> is NOT the root's: the root wrapper reads its
// own direct children only, so the suite-level guard keeps naming the suite that failed.
func TestReadJUnitFileRootGuardDoesNotClaimANestedSuitesFailure(t *testing.T) {
	const nested = `<testsuites name="run">
  <testsuite name="tests/broken.ts"><failure message="import failed"/></testsuite>
</testsuites>`
	_, err := ReadJUnitFile(writeXML(t, nested))
	if !errors.Is(err, ErrSuiteFailure) {
		t.Fatalf("error = %v, want ErrSuiteFailure", err)
	}
	if !strings.Contains(err.Error(), "tests/broken.ts") {
		t.Errorf("error %q must name the SUITE that failed, not the root", err)
	}
}

// A <testcase> written directly under the <testsuites> ROOT is not in the schema, and the
// root wrapper decoded <testsuite>, <failure> and <error> by struct tag — so encoding/xml
// discarded it, silently, along with whatever it reported. go-junit-report and jest-junit
// both emit one for a test that belongs to no suite, and a failing case that vanishes is
// the false green this parser refuses everywhere else: the case is surfaced, attributed to
// the root, rather than dropped.
func TestReadJUnitFileSurfacesATestcaseWrittenDirectlyUnderTheRoot(t *testing.T) {
	const rootCase = `<?xml version="1.0"?>
<testsuites name="go test">
  <testcase classname="pkg" name="TestLost" time="0.02"><failure message="boom"/></testcase>
  <testsuite name="pkg/inner"><testcase classname="pkg.inner" name="TestKept"/></testsuite>
</testsuites>
`
	cases, err := ReadJUnitFile(writeXML(t, rootCase))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("ReadJUnitFile = %+v, want the root-level case AND the nested suite's case", cases)
	}
	// Document order: the root-level case was written first.
	lost := cases[0]
	if lost.Name != "TestLost" || lost.Classname != "pkg" {
		t.Fatalf("first case = %+v, want the root-level TestLost", lost)
	}
	if lost.Status != "fail" {
		t.Errorf("root-level case status = %q, want %q — its <failure> travels with it", lost.Status, "fail")
	}
	if lost.DurationMS != 20 {
		t.Errorf("root-level case DurationMS = %d, want 20", lost.DurationMS)
	}
	if lost.Suite != "go test" {
		t.Errorf("root-level case Suite = %q, want the root's own name %q", lost.Suite, "go test")
	}
	if cases[1].Name != "TestKept" {
		t.Errorf("second case = %+v, want the nested suite's TestKept", cases[1])
	}
}

// The root's name is what every message in this package points a human at, and a root that
// names itself nothing still has to say something: the element itself, exactly as the
// root-level failure message already falls back to.
func TestReadJUnitFileNamesAnUnnamedRootAsTheSuiteOfItsOwnTestcase(t *testing.T) {
	const rootCase = `<testsuites><testcase classname="pkg" name="TestLoose"/></testsuites>`
	cases, err := ReadJUnitFile(writeXML(t, rootCase))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("ReadJUnitFile = %+v, want the one root-level case", cases)
	}
	if cases[0].Suite != "<testsuites>" {
		t.Errorf("Suite = %q, want %q", cases[0].Suite, "<testsuites>")
	}
}

// A root-level <failure> still wins over a root-level <testcase>: there is no summary
// reading of a root, so a report that says the run itself blew up is never traded for the
// part of it that happened to be written.
func TestReadJUnitFileRootLevelFailureWinsOverARootLevelTestcase(t *testing.T) {
	const both = `<testsuites name="run"><failure message="root blew up"/>
  <testcase classname="pkg" name="TestLoose"/>
</testsuites>`
	got, err := ReadJUnitFile(writeXML(t, both))
	if err == nil {
		t.Fatalf("ReadJUnitFile = %v, nil error", got)
	}
	if !errors.Is(err, ErrSuiteFailure) || !strings.Contains(err.Error(), "root blew up") {
		t.Fatalf("error = %v, want the root's own message under ErrSuiteFailure", err)
	}
}
