// outcomes_test.go guards the outcome-README swap mechanism described in
// docs/outcomes/README.md: the repo root README.md must always be byte-identical
// to whichever docs/outcomes/README.<SELECTED>.md the docs/outcomes/SELECTED
// marker names, so the two READMEs can never silently diverge.
package installtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootREADMEMatchesSelectedOutcome(t *testing.T) {
	selectedRaw, err := os.ReadFile(filepath.Join("docs", "outcomes", "SELECTED"))
	if err != nil {
		t.Fatalf("read docs/outcomes/SELECTED: %v", err)
	}
	selected := strings.TrimSpace(string(selectedRaw))
	if selected != "positive" && selected != "negative" {
		t.Fatalf("docs/outcomes/SELECTED must be exactly %q or %q, got %q", "positive", "negative", selected)
	}

	outcomePath := filepath.Join("docs", "outcomes", "README."+selected+".md")
	outcome, err := os.ReadFile(outcomePath)
	if err != nil {
		t.Fatalf("read %s: %v", outcomePath, err)
	}

	root, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}

	if string(root) != string(outcome) {
		t.Fatalf("README.md is not byte-identical to %s (selected outcome %q) — the root README was edited without updating the selected outcome file, or SELECTED is stale", outcomePath, selected)
	}
}

func TestBothOutcomeFilesExist(t *testing.T) {
	for _, name := range []string{"README.positive.md", "README.negative.md"} {
		p := filepath.Join("docs", "outcomes", name)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s must exist so the swap is a copy, not a rewrite: %v", p, err)
		}
	}
}

// PRD #233 AC11: "It is Python only." was true before the static tier and is not true
// after it. The replacement is the two-tier statement, not a deletion — the Python-only
// LIMIT is still real for execution-derived selection, and dropping the bullet would
// quietly upgrade every non-Python repository's evidence.
func TestREADMEStatesTheTwoTiersRatherThanPythonOnly(t *testing.T) {
	for _, name := range []string{
		"README.md",
		filepath.Join("docs", "outcomes", "README.positive.md"),
		filepath.Join("docs", "outcomes", "README.negative.md"),
	} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		for _, stale := range []string{"**It is Python only.**", "Python only, today."} {
			if strings.Contains(text, stale) {
				t.Errorf("%s still claims %q", name, stale)
			}
		}
		for _, needle := range []string{"execution-derived", "static", "weaker evidence"} {
			if !strings.Contains(text, needle) {
				t.Errorf("%s does not state %q", name, needle)
			}
		}
	}
}

// PRD #233 AC12 / #351: the pre-registered static-tier kill condition is reported in the
// README, win or lose, and it names the measurement rather than asserting a conclusion.
func TestREADMEReportsThePreRegisteredStaticVerdict(t *testing.T) {
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{
		"bench/results/flask/summary.json",
		"bench/results/httpie/summary.json",
		"path heuristic",
		"pre-registered",
	} {
		if !strings.Contains(text, needle) {
			t.Errorf("README.md does not cite %q for the static-tier verdict", needle)
		}
	}
}

// The pre-registration, from spec §7: the static tier must BEAT the path baseline on
// change-level recall at comparable or better selected-duration fraction. A tie is not a
// beat, and recall bought by selecting more of the suite is not a beat either.
const (
	staticKillFiredClaim = "does not carry its weight as a distinct tier"
	staticKillHeldClaim  = "beats the `path` baseline at comparable or better selected duration"
)

// The repos whose committed summaries the comparison is read from. `sqlfluff` is excluded
// at corpus_version 2 and carries no static arm (#184, #349).
var staticVerdictRepos = []string{"flask", "httpie"}

type replayMetric struct {
	Value *float64 `json:"value"`
}

type replayArm struct {
	ChangeLevelRecall        replayMetric `json:"change_level_recall"`
	SelectedDurationFraction replayMetric `json:"selected_duration_fraction"`
}

type replaySummary struct {
	ByVariant map[string]map[string]replayArm `json:"by_variant"`
}

