package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// .rtdd/ is rtdd's own bookkeeping. It must never reach the selector: meta.json is
// rewritten at the end of every completed run, `**/*.json` is opaque in the python
// adapter, and an opaque change escalates — so without this filter every run after
// the first selects a tier above the one the developer's own edit puts it in.
func TestExcludeRtddDirDropsRtddsOwnBookkeeping(t *testing.T) {
	for _, p := range []string{
		".rtdd",
		".rtdd/meta.json",
		".rtdd/map.jsonl",
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
		"src/.rtdd/meta.json",
		".rtddmeta.json",
	} {
		got := excludeRtddDir([]gitctx.Change{{Path: p, Status: gitctx.Modified}})
		if len(got) != 1 || got[0].Path != p {
			t.Errorf("excludeRtddDir dropped %q: %+v", p, got)
		}
	}
}

// The selector feeds OldPath into the changed set too, so a rename OUT of .rtdd/
// would smuggle a `.rtdd/` path past a Path-only filter. The file itself is now a
// real user file and must survive; only its .rtdd/ provenance is dropped.
func TestExcludeRtddDirClearsAnOldPathUnderRtdd(t *testing.T) {
	got := excludeRtddDir([]gitctx.Change{
		{Path: "src/kept.py", OldPath: ".rtdd/meta.json", Status: gitctx.Renamed},
	})
	if len(got) != 1 {
		t.Fatalf("excludeRtddDir dropped a rename out of .rtdd/: %+v", got)
	}
	if got[0].Path != "src/kept.py" {
		t.Errorf("path = %q, want src/kept.py", got[0].Path)
	}
	if got[0].OldPath != "" {
		t.Errorf("old path = %q, want it cleared: no .rtdd/ path may reach the selector", got[0].OldPath)
	}
}

// A rename INSIDE .rtdd/ carries a .rtdd/ path in both fields and is dropped whole.
func TestExcludeRtddDirDropsARenameWithinRtdd(t *testing.T) {
	got := excludeRtddDir([]gitctx.Change{
		{Path: ".rtdd/map.jsonl", OldPath: ".rtdd/old.jsonl", Status: gitctx.Renamed},
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

// captureStdout runs f with os.Stdout redirected and returns what it printed.
// cmdRun prints its tier line with fmt.Printf, which is the only place the tier
// is observable from outside.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	func() {
		defer func() {
			os.Stdout = saved
			_ = w.Close()
		}()
		f()
	}()
	out := <-done
	_ = r.Close()
	return out
}

// The regression this issue is about, end to end against real pytest: seed, change
// ONE source file, run twice. The second run's only new dirt is rtdd's own
// meta.json rewrite, so it must report exactly the tier the first one did.
func TestCmdRunTierIsStableAcrossConsecutiveRuns(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	touchLogic(t, repo)

	var lines []string
	for i := 1; i <= 2; i++ {
		out := captureStdout(t, func() {
			if code := cmdRun(nil); code != 0 && code != 1 {
				t.Errorf("cmdRun #%d = %d, want 0 or 1", i, code)
			}
		})
		line := tierLine(t, out)
		if !strings.HasPrefix(line, "tier T0:") {
			t.Errorf("run #%d reported %q, want tier T0 — only src/logic.py changed", i, line)
		}
		if strings.Contains(line, ".rtdd") {
			t.Errorf("run #%d reason names rtdd's own bookkeeping: %q", i, line)
		}
		lines = append(lines, line)
	}
	if lines[0] != lines[1] {
		t.Errorf("two runs over the same source change disagree:\n #1 %s\n #2 %s", lines[0], lines[1])
	}
}

// The filter must not blunt the real signal: a user-authored .json is still an
// opaque change and still escalates.
func TestCmdRunStillEscalatesOnAUserAuthoredJSON(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	touchLogic(t, repo)
	data := filepath.Join(repo, "fixtures", "data.json")
	if err := os.MkdirAll(filepath.Dir(data), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(data, []byte("{\"a\": 1}\n"), 0o644); err != nil {
		t.Fatalf("write data.json: %v", err)
	}

	out := captureStdout(t, func() {
		if code := cmdRun(nil); code != 0 && code != 1 {
			t.Errorf("cmdRun = %d, want 0 or 1", code)
		}
	})
	line := tierLine(t, out)
	if !strings.HasPrefix(line, "tier T1:") {
		t.Errorf("reported %q, want tier T1 — a user-authored opaque .json changed", line)
	}
	if !strings.Contains(line, "fixtures/data.json") {
		t.Errorf("reported %q, want the reason to name fixtures/data.json", line)
	}
}

func tierLine(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "tier ") {
			return line
		}
	}
	t.Fatalf("no tier line in output:\n%s", out)
	return ""
}
