package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/selector"
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

	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
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
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
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
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
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
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
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
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
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
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
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

// makeSuiteGreen rewrites the fixture's one deliberate failure so a run over the whole
// selection passes. The uncovered report must be reachable on a GREEN run — that is the
// case that proves RTDD reports without gating (spec §2 non-goals, §6, decision D3).
func makeSuiteGreen(t *testing.T, repo string) {
	t.Helper()
	p := filepath.Join(repo, "tests", "test_b.py")
	src, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read test_b.py: %v", err)
	}
	fixed := strings.Replace(string(src), "assert mul(2, 3) == 7", "assert mul(2, 3) == 6", 1)
	if fixed == string(src) {
		t.Fatalf("test_b.py no longer carries the deliberate failure this helper repairs")
	}
	if err := os.WriteFile(p, []byte(fixed), 0o644); err != nil {
		t.Fatalf("write test_b.py: %v", err)
	}
}

// A non-empty uncovered report is a REPORT, not a gate: every test passed, so the exit
// code is 0 even though changed lines went unexecuted (spec §2, §6, decision D3).
func TestCmdRunExitsZeroWithANonEmptyUncoveredReport(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	makeSuiteGreen(t, repo)

	if code := cmdSeed(nil, io.Discard, io.Discard); code != 0 {
		t.Fatalf("cmdSeed = %d, want 0 once the suite is green", code)
	}
	touchLogic(t, repo)

	var code int
	out := captureStdout(t, func() { code = cmdRun(nil) })

	if code != 0 {
		t.Fatalf("cmdRun = %d, want 0: every test passed.\n%s", code, out)
	}
	if !strings.Contains(out, "UNCOVERED: src/logic.py:") {
		t.Fatalf("run printed no uncovered report for the added, unexecuted body:\n%s", out)
	}
	// The changed test file is not instrumentable and coverage never measures it, so
	// reporting it would mean calling every one of its lines uncovered.
	if strings.Contains(out, "UNCOVERED: tests/") {
		t.Fatalf("run reported a test file as uncovered:\n%s", out)
	}
}

// The signal must come from the coverage this run just produced. map.jsonl carries no
// line data at all, so a run that classified from it could only ever report whole files.
func TestCmdRunJSONEmitsTheFreshUncoveredReport(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	makeSuiteGreen(t, repo)

	if code := cmdSeed(nil, io.Discard, io.Discard); code != 0 {
		t.Fatalf("cmdSeed = %d, want 0 once the suite is green", code)
	}
	touchLogic(t, repo)

	var code int
	raw := captureStdout(t, func() { code = cmdRun([]string{"--json"}) })
	if code != 0 {
		t.Fatalf("cmdRun --json = %d, want 0.\n%s", code, raw)
	}

	var got Output
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("--json stdout is not a single JSON document: %v\n%s", err, raw)
	}
	if got.Schema != SchemaVersion {
		t.Fatalf("schema = %d, want %d", got.Schema, SchemaVersion)
	}
	if got.Command != "run" {
		t.Fatalf("command = %q, want \"run\"", got.Command)
	}
	if !got.Uncovered.Available {
		t.Fatalf("uncovered.available = false; a completed run always has fresh coverage.\n%s", raw)
	}
	if got.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0", got.ExitCode)
	}
	if got.Uncovered.Summary.UncoveredLines == 0 {
		t.Fatalf("uncovered.summary.uncovered_lines = 0, want the added body counted.\n%s", raw)
	}
	var logic *JSONFileReport
	for i := range got.Uncovered.Files {
		if got.Uncovered.Files[i].Path == "src/logic.py" {
			logic = &got.Uncovered.Files[i]
		}
		if strings.HasPrefix(got.Uncovered.Files[i].Path, "tests/") {
			t.Fatalf("uncovered.files names a test file: %q", got.Uncovered.Files[i].Path)
		}
	}
	if logic == nil {
		t.Fatalf("uncovered.files has no src/logic.py:\n%s", raw)
	}
	if logic.UncoveredLines == 0 {
		t.Fatalf("src/logic.py uncovered_lines = 0, want the added body counted:\n%s", raw)
	}
	// Line-granular ranges are only possible against fresh coverage; map.jsonl has none.
	if len(logic.Ranges) == 0 {
		t.Fatalf("src/logic.py has no classified ranges:\n%s", raw)
	}
	for _, r := range logic.Ranges {
		if r.Start < 1 || r.End < r.Start {
			t.Fatalf("src/logic.py range %#v is not a 1-indexed inclusive span", r)
		}
	}
	// The changed set carries the verdict for every path, instrumentable or not.
	seenTest := false
	for _, c := range got.Changed {
		if strings.HasPrefix(c.Path, "tests/") {
			seenTest = true
			if c.Instrumentable {
				t.Fatalf("changed entry %q is marked instrumentable", c.Path)
			}
		}
	}
	if !seenTest {
		t.Fatalf("changed set omits the modified test file:\n%s", raw)
	}
}

