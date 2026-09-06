package report_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/report"
)

// shippedTemplate returns the id_template the named SHIPPED adapter declares, read from
// the embedded set rather than copied into this file. A test that restated the template
// would prove that a string round-trips, not that the adapter's own declaration does: the
// declaration could be edited to name {file} against a runner that emits only classname=
// and this file would keep passing on the template it remembered. So the templates come
// from adapters/*.yaml, and an edit to one is an edit to what these tests assert.
//
// This is also why the file is package report_test: internal/adapter imports
// internal/report, so only the external test package may read the adapters back.
func shippedTemplate(t *testing.T, name string) string {
	t.Helper()
	built, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("adapter.Builtin: %v", err)
	}
	for _, a := range built {
		if a.Name == name {
			if a.IDTemplate == "" {
				t.Fatalf("shipped adapter %q declares no id_template", name)
			}
			return a.IDTemplate
		}
	}
	t.Fatalf("no shipped adapter named %q", name)
	return ""
}

// Decision 7's table, asserted against the runners' OWN output. A hand-written fixture
// records what its author believes the runner emits; these record what six — now nine —
// ecosystems actually emit, which is what keeps disagreeing.
//
// The property is: every case in the capture renders a NON-EMPTY id under the shipped
// adapter's id_template, and re-rendering the parsed id reproduces it exactly. An id that
// renders empty folds the whole suite into one row (ErrEmptyRenderedID); an id that cannot
// be read back cannot be checked against the ids the runner was handed. An adapter whose
// template names a placeholder its runner does not emit — {file} against Vitest, which
// writes only classname= — renders empty here and fails rather than shipping.
func TestShippedIDTemplatesRoundTrip(t *testing.T) {
	cases := []struct {
		adapter      string
		fixture      string
		wantTemplate string // decision 7's table, cross-checked against the declaration
		wantID       string // the id the FIRST testcase in the capture must render
	}{
		// Every wantID below is read off the capture, never chosen for it. The dotnet row
		// is the one that shows the difference: JunitXml.TestLogger writes VSTest's outcome
		// groups in order — failures, then passes, then skips — so the first <testcase> in
		// dotnet.xml is the FAILING one, and asserting the passing case here would be
		// asserting a shape .NET does not emit. The fixtures are ground truth; the
		// expectations follow them.
		{"vitest", "vitest.xml", "{classname}", "test/calc.test.js"},
		{"jest", "jest-filepath.xml", "{classname}", "test/calc.test.js"},
		{"rspec", "rspec.xml", "{file}", "./spec/calc_spec.rb"},
		{"go", "go-junit-report.xml", "{name}", "TestAddsTwoNumbers"},
		{"cargo-nextest", "nextest.xml", "{name}", "tests::adds_two_numbers"},
		{"maven", "surefire.xml", "{classname}#{name}", "calc.CalcTest#addsTwoNumbers"},
		{"gradle", "surefire.xml", "{classname}.{name}", "calc.CalcTest.addsTwoNumbers"},
		{"dotnet", "dotnet.xml", "{classname}.{name}", "Calc.Tests.CalcTest.FailsOnAWrongSum"},
		{"phpunit", "phpunit.xml", "{classname}::{name}", "CalcTest::testAddsTwoNumbers"},
	}
	for _, tc := range cases {
		t.Run(tc.adapter, func(t *testing.T) {
			template := shippedTemplate(t, tc.adapter)
			if template != tc.wantTemplate {
				t.Errorf("adapters/%s.yaml id_template = %q, decision 7's table says %q: "+
					"the capture below is the evidence for the table's entry, so an adapter "+
					"that changes its template changes which capture proves it",
					tc.adapter, template, tc.wantTemplate)
			}
			rp, err := report.NewReportPathFor(tc.adapter, "testdata/junit", filepath.Base(tc.fixture))
			if err != nil {
				t.Fatalf("NewReportPathFor: %v", err)
			}
			outcomes, err := report.ReadJUnitReport(rp, template)
			if err != nil {
				t.Fatalf("ReadJUnitReport(%s, %q): %v", tc.fixture, template, err)
			}
			if len(outcomes) == 0 {
				t.Fatalf("%s produced no outcomes", tc.fixture)
			}
			if outcomes[0].Test != tc.wantID {
				t.Errorf("first id = %q, want %q", outcomes[0].Test, tc.wantID)
			}
			for _, o := range outcomes {
				if strings.TrimSpace(o.Test) == "" {
					t.Fatalf("%s rendered an empty id under %q", tc.fixture, template)
				}
				parsed, err := report.ParseID(template, o.Test)
				if err != nil {
					t.Fatalf("ParseID(%q, %q): %v", template, o.Test, err)
				}
				again, err := report.RenderID(template, parsed)
				if err != nil {
					t.Fatalf("RenderID: %v", err)
				}
				if again != o.Test {
					t.Errorf("round trip: %q -> %q", o.Test, again)
				}
			}
		})
	}
}

// Every shipped adapter that declares report: junit-xml is in the table above. A tenth
// junit-xml adapter added without a capture to round-trip it would otherwise ship with its
// id_template unproven, which is the exact hole this task exists to close.
func TestEveryShippedJUnitAdapterIsRoundTripped(t *testing.T) {
	covered := map[string]bool{
		"vitest": true, "jest": true, "rspec": true, "go": true, "cargo-nextest": true,
		"maven": true, "gradle": true, "dotnet": true, "phpunit": true,
	}
	built, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("adapter.Builtin: %v", err)
	}
	for _, a := range built {
		if a.Report != "junit-xml" {
			continue
		}
		if !covered[a.Name] {
			t.Errorf("shipped adapter %q declares report: junit-xml and id_template %q, "+
				"but no capture in TestShippedIDTemplatesRoundTrip proves that template "+
				"round-trips over its runner's real output",
				a.Name, a.IDTemplate)
		}
	}
}

// Decision 7: three of the nine share one id across every case in a file, and the fold is
// worst-status-wins. A file whose only failing case folded to `pass` is a false green.
func TestFileGranularAdaptersFoldAFileToOneOutcome(t *testing.T) {
	for _, tc := range []struct{ adapter, fixture string }{
		{"vitest", "vitest.xml"},
		{"jest", "jest-filepath.xml"},
		{"rspec", "rspec.xml"},
	} {
		t.Run(tc.adapter, func(t *testing.T) {
			template := shippedTemplate(t, tc.adapter)
			rp, err := report.NewReportPathFor(tc.adapter, "testdata/junit", tc.fixture)
			if err != nil {
				t.Fatalf("NewReportPathFor: %v", err)
			}
			outcomes, err := report.ReadJUnitReport(rp, template)
			if err != nil {
				t.Fatalf("ReadJUnitReport: %v", err)
			}
			// Each capture is one file holding one pass, one fail and one skip.
			if len(outcomes) != 1 {
				t.Fatalf("outcomes = %d, want 1: every case in the file shares the id", len(outcomes))
			}
			if outcomes[0].Status != "fail" {
				t.Errorf("folded status = %q, want %q (worst-status-wins)", outcomes[0].Status, "fail")
			}
		})
	}
}
