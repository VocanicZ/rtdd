package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/selector"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

func TestBuildOutputRunWithUncoveredReport(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
	}
	reports := []uncovered.FileReport{
		{Path: "src/constants.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 15}, Class: uncovered.ImportTime},
		}},
		{Path: "src/logic.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 8, End: 8}, Class: uncovered.ImportTime},
			{Range: gitctx.LineRange{Start: 9, End: 9}, Class: uncovered.Uncovered},
		}},
	}
	in := OutputInput{
		Command: "run",
		Base:    "HEAD",
		Adapter: "python",
		Sel: selector.Selection{
			Tier:   selector.TierT0,
			Tests:  []string{"tests/test_new.py", "tests/test_it.py::test_logic"},
			Direct: []string{"tests/test_new.py"},
			Reason: "3 map rows intersect the changed set",
		},
		Changes:        changes,
		Instrumentable: map[string]bool{"src/constants.py": true, "src/logic.py": true},
		Executed:       true,
		Outcomes: []report.Outcome{
			{Test: "tests/test_new.py::test_x", Status: "pass", DurationMS: 400},
			{Test: "tests/test_it.py::test_logic", Status: "pass", DurationMS: 1000},
		},
		Reports:        reports,
		UncoveredOK:    true,
		UnmappedFiles:  []string{"src/constants.py"},
		ImportFallback: map[string][]string{"src/constants.py": {"tests/test_it.py"}},
	}

	out := BuildOutput(in)

	if out.Schema != 1 {
		t.Fatalf("schema = %d, want 1", out.Schema)
	}
	if out.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0 — an uncovered report is a signal, never a verdict", out.ExitCode)
	}
	if out.Uncovered.Summary.UncoveredLines != 1 {
		t.Fatalf("uncovered_lines = %d, want 1", out.Uncovered.Summary.UncoveredLines)
	}
	if out.Uncovered.Summary.ImportTimeLines != 16 {
		t.Fatalf("import_time_lines = %d, want 16", out.Uncovered.Summary.ImportTimeLines)
	}
	if out.Run.DurationMS != 1400 {
		t.Fatalf("duration_ms = %d, want 1400", out.Run.DurationMS)
	}
	if out.Tier != "T0" {
		t.Fatalf("tier = %q, want \"T0\"", out.Tier)
	}
	if len(out.Uncovered.Files) != 2 || out.Uncovered.Files[0].Path != "src/constants.py" {
		t.Fatalf("uncovered.files = %#v", out.Uncovered.Files)
	}
	if out.Uncovered.Files[0].Ranges[0].Class != "import-time" {
		t.Fatalf("class = %q, want \"import-time\"", out.Uncovered.Files[0].Ranges[0].Class)
	}
	if out.Uncovered.Files[0].UncoveredLines != 0 {
		t.Fatalf("constants.py uncovered_lines = %d, want 0", out.Uncovered.Files[0].UncoveredLines)
	}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, k := range []string{"schema", "command", "base", "adapter", "tier", "reason",
		"complete", "warnings", "changed", "selection", "run", "uncovered",
		"unmapped_files", "exit_code"} {
		if _, ok := round[k]; !ok {
			t.Fatalf("marshalled object is missing required key %q: %s", k, b)
		}
	}
}

func TestBuildOutputNeverNullsSlices(t *testing.T) {
	out := BuildOutput(OutputInput{Command: "which", Base: "HEAD", Adapter: "python"})
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(b)
	for _, needle := range []string{
		`"changed":[]`,
		`"direct":[]`,
		`"tests":[]`,
		`"import_fallback":{}`,
		`"failures":[]`,
		`"unmapped_files":[]`,
		`"warnings":[]`,
	} {
		if !strings.Contains(s, needle) {
			t.Fatalf("output must never emit null for a collection; missing %s in %s", needle, s)
		}
	}
	if strings.Contains(s, "null") {
		t.Fatalf("output must never contain null at all: %s", s)
	}
}

