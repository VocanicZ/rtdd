package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Spec §5, the ≥1-adapter branch: init records the detected adapters and their selection
// in .rtdd/config.yaml. The record is for a human reading the repo later — nothing parses
// it back, and `rtdd doctor` derives fidelity live and stays the source of truth.
func TestConfigWithAdaptersRecordsNameSelectionAndFidelity(t *testing.T) {
	got := ConfigWithAdapters([]AdapterRecord{
		{Name: "python", Selection: "coverage", Fidelity: "execution-derived"},
	})

	for _, want := range []string{"adapters:", "name: python", "selection: coverage", "fidelity: execution-derived"} {
		if !strings.Contains(got, want) {
			t.Errorf("config does not record %q:\n%s", want, got)
		}
	}
}

// The v1 defaults are not replaced by the record; they are joined by it.
func TestConfigWithAdaptersKeepsTheDefaults(t *testing.T) {
	got := ConfigWithAdapters([]AdapterRecord{{Name: "python", Selection: "coverage", Fidelity: "execution-derived"}})
	for _, want := range []string{"stale_commits: 50", "drift_guard: 100", "hub_threshold: 0.40"} {
		if !strings.Contains(got, want) {
			t.Errorf("config lost the default %q:\n%s", want, got)
		}
	}
}

// A repo two adapters serve records both, in the order it was given them — which is the
// order DetectAll returns, so repeated runs on one repo write the same file.
func TestConfigWithAdaptersRecordsEveryDetectedAdapterInOrder(t *testing.T) {
	got := ConfigWithAdapters([]AdapterRecord{
		{Name: "vitest", Selection: "static", Fidelity: "static"},
		{Name: "python", Selection: "coverage", Fidelity: "execution-derived"},
	})
	iv, ip := strings.Index(got, "name: vitest"), strings.Index(got, "name: python")
	if iv < 0 || ip < 0 {
		t.Fatalf("config does not record both adapters:\n%s", got)
	}
	if iv > ip {
		t.Errorf("adapters recorded out of order:\n%s", got)
	}
}

// No detected adapter is the `--force` install. It gets the unchanged three-key default:
// an empty `adapters:` key would read as "RTDD looked and found an empty set of them",
// which is a different claim from "this install made no detection promise at all".
func TestConfigWithNoAdaptersIsTheUnchangedDefault(t *testing.T) {
	if got := ConfigWithAdapters(nil); got != defaultConfig {
		t.Errorf("ConfigWithAdapters(nil) = %q, want the unchanged default config", got)
	}
}

// Plan is what puts the record on disk: given the detected set, the config step it emits
// carries the record rather than the bare defaults.
func TestPlanWritesTheDetectedAdaptersIntoTheConfigStep(t *testing.T) {
	root := t.TempDir()
	recs := []AdapterRecord{{Name: "python", Selection: "coverage", Fidelity: "execution-derived"}}

	steps, err := Plan(root, fakeFiles(), false, recs)
	if err != nil {
		t.Fatal(err)
	}
	s := stepFor(t, steps, ".rtdd/config.yaml")
	if s.Action != Create {
		t.Fatalf("action = %v, want Create", s.Action)
	}
	if s.Content != ConfigWithAdapters(recs) {
		t.Errorf("config step content = %q, want ConfigWithAdapters(recs)", s.Content)
	}
}

// An existing config is a config someone tuned. The record is worth having on a first
// install and is never worth clobbering a tuned file for.
func TestPlanNeverRewritesAnExistingConfigToAddTheRecord(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	tuned := "stale_commits: 999\n"
	if err := os.WriteFile(filepath.Join(root, ".rtdd", "config.yaml"), []byte(tuned), 0o644); err != nil {
		t.Fatal(err)
	}

	steps, err := Plan(root, fakeFiles(), false, []AdapterRecord{{Name: "python", Selection: "coverage", Fidelity: "execution-derived"}})
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".rtdd/config.yaml"); s.Action != Skip {
		t.Fatalf("action = %v, want Skip", s.Action)
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".rtdd", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != tuned {
		t.Errorf("config.yaml = %q, want it untouched at %q", string(b), tuned)
	}
}
