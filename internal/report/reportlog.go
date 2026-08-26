// Package report parses pytest's --report-log JSONL. It is the ONLY source of the
// map's `s` (outcome) and `d` (duration) fields — no coverage report carries
// either one.
package report

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// Outcome is one test's result for a single run.
type Outcome struct {
	Test       string
	Status     string // "pass" | "fail" | "skip" | "error"
	DurationMS int
}

// rawReport is the subset of a --report-log envelope this package reads. Every
// envelope carries $report_type; only "TestReport" carries the rest.
type rawReport struct {
	Type     string  `json:"$report_type"`
	NodeID   string  `json:"nodeid"`
	When     string  `json:"when"`
	Outcome  string  `json:"outcome"`
	Duration float64 `json:"duration"` // SECONDS
}

// phaseAcc collects one test id's phases as its lines arrive. Phases for a single
// test are not guaranteed to be contiguous in the log, so they accumulate by id
// rather than being folded on sight.
type phaseAcc struct {
	setupOutcome, callOutcome, teardownOutcome string
	setupDur, callDur, teardownDur             float64
	hasCall                                    bool
}

// ReadReportLog parses pytest --report-log JSONL into one Outcome per test, in
// first-seen order.
//
// The call phase is the primary source, but it is not always present: a skipped
// test emits setup(skipped)+teardown(passed) and NO call entry, and a test whose
// fixture raises emits setup(failed)+teardown(passed) and no call entry. Both are
// measured on pytest 9.0.3. A reader that only inspects the call phase silently
// loses those tests.
//
// A malformed line is a fatal error, never a silent skip — dropping outcomes
// narrows the map.
func ReadReportLog(path string) ([]Outcome, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("report: opening %s: %w", path, err)
	}
	defer f.Close()

	accs := map[string]*phaseAcc{}
	var order []string

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // longrepr tracebacks get long
	for ln := 1; sc.Scan(); ln++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r rawReport
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("report: %s line %d: %w", path, ln, err)
		}
		// A CollectReport's nodeid is a file path, not a test id, so the type
		// check has to come before the id is used as a key.
		if r.Type != "TestReport" || r.NodeID == "" {
			continue
		}
		a := accs[r.NodeID]
		if a == nil {
			a = &phaseAcc{}
			accs[r.NodeID] = a
			order = append(order, r.NodeID)
		}
		switch r.When {
		case "setup":
			a.setupOutcome, a.setupDur = r.Outcome, r.Duration
		case "call":
			a.hasCall = true
			a.callOutcome, a.callDur = r.Outcome, r.Duration
		case "teardown":
			a.teardownOutcome, a.teardownDur = r.Outcome, r.Duration
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("report: reading %s: %w", path, err)
	}

	out := make([]Outcome, 0, len(order))
	for _, id := range order {
		status, ms := finalize(accs[id])
		out = append(out, Outcome{Test: id, Status: status, DurationMS: ms})
	}
	return out, nil
}

// finalize folds one test's phases into a status and a millisecond duration. The
// default is "error": a test whose phases say nothing recognisable did not pass,
// and reporting it as a pass would be a false green.
func finalize(a *phaseAcc) (string, int) {
	status := "error"
	switch {
	case a.setupOutcome == "failed":
		status = "error"
	case a.setupOutcome == "skipped":
		status = "skip"
	case a.hasCall:
		switch a.callOutcome {
		case "passed":
			status = "pass"
		case "failed":
			status = "fail"
		case "skipped":
			status = "skip"
		}
		if status == "pass" && a.teardownOutcome == "failed" {
			status = "error"
		}
	}
	sec := a.callDur
	if !a.hasCall {
		sec = a.setupDur + a.teardownDur
	}
	return status, int(math.Round(sec * 1000))
}
