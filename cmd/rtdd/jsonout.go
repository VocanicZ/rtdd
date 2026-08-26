package main

import (
	"encoding/json"
	"sort"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/selector"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// SchemaVersion is the --json contract version. Consumers MUST reject an unknown value.
const SchemaVersion = 1

// unavailableReason explains an absent uncovered report. The signal is derived from fresh
// post-run coverage, so a command that executes nothing can never carry one.
const unavailableReason = "the uncovered-change signal requires fresh post-run coverage; run `rtdd run`"

// JSONLineRange is a 1-indexed inclusive range in the NEW file.
type JSONLineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// JSONChange is one entry of the changed set.
type JSONChange struct {
	Path           string          `json:"path"`
	Status         string          `json:"status"`
	Instrumentable bool            `json:"instrumentable"`
	Lines          []JSONLineRange `json:"lines"`
}

// JSONSelection is the ranked selection.
type JSONSelection struct {
	Count          int                 `json:"count"`
	Direct         []string            `json:"direct"`
	Tests          []string            `json:"tests"`
	ImportFallback map[string][]string `json:"import_fallback"`
}

// JSONRun is the outcome of the executed subset. All zero when Executed is false.
type JSONRun struct {
	Executed   bool     `json:"executed"`
	Passed     int      `json:"passed"`
	Failed     int      `json:"failed"`
	Skipped    int      `json:"skipped"`
	Errored    int      `json:"errored"`
	Failures   []string `json:"failures"`
	DurationMS int      `json:"duration_ms"`
}

// JSONClassifiedRange is one maximal run of changed lines sharing a class.
type JSONClassifiedRange struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Class string `json:"class"` // "covered" | "uncovered" | "import-time"
}

// JSONFileReport is one file's classification.
type JSONFileReport struct {
	Path           string                `json:"path"`
	Ranges         []JSONClassifiedRange `json:"ranges"`
	UncoveredLines int                   `json:"uncovered_lines"`
}

// JSONSummary totals the file reports.
type JSONSummary struct {
	Files           int `json:"files"`
	CoveredLines    int `json:"covered_lines"`
	UncoveredLines  int `json:"uncovered_lines"`
	ImportTimeLines int `json:"import_time_lines"`
}

// JSONUncovered is the uncovered-change signal. Available is false for `which`,
// which runs nothing and therefore has no fresh coverage.
type JSONUncovered struct {
	Available bool             `json:"available"`
	Reason    string           `json:"reason,omitempty"`
	Files     []JSONFileReport `json:"files,omitempty"`
	Summary   JSONSummary      `json:"summary"`
}

// uncoveredWire is JSONUncovered as it goes over the wire. `files` is a pointer here for
// one reason: an available report with nothing uncovered must emit `[]`, while an
// unavailable one must omit the key altogether, and those two mean opposite things to a
// consumer. A plain slice with `omitempty` collapses them, so the distinction is carried
// by nil-ness of the pointer instead.
type uncoveredWire struct {
	Available bool              `json:"available"`
	Reason    string            `json:"reason,omitempty"`
	Files     *[]JSONFileReport `json:"files,omitempty"`
	Summary   JSONSummary       `json:"summary"`
}

// MarshalJSON emits `files` only when the report is available — see uncoveredWire.
func (u JSONUncovered) MarshalJSON() ([]byte, error) {
	w := uncoveredWire{Available: u.Available, Reason: u.Reason, Summary: u.Summary}
	if u.Files != nil {
		w.Files = &u.Files
	}
	return json.Marshal(w)
}

// Output is the top-level --json document. Field order here is the emitted key order:
// the schema is a struct, never a map, so two identical runs marshal to identical bytes.
type Output struct {
	Schema        int           `json:"schema"`
	Command       string        `json:"command"`
	Base          string        `json:"base"`
	Adapter       string        `json:"adapter"`
	Tier          string        `json:"tier"`
	Reason        string        `json:"reason"`
	Changed       []JSONChange  `json:"changed"`
	Selection     JSONSelection `json:"selection"`
	Run           JSONRun       `json:"run"`
	Uncovered     JSONUncovered `json:"uncovered"`
	UnmappedFiles []string      `json:"unmapped_files"`
	ExitCode      int           `json:"exit_code"`
}

// OutputInput is everything BuildOutput needs. It is a plain struct so the schema can be
// tested without a repo, a runner, or coverage.
type OutputInput struct {
	Command        string
	Base           string
	Adapter        string
	Sel            selector.Selection
	Changes        []gitctx.Change
	Instrumentable map[string]bool
	Executed       bool
	Outcomes       []report.Outcome
	Reports        []uncovered.FileReport
	UncoveredOK    bool
	UnmappedFiles  []string
	ImportFallback map[string][]string
}

