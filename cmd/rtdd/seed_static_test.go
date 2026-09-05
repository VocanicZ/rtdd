package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// Exiting 0 having built no map is the worse failure: it leaves the caller believing a
// map exists. `rtdd seed` against a selection: static adapter refuses instead, and the
// refusal names the adapter — "seeding is unsupported" is unusable to someone who does
// not know which of their adapters is being talked about.
func TestCmdSeedRefusesAStaticAdapter(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "")

	code, stdout, stderr := rtdd(t, dir, "seed")

	// Exit 2: the adapter's own declaration is what makes the request impossible, which
	// is a configuration error rather than a test failure or a broken environment.
	if code != 2 {
		t.Fatalf("rtdd seed = %d against a selection: static adapter, want 2; it built no map, so it must not report success (stdout: %s)", code, stdout)
	}
	if !strings.Contains(stderr, "vitest") {
		t.Errorf("refusal does not name the adapter:\n%s", stderr)
	}
	if !strings.Contains(stderr, "records nothing") {
		t.Errorf("refusal does not say the adapter records nothing:\n%s", stderr)
	}
	if strings.Contains(stdout, "seeding with the") {
		t.Errorf("seed announced a run it must never start:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".rtdd", "map.jsonl")); err == nil {
		t.Error(".rtdd/map.jsonl was written by a refused seed")
	}
}

// The refusal is scoped to the adapter that cannot be seeded. A coverage adapter is what
// `rtdd seed` is for, and a blanket refusal would break every repository that has one.
func TestSeedRefusalIsScopedToStaticAdapters(t *testing.T) {
	coverage := &adapter.Adapter{Name: "python", Selection: adapter.SelectionCoverage, Coverage: "sqlite"}
	if msg := staticSeedRefusal(coverage); msg != "" {
		t.Errorf("staticSeedRefusal(python) = %q, want \"\" — a coverage adapter is seedable", msg)
	}

	static := &adapter.Adapter{Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone}
	if msg := staticSeedRefusal(static); msg == "" {
		t.Fatal("staticSeedRefusal(vitest) = \"\", want a refusal")
	}
}