// A run whose selection is empty executed nothing, so it has no fresh coverage and must
// say so rather than emit a report a consumer would read as "nothing uncovered".
func TestCmdRunJSONIsTheOnlyThingOnStdout(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	makeSuiteGreen(t, repo)

	if code := cmdSeed(nil, io.Discard, io.Discard); code != 0 {
		t.Fatalf("cmdSeed = %d, want 0 once the suite is green", code)
	}
	touchLogic(t, repo)

	raw := captureStdout(t, func() { cmdRun([]string{"--json"}) })
	if strings.Contains(raw, "tier ") || strings.Contains(raw, "rows in the map") {
		t.Fatalf("--json stdout carries human-readable text a parser would choke on:\n%s", raw)
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	var first Output
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("decode: %v\n%s", err, raw)
	}
	if dec.More() {
		t.Fatalf("--json emitted more than one document:\n%s", raw)
	}
}

// map.jsonl holds file-level rows and NO line numbers, so classifying against it could
// only ever answer at whole-file granularity, on numbers stale the moment a file is
// edited — the exact drift spec §4 eliminates. The one legitimate classification input
// is the *coverage.Result the run just produced. This reads the source because the
// wrong input would still compile, still run, and still print a plausible report.
func TestRunClassifiesAgainstFreshCoverageOnly(t *testing.T) {
	b, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatalf("read run.go: %v", err)
	}
	src := string(b)
	if !strings.Contains(src, "Cov:              res.Coverage") {
		t.Fatal("run.go does not classify against res.Coverage — the coverage this run produced")
	}
	// mapstore.Map is still the right source for the FILE-level unmapped set; what it
	// must never feed is a line-level classification.
	for _, bad := range []string{"Cov:              m", "Cov: m", "Cov:              mapstore"} {
		if strings.Contains(src, bad) {
			t.Fatalf("run.go passes the map as classification coverage: %q", bad)
		}
	}
}