// An empty run is still a run: every collection stays an empty collection even when the
// uncovered report IS available, which is the branch that populates uncovered.files.
func TestBuildOutputNeverNullsSlicesWithUncoveredAvailable(t *testing.T) {
	out := BuildOutput(OutputInput{Command: "run", Base: "HEAD", Adapter: "python",
		Executed: true, UncoveredOK: true})
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(b)
	for _, needle := range []string{`"failures":[]`, `"files":[]`, `"import_fallback":{}`} {
		if !strings.Contains(s, needle) {
			t.Fatalf("missing %s in %s", needle, s)
		}
	}
}

func TestBuildOutputWhichHasNoUncoveredReport(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "which", Base: "HEAD", Adapter: "python",
		Sel:           selector.Selection{Tier: selector.TierT0, Tests: []string{"tests/a.py::t"}},
		UnmappedFiles: []string{"src/constants.py"},
	})
	if out.Run.Executed {
		t.Fatal("which must report run.executed = false")
	}
	if out.Uncovered.Available {
		t.Fatal("which must report uncovered.available = false; it runs nothing, so there is no fresh coverage")
	}
	if out.Uncovered.Reason == "" {
		t.Fatal("uncovered.reason must explain why the report is unavailable")
	}
	if out.Uncovered.Files != nil {
		t.Fatalf("uncovered.files must be omitted when unavailable, got %#v", out.Uncovered.Files)
	}
	if out.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0", out.ExitCode)
	}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(b)
	// `"files":` alone would also match summary.files, which is always present.
	if strings.Contains(s, `"files":[`) {
		t.Fatalf("uncovered.files must be omitted from the document when unavailable: %s", s)
	}
	if !strings.Contains(s, `"reason":"`) {
		t.Fatalf("uncovered.reason must be present when unavailable: %s", s)
	}
}

