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

// JSONAdapterSelection is ONE adapter's answer inside a polyglot document.
//
// It exists because `selection.tests` is a single list and two adapters' ids must never
// become one: a pytest nodeid handed to `npx vitest run` selects nothing and reports
// green. A consumer that means to invoke a runner reads `selections` and takes exactly
// one block's ids; the flat `selection` stays what it always was — everything rtdd
// selected, in rank order — for the consumers that only display it.
type JSONAdapterSelection struct {
	Adapter   string        `json:"adapter"`
	Tier      string        `json:"tier"`
	Reason    string        `json:"reason"`
	Complete  bool          `json:"complete"`
	Selection JSONSelection `json:"selection"`
}

// Output is the top-level --json document. Field order here is the emitted key order:
// the schema is a struct, never a map, so two identical runs marshal to identical bytes.
//
// `complete` and `warnings` carry the never-narrow-silently guarantee into the document
// itself. Under --json the document is the WHOLE of stdout and a consumer normally
// discards stderr, so a caveat that lives only on stderr is a caveat the agent front-end
// never sees — which is exactly the silent narrowing the contract forbids. The human
// stderr/stdout notes stay; these two fields are the machine-readable half of the same
// facts.
type Output struct {
	Schema        int           `json:"schema"`
	Command       string        `json:"command"`
	Base          string        `json:"base"`
	Adapter       string        `json:"adapter"`
	Tier          string        `json:"tier"`
	Reason        string        `json:"reason"`
	Complete      bool          `json:"complete"`
	Warnings      []string      `json:"warnings"`
	Changed       []JSONChange  `json:"changed"`
	Selection     JSONSelection `json:"selection"`
	Run           JSONRun       `json:"run"`
	Uncovered     JSONUncovered `json:"uncovered"`
	UnmappedFiles []string      `json:"unmapped_files"`
	// Selections is the per-adapter split, present ONLY in a repository where more than
	// one adapter was detected. `omitempty` is the compatibility promise: a
	// single-adapter document is byte-identical to the one schema v1 has always emitted,
	// so no existing consumer sees a new key, and one that does see it knows the flat
	// `selection` spans toolchains.
	Selections []JSONAdapterSelection `json:"selections,omitempty"`
	ExitCode   int                    `json:"exit_code"`
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
	// SuiteEnumerated records that the full suite WAS listed. It is the one thing that
	// can make a T2 selection complete, and only a command that pays for a collection
	// run (`rtdd run`) may set it.
	SuiteEnumerated bool
	// Warnings are the caveats the command already reports to a human, verbatim and in
	// the order it produced them.
	Warnings    []string
	Reports     []uncovered.FileReport
	UncoveredOK bool
	// UncoveredReason overrides the default explanation for an ABSENT uncovered report.
	// The default says the signal needs a run; an adapter that ran and instrumented
	// nothing needs the other sentence, and sending "run `rtdd run`" to a consumer that
	// just did is how a suppression turns into a wrong instruction (issue #345).
	UncoveredReason string
	UnmappedFiles   []string
	ImportFallback  map[string][]string
	// Blocks is the per-adapter split for a polyglot repository. One element — or none —
	// leaves the document exactly as it was before per-adapter selection existed.
	Blocks []AdapterSelection
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
		Complete:  buildComplete(in),
		Warnings:  nonNilStrings(in.Warnings),
	}

	out.Selections = buildSelections(in)
	out.UnmappedFiles = nonNilStrings(in.UnmappedFiles)
	sort.Strings(out.UnmappedFiles)

	if out.Run.Failed+out.Run.Errored > 0 {
		out.ExitCode = 1
	}
	return out
}

// buildSelections renders the per-adapter blocks, and only when there is more than one:
// a lone block says nothing the top-level `adapter` and `selection` do not already say,
// and emitting it would put a new key in every single-adapter document.
func buildSelections(in OutputInput) []JSONAdapterSelection {
	if len(in.Blocks) < 2 {
		return nil
	}
	out := make([]JSONAdapterSelection, 0, len(in.Blocks))
	for _, blk := range in.Blocks {
		out = append(out, JSONAdapterSelection{
			Adapter:  blk.Adapter,
			Tier:     blk.Selection.Tier.String(),
			Reason:   blk.Selection.Reason,
			Complete: blk.Selection.Tier != selector.TierT2 || blk.SuiteEnumerated,
			Selection: buildSelection(OutputInput{
				Sel:            blk.Selection,
				ImportFallback: blk.ImportFallback,
			}),
		})
	}
	return out
}

// buildComplete answers "is selection.tests the whole run?".
//
// T2 means the full suite, and a command that did not enumerate it can only list the map
// rows it happens to know — a PARTIAL list a consumer would otherwise read as the run.
// Direct tests in that list do not make it complete. Every other tier names its tests
// exhaustively, the empty tier included: its emptiness is fully known, and `warnings`
// rather than `complete` is what says an empty selection is not a pass.
func buildComplete(in OutputInput) bool {
	return in.Sel.Tier != selector.TierT2 || in.SuiteEnumerated
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
		if in.UncoveredReason != "" {
			return JSONUncovered{Reason: in.UncoveredReason}
		}
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
