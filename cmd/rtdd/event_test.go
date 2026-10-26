package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// eventLines reads RTDD_EVENT_LOG's file as its JSON lines, failing on any line that is
// not a whole JSON object.
func eventLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("event log %q does not end in a newline", b)
	}
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("event line %q is not JSON: %v", l, err)
		}
		if len(l)+1 >= maxEventLine {
			t.Errorf("event line is %d bytes, want under %d", len(l)+1, maxEventLine)
		}
		out = append(out, m)
	}
	return out
}

// #503 AC1: one line per invocation, and its counts are `rtdd which --json`'s.
func TestEventLogWhichWritesOneLineWithTheJSONCounts(t *testing.T) {
	dir := goldenWhichRepo(t)
	code, out, _ := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("rtdd which --json = %d", code)
	}
	var doc struct {
		ChangedNodes []any `json:"changed_nodes"`
		Rounds       []struct {
			Tests []any `json:"tests"`
		} `json:"rounds"`
		Untested []any    `json:"untested"`
		Warnings []string `json:"warnings"`
		Graph    struct {
			Source     string `json:"source"`
			BuiltAt    string `json:"built_at_commit"`
			StaleFiles int    `json:"stale_files"`
			Nodes      int    `json:"nodes"`
			Edges      int    `json:"edges"`
		} `json:"graph"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}

	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(eventLogVar, log)
	t.Setenv("HARNESS_ISSUE", "503")
	code, _, _ = rtdd(t, dir, "which", "--base", "HEAD")
	if code != 0 {
		t.Fatalf("rtdd which = %d", code)
	}
	lines := eventLines(t, log)
	if len(lines) != 1 {
		t.Fatalf("event log holds %d lines, want 1", len(lines))
	}
	e := lines[0]
	want := map[string]any{
		"v": 1.0, "cmd": "which", "base": "HEAD", "exit": 0.0, "version": version,
		"head": gittest.HeadShort(t, dir), "changed_nodes": float64(len(doc.ChangedNodes)),
		"round1": float64(len(doc.Rounds[0].Tests)), "round2": float64(len(doc.Rounds[1].Tests)),
		"untested": float64(len(doc.Untested)), "deleted_warnings": float64(len(doc.Warnings)),
	}
	for k, v := range want {
		if e[k] != v {
			t.Errorf("event[%q] = %v, want %v", k, e[k], v)
		}
	}
	if len(doc.Warnings) == 0 || len(doc.Rounds[0].Tests) == 0 {
		t.Fatalf("fixture lost its deleted file or its Round 1: %+v", doc)
	}
	g, _ := e["graph"].(map[string]any)
	if g["source"] != doc.Graph.Source || g["built_at"] != doc.Graph.BuiltAt || g["stale_files"] != float64(doc.Graph.StaleFiles) ||
		g["nodes"] != float64(doc.Graph.Nodes) || g["edges"] != float64(doc.Graph.Edges) || g["cache"] != "warm" {
		t.Errorf("event graph = %v, want %+v with a warm cache", g, doc.Graph)
	}
	if ctx, _ := e["ctx"].(map[string]any); ctx["HARNESS_ISSUE"] != "503" {
		t.Errorf("event ctx = %v, want HARNESS_ISSUE=503", e["ctx"])
	}
	if argv, _ := e["argv"].([]any); fmt.Sprint(argv) != "[which --base HEAD]" {
		t.Errorf("event argv = %v", e["argv"])
	}
	if repo, _ := e["repo"].(string); repo == "" || e["cwd"] == "" {
		t.Errorf("event repo = %v, cwd = %v, want both set", e["repo"], e["cwd"])
	}
	if _, ok := e["duration_ms"].(float64); !ok {
		t.Errorf("event duration_ms = %v", e["duration_ms"])
	}
}

// #503 AC2: unset, rtdd creates no file; unwritable, rtdd answers exactly as unset.
func TestEventLogUnsetOrUnwritableChangesNothing(t *testing.T) {
	dir := goldenWhichRepo(t)
	t.Setenv(eventLogVar, "")
	code, out, errOut := rtdd(t, dir, "which")
	before := gitRun(t, dir, "status", "--porcelain", "--ignored")

	t.Setenv(eventLogVar, filepath.Join(t.TempDir(), "no", "such", "dir", "events.jsonl"))
	code2, out2, errOut2 := rtdd(t, dir, "which")
	if code2 != code || out2 != out || errOut2 != errOut {
		t.Errorf("unwritable log changed the answer: exit %d/%d\nstdout %q\nvs     %q\nstderr %q\nvs     %q",
			code2, code, out2, out, errOut2, errOut)
	}
	if after := gitRun(t, dir, "status", "--porcelain", "--ignored"); after != before {
		t.Errorf("rtdd which wrote into the repository: %q -> %q", before, after)
	}
}

func TestEventLogUnsetCreatesNoFile(t *testing.T) {
	dir := newTestRepo(t)
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(eventLogVar, "")
	rtdd(t, dir, "which")
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Errorf("stat %s = %v, want not exist", log, err)
	}
	if current != nil {
		t.Error("an unset RTDD_EVENT_LOG left an event open")
	}
}

// #503 AC3: 20 rtdd processes writing at once leave 20 whole lines.
func TestEventLogParallelProcessesWriteWholeLines(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "rtdd")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	dir := goldenWhichRepo(t)
	log := filepath.Join(t.TempDir(), "events.jsonl")
	// A long argv and environment make each line near the bound, where an interleaved
	// write would show.
	long := strings.Repeat("x", 1500)
	env := append(os.Environ(), eventLogVar+"="+log, "RTDD_HOME="+t.TempDir(), "HARNESS_PAD="+long)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(bin, "which", "--base", "HEAD")
			cmd.Dir, cmd.Env = dir, env
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("rtdd which: %v\n%s", err, out)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	lines := eventLines(t, log)
	if len(lines) != 20 {
		t.Fatalf("event log holds %d lines, want 20", len(lines))
	}
	for _, e := range lines {
		if e["cmd"] != "which" || e["exit"] != 0.0 {
			t.Errorf("event = %v, want cmd which, exit 0", e)
		}
	}
}

// A line never outgrows maxEventLine: argv goes first, then ctx and cwd.
func TestEventLogLineStaysUnderTheBound(t *testing.T) {
	dir := newTestRepo(t)
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(eventLogVar, log)
	t.Setenv("HARNESS_SESSION", "s1")
	rtdd(t, dir, "explain", strings.Repeat("y", 5000))
	t.Setenv("HARNESS_PAD", strings.Repeat("z", 5000))
	rtdd(t, dir, "--version")
	lines := eventLines(t, log)
	if len(lines) != 2 {
		t.Fatalf("event log holds %d lines, want 2", len(lines))
	}
	if e := lines[0]; e["argv"] != nil || e["truncated"] != true || e["exit"] != 2.0 {
		t.Errorf("long argv event = %v, want argv dropped, truncated, exit 2", e)
	} else if ctx, _ := e["ctx"].(map[string]any); ctx["HARNESS_SESSION"] != "s1" {
		t.Errorf("long argv event ctx = %v, want it kept", e["ctx"])
	}
	if e := lines[1]; e["ctx"] != nil || e["cwd"] != nil || e["truncated"] != true || e["cmd"] != "--version" {
		t.Errorf("long env event = %v, want ctx and cwd dropped", e)
	}
}

// Each command records its own fields: init its plan, graph and doctor the graph, doctor
// its findings; a command outside a repository records no repo.
func TestEventLogRecordsEachCommandsFields(t *testing.T) {
	dir := newTestRepo(t)
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(eventLogVar, log)
	for _, args := range [][]string{{"init", "--dry-run"}, {"init"}, {"graph"}, {"graph", "--json"}, {"doctor"}} {
		if code, _, errOut := rtdd(t, dir, args...); code != 0 {
			t.Fatalf("rtdd %v = %d, stderr %q", args, code, errOut)
		}
	}
	rtdd(t, t.TempDir(), "--version")
	lines := eventLines(t, log)
	if len(lines) != 6 {
		t.Fatalf("event log holds %d lines, want 6", len(lines))
	}
	dry, _ := lines[0]["init"].(map[string]any)
	if steps, _ := dry["steps"].(map[string]any); dry["dry_run"] != true || steps["create"] != 2.0 {
		t.Errorf("init --dry-run event init = %v, want dry_run and 2 creates", dry)
	}
	if in, _ := lines[1]["init"].(map[string]any); in["dry_run"] != nil {
		t.Errorf("init event init = %v, want no dry_run", in)
	}
	if g, _ := lines[2]["graph"].(map[string]any); g["cache"] != "cold" || g["source"] != "scanner" || g["nodes"] == 0.0 {
		t.Errorf("first graph event graph = %v, want a cold scanner graph", g)
	}
	if g, _ := lines[3]["graph"].(map[string]any); g["cache"] != "warm" {
		t.Errorf("second graph event graph = %v, want a warm cache", g)
	}
	if lines[4]["findings"] != 0.0 || lines[4]["graph"] == nil {
		t.Errorf("doctor event = %v, want findings 0 and a graph", lines[4])
	}
	for _, k := range []string{"round1", "base", "init", "findings"} {
		if _, ok := lines[2][k]; ok {
			t.Errorf("graph event carries %q: %v", k, lines[2])
		}
	}
	if e := lines[5]; e["repo"] != nil || e["head"] != nil || e["exit"] != 0.0 {
		t.Errorf("--version outside a repo event = %v, want no repo or head", e)
	}
}
