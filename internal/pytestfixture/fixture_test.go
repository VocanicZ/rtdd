package pytestfixture

import (
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestMaterializeWritesTheProject(t *testing.T) {
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	for _, rel := range []string{
		"pyproject.toml", "src/__init__.py", "src/constants.py", "src/logic.py",
		"tests/__init__.py", "tests/test_a.py", "tests/test_b.py",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

// Materialize needs no network and may be called twice over the same tree: every
// integration test in M1b materialises into a fresh t.TempDir() and some of them
// re-materialise to undo an edit.
func TestMaterializeIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	logic := filepath.Join(dir, "src", "logic.py")
	if err := os.WriteFile(logic, []byte("# clobbered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize (second): %v", err)
	}
	b, err := os.ReadFile(logic)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != Logic {
		t.Errorf("src/logic.py = %q, want the pinned fixture body", b)
	}
}

// The line numbers in every coverage assertion in this milestone depend on these
// exact layouts. Pin them.
func TestFixtureLineNumbersArePinned(t *testing.T) {
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	cases := []struct {
		rel  string
		line int
		want string
	}{
		{"src/constants.py", 1, "from dataclasses import dataclass"},
		{"src/constants.py", 3, "MAX = 10"},
		{"src/constants.py", 5, "@dataclass"},
		{"src/constants.py", 6, "class Cfg:"},
		{"src/constants.py", 7, "    a: int = 1"},
		{"src/logic.py", 1, "from src.constants import MAX"},
		{"src/logic.py", 4, "def add(a, b):"},
		{"src/logic.py", 5, "    return a + b"},
		{"src/logic.py", 8, "def mul(a, b):"},
		{"src/logic.py", 9, "    return a * b"},
		{"src/logic.py", 12, "def unused(x):"},
		{"src/logic.py", 13, "    return x - 1"},
	}
	for _, tc := range cases {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(tc.rel)))
		if err != nil {
			t.Fatalf("read %s: %v", tc.rel, err)
		}
		lines := strings.Split(string(b), "\n")
		if len(lines) < tc.line {
			t.Fatalf("%s has %d lines, want at least %d", tc.rel, len(lines), tc.line)
		}
		if got := lines[tc.line-1]; got != tc.want {
			t.Errorf("%s:%d = %q, want %q", tc.rel, tc.line, got, tc.want)
		}
	}
}

func TestInitGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if err := InitGit(dir); err != nil {
		t.Fatalf("InitGit: %v", err)
	}
	// The read-back goes through gittest for the same reason InitGit does: the git
	// shell-out lives in internal/gitctx and nowhere else.
	if got := gittest.HeadShort(t, dir); len(got) < 4 {
		t.Fatalf("HEAD = %q, want a short SHA", got)
	}
	if got := strings.TrimSpace(gittest.Run(t, dir, "status", "--porcelain")); got != "" {
		t.Errorf("fixture not fully committed: %q", got)
	}
}

// This is the audit A1 case reproduced end to end: a correctly tested
// constants.py is attributed to no test at all.
func TestFixtureRunsUnderRealPytest(t *testing.T) {
	if !HavePytest() {
		t.Skip("pytest not on PATH")
	}
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	s := runPytest(t, dir)
	if !strings.Contains(s, "1 failed") || !strings.Contains(s, "1 skipped") {
		t.Fatalf("fixture suite summary changed; want 1 failed and 1 skipped. output:\n%s", s)
	}
	if strings.Contains(s, "no-sysmon-context") {
		t.Fatalf("COVERAGE_CORE=ctrace did not take effect:\n%s", s)
	}
	if _, err := os.Stat(filepath.Join(dir, ".coverage")); err != nil {
		t.Fatalf(".coverage not written to the repo root: %v", err)
	}
}

// Drift in the fixture must fail here, not three tasks later inside a coverage
// assertion that has no idea why its line numbers moved. These are the measured
// attributions the rest of M1b is written against.
func TestMeasuredCoverageAttributionsHold(t *testing.T) {
	if !HavePytest() {
		t.Skip("pytest not on PATH")
	}
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	runPytest(t, dir)
	got := readCoverage(t, dir)

	// src/constants.py is audit A1: import-time lines only, zero test contexts.
	constants := got["src/constants.py"]
	if want := []int{1, 3, 5, 6, 7}; !reflect.DeepEqual(constants[""], want) {
		t.Errorf("src/constants.py empty context = %v, want %v", constants[""], want)
	}
	for ctx := range constants {
		if ctx != "" {
			t.Errorf("src/constants.py is attributed to context %q, want zero test contexts", ctx)
		}
	}

	logic := got["src/logic.py"]
	if want := []int{1, 4, 8, 12}; !reflect.DeepEqual(logic[""], want) {
		t.Errorf("src/logic.py empty context = %v, want %v", logic[""], want)
	}
	if want := []int{5}; !reflect.DeepEqual(logic["tests/test_a.py::test_add|run"], want) {
		t.Errorf("test_add context = %v, want %v", logic["tests/test_a.py::test_add|run"], want)
	}
	for _, ctx := range []string{"tests/test_b.py::test_mul|run", "tests/test_b.py::test_fail|run"} {
		if want := []int{9}; !reflect.DeepEqual(logic[ctx], want) {
			t.Errorf("%s context = %v, want %v", ctx, logic[ctx], want)
		}
	}
	for ctx, lines := range logic {
		if slices.Contains(lines, 13) {
			t.Errorf("src/logic.py line 13 (unused) is covered by context %q, want nothing", ctx)
		}
	}

	// The two awkward parametrised ids the selector round-trip depends on.
	for _, ctx := range []string{
		"tests/test_a.py::test_param[1-one two]|run",
		"tests/test_a.py::test_param[2-a-b]|run",
	} {
		if _, ok := logic[ctx]; !ok {
			t.Errorf("missing context %q; got %v", ctx, slices.Sorted(maps.Keys(logic)))
		}
	}
}

// runPytest runs the fixture suite exactly the way the shipped adapter will, and
// returns the combined output. The suite exits 1 by design, so the status is ignored.
func runPytest(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("pytest", "--cov", "--cov-context=test", "--cov-report=", "-q")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "COVERAGE_CORE=ctrace", "COVERAGE_FILE=.coverage")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// readCoverage decodes dir/.coverage into repo-relative path -> context -> lines.
// internal/coverage does not exist yet, so the decode goes through coverage.py's own
// numbits helper: this test is about the fixture, not about RTDD's reader.
const decodeScript = `
import json, sqlite3, sys
from coverage.numbits import numbits_to_nums
con = sqlite3.connect(sys.argv[1])
files = dict(con.execute("select id, path from file"))
ctxs = dict(con.execute("select id, context from context"))
out = {}
for fid, cid, nb in con.execute("select file_id, context_id, numbits from line_bits"):
    out.setdefault(files[fid], {})[ctxs[cid]] = numbits_to_nums(nb)
print(json.dumps(out))
`

func readCoverage(t *testing.T, dir string) map[string]map[string][]int {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	cmd := exec.Command(python, "-c", decodeScript, filepath.Join(dir, ".coverage"))
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("decode .coverage: %v\n%s", err, out)
	}
	var raw map[string]map[string][]int
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("decode .coverage json: %v", err)
	}
	got := make(map[string]map[string][]int, len(raw))
	for p, ctxs := range raw {
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			rel = p
		}
		got[filepath.ToSlash(rel)] = ctxs
	}
	return got
}