// `rtdd run` must fire the SAME static-import fallback `rtdd which` fires. Otherwise the
// advisory command and the executing command disagree about what to run, and the case
// M2 exists to serve — an import-time-only file, whose lines are attributed to no test
// and therefore enter no map row's f — selects tests under `which` and executes NOTHING
// under `run` (spec §6, D14).
//
// src/constants.py is that file in the real fixture: TestAcceptanceImportOnlyFileIsInNoMapRow
// proves no row covers it. tests/test_a.py imports it directly, tests/test_b.py through
// src/logic.py, so the scan has a real answer to give.
func TestCmdRunFiresTheStaticImportFallbackAndAgreesWithWhich(t *testing.T) {
	// `which` reads .rtdd/adapter.yaml while `run` detects the builtin; the two commands
	// can only be compared when both classify with the SAME adapter, so install it.
	builtin, err := os.ReadFile(filepath.Join("..", "..", "adapters", "python.yaml"))
	if err != nil {
		t.Fatalf("read the builtin python adapter: %v", err)
	}
	repo := realRepo(t)
	chdir(t, repo)
	writeFile(t, repo, ".rtdd/adapter.yaml", string(builtin))
	makeSuiteGreen(t, repo)
	// Commit the repair so the ONLY uncommitted change is the import-time-only file:
	// a dirty test file would be selected by the direct tier and mask the fallback.
	gitRun(t, repo, "commit", "-am", "green suite")

	if code := cmdSeed(nil, io.Discard, io.Discard); code != 0 {
		t.Fatalf("cmdSeed = %d, want 0 once the suite is green", code)
	}

	consts := filepath.Join(repo, "src", "constants.py")
	src, err := os.ReadFile(consts)
	if err != nil {
		t.Fatalf("read constants.py: %v", err)
	}
	if err := os.WriteFile(consts, append(src, []byte("\nMIN = 1\n")...), 0o644); err != nil {
		t.Fatalf("write constants.py: %v", err)
	}

	var wout, werr bytes.Buffer
	if code := cmdWhich([]string{"--json"}, &wout, &werr); code != 0 {
		t.Fatalf("cmdWhich --json = %d, want 0 (stderr: %s)", code, werr.String())
	}
	wantSel := decodeOutput(t, wout.String())
	if len(wantSel.Selection.ImportFallback) == 0 {
		t.Fatalf("precondition: which did not fire the import fallback:\n%s", wout.String())
	}

	var code int
	raw := captureStdout(t, func() { code = cmdRun([]string{"--json"}) })
	if code != 0 {
		t.Fatalf("cmdRun --json = %d, want 0.\n%s", code, raw)
	}
	got := decodeOutput(t, raw)

	if !reflect.DeepEqual(got.Selection.Tests, wantSel.Selection.Tests) {
		t.Fatalf("run selected %#v, which selected %#v — the two commands must agree",
			got.Selection.Tests, wantSel.Selection.Tests)
	}
	if !reflect.DeepEqual(got.Selection.ImportFallback, wantSel.Selection.ImportFallback) {
		t.Fatalf("run selection.import_fallback = %#v, which = %#v",
			got.Selection.ImportFallback, wantSel.Selection.ImportFallback)
	}
	if len(got.Selection.ImportFallback["src/constants.py"]) == 0 {
		t.Fatalf("run selection.import_fallback has no src/constants.py entry:\n%s", raw)
	}
	if got.Tier != "T1" {
		t.Errorf("run tier = %q, want T1 (reason: %s)", got.Tier, got.Reason)
	}
	// The point of the whole exercise: run must EXECUTE the importing tests, not report
	// an empty selection that reads as a pass.
	if !got.Run.Executed {
		t.Fatalf("run.executed = false — run selected tests but executed nothing:\n%s", raw)
	}
	if got.Run.Passed == 0 {
		t.Fatalf("run.passed = 0, want the importing tests actually run:\n%s", raw)
	}
}

// The comment that deferred the fallback to M2 is stale: this IS M2, and a reader who
// believes it will not look for the wiring. run must build the same scan `which` does.
//
// Since per-adapter selection (#317) both commands reach it through selectPerAdapter,
// which is a stronger guarantee than each file wiring its own: there is now exactly one
// place the fallback can be built, so the advisory command and the executing command
// cannot drift apart. The guard therefore asserts run.go goes through that path and that
// the path itself builds the scan.
func TestRunWiresTheSharedImportFallbackHelper(t *testing.T) {
	b, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatalf("read run.go: %v", err)
	}
	src := string(b)
	if strings.Contains(src, "static import fallback lands in M2") {
		t.Error("run.go still carries the stale \"lands in M2\" comment")
	}
	if !strings.Contains(src, "selectPerAdapter(") {
		t.Error("run.go does not select through selectPerAdapter, the one path that wires the fallback")
	}
	if strings.Contains(src, "ImportOnly: func(string) []string { return nil }") {
		t.Error("run.go still passes a stub for selector.Inputs.ImportOnly")
	}
	shared, err := os.ReadFile("polyglot.go")
	if err != nil {
		t.Fatalf("read polyglot.go: %v", err)
	}
	if !strings.Contains(string(shared), "newImportFallback(") {
		t.Error("selectPerAdapter does not build the shared importFallbackScan")
	}
}

