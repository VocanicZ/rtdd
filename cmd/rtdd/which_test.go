package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

// anyWarningContains reports whether some warning carries the given sentence fragment.
func anyWarningContains(warnings []string, want string) bool {
	for _, w := range warnings {
		if strings.Contains(w, want) {
			return true
		}
	}
	return false
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

// which is the cheap question. It must cost one map load, one git diff and one file
// listing — never a test execution. The adapter's every command is a sentinel
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
unit_cmd: "sh -c 'touch RAN {unit}'"
coverage_file: "{tmp}/lcov.info"
coverage_format: lcov
test_globs: ["tests/**/*.py", "**/test_*.py"]
source_globs: ["src/**/*.py"]
opaque: ["**/*.yaml", "**/*.html", "**/fixtures/**"]
full_escalate: ["requirements.txt", "pyproject.toml", "**/conftest.py"]
`
