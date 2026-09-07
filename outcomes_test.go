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

	// Every figure the comparison turns on is quoted, so a regeneration that moves a
	// number cannot leave the README's table describing the previous run.
	root, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range quoted {
		if !strings.Contains(string(root), q) {
			t.Errorf("README.md does not quote %q from the committed summaries", q)
		}
	}
}

func derefOr(p *float64, fallback float64) float64 {
	if p == nil {
		return fallback
	}
	return *p
}