// TestREADMEStaticVerdictMatchesTheCommittedSummaries derives the verdict rather than
// trusting a hand-typed one (#351): a regenerated `summary.json` that flipped the
// comparison would otherwise leave the README's claim standing and stale, which is the one
// failure mode a pre-registration exists to prevent.
func TestREADMEStaticVerdictMatchesTheCommittedSummaries(t *testing.T) {
	beaten, computable := false, false
	var quoted []string
	for _, repo := range staticVerdictRepos {
		path := filepath.Join("bench", "results", repo, "summary.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var summary replaySummary
		if err := json.Unmarshal(raw, &summary); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if len(summary.ByVariant) == 0 {
			t.Fatalf("%s publishes no by_variant populations to compare", path)
		}
		for variant, arms := range summary.ByVariant {
			static, okS := arms["static"]
			base, okP := arms["path"]
			if !okS || !okP {
				t.Fatalf("%s/%s is missing the static or path arm", path, variant)
			}
			// Cost is comparable in every population; recall needs a denominator.
			quoted = append(quoted,
				fmt.Sprintf("%.3f", derefOr(static.SelectedDurationFraction.Value, 0)),
				fmt.Sprintf("%.3f", derefOr(base.SelectedDurationFraction.Value, 0)))
			if static.ChangeLevelRecall.Value == nil || base.ChangeLevelRecall.Value == nil {
				continue
			}
			computable = true
			quoted = append(quoted,
				fmt.Sprintf("%.3f", *static.ChangeLevelRecall.Value),
				fmt.Sprintf("%.3f", *base.ChangeLevelRecall.Value))
			sd, pd := static.SelectedDurationFraction.Value, base.SelectedDurationFraction.Value
			cheaper := sd == nil || pd == nil || *sd <= *pd
			if *static.ChangeLevelRecall.Value > *base.ChangeLevelRecall.Value && cheaper {
				beaten = true
			}
		}
	}
	if !computable {
		t.Fatal("no population scores change-level recall for both arms; the README's verdict would rest on nothing")
	}

	fired := !beaten
	want, unwanted := staticKillHeldClaim, staticKillFiredClaim
	if fired {
		want, unwanted = staticKillFiredClaim, staticKillHeldClaim
	}
	for _, name := range []string{
		"README.md",
		filepath.Join("docs", "outcomes", "README.positive.md"),
		filepath.Join("docs", "outcomes", "README.negative.md"),
	} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if !strings.Contains(text, want) {
			t.Errorf("the committed summaries say the kill condition fired=%v, but %s does not state %q", fired, name, want)
		}
		if strings.Contains(text, unwanted) {
			t.Errorf("%s still states %q, which the committed summaries contradict", name, unwanted)
		}
	}

	// Every figure the comparison turns on is quoted wherever the comparison is
	// published, so a regeneration that moves a number cannot leave a table describing
	// the previous run. That page is no longer the README: the README was cut back to the
	// one comparison the project exists to make — RTDD against running the whole suite —
	// and states this verdict with its sources rather than its arithmetic. The guarantee
	// is unchanged, only the file it is enforced against.
	page := filepath.Join("docs", "results", "axis2-selection-baselines.md")
	root, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("read %s: %v", page, err)
	}
	for _, q := range quoted {
		if !strings.Contains(string(root), q) {
			t.Errorf("%s does not quote %q from the committed summaries", page, q)
		}
	}
}

func derefOr(p *float64, fallback float64) float64 {
	if p == nil {
		return fallback
	}
	return *p
}

// --- the time axis is derived from the committed record, never hand-typed ---------
//
// The README's answer to "does it save the agent time?" is the agent-session measurement
// in bench/results/agent-session/flask.json. It does not come from bench/'s wall-clock
// columns and must not: bench/ drives pytest itself and never invokes `rtdd run`, so
// RTDD's own cost — the 9774 ms selection step this fixed — was outside every column it
// publishes. A re-measurement that moved these numbers would otherwise leave the README's
// table standing and stale, which is the failure mode #351 exists to prevent.

type agentSession struct {
	SessionMs struct {
		Full   int `json:"full"`
		Always int `json:"always"`
		Auto   int `json:"auto"`
	} `json:"session_ms"`
	SingleCommandMs struct {
		WhichBefore int `json:"rtdd_which_before_memoisation"`
		WhichAfter  int `json:"rtdd_which_after_memoisation"`
	} `json:"single_command_ms"`
}

