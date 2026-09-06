package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The runner is where a selection stops being a list of test FILES and becomes the
// runner's own selectors (plan 06-m6d decision 13). It happens here, before Chunk, for two
// reasons: the argv byte budget must measure what is actually spliced, and two test files
// that render one selector must collapse to one before a chunk boundary can separate them.
//
// `rtdd which` still prints the test files — that is the answer a reader wants — so the
// translation belongs at the invocation boundary and nowhere earlier.
func TestRunSplicesTheRenderedSelectorRatherThanTheTestFilePath(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	writeStubReport(t, reportFile, testReportEntry("calc/calc_test.go::TestAdd", "call", "passed", 0.01))

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":   argsFile,
		"RTDD_STUB_REPORT": reportFile,
	})
	// No .coverage: the tier this key belongs to records none, and the runner must not
	// need one to splice a selector.
	a.Coverage = "none"
	// go's shape: the selector is a PACKAGE, so two test files in one package are one
	// selector and the ranked order of the rest survives.
	a.TestSelector = "./{dir}"

	if _, err := Run(a, repo, []string{"calc/calc_test.go", "calc/mul_test.go", "util/u_test.go"}, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	argv := readArgv(t, argsFile)
	want := []string{"./calc", "./util"}
	if !equalStrings(argv, want) {
		t.Fatalf("subset argv = %v, want %v; the runner spliced test file paths into a selector that takes packages", argv, want)
	}
}

// An adapter declaring no test_selector is the identity, which is every coverage-tier
// adapter: a measured pytest node id reaches `subset` byte for byte, as it always has.
func TestRunWithNoTestSelectorSplicesTheIDsUnchanged(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	writeStubReport(t, reportFile, testReportEntry("tests/test_b.py::test_b", "call", "passed", 0.01))

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":   argsFile,
		"RTDD_STUB_REPORT": reportFile,
	})

	a.Coverage = "none"

	ids := []string{"tests/test_a.py::test_param[1-one two]", "tests/test_b.py::test_b"}
	if _, err := Run(a, repo, ids, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	argv := readArgv(t, argsFile)
	if !equalStrings(argv, ids) {
		t.Fatalf("subset argv = %v, want the ids unchanged %v", argv, ids)
	}
}

// readArgv returns the stub's recorded arguments minus the ones the template contributes,
// so what is left is exactly what was spliced at {tests}.
func readArgv(t *testing.T, argsFile string) []string {
	t.Helper()
	b, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || line == "--- invocation ---" || strings.HasPrefix(line, "--report-log=") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