// uncovered.reason is present ONLY when available is false — the mirror of the omission
// rule above, and the half a consumer keying on `reason` would otherwise trip over.
func TestBuildOutputUncoveredReasonOnlyWhenUnavailable(t *testing.T) {
	out := BuildOutput(OutputInput{Command: "run", Base: "HEAD", Adapter: "python",
		Executed: true, UncoveredOK: true})
	if out.Uncovered.Reason != "" {
		t.Fatalf("uncovered.reason = %q, want empty when available", out.Uncovered.Reason)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var doc struct {
		Uncovered map[string]any `json:"uncovered"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := doc.Uncovered["reason"]; ok {
		t.Fatalf("uncovered.reason must be omitted when available: %s", b)
	}
	if _, ok := doc.Uncovered["files"]; !ok {
		t.Fatalf("uncovered.files must be present when available: %s", b)
	}
}

func TestBuildOutputFailingTestExitsOne(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "run", Base: "HEAD", Adapter: "python",
		Sel:      selector.Selection{Tier: selector.TierT0, Tests: []string{"tests/a.py::t"}},
		Executed: true,
		Outcomes: []report.Outcome{
			{Test: "tests/a.py::t", Status: "fail", DurationMS: 5},
			{Test: "tests/b.py::t", Status: "error", DurationMS: 3},
			{Test: "tests/c.py::t", Status: "skip", DurationMS: 1},
		},
		UncoveredOK: true,
	})
	if out.ExitCode != 1 {
		t.Fatalf("exit_code = %d, want 1", out.ExitCode)
	}
	if out.Run.Failed != 1 || out.Run.Errored != 1 || out.Run.Skipped != 1 {
		t.Fatalf("counts = %+v", out.Run)
	}
	want := []string{"tests/a.py::t", "tests/b.py::t"}
	if len(out.Run.Failures) != 2 || out.Run.Failures[0] != want[0] || out.Run.Failures[1] != want[1] {
		t.Fatalf("failures = %#v, want %#v", out.Run.Failures, want)
	}
}

// The load-bearing invariant, asserted as an iff over the whole outcome cross-product:
// exit_code is 1 exactly when run.failed + run.errored > 0, and an uncovered report of
// any size never moves it.
func TestBuildOutputExitCodeIffFailedOrErrored(t *testing.T) {
	loud := []uncovered.FileReport{
		{Path: "src/a.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 99}, Class: uncovered.Uncovered},
		}},
	}
	cases := []struct {
		name     string
		outcomes []report.Outcome
		want     int
	}{
		{"no tests", nil, 0},
		{"all pass", []report.Outcome{{Test: "a", Status: "pass"}}, 0},
		{"skips only", []report.Outcome{{Test: "a", Status: "skip"}}, 0},
		{"one fail", []report.Outcome{{Test: "a", Status: "pass"}, {Test: "b", Status: "fail"}}, 1},
		{"one error", []report.Outcome{{Test: "a", Status: "pass"}, {Test: "b", Status: "error"}}, 1},
		{"fail and error", []report.Outcome{{Test: "a", Status: "fail"}, {Test: "b", Status: "error"}}, 1},
	}
	for _, tc := range cases {
		for _, withReport := range []bool{false, true} {
			in := OutputInput{Command: "run", Base: "HEAD", Adapter: "python",
				Executed: true, Outcomes: tc.outcomes, UncoveredOK: true}
			if withReport {
				in.Reports = loud
			}
			out := BuildOutput(in)
			if out.ExitCode != tc.want {
				t.Fatalf("%s (uncovered report: %v): exit_code = %d, want %d",
					tc.name, withReport, out.ExitCode, tc.want)
			}
			if got := out.Run.Failed + out.Run.Errored; (got > 0) != (out.ExitCode == 1) {
				t.Fatalf("%s: exit_code = %d but failed+errored = %d — the iff is broken",
					tc.name, out.ExitCode, got)
			}
			if withReport && out.Uncovered.Summary.UncoveredLines != 99 {
				t.Fatalf("%s: fixture is wrong: uncovered_lines = %d, want 99",
					tc.name, out.Uncovered.Summary.UncoveredLines)
			}
		}
	}
}

func TestBuildOutputChangedStatusNames(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "run", Base: "HEAD", Adapter: "python",
		Changes: []gitctx.Change{
			{Path: "a.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 3}}},
			{Path: "b.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 2, End: 2}}},
			{Path: "c.py", Status: gitctx.Deleted},
			{Path: "d.py", Status: gitctx.Renamed, OldPath: "old.py", Lines: []gitctx.LineRange{{Start: 4, End: 5}}},
			{Path: "e.py", Status: gitctx.Untracked, Lines: []gitctx.LineRange{{Start: 1, End: 1}}},
		},
		Instrumentable: map[string]bool{"a.py": true, "c.py": true},
	})
	want := []string{"added", "modified", "deleted", "renamed", "untracked"}
	if len(out.Changed) != len(want) {
		t.Fatalf("changed = %#v", out.Changed)
	}
	for i, w := range want {
		if out.Changed[i].Status != w {
			t.Fatalf("changed[%d].status = %q, want %q", i, out.Changed[i].Status, w)
		}
	}
	if len(out.Changed[2].Lines) != 0 {
		t.Fatalf("lines must be empty for a deleted file, got %#v", out.Changed[2].Lines)
	}
	if !out.Changed[0].Instrumentable || out.Changed[1].Instrumentable {
		t.Fatalf("instrumentable mis-mapped: %#v", out.Changed)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"path":"c.py","status":"deleted","instrumentable":true,"lines":[]`) {
		t.Fatalf("a deleted file must serialize lines as []: %s", b)
	}
}

func TestBuildOutputTierNames(t *testing.T) {
	for tier, want := range map[selector.Tier]string{
		selector.TierEmpty:  "empty",
		selector.TierDirect: "direct",
		selector.TierT0:     "T0",
		selector.TierT1:     "T1",
		selector.TierT2:     "T2",
	} {
		out := BuildOutput(OutputInput{Command: "which", Sel: selector.Selection{Tier: tier}})
		if out.Tier != want {
			t.Fatalf("tier = %q, want %q", out.Tier, want)
		}
	}
}

// Audit finding A1, at the wire: an import-time line is reported as "import-time" and is
// never spelled "uncovered", and it never lands in uncovered_lines.
func TestBuildOutputImportTimeIsNeverEmittedAsUncovered(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "run", Base: "HEAD", Adapter: "python", Executed: true, UncoveredOK: true,
		Reports: []uncovered.FileReport{
			{Path: "src/a.py", Ranges: []uncovered.ClassifiedRange{
				{Range: gitctx.LineRange{Start: 1, End: 4}, Class: uncovered.ImportTime},
				{Range: gitctx.LineRange{Start: 5, End: 6}, Class: uncovered.Covered},
			}},
		},
	})
	if out.Uncovered.Summary.UncoveredLines != 0 {
		t.Fatalf("uncovered_lines = %d, want 0 — import-time is not uncovered",
			out.Uncovered.Summary.UncoveredLines)
	}
	if out.Uncovered.Files[0].UncoveredLines != 0 {
		t.Fatalf("files[0].uncovered_lines = %d, want 0", out.Uncovered.Files[0].UncoveredLines)
	}
	classes := []string{}
	for _, r := range out.Uncovered.Files[0].Ranges {
		classes = append(classes, r.Class)
	}
	if len(classes) != 2 || classes[0] != "import-time" || classes[1] != "covered" {
		t.Fatalf("classes = %#v, want [import-time covered]", classes)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(b), `"class":"uncovered"`) {
		t.Fatalf("no range here is uncovered, yet the document says so: %s", b)
	}
	if strings.Contains(string(b), `"class":"unknown"`) {
		t.Fatalf("class must be one of covered/uncovered/import-time: %s", b)
	}
}

