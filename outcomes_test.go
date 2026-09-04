// outcomes_test.go guards the outcome-README swap mechanism described in
// docs/outcomes/README.md: the repo root README.md must always be byte-identical
// to whichever docs/outcomes/README.<SELECTED>.md the docs/outcomes/SELECTED
// marker names, so the two READMEs can never silently diverge.
package installtest

import (
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
