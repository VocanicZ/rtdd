package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// PRD #410 AC8 (#454): on this repository, with a warm .rtdd/graph.json and a small
// working-tree edit, `rtdd which` — graph load, changed set, Rounds and the text answer —
// finishes in under 1 s. The repository is cloned into a temp dir so the run writes its
// cache and makes its edit there, never in the checkout; the clone is HEAD's tree, which
// is this repository less any uncommitted edit.
func TestWhichOnThisRepositoryWithAWarmCacheIsUnder1s(t *testing.T) {
	wd, _ := os.Getwd()
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	parent := t.TempDir()
	gittest.Run(t, parent, "clone", "-q", root, "rtdd")
	dir := filepath.Join(parent, "rtdd")

	// The small edit: one line inserted halfway down a source file, so it lands inside
	// whatever node is there rather than only at the top level.
	edited := filepath.Join(dir, "internal", "rounds", "rounds.go")
	src, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(src), "\n")
	mid := len(lines) / 2
	lines = append(lines[:mid], append([]string{"\t// a small working-tree edit\n"}, lines[mid:]...)...)
	if err := os.WriteFile(edited, []byte(strings.Join(lines, "")), 0o644); err != nil {
		t.Fatal(err)
	}

	if code, _, errOut := rtdd(t, dir, "which"); code != 0 {
		t.Fatalf("cold rtdd which = %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, ".rtdd", "graph.json")); err != nil {
		t.Fatalf("the first rtdd which left no graph cache to be warm from: %v", err)
	}
	start := time.Now()
	code, out, errOut := rtdd(t, dir, "which")
	took := time.Since(start)
	if code != 0 {
		t.Fatalf("warm rtdd which = %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "internal/rounds/rounds.go::") {
		t.Fatalf("warm rtdd which names no changed node in the edited file:\n%s", out)
	}
	t.Logf("warm rtdd which: %d lines of output in %v", strings.Count(out, "\n"), took)
	if took >= time.Second {
		t.Errorf("warm rtdd which took %v, want < 1s", took)
	}
}
