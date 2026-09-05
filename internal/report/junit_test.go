package report

import (
	"os"
	"path/filepath"
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