// BuildOutput assembles the --json document.
//
// ExitCode is 1 if and only if a test failed or errored. A non-empty uncovered report
// NEVER changes it: RTDD reports, it does not gate (spec §2, §6).
func BuildOutput(in OutputInput) Output {
	out := Output{
		Schema:    SchemaVersion,
		Command:   in.Command,
		Base:      in.Base,
		Adapter:   in.Adapter,
		Tier:      in.Sel.Tier.String(),
		Reason:    in.Sel.Reason,
		Changed:   buildChanged(in),
		Selection: buildSelection(in),
		Run:       buildRun(in),
		Uncovered: buildUncovered(in),
	}

	out.UnmappedFiles = nonNilStrings(in.UnmappedFiles)
	sort.Strings(out.UnmappedFiles)

	if out.Run.Failed+out.Run.Errored > 0 {
		out.ExitCode = 1
	}
	return out
}

// buildChanged preserves the changed set's own order — it is the diff's order, not noise.
func buildChanged(in OutputInput) []JSONChange {
	changed := make([]JSONChange, 0, len(in.Changes))
	for _, c := range in.Changes {
		lines := make([]JSONLineRange, 0, len(c.Lines))
		for _, r := range c.Lines {
			lines = append(lines, JSONLineRange{Start: r.Start, End: r.End})
		}
		changed = append(changed, JSONChange{
			Path:           c.Path,
			Status:         c.Status.String(),
			Instrumentable: in.Instrumentable[c.Path],
			Lines:          lines,
		})
	}
	return changed
}

// buildSelection keeps Direct and Tests in rank order: the selector already decided what
// runs first, and re-sorting here would throw that decision away.
func buildSelection(in OutputInput) JSONSelection {
	sel := JSONSelection{
		Count:          len(in.Sel.Tests),
		Direct:         nonNilStrings(in.Sel.Direct),
		Tests:          nonNilStrings(in.Sel.Tests),
		ImportFallback: make(map[string][]string, len(in.ImportFallback)),
	}
	for k, v := range in.ImportFallback {
		sel.ImportFallback[k] = nonNilStrings(v)
	}
	return sel
}

// buildRun counts outcomes. Nothing executed means every count is zero, whatever outcomes
// a caller happens to be holding: `which` runs no tests, so it reports none.
func buildRun(in OutputInput) JSONRun {
	run := JSONRun{Executed: in.Executed, Failures: []string{}}
	if !in.Executed {
		return run
	}
	for _, o := range in.Outcomes {
		run.DurationMS += o.DurationMS
		switch o.Status {
		case "pass":
			run.Passed++
		case "fail":
			run.Failed++
			run.Failures = append(run.Failures, o.Test)
		case "error":
			run.Errored++
			run.Failures = append(run.Failures, o.Test)
		case "skip":
			run.Skipped++
		}
	}
	sort.Strings(run.Failures)
	return run
}

// buildUncovered emits the report only when fresh post-run coverage backed it. When it
// did not, `files` is omitted entirely rather than sent as an empty list a consumer would
// read as "nothing uncovered" — and `reason` says why, present only in that case.
func buildUncovered(in OutputInput) JSONUncovered {
	if !in.UncoveredOK {
		return JSONUncovered{Reason: unavailableReason}
	}

	u := JSONUncovered{Available: true, Files: make([]JSONFileReport, 0, len(in.Reports))}
	for _, r := range in.Reports {
		fr := JSONFileReport{
			Path:           r.Path,
			Ranges:         make([]JSONClassifiedRange, 0, len(r.Ranges)),
			UncoveredLines: r.UncoveredLines(),
		}
		for _, cr := range r.Ranges {
			fr.Ranges = append(fr.Ranges, JSONClassifiedRange{
				Start: cr.Range.Start,
				End:   cr.Range.End,
				Class: cr.Class.String(),
			})
		}
		u.Files = append(u.Files, fr)
	}
	sort.Slice(u.Files, func(i, j int) bool { return u.Files[i].Path < u.Files[j].Path })

	s := uncovered.Summarize(in.Reports)
	u.Summary = JSONSummary{
		Files:           s.Files,
		CoveredLines:    s.CoveredLines,
		UncoveredLines:  s.UncoveredLines,
		ImportTimeLines: s.ImportTimeLines,
	}
	return u
}
