package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
