package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// touchLogic appends a function to the fixture's src/logic.py, which is the file
// tests/test_a.py and tests/test_b.py both record coverage on. It is what makes a
// selection non-empty without adding a new test.
func touchLogic(t *testing.T, repo string) {
	t.Helper()
	logic := filepath.Join(repo, "src", "logic.py")
	src, err := os.ReadFile(logic)
	if err != nil {
		t.Fatalf("read logic.py: %v", err)
	}
	if err := os.WriteFile(logic, append(src, []byte("\n\ndef added():\n    return 42\n")...), 0o644); err != nil {
		t.Fatalf("write logic.py: %v", err)
	}
}

// This is audit A4, executed. A subset run legitimately records LESS coverage than
// the seed did, so a run that Replaced would silently narrow every row it touched —
// on a single branch, with no merge involved. The hand-added path is one this run
// cannot possibly re-record, so only a Union can keep it.
func TestCmdRunUnionsAndNeverNarrows(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)

	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}

	rows := readMapJSONL(t, repo)
	add := rows["tests/test_a.py::test_add"]
	if len(add.F) == 0 {
		t.Fatalf("precondition: test_add has empty f")
	}
	widened := add
	widened.F = append(append([]string{}, add.F...), "src/legacy_import_time_only.py")
	sort.Strings(widened.F)
	b, err := json.Marshal(widened)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := appendLine(mapPath(repo), string(b)); err != nil {
		t.Fatalf("append: %v", err)
	}

	touchLogic(t, repo)

	code := cmdRun(nil)
	if code != 0 && code != 1 {
		t.Fatalf("cmdRun = %d, want 0 or 1", code)
	}

	after := readMapJSONL(t, repo)["tests/test_a.py::test_add"]
	if !containsStr(after.F, "src/legacy_import_time_only.py") {
		t.Fatalf("after run, test_add f = %v; the hand-added file was dropped.\n"+
			"run MUST Union, never Replace (spec §4, D11, audit A4)", after.F)
	}
	if !containsStr(after.F, "src/logic.py") {
		t.Fatalf("after run, test_add f = %v, want it to still contain src/logic.py", after.F)
	}
	if !sort.StringsAreSorted(after.F) {
		t.Errorf("after run, test_add f = %v, want it sorted", after.F)
	}
	// The union merge driver leaves duplicate `t` lines; Load resolves them, and
	// Save must emit exactly one line per test.
	seen := map[string]int{}
	f, err := os.ReadFile(mapPath(repo))
	if err != nil {
		t.Fatalf("read map: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(f)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r jsonlRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("map line %q: %v", line, err)
		}
		seen[r.T]++
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("map has %d rows for %q after run, want 1", n, id)
		}
	}
}

// A failing streak must still buy an eventual DriftGuard full run, so `cycles`
// counts completed runs, not successful ones.
func TestCmdRunBumpsCyclesOnPassAndOnFail(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	if m, _ := readMeta(repo); m.Cycles != 0 {
		t.Fatalf("cycles after seed = %d, want 0", m.Cycles)
	}

	touchLogic(t, repo)

	for i := 1; i <= 2; i++ {
		if code := cmdRun(nil); code != 0 && code != 1 {
			t.Fatalf("cmdRun = %d, want 0 or 1", code)
		}
		m, err := readMeta(repo)
		if err != nil {
			t.Fatalf("readMeta: %v", err)
		}
		if m.Cycles != i {
			t.Fatalf("cycles after %d runs = %d, want %d — a failing streak must still buy an eventual DriftGuard run", i, m.Cycles, i)
		}
	}
}

func TestCmdRunEmptySelectionExitsZeroAndSaysSo(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	// Nothing changed since the seed commit and nothing is untracked apart from
	// .rtdd, so the selection is empty.
	code := cmdRun(nil)
	if code != 0 {
		t.Fatalf("cmdRun on an empty selection = %d, want 0 — an empty selection is a signal, not a failure", code)
	}
	m, err := readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if m.Cycles != 1 {
		t.Errorf("cycles after an empty-selection run = %d, want 1", m.Cycles)
	}
}

func TestCmdRunFailingTestExitsOne(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	// Change the file test_fail covers, so test_fail is selected.
	touchLogic(t, repo)
	if code := cmdRun(nil); code != 1 {
		t.Fatalf("cmdRun with a failing selected test = %d, want 1", code)
	}
}

func TestCmdRunBadBaseIsAUsageError(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	if code := cmdRun([]string{"--base", "no-such-ref-anywhere"}); code == 0 || code == 1 {
		t.Fatalf("cmdRun with an unresolvable --base = %d, want a nonzero non-test-failure code", code)
	}
}

func TestCmdRunRejectsUnknownFlag(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdRun([]string{"--nonsense"}); code != 2 {
		t.Fatalf("cmdRun with an unknown flag = %d, want 2", code)
	}
}

func TestCmdRunRejectsPositionalArguments(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdRun([]string{"tests/test_a.py"}); code != 2 {
		t.Fatalf("cmdRun with a positional argument = %d, want 2 — run takes its selection from the map, never from argv", code)
	}
}

// The dispatcher must reach cmdRun, and usage must name it: a command the help
// text hides is a command an agent never runs.
func TestRunIsDispatchedAndDocumented(t *testing.T) {
	if !strings.Contains(usage, "rtdd run") {
		t.Errorf("usage does not document `rtdd run`:\n%s", usage)
	}
	dir := t.TempDir()
	chdir(t, dir)
	var out, errBuf strings.Builder
	if code := run([]string{"run"}, &out, &errBuf); code != 2 {
		t.Fatalf("run([run]) outside a git repo = %d, want 2 (the dispatcher must reach cmdRun)", code)
	}
}

// End to end against the real toolchain: seed, then run, and assert the map the
// pair leaves behind is one the selector can actually act on. A row that is
// structurally present but empty selects nothing — the failure mode a row count
// alone cannot see.
//
// `d` is asserted in the aggregate, not per row: it is whole milliseconds and the
// fixture's arithmetic tests genuinely finish in under one. What must hold is that
// a duration reaches the map at all; a wiring break zeroes EVERY row.
func TestCmdRunEndToEndProducesUsableRows(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	touchLogic(t, repo)
	if code := cmdRun(nil); code != 0 && code != 1 {
		t.Fatalf("cmdRun = %d, want 0 or 1", code)
	}

	sha, err := gitctx.HeadSHA(repo)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	rows := readMapJSONL(t, repo)
	if len(rows) == 0 {
		t.Fatal("map.jsonl is empty after seed + run")
	}
	valid := map[string]bool{"pass": true, "fail": true, "skip": true, "error": true}
	timed := 0
	for id, r := range rows {
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
			continue // a skipped test executes no body and records no coverage
		}
		if len(r.F) == 0 {
			t.Errorf("row %q f is empty, want the files it covered", id)
		}
	}
	if timed == 0 {
		t.Error("no row carries a non-zero d; the report log's duration never reached the map")
	}
	if !containsStr(rows["tests/test_a.py::test_add"].F, "src/logic.py") {
		t.Errorf("test_add f = %v, want src/logic.py", rows["tests/test_a.py::test_add"].F)
	}
}
