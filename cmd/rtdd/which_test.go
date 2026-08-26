package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// rawObject decodes one JSON object as raw keys, so a test can assert a key is ABSENT.
// An omitted `uncovered.files` and an empty one mean opposite things to a consumer, and
// only the raw form can tell them apart.
func rawObject(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	var b []byte
	switch x := v.(type) {
	case string:
		b = []byte(x)
	case json.RawMessage:
		b = x
	default:
		t.Fatalf("rawObject: unsupported %T", v)
	}
	out := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("rawObject: %v\n%s", err, b)
	}
	return out
}

// decodeOutput parses the frozen v1 document `which --json` emits. It is the same struct
// the agent front-ends bind to, so a schema drift breaks this decode first.
func decodeOutput(t *testing.T, s string) Output {
	t.Helper()
	var out Output
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("which --json emitted unparseable JSON: %v\n%s", err, s)
	}
	return out
}

// which runs nothing, so it has no fresh coverage. Emitting a stale line-level report
// would reintroduce exactly the line-drift problem spec §4 removes: the honest answer is
// `available: false` with a reason, and NO `files` key at all — an empty `files` array
// reads as "nothing uncovered", which is the opposite claim.
func TestWhichJSONNeverClaimsAFreshUncoveredReport(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)

	if got.Schema != SchemaVersion {
		t.Errorf("schema = %d, want %d", got.Schema, SchemaVersion)
	}
	if got.Command != "which" {
		t.Errorf("command = %q, want \"which\"", got.Command)
	}
	if got.Uncovered.Available {
		t.Error("uncovered.available = true; which executes nothing and has no fresh coverage")
	}
	if got.Uncovered.Reason == "" {
		t.Error("uncovered.reason is empty; an unavailable report must say why")
	}
	if _, ok := rawObject(t, rawObject(t, stdout)["uncovered"])["files"]; ok {
		t.Errorf("which --json emitted an uncovered.files key; it must be omitted:\n%s", stdout)
	}
}

// `run` is the outcome of the executed subset, and which executes nothing.
func TestWhichJSONReportsNothingExecuted(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)

	if got.Run.Executed {
		t.Error("run.executed = true; which runs no tests")
	}
	if n := got.Run.Passed + got.Run.Failed + got.Run.Skipped + got.Run.Errored; n != 0 {
		t.Errorf("outcome counts sum to %d, want 0 when nothing executed: %+v", n, got.Run)
	}
	if got.Run.DurationMS != 0 {
		t.Errorf("run.duration_ms = %d, want 0", got.Run.DurationMS)
	}
	if got.Run.Failures == nil || len(got.Run.Failures) != 0 {
		t.Errorf("run.failures = %#v, want an empty array", got.Run.Failures)
	}
	if got.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0", got.ExitCode)
	}
}

// unmapped_files is the file-level signal which CAN honestly compute: the changed
// instrumentable files no map row covers. It is the import-fallback trigger set.
func TestWhichJSONUnmappedFilesIsFileGranularAndNeverNull(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n") // mapped
	writeFile(t, dir, "src/orphan_module.py", "def orphan():\n    return 0\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)

	want := []string{"src/orphan_module.py"}
	if !reflect.DeepEqual(got.UnmappedFiles, want) {
		t.Errorf("unmapped_files = %#v, want %#v", got.UnmappedFiles, want)
	}
	if strings.Contains(stdout, "null") {
		t.Errorf("which --json emitted null:\n%s", stdout)
	}
}

func TestWhichJSONUnmappedFilesIsAnEmptyArrayWhenEveryFileIsMapped(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, _ := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := decodeOutput(t, stdout)
	if got.UnmappedFiles == nil || len(got.UnmappedFiles) != 0 {
		t.Errorf("unmapped_files = %#v, want an empty array", got.UnmappedFiles)
	}
}

// The instrumentable filter runs BEFORE anything file-level is reported. A changed test
// file and a changed .yaml are not source coverage can attribute, so neither may be
// called unmapped and neither may reach the uncovered path.
func TestWhichKeepsTestFilesAndOpaqueFilesOffTheUncoveredPath(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "tests/test_brand_new.py", "def test_new():\n    assert True\n")
	writeFile(t, dir, "src/fixtures/data.yaml", "k: v\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)

	seen := map[string]bool{}
	for _, c := range got.Changed {
		seen[c.Path] = true
		if c.Instrumentable {
			t.Errorf("changed[%s].instrumentable = true; neither a test file nor a .yaml is source", c.Path)
		}
	}
	for _, want := range []string{"tests/test_brand_new.py", "src/fixtures/data.yaml"} {
		if !seen[want] {
			t.Errorf("changed set is missing %s: %#v", want, got.Changed)
		}
	}
	if len(got.UnmappedFiles) != 0 {
		t.Errorf("unmapped_files = %#v; a non-instrumentable file is never unmapped", got.UnmappedFiles)
	}
}

// which is the cheap question. It must cost one map load, one git diff and (at most) a
// static import scan — never a test execution. The adapter's every command is a sentinel
// writer here, so running any of them leaves evidence on disk.
func TestWhichRunsNoTests(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, ".rtdd/adapter.yaml", sentinelAdapter)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")
	writeFile(t, dir, "tests/test_brand_new.py", "def test_new():\n    assert True\n")

	for _, args := range [][]string{{"which"}, {"which", "--json"}} {
		code, _, stderr := rtdd(t, dir, args...)
		if code != 0 {
			t.Fatalf("%v exit code = %d, want 0 (stderr: %s)", args, code, stderr)
		}
		if _, err := os.Stat(filepath.Join(dir, "RAN")); err == nil {
			t.Fatalf("%v executed an adapter command; which must run no tests", args)
		}
	}
}