// Every consumer diffs these documents, so the same input must marshal to the same bytes:
// stable key order comes from the structs, stable element order from sorting.
func TestBuildOutputIsDeterministic(t *testing.T) {
	in := func() OutputInput {
		return OutputInput{
			Command: "run", Base: "HEAD", Adapter: "python", Executed: true, UncoveredOK: true,
			Sel: selector.Selection{Tier: selector.TierT1, Tests: []string{"t/z.py", "t/a.py"},
				Direct: []string{"t/z.py"}},
			Outcomes: []report.Outcome{
				{Test: "t/z.py::t", Status: "fail", DurationMS: 2},
				{Test: "t/a.py::t", Status: "error", DurationMS: 3},
			},
			Reports: []uncovered.FileReport{
				{Path: "src/z.py", Ranges: []uncovered.ClassifiedRange{
					{Range: gitctx.LineRange{Start: 1, End: 1}, Class: uncovered.Uncovered}}},
				{Path: "src/a.py", Ranges: []uncovered.ClassifiedRange{
					{Range: gitctx.LineRange{Start: 2, End: 2}, Class: uncovered.Covered}}},
			},
			UnmappedFiles:  []string{"src/z.py", "src/a.py"},
			ImportFallback: map[string][]string{"src/z.py": {"t/z.py"}, "src/a.py": {"t/a.py"}},
		}
	}
	first, err := json.Marshal(BuildOutput(in()))
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	for i := 0; i < 20; i++ {
		again, err := json.Marshal(BuildOutput(in()))
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if string(again) != string(first) {
			t.Fatalf("output is not deterministic:\n%s\n%s", first, again)
		}
	}
	out := BuildOutput(in())
	if out.UnmappedFiles[0] != "src/a.py" {
		t.Fatalf("unmapped_files must be sorted: %#v", out.UnmappedFiles)
	}
	if out.Uncovered.Files[0].Path != "src/a.py" {
		t.Fatalf("uncovered.files must be sorted by path: %#v", out.Uncovered.Files)
	}
	if out.Run.Failures[0] != "t/a.py::t" {
		t.Fatalf("failures must be sorted: %#v", out.Run.Failures)
	}
	// Ranking is meaning, not noise: the selection keeps the order the selector chose.
	if out.Selection.Tests[0] != "t/z.py" || out.Selection.Count != 2 {
		t.Fatalf("selection must preserve rank: %#v", out.Selection)
	}
}

