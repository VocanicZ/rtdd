package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// .rtdd/ is rtdd's own bookkeeping. It must never reach the changed set: graph.json is
// rewritten by every command that builds the graph, so without this filter rtdd's own
// cache write would read as a change the developer made.
func TestExcludeRtddDirDropsRtddsOwnBookkeeping(t *testing.T) {
	for _, p := range []string{
		".rtdd",
		".rtdd/config.yaml",
		".rtdd/graph.json",
		".rtdd/adapter.yaml",
		".rtdd/nested/deeper.json",
	} {
		got := excludeRtddDir([]gitctx.Change{{Path: p, Status: gitctx.Modified}})
		if len(got) != 0 {
			t.Errorf("excludeRtddDir kept %q: %+v", p, got)
		}
	}
}

// The filter is rtdd's OWN directory at the repo root, not any path that looks like
// it. Over-filtering silently stops selecting on files the developer wrote.
func TestExcludeRtddDirKeepsEverythingElse(t *testing.T) {
	for _, p := range []string{
		"src/logic.py",
		"fixtures/data.json",
		".rtddx/meta.json",
		"src/.rtdd/config.yaml",
		".rtddmeta.json",
	} {
		got := excludeRtddDir([]gitctx.Change{{Path: p, Status: gitctx.Modified}})
		if len(got) != 1 || got[0].Path != p {
			t.Errorf("excludeRtddDir dropped %q: %+v", p, got)
		}
	}
}

// A change carries OldPath too, so a rename OUT of .rtdd/
// would smuggle a `.rtdd/` path past a Path-only filter. The file itself is now a
// real user file and must survive; only its .rtdd/ provenance is dropped.
func TestExcludeRtddDirClearsAnOldPathUnderRtdd(t *testing.T) {
	got := excludeRtddDir([]gitctx.Change{
		{Path: "src/kept.py", OldPath: ".rtdd/config.yaml", Status: gitctx.Renamed},
	})
	if len(got) != 1 {
		t.Fatalf("excludeRtddDir dropped a rename out of .rtdd/: %+v", got)
	}
	if got[0].Path != "src/kept.py" {
		t.Errorf("path = %q, want src/kept.py", got[0].Path)
	}
	if got[0].OldPath != "" {
		t.Errorf("old path = %q, want it cleared: no .rtdd/ path may reach the changed set", got[0].OldPath)
	}
}

// A rename INSIDE .rtdd/ carries a .rtdd/ path in both fields and is dropped whole.
func TestExcludeRtddDirDropsARenameWithinRtdd(t *testing.T) {
	got := excludeRtddDir([]gitctx.Change{
		{Path: ".rtdd/graph.json", OldPath: ".rtdd/old.jsonl", Status: gitctx.Renamed},
	})
	if len(got) != 0 {
		t.Errorf("excludeRtddDir kept a rename within .rtdd/: %+v", got)
	}
}

// The whole point of routing every command through changedSet: nothing in cmd/rtdd
// may reach gitctx.ChangedSet directly, or the filter is one forgotten call site
// away from being reintroduced.
func TestOnlyChangedGoCallsGitctxChangedSet(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var offenders []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if name == "changed.go" {
			continue
		}
		b, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		if strings.Contains(string(b), "gitctx.ChangedSet(") {
			offenders = append(offenders, name)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("gitctx.ChangedSet is called outside cmd/rtdd/changed.go: %v\n"+
			"Every command must go through changedSet, which strips rtdd's own .rtdd/ writes.", offenders)
	}
}
