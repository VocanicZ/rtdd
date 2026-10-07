package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// graphRepo is a two-file project: two functions, one test, two calls edges.
func graphRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// PRD #409 AC9: `rtdd graph` prints source, counts, built_at_commit and stale_files.
func TestGraphCommandPrintsSourceCountsAndStaleness(t *testing.T) {
	dir := graphRepo(t)
	code, out, errOut := rtdd(t, dir, "graph")
	if code != 0 {
		t.Fatalf("rtdd graph = %d, stderr %q", code, errOut)
	}
	want := "source:          scanner\n" +
		"nodes:           3\n" +
		"edges:           2\n" +
		"tests:           1\n" +
		"built_at_commit: " + gittest.HeadShort(t, dir) + "\n" +
		"stale_files:     0\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}
}

func TestGraphCommandJSONShape(t *testing.T) {
	dir := graphRepo(t)
	code, out, errOut := rtdd(t, dir, "graph", "--json")
	if code != 0 {
		t.Fatalf("rtdd graph --json = %d, stderr %q", code, errOut)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if doc["schema"] != 3.0 || doc["command"] != "graph" {
		t.Errorf("envelope = schema %v, command %v; want 3, graph", doc["schema"], doc["command"])
	}
	g, _ := doc["graph"].(map[string]any)
	want := map[string]any{"source": "scanner", "built_at_commit": gittest.HeadShort(t, dir),
		"stale_files": 0.0, "nodes": 3.0, "edges": 2.0, "tests": 1.0}
	for k, v := range want {
		if g[k] != v {
			t.Errorf("graph.%s = %v, want %v", k, g[k], v)
		}
	}
	if len(g) != len(want) {
		t.Errorf("graph has keys %v, want exactly %v (graphify_ignored only when graphify was ignored)", g, want)
	}
}

func TestGraphCommandExitCodes(t *testing.T) {
	if code, _, _ := rtdd(t, t.TempDir(), "graph"); code != 3 {
		t.Errorf("outside a git repository: exit %d, want 3", code)
	}
	dir := graphRepo(t)
	if code, _, _ := rtdd(t, dir, "graph", "--bogus"); code != 2 {
		t.Errorf("unknown flag: exit %d, want 2", code)
	}
	if _, out, _ := rtdd(t, dir, "--help"); !strings.Contains(out, "rtdd graph") {
		t.Errorf("--help does not list `rtdd graph`:\n%s", out)
	}
}