// run.* is all zero for `which`, which executes nothing — even if outcomes are handed in.
func TestBuildOutputNotExecutedHasZeroRunCounts(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "which", Base: "HEAD", Adapter: "python",
		Outcomes: []report.Outcome{{Test: "a", Status: "fail", DurationMS: 9}},
	})
	if out.Run.Executed || out.Run.Passed != 0 || out.Run.Failed != 0 || out.Run.Errored != 0 ||
		out.Run.Skipped != 0 || out.Run.DurationMS != 0 || len(out.Run.Failures) != 0 {
		t.Fatalf("run must be zero when nothing executed: %+v", out.Run)
	}
	if out.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0", out.ExitCode)
	}
}

// `complete` carries the never-narrow-silently guarantee into the document. A T2 tier
// whose suite was never enumerated is a PARTIAL list of the run, and a consumer that
// reads `selection.tests` as "run these ids" under-runs the suite. The flag says so on
// stdout, where a --json consumer actually reads.
func TestBuildOutputCompleteIsFalseForAnUnenumeratedT2(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "which", Base: "HEAD", Adapter: "python",
		Sel: selector.Selection{Tier: selector.TierT2, Tests: []string{"tests/a.py::t"}},
	})
	if out.Complete {
		t.Fatal("complete must be false for a T2 selection whose suite was never enumerated")
	}
}

// A direct test in the list does not make it complete: the run is still the whole suite.
func TestBuildOutputCompleteIsFalseForT2EvenWithADirectTest(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "which", Base: "HEAD", Adapter: "python",
		Sel: selector.Selection{
			Tier:   selector.TierT2,
			Direct: []string{"tests/test_new.py"},
			Tests:  []string{"tests/test_new.py"},
		},
	})
	if out.Complete {
		t.Fatal("a T2 selection carrying a direct test is still a partial list; complete must be false")
	}
}

// Enumerating the suite is what makes a T2 list whole, and `run` pays for it.
func TestBuildOutputCompleteIsTrueForAnEnumeratedT2(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "run", Base: "HEAD", Adapter: "python",
		Sel:             selector.Selection{Tier: selector.TierT2, Tests: []string{"tests/a.py::t"}},
		SuiteEnumerated: true,
	})
	if !out.Complete {
		t.Fatal("an enumerated T2 selection IS the whole suite; complete must be true")
	}
}

// Every other tier names its own tests exhaustively, so the list is complete by
// construction — including the empty tier, whose emptiness is fully known.
func TestBuildOutputCompleteIsTrueForEveryOtherTier(t *testing.T) {
	for _, tier := range []selector.Tier{
		selector.TierEmpty, selector.TierDirect, selector.TierT0, selector.TierT1,
	} {
		out := BuildOutput(OutputInput{
			Command: "which", Base: "HEAD", Adapter: "python",
			Sel: selector.Selection{Tier: tier},
		})
		if !out.Complete {
			t.Errorf("tier %s names its tests exhaustively; complete must be true", tier)
		}
	}
}

// `warnings` is the other half of the guarantee: the caveats that say the selection is
// narrower, or less authoritative, than it looks. They reach the document verbatim and in
// the order the command produced them.
func TestBuildOutputCarriesWarningsInOrder(t *testing.T) {
	want := []string{"no adapter - file classification is disabled", "an empty selection is not a pass."}
	out := BuildOutput(OutputInput{
		Command: "which", Base: "HEAD", Adapter: "",
		Warnings: want,
	})
	if len(out.Warnings) != len(want) {
		t.Fatalf("warnings = %#v, want %#v", out.Warnings, want)
	}
	for i := range want {
		if out.Warnings[i] != want[i] {
			t.Fatalf("warnings[%d] = %q, want %q", i, out.Warnings[i], want[i])
		}
	}
}
