package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// specExampleReports is spec §6's own example, the golden the uncovered surface has
// rendered since it shipped.
func specExampleReports() []uncovered.FileReport {
	return []uncovered.FileReport{
		{Path: "src/auth.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 40, End: 51}, Class: uncovered.Covered},
			{Range: gitctx.LineRange{Start: 52, End: 58}, Class: uncovered.Uncovered},
		}},
		{Path: "src/constants.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 12}, Class: uncovered.ImportTime},
		}},
	}
}

// The gate is about the adapter's DECLARATION, not about what a particular run happened
// to record: an execution-derived adapter whose suite recorded nothing still has an
// honest uncovered report to print — every changed line really is unexecuted.
func TestRecordsCoverageIsFalseOnlyForACoverageNoneAdapter(t *testing.T) {
	cases := []struct {
		name string
		ad   *adapter.Adapter
		want bool
	}{
		{"coverage adapter", &adapter.Adapter{Name: "python", Selection: adapter.SelectionCoverage, Coverage: "sqlite"}, true},
		{"static adapter", &adapter.Adapter{Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone}, false},
		// A nil adapter keeps the coverage reading, exactly as unmappedNoticeApplies
		// does: nothing declared otherwise, and that is the reading every existing
		// repository has.
		{"no adapter", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := recordsCoverage(tc.ad); got != tc.want {
				t.Errorf("recordsCoverage(%v) = %v, want %v", tc.ad, got, tc.want)
			}
		})
	}
}

// PRD #233 AC9b's other half: for an execution-derived adapter the uncovered report is
// BYTE-IDENTICAL to what it has always been. The suppression may only ever remove a claim
// that was never true; it may not reword the one that is.
func TestUncoveredSurfaceForAnExecutionDerivedAdapterIsByteIdentical(t *testing.T) {
	reports := specExampleReports()
	want := "" +
		"  UNCOVERED: src/auth.py:52-58  (7 changed lines, no executing test)\n" +
		"  import-time: src/constants.py:1-12  (executed during collection, not attributed)\n"

	if got := RenderUncovered(reports); got != want {
		t.Fatalf("RenderUncovered()\n got:\n%s\nwant:\n%s", got, want)
	}
	if got := RenderUncoveredFor(reports, nil); got != want {
		t.Fatalf("RenderUncoveredFor() changed the execution-derived surface\n got:\n%s\nwant:\n%s", got, want)
	}
}

// The suppression states its reason, once, and names the adapter that owns it — the same
// suppression-with-a-reason shape unmappedNoticeApplies uses on the `which` surface.
func TestUncoveredSurfaceForACoverageNoneAdapterIsOneStatedReason(t *testing.T) {
	got := RenderUncoveredFor(nil, []string{"vitest"})
	if strings.Contains(got, "UNCOVERED") {
		t.Fatalf("the suppression line still claims uncovered lines:\n%s", got)
	}
	if n := strings.Count(got, "\n"); n != 1 {
		t.Fatalf("want exactly one line, got %d:\n%s", n, got)
	}
	for _, want := range []string{"vitest", "coverage: none", "uncovered"} {
		if !strings.Contains(got, want) {
			t.Errorf("the suppression line does not name %q:\n%s", want, got)
		}
	}
}

// A polyglot repository gets both halves: the coverage adapter's real report, and one
// stated reason for the adapter that has none. Neither may swallow the other.
func TestUncoveredSurfaceCarriesBothHalvesInAPolyglotRun(t *testing.T) {
	got := RenderUncoveredFor(specExampleReports(), []string{"vitest"})
	if !strings.Contains(got, "UNCOVERED: src/auth.py:52-58") {
		t.Errorf("the coverage adapter's report is gone:\n%s", got)
	}
	if !strings.Contains(got, "vitest") {
		t.Errorf("the static adapter's suppression reason is gone:\n%s", got)
	}
}

// The document and the terminal explain the same absence with the same sentence.
func TestTheJSONReasonAndTheTextLineShareOneWording(t *testing.T) {
	reason := noCoverageReason("vitest")
	line := RenderUncoveredFor(nil, []string{"vitest"})
	if !strings.Contains(line, reason) {
		t.Fatalf("the text line %q does not carry the document's reason %q", line, reason)
	}
}

// The repository-level reading. "Any", not "all": a mixed repository's Python half really
// did record, and its map-row count is a measured number that must survive.
func TestCoverageWasRecordedIsAnyNotAll(t *testing.T) {
	python := &adapter.Adapter{Name: "python", Selection: adapter.SelectionCoverage, Coverage: "sqlite"}
	vitest := &adapter.Adapter{Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone}
	cases := []struct {
		name string
		ads  []*adapter.Adapter
		want bool
	}{
		{"coverage only", []*adapter.Adapter{python}, true},
		{"static only", []*adapter.Adapter{vitest}, false},
		{"polyglot", []*adapter.Adapter{vitest, python}, true},
		{"none detected", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := coverageWasRecorded(tc.ads); got != tc.want {
				t.Errorf("coverageWasRecorded(%v) = %v, want %v", adapterNames(tc.ads), got, tc.want)
			}
		})
	}
}