// Every adapter command writes a sentinel: if which ever executes one, the file appears.
const sentinelAdapter = `name: python
detect: ["pyproject.toml"]
seed: "sh -c 'touch RAN'"
subset: "sh -c 'touch RAN {tests}'"
list: "sh -c 'touch RAN'"
coverage: sqlite
report: pytest-reportlog
test_globs: ["tests/**/*.py", "**/test_*.py"]
source_globs: ["src/**/*.py"]
opaque: ["**/*.yaml", "**/*.html", "**/fixtures/**"]
full_escalate: ["requirements.txt", "pyproject.toml", "**/conftest.py"]
`

// The static-import fallback: a changed instrumentable file that NO map row covers is
// exactly the import-time-only case (spec §6, D14), because import-time lines are
// attributed to no test and so never enter any row's f. which must fire the scan and
// report which tests it produced, per file.
func TestWhichImportFallbackFiresForAnUnmappedInstrumentableFile(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		if _, err2 := exec.LookPath("python"); err2 != nil {
			t.Skip("no python interpreter on PATH")
		}
	}
	dir := importFallbackRepo(t)
	writeFile(t, dir, "src/constants.py", "MAX_RETRIES = 4\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)

	fallback := got.Selection.ImportFallback["src/constants.py"]
	if !reflect.DeepEqual(fallback, []string{"tests/test_it.py"}) {
		t.Fatalf("selection.import_fallback = %#v, want {src/constants.py: [tests/test_it.py]}",
			got.Selection.ImportFallback)
	}
	if !reflect.DeepEqual(got.UnmappedFiles, []string{"src/constants.py"}) {
		t.Errorf("unmapped_files = %#v, want [src/constants.py]", got.UnmappedFiles)
	}
	found := false
	for _, id := range got.Selection.Tests {
		if id == "tests/test_it.py" {
			found = true
		}
	}
	if !found {
		t.Errorf("selection.tests = %#v, want the importing test selected", got.Selection.Tests)
	}
	if got.Tier != "T1" {
		t.Errorf("tier = %q, want T1 (reason: %s)", got.Tier, got.Reason)
	}
}

// The fallback fires only for files no map row covers: a mapped file is answered by the
// coverage relation, and paying for an AST scan there would be pure cost.
func TestWhichImportFallbackIsEmptyWhenEveryChangedFileIsMapped(t *testing.T) {
	dir := importFallbackRepo(t)
	writeFile(t, dir, "src/logic.py", "def retries_left(used):\n    return 1 - used\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if len(got.Selection.ImportFallback) != 0 {
		t.Errorf("selection.import_fallback = %#v, want {}", got.Selection.ImportFallback)
	}
	if !strings.Contains(stdout, `"import_fallback": {}`) {
		t.Errorf("import_fallback must be an object, never null:\n%s", stdout)
	}
}

// importFallbackRepo is fixture F1: src/constants.py executes only at import time and is
// therefore in no map row, while tests/test_it.py imports it transitively through
// src/logic.py.
func importFallbackRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/__init__.py", "")
	gittest.Write(t, dir, "src/constants.py", "MAX_RETRIES = 3\n")
	gittest.Write(t, dir, "src/logic.py",
		"from src.constants import MAX_RETRIES\n\n\ndef retries_left(used):\n    return MAX_RETRIES - used\n")
	gittest.Write(t, dir, "tests/test_it.py",
		"from src.logic import retries_left\n\n\ndef test_logic():\n    assert retries_left(1) == 2\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")

	sha := gittest.HeadShort(t, dir)
	gittest.Write(t, dir, ".rtdd/map.jsonl",
		`{"t":"tests/test_it.py","f":["src/logic.py"],"c":"`+sha+`","d":12,"s":"pass"}`+"\n")
	ad, err := os.ReadFile("testdata/adapter.yaml")
	if err != nil {
		t.Fatal(err)
	}
	gittest.Write(t, dir, ".rtdd/adapter.yaml", string(ad))
	gittest.Write(t, dir, ".rtdd/meta.json",
		`{"v":1,"adapter":"python","seeded_at":"`+sha+`","cycles":`+strconv.Itoa(0)+"}\n")
	return dir
}
