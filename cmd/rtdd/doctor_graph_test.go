package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// PRD #410 AC4: doctor prints graph source, graphify staleness, test-file counts and the
// names defined at least 8 times — and nothing about adapters or a coverage map.
func TestDoctorReportsTheGraphTestFilesAndOverLinkedNames(t *testing.T) {
	dir := gittest.Init(t)
	for i := 0; i < 8; i++ {
		src := "def run():\n    return 0\n"
		if i < 7 {
			src += "\n\ndef step():\n    return 1\n"
		}
		gittest.Write(t, dir, fmt.Sprintf("src/m%d.py", i), src)
	}
	gittest.Write(t, dir, "tests/test_run.py", "def test_run():\n    assert run() == 0\n")
	gittest.Write(t, dir, "tests/conftest_data.txt", "fixture data, not code\n")
	gittest.Commit(t, dir, "init")

	code, out, errOut := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("rtdd doctor = %d, stderr %q", code, errOut)
	}
	want := "graph source:  scanner\n" +
		"built at:      " + gittest.HeadShort(t, dir) + "\n" +
		"graphify:      not present (graphify-out/graph.json)\n" +
		"nodes:         16\n" +
		"edges:         8\n" +
		"test files:    2 matched by test_files, 1 with a test node\n" +
		"test nodes:    1\n" +
		"names defined 8 or more times (a call to one links to every definition):\n" +
		"  run  8\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}
	for _, banned := range []string{"adapter", "map.jsonl", "coverage", "fan-out"} {
		if strings.Contains(strings.ToLower(out), banned) {
			t.Errorf("doctor still mentions %q:\n%s", banned, out)
		}
	}
}

// Graphify present: used (with its staleness) when fresh, ignored with the reason when
// more than max_stale_ratio of its code files are stale.
func TestDoctorReportsGraphifyFreshAndTooStale(t *testing.T) {
	dir := graphRepo(t)
	writeGraphifyFor(t, dir, gittest.HeadShort(t, dir))
	gittest.Commit(t, dir, "graphify")
	_, out, _ := rtdd(t, dir, "doctor")
	if !strings.Contains(out, "graph source:  graphify+scanner\n") || !strings.Contains(out, "graphify:      used, built at ") {
		t.Errorf("fresh graphify not reported as used:\n%s", out)
	}

	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return b + a\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add\n\n\ndef test_add():\n    assert add(2, 1) == 3\n")
	_, out, _ = rtdd(t, dir, "doctor")
	if !strings.Contains(out, "graph source:  scanner\n") || !strings.Contains(out, "graphify:      ignored — ") ||
		!strings.Contains(out, "graphify --update") {
		t.Errorf("too-stale graphify not reported as ignored with its reason:\n%s", out)
	}
}

// AC6: environment errors exit 3, usage errors 2.
func TestDoctorExitCodes(t *testing.T) {
	if code, _, _ := rtdd(t, t.TempDir(), "doctor"); code != 3 {
		t.Errorf("outside a git repository: exit %d, want 3", code)
	}
	if code, _, _ := rtdd(t, graphRepo(t), "doctor", "--limit", "5"); code != 2 {
		t.Errorf("the removed --limit flag: exit %d, want 2", code)
	}
}

// PRD #410 AC4: doctor reads the graph, not the v0.2 coverage pipeline.
func TestDoctorImportsNoCoveragePipelinePackage(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "doctor.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		for _, banned := range []string{"internal/mapstore", "internal/adapter", "internal/doctor"} {
			if strings.HasSuffix(strings.Trim(imp.Path.Value, `"`), banned) {
				t.Errorf("doctor.go imports %s", imp.Path.Value)
			}
		}
	}
}
