package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// writeGraphifyFor records graphify's view of graphRepo at builtAt (spec §5).
func writeGraphifyFor(t *testing.T, dir, builtAt string) {
	t.Helper()
	commit := ""
	if builtAt != "" {
		commit = `,"built_at_commit":"` + builtAt + `"`
	}
	gittest.Write(t, dir, "graphify-out/graph.json", `{"nodes":[`+
		`{"id":"add","label":"add()","file_type":"code","source_file":"src/calc.py","source_location":"L1"},`+
		`{"id":"total","label":"total()","file_type":"code","source_file":"src/calc.py","source_location":"L5"},`+
		`{"id":"t","label":"test_add()","file_type":"code","source_file":"tests/test_calc.py","source_location":"L4"}],`+
		`"links":[{"source":"total","target":"add","relation":"calls"},{"source":"t","target":"add","relation":"calls"}]`+commit+`}`)
}

// PRD #409 AC9, graphify half: the source, graphify's commit and the stale count.
func TestGraphCommandReportsGraphifyAndStaleness(t *testing.T) {
	dir := graphRepo(t)
	full := strings.TrimSpace(gittest.Run(t, dir, "rev-parse", "HEAD"))
	writeGraphifyFor(t, dir, full)
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add\n\n\ndef test_add():\n    assert add(2, 2) == 4\n")

	code, out, errOut := rtdd(t, dir, "graph")
	if code != 0 {
		t.Fatalf("rtdd graph = %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"source:          graphify+scanner\n", "built_at_commit: " + full + "\n", "stale_files:     1\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	_, out, _ = rtdd(t, dir, "graph", "--json")
	var doc struct {
		Graph map[string]any `json:"graph"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Graph["source"] != "graphify+scanner" || doc.Graph["built_at_commit"] != full || doc.Graph["stale_files"] != 1.0 {
		t.Errorf("graph = %v", doc.Graph)
	}
}

// PRD #409 AC6: an ignored graphify says why — in words and as graph.graphify_ignored —
// reports source scanner, and suggests `graphify --update`.
func TestGraphCommandSaysWhyGraphifyWasIgnored(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		why     string
		jsonWhy string
	}{
		{"built_at_commit missing", func(t *testing.T, dir string) { writeGraphifyFor(t, dir, "") },
			"ignored — its graph records no built_at_commit;", "built_at_commit_missing"},
		{"built_at_commit unknown to git", func(t *testing.T, dir string) {
			writeGraphifyFor(t, dir, "0123456789abcdef0123456789abcdef01234567")
		}, "ignored — its built_at_commit 0123456789abcdef0123456789abcdef01234567 is unknown to git;", "built_at_commit_unknown"},
		{"more than half stale", func(t *testing.T, dir string) {
			writeGraphifyFor(t, dir, strings.TrimSpace(gittest.Run(t, dir, "rev-parse", "HEAD")))
			gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return b + a\n")
			gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add\n\n\ndef test_add():\n    assert add(2, 2) == 4\n")
		}, "ignored — 2 of its 2 code files are stale, more than max_stale_ratio 0.50;", "too_stale"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := graphRepo(t)
			c.setup(t, dir)
			code, out, errOut := rtdd(t, dir, "graph")
			if code != 0 {
				t.Fatalf("rtdd graph = %d, stderr %q", code, errOut)
			}
			for _, want := range []string{"source:          scanner\n", "graphify:        " + c.why, "run `graphify --update` to use it again\n"} {
				if !strings.Contains(out, want) {
					t.Errorf("stdout lacks %q:\n%s", want, out)
				}
			}
			_, out, _ = rtdd(t, dir, "graph", "--json")
			var doc struct {
				Graph map[string]any `json:"graph"`
			}
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.Graph["source"] != "scanner" || doc.Graph["graphify_ignored"] != c.jsonWhy {
				t.Errorf("graph = %v, want source scanner and graphify_ignored %q", doc.Graph, c.jsonWhy)
			}
		})
	}
}

// Spec §5, §12: rtdd never runs graphify itself, not even when it ignores a stale one.
func TestGraphCommandNeverRunsGraphify(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	script := "#!/bin/sh\necho \"$@\" > '" + marker + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "graphify"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := graphRepo(t)
	writeGraphifyFor(t, dir, "")
	if code, _, errOut := rtdd(t, dir, "graph"); code != 0 {
		t.Fatalf("rtdd graph = %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("rtdd graph executed graphify")
	}
}