// An empty selection is the narrowest possible run, and under --json the "this is not a
// pass" line is not printed at all — the text branch is skipped. Without `warnings` the
// document a front-end reads is indistinguishable from a green run of a real subset.
func TestCmdRunJSONWarnsThatAnEmptySelectionIsNotAPass(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil, io.Discard, io.Discard); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}

	var code int
	raw := captureStdout(t, func() { code = cmdRun([]string{"--json"}) })
	if code != 0 {
		t.Fatalf("cmdRun --json on an empty selection = %d, want 0\n%s", code, raw)
	}

	var got Output
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("--json stdout is not a single JSON document: %v\n%s", err, raw)
	}
	if got.Tier != "empty" {
		t.Fatalf("tier = %q, want empty (reason: %s)", got.Tier, got.Reason)
	}
	if !anyWarningContains(got.Warnings, "not a pass") {
		t.Errorf("warnings must say an empty selection is not a pass, got %#v", got.Warnings)
	}
	// The emptiness itself is fully known; nothing about it is partial.
	if !got.Complete {
		t.Errorf("complete = false; an empty selection is exhaustively known:\n%s", raw)
	}
}

// A real subset run names every test it executed, so it is complete and unremarkable.
func TestCmdRunJSONReportsACompleteSelectionWithNoWarnings(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	makeSuiteGreen(t, repo)

	if code := cmdSeed(nil, io.Discard, io.Discard); code != 0 {
		t.Fatalf("cmdSeed = %d, want 0 once the suite is green", code)
	}
	touchLogic(t, repo)

	var code int
	raw := captureStdout(t, func() { code = cmdRun([]string{"--json"}) })
	if code != 0 {
		t.Fatalf("cmdRun --json = %d, want 0.\n%s", code, raw)
	}

	var got Output
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("--json stdout is not a single JSON document: %v\n%s", err, raw)
	}
	if !got.Complete {
		t.Errorf("complete = false; the executed subset is exactly selection.tests:\n%s", raw)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("warnings = %#v, want none for an unremarkable run", got.Warnings)
	}
}

// AC8 reaches `run` too: the two commands disagreeing about one selection is the defect
// this repo has been bitten by before, and a fidelity stated by `which` but dropped by
// `run` is that same disagreement in the surface that actually executes tests.
func TestRunTierLineStatesTheStaticFidelity(t *testing.T) {
	blocks := []AdapterSelection{{
		Adapter:   "vitest",
		Selection: selector.Selection{Tier: selector.TierTS, Tests: []string{"src/logic.test.ts"}},
	}}
	want := "tier TS (static): 1 selected\n"
	if got := renderRunTiers(blocks); got != want {
		t.Fatalf("renderRunTiers() = %q, want %q", got, want)
	}
}

// And nowhere else: an execution-derived tier prints the line it printed before.
func TestRunTierLineIsUnchangedForAnExecutionDerivedTier(t *testing.T) {
	blocks := []AdapterSelection{{
		Adapter:   "python",
		Selection: selector.Selection{Tier: selector.TierT1, Tests: []string{"tests/test_it.py"}},
	}}
	want := "tier T1: 1 selected\n"
	if got := renderRunTiers(blocks); got != want {
		t.Fatalf("renderRunTiers() = %q, want %q", got, want)
	}
}
