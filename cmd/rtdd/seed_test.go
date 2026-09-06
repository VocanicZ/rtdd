package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/pytestfixture"
)

// chdir moves into dir for the duration of the test. cmdSeed and cmdRun resolve
// the repo root from the working directory, exactly as the CLI does.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func realRepo(t *testing.T) string {
	t.Helper()
	if !pytestfixture.HavePytest() {
		t.Skip("pytest not on PATH")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if err := pytestfixture.Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if err := pytestfixture.InitGit(dir); err != nil {
		t.Fatalf("InitGit: %v", err)
	}
	return dir
}

type jsonlRow struct {
	T string   `json:"t"`
	F []string `json:"f"`
	C string   `json:"c"`
	D int      `json:"d"`
	S string   `json:"s"`
}

func readMapJSONL(t *testing.T, repo string) map[string]jsonlRow {
	t.Helper()
	f, err := os.Open(mapPath(repo))
	if err != nil {
		t.Fatalf("open map.jsonl: %v", err)
	}
	defer f.Close()
	out := map[string]jsonlRow{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r jsonlRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("map.jsonl line %q: %v", line, err)
		}
		out[r.T] = r
	}
	return out
}

func TestCmdSeedWritesTheMap(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)

	// The fixture has one deliberate failure, so seed exits 1.
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1 (tests/test_b.py::test_fail fails on purpose)", code)
	}

	rows := readMapJSONL(t, repo)
	wantTests := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_const",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_b.py::test_fail",
		"tests/test_b.py::test_mul",
		"tests/test_b.py::test_skipped",
	}
	var got []string
	for k := range rows {
		got = append(got, k)
	}
	sort.Strings(got)
	if len(got) != len(wantTests) {
		t.Fatalf("map has %d rows (%v), want %d (%v)", len(got), got, len(wantTests), wantTests)
	}
	for i := range wantTests {
		if got[i] != wantTests[i] {
			t.Fatalf("map row %d = %q, want %q", i, got[i], wantTests[i])
		}
	}

	add := rows["tests/test_a.py::test_add"]
	if add.S != "pass" {
		t.Errorf("test_add s = %q, want pass", add.S)
	}
	if add.C == "" {
		t.Error("test_add c is empty, want the short HEAD SHA")
	}
	if !containsStr(add.F, "src/logic.py") {
		t.Errorf("test_add f = %v, want it to contain src/logic.py", add.F)
	}
	if !sort.StringsAreSorted(add.F) {
		t.Errorf("test_add f = %v, want it sorted", add.F)
	}
	if rows["tests/test_b.py::test_fail"].S != "fail" {
		t.Errorf("test_fail s = %q, want fail", rows["tests/test_b.py::test_fail"].S)
	}
	if rows["tests/test_b.py::test_skipped"].S != "skip" {
		t.Errorf("test_skipped s = %q, want skip", rows["tests/test_b.py::test_skipped"].S)
	}

	// src/constants.py is import-time only (audit A1): no test may claim it.
	for id, r := range rows {
		if containsStr(r.F, "src/constants.py") {
			t.Errorf("row %q claims import-time file src/constants.py", id)
		}
	}
}

// Every row the integration run produces must carry a usable payload: a non-empty
// file list, a real commit, a sane duration and a status the selector knows. A row
// that is structurally present but empty selects nothing, which is the failure
// mode a row-count assertion alone cannot see.
//
// `d` is asserted in the aggregate, not per row: it is whole milliseconds, and the
// fixture's arithmetic tests genuinely finish in under one, so a per-row `d > 0`
// would assert that a passing test is slow. What must hold is that the duration
// reaches the map at all — a wiring break zeroes EVERY row, including the slowest.
func TestCmdSeedRowsAreUsable(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	// gitctx, not exec.Command: internal/gitctx is the only package in the tree
	// allowed to shell out to git (internal/contract).
	sha, err := gitctx.HeadSHA(repo)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	valid := map[string]bool{"pass": true, "fail": true, "skip": true, "error": true}
	timed := 0
	for id, r := range readMapJSONL(t, repo) {
		if !valid[r.S] {
			t.Errorf("row %q s = %q, want one of pass/fail/skip/error", id, r.S)
		}
		if r.C != sha {
			t.Errorf("row %q c = %q, want the short HEAD SHA %q", id, r.C, sha)
		}
		if r.D < 0 {
			t.Errorf("row %q d = %d, want a duration in ms", id, r.D)
		}
		if r.D > 0 {
			timed++
		}
		if r.S == "skip" {
			// A skipped test executes no body, so it records no coverage. Its
			// status still belongs in the map.
			continue
		}
		if len(r.F) == 0 {
			t.Errorf("row %q f is empty, want the files it covered", id)
		}
	}
	if timed == 0 {
		t.Error("no row carries a non-zero d; the report log's duration never reached the map")
	}
}

