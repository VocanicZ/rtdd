package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// writeHostAdapter drops one YAML file into <root>/.rtdd/adapters/.
func writeHostAdapter(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".rtdd", "adapters")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// A host adapter for a toolchain the binary has never heard of.
const hostVitestYAML = `name: vitest
detect: ["vitest.config.ts"]
subset: "npx vitest run {tests}"
list: "npx vitest list"
selection: static
coverage: none
report: pytest-reportlog
test_globs: ["**/*.test.ts"]
source_globs: ["src/**/*.ts"]
`

// A host override of the shipped python adapter, told apart by its subset.
const hostPythonOverrideYAML = `name: python
detect: ["pyproject.toml"]
seed: "pytest --cov --cov-context=test"
subset: "pytest {tests} --cov --cov-context=test -p no:randomly"
list: "pytest --collect-only -q"
coverage: sqlite
report: pytest-reportlog
test_globs: ["tests/**/*.py"]
source_globs: ["**/*.py"]
`

// A host adapter that violates the contract in a way that names a field.
const hostBrokenYAML = `name: broken
detect: ["go.mod"]
seed: "go test ./..."
subset: "go test ./..."
coverage: sqlite
report: pytest-reportlog
`

type errString string

func (e errString) Error() string { return string(e) }

// newHostAdapterRepo is a python repo — the built-in detects it — that also carries host
// adapter files.
func newHostAdapterRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	gittest.Write(t, dir, "src/logic.py", "def add(a, b):\n    return a + b\n")
	gittest.Write(t, dir, "tests/test_logic.py", "def test_add():\n    pass\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// The override is not silent: doctor names the adapter and says it came from the host repo
// instead of the binary.
func TestDoctorReportsAHostOverrideByName(t *testing.T) {
	dir := newHostAdapterRepo(t)
	writeHostAdapter(t, dir, "python.yaml", hostPythonOverrideYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"python", ".rtdd/adapters/python.yaml", "host-authored", "overrides built-in"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor output does not contain %q:\n%s", want, stdout)
		}
	}
}

// A repo with no .rtdd/adapters/ is every repo that exists today. The resolved set is the
// shipped set, so the fidelity block reports built-ins only: nothing is host-authored,
// nothing overrides anything, and no file failed to load.
func TestDoctorOnARepoWithNoHostAdaptersSaysNothingAboutAdapters(t *testing.T) {
	dir := newHostAdapterRepo(t)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, unwanted := range []string{"host-authored", "overrides built-in", "not loaded"} {
		if strings.Contains(stdout, unwanted) {
			t.Errorf("doctor said %q on a repo with no host adapters:\n%s", unwanted, stdout)
		}
	}
}

// The failing file AND the failing field, with the valid adapters still reported and the
// exit code still 0: doctor reports, it does not have an opinion.
func TestDoctorNamesAnUnloadableHostAdapterAndStillExitsZero(t *testing.T) {
	dir := newHostAdapterRepo(t)
	writeHostAdapter(t, dir, "broken.yaml", hostBrokenYAML)
	writeHostAdapter(t, dir, "vitest.yaml", hostVitestYAML)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"not loaded", "broken.yaml", "{tests}"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor output does not contain %q:\n%s", want, stdout)
		}
	}
	// The half-written file did not take the working ones down with it.
	for _, want := range []string{"vitest", "python"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor lost the adapter %q to a malformed sibling:\n%s", want, stdout)
		}
	}
}

// A host adapter with a fresh name is available everywhere a built-in is, detection
// included — that is what makes a language RTDD has never heard of supportable in YAML.
func TestDetectionResolvesAHostAuthoredAdapter(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "vitest.config.ts", "export default {}\n")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b\n")
	gittest.Write(t, dir, "src/logic.test.ts", "test('add', () => {})\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	writeHostAdapter(t, dir, "vitest.yaml", hostVitestYAML)

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if got := decodeOutput(t, stdout).Adapter; got != "vitest" {
		t.Errorf("adapter = %q, want the host-authored %q", got, "vitest")
	}
}

// The run path keeps going on the adapters that are valid, and says on stderr which file
// it dropped and why — a silent skip would let a typo in a host override look like the
// built-in simply winning.
func TestWhichWarnsAboutAnUnloadableHostAdapterAndCarriesOn(t *testing.T) {
	dir := newHostAdapterRepo(t)
	writeHostAdapter(t, dir, "broken.yaml", hostBrokenYAML)

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if got := decodeOutput(t, stdout).Adapter; got != "python" {
		t.Errorf("adapter = %q, want the built-in %q to still resolve", got, "python")
	}
	for _, want := range []string{"broken.yaml", "{tests}"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not name %q:\n%s", want, stderr)
		}
	}
}