func TestREADMETimeAxisMatchesTheCommittedAgentSession(t *testing.T) {
	path := filepath.Join("bench", "results", "agent-session", "flask.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var s agentSession
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)

	for what, want := range map[string]int{
		"full-suite baseline":    s.SessionMs.Full,
		"shipped default":        s.SessionMs.Always,
		"--record=auto":          s.SessionMs.Auto,
		"which before memoising": s.SingleCommandMs.WhichBefore,
		"which after memoising":  s.SingleCommandMs.WhichAfter,
	} {
		if !strings.Contains(text, fmt.Sprintf("%d ms", want)) {
			t.Errorf("README.md does not carry the committed %s (%d ms)", what, want)
		}
	}

	// A speedup claim that does not follow from the two numbers beside it is the one
	// thing a reader cannot check for themselves.
	if s.SessionMs.Always >= s.SessionMs.Full {
		t.Errorf("the record: shipped default %d ms is not faster than the baseline %d ms, "+
			"but the README claims a speedup", s.SessionMs.Always, s.SessionMs.Full)
	}
	if s.SessionMs.Auto >= s.SessionMs.Always {
		t.Errorf("the record: --record=auto %d ms is not faster than the default %d ms",
			s.SessionMs.Auto, s.SessionMs.Always)
	}
}

// The uncommitted-session curves were measured under the previous tier rule, where a
// full-escalate edit re-escalated for as long as it sat in the diff. They describe a rule
// the selector no longer implements, so the README must not summarise them as the tool's
// behaviour — it may only point at the file and say what they are.
func TestREADMEDoesNotPublishThePreFixSessionCurves(t *testing.T) {
	for _, name := range []string{
		"README.md",
		filepath.Join("docs", "outcomes", "README.positive.md"),
	} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		for _, stale := range []string{
			"24 of 25",
			"16 of 16",
			"24/25",
			"16/16",
		} {
			if strings.Contains(text, stale) {
				t.Errorf("%s quotes the pre-fix session curve %q as if it described the tool", name, stale)
			}
		}
	}
}

// --- the safety axis is derived from the paired run, never hand-typed -------------
//
// bench/results/paired/flask/ is a DIFFERENT population from the published corpus: only
// commits touching both a test file and a non-test file. It is kept apart from
// bench/results/flask/ for exactly that reason, and the README's safety table must read
// from it rather than restate it, so a re-run that moved a recall figure cannot leave the
// claim standing and stale.

type pairedArm struct {
	ChangeLevelRecall struct {
		Num   int      `json:"num"`
		Den   int      `json:"den"`
		Value *float64 `json:"value"`
	} `json:"change_level_recall"`
	DetectingCommits int `json:"detecting_commits"`
}

type pairedSummary struct {
	Strategies map[string]pairedArm `json:"strategies"`
}

func TestREADMESafetyAxisMatchesThePairedRun(t *testing.T) {
	path := filepath.Join("bench", "results", "paired", "flask", "summary.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var s pairedSummary
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)

	rtdd, ok := s.Strategies["rtdd"]
	if !ok {
		t.Fatalf("%s carries no rtdd arm", path)
	}

	// The claim the README makes is parity with the full suite. Deriving it means
	// reading BOTH arms rather than trusting the sentence.
	full, ok := s.Strategies["full"]
	if !ok {
		t.Fatalf("%s carries no full arm to compare against", path)
	}
	if rtdd.ChangeLevelRecall.Value == nil || full.ChangeLevelRecall.Value == nil {
		t.Fatal("recall is not computable in the paired run; the README must not claim parity")
	}
	if *rtdd.ChangeLevelRecall.Value < *full.ChangeLevelRecall.Value {
		t.Errorf("the record: rtdd recall %.3f is below the full suite's %.3f, "+
			"but the README claims it loses nothing a full run would have caught",
			*rtdd.ChangeLevelRecall.Value, *full.ChangeLevelRecall.Value)
	}

	// The rtdd ROW, not merely the digits: `1.000 (5/5)` also appears in testmon's row,
	// so a substring search passes against a README that has quietly halved rtdd's own
	// number. Confirmed: the loose form did exactly that before this line replaced it.
	want := fmt.Sprintf("%d.000 (%d/%d)", int(*rtdd.ChangeLevelRecall.Value),
		rtdd.ChangeLevelRecall.Num, rtdd.ChangeLevelRecall.Den)
	var row string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "**rtdd**") && strings.Contains(line, "|") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatal("README.md has no rtdd row in the safety table")
	}
	if !strings.Contains(row, want) {
		t.Errorf("README.md's rtdd row is %q, which does not carry the committed paired recall %q", row, want)
	}
	if !strings.Contains(text, fmt.Sprintf("| **%d** |", rtdd.DetectingCommits)) {
		t.Errorf("README.md does not carry the paired run's detecting count (%d)", rtdd.DetectingCommits)
	}
}