func TestCmdSeedWritesMeta(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	m, err := readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if m.V != 1 {
		t.Errorf("meta.V = %d, want 1", m.V)
	}
	if m.Adapter != "python" {
		t.Errorf("meta.Adapter = %q, want python", m.Adapter)
	}
	if m.SeededAt == "" {
		t.Error("meta.SeededAt is empty, want the short HEAD SHA")
	}
	if m.Cycles != 0 {
		t.Errorf("meta.Cycles = %d, want 0 - seeding resets the cycle counter", m.Cycles)
	}
}

// seed is the ONLY operation that may shrink a row. Re-seeding after a source
// file is deleted must drop it, not keep it forever.
func TestCmdSeedMayShrinkARow(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
		t.Fatalf("first cmdSeed = %d, want 1", code)
	}
	before := readMapJSONL(t, repo)["tests/test_a.py::test_add"]
	if !containsStr(before.F, "src/logic.py") {
		t.Fatalf("precondition: test_add f = %v, want src/logic.py", before.F)
	}

	// Hand-widen the row, then re-seed: seed replaces, so the phantom must go.
	widened := before
	widened.F = append(append([]string{}, before.F...), "src/phantom.py")
	sort.Strings(widened.F)
	b, err := json.Marshal(widened)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := appendLine(mapPath(repo), string(b)); err != nil {
		t.Fatalf("append: %v", err)
	}

	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
		t.Fatalf("second cmdSeed = %d, want 1", code)
	}
	after := readMapJSONL(t, repo)["tests/test_a.py::test_add"]
	if containsStr(after.F, "src/phantom.py") {
		t.Fatalf("after re-seed f = %v, still contains src/phantom.py; seed must Replace, not Union", after.F)
	}
}

func TestCmdSeedOutsideARepoIsAUsageError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 2 {
		t.Fatalf("cmdSeed outside a git repo = %d, want 2", code)
	}
}

func TestCmdSeedWithNoAdapterIsAUsageError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := pytestfixture.InitGit(dir); err != nil {
		t.Fatalf("InitGit: %v", err)
	}
	chdir(t, dir)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 2 {
		t.Fatalf("cmdSeed with no detectable adapter = %d, want 2", code)
	}
}

// Trailing arguments are a usage error, not a silently ignored typo: `rtdd seed
// --base main` must not report a successful seed of something it never did.
func TestCmdSeedRejectsArguments(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if code := cmdSeed([]string{"extra"}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("cmdSeed with a positional argument = %d, want 2", code)
	}
}

// The dispatcher must reach cmdSeed, and usage must name it: a command the help
// text hides is a command an agent never runs.
func TestSeedIsDispatchedAndDocumented(t *testing.T) {
	if !strings.Contains(usage, "rtdd seed") {
		t.Errorf("usage does not document `rtdd seed`:\n%s", usage)
	}
	dir := t.TempDir()
	chdir(t, dir)
	var out, errBuf strings.Builder
	if code := run([]string{"seed"}, &out, &errBuf); code != 2 {
		t.Fatalf("run([seed]) outside a git repo = %d, want 2 (the dispatcher must reach cmdSeed)", code)
	}
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

// This issue's second acceptance criterion: .rtdd/meta.json records the DETECTED SET, not
// just the coverage adapter that produced the map. The singular stays — it is the answer
// to "whose is this untagged row?" that every pre-PRD map.jsonl needs (decision 4).
func TestCmdSeedRecordsTheDetectedAdapterSet(t *testing.T) {
	repo := realRepo(t)
	gittest.Write(t, repo, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	writeVitestAdapter(t, repo, "")
	chdir(t, repo)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}

	m, err := readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if got := m.DetectedAdapters(); len(got) != 2 || got[0] != "python" || got[1] != "vitest" {
		t.Errorf("meta.DetectedAdapters() = %v, want [python vitest]", got)
	}
	if m.Adapter != "python" {
		t.Errorf("meta.Adapter = %q, want python: the singular still names the COVERAGE adapter "+
			"that produced the map", m.Adapter)
	}
}
