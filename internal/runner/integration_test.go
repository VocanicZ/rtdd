package runner

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/pytestfixture"
)

func realPythonRepo(t *testing.T) (string, *adapter.Adapter) {
	t.Helper()
	if !pytestfixture.HavePytest() {
		t.Skip("pytest not on PATH")
	}
	repo := t.TempDir()
	if err := pytestfixture.Materialize(repo); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	detected, err := adapter.Detect(repo, all)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	// The pytest fixture is a single-toolchain repo, so the set it detects has exactly
	// one member; Detect returning a set (spec §4.4) does not change that.
	if len(detected) != 1 || detected[0].Name != "python" {
		t.Fatalf("Detect matched %d adapters, want exactly [python]", len(detected))
	}
	return repo, detected[0]
}

func TestIntegrationSeedAgainstRealPytest(t *testing.T) {
	repo, a := realPythonRepo(t)

	res, err := Seed(a, repo)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	// The fixture has one deliberate failure.
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1 (tests/test_b.py::test_fail fails on purpose)", res.ExitCode)
	}
	if want := []string{"tests/test_b.py::test_fail"}; !reflect.DeepEqual(res.Failed, want) {
		t.Errorf("Failed = %v, want %v", res.Failed, want)
	}

	byTest := map[string]string{}
	for _, o := range res.Outcomes {
		byTest[o.Test] = o.Status
	}
	wantStatus := map[string]string{
		"tests/test_a.py::test_add":              "pass",
		"tests/test_a.py::test_param[1-one two]": "pass",
		"tests/test_a.py::test_param[2-a-b]":     "pass",
		"tests/test_a.py::test_const":            "pass",
		"tests/test_b.py::test_mul":              "pass",
		"tests/test_b.py::test_fail":             "fail",
		"tests/test_b.py::test_skipped":          "skip",
	}
	for id, want := range wantStatus {
		got, ok := byTest[id]
		if !ok {
			t.Errorf("no Outcome for %q; got %v", id, byTest)
			continue
		}
		if got != want {
			t.Errorf("Outcome[%q] = %q, want %q", id, got, want)
		}
	}

	// AUDIT A1: src/constants.py is imported and asserted on by two passing tests
	// and is attributed to ZERO test contexts. It must land in ImportTime.
	gotImport, ok := res.Coverage.ImportTime["src/constants.py"]
	if !ok {
		t.Fatalf("src/constants.py missing from ImportTime; got keys %v", keysOf(res.Coverage.ImportTime))
	}
	if want := []int{1, 3, 5, 6, 7}; !reflect.DeepEqual(gotImport, want) {
		t.Errorf("ImportTime[src/constants.py] = %v, want %v", gotImport, want)
	}
	for _, tc := range res.Coverage.PerTest {
		if _, hit := tc.Files["src/constants.py"]; hit {
			t.Errorf("src/constants.py is attributed to test %q; it must be import-time only", tc.Test)
		}
	}

	// Per-test attribution must be real, not empty.
	covByTest := map[string]map[string][]int{}
	for _, tc := range res.Coverage.PerTest {
		covByTest[tc.Test] = tc.Files
	}
	if got := covByTest["tests/test_a.py::test_add"]["src/logic.py"]; !reflect.DeepEqual(got, []int{5}) {
		t.Errorf("test_add covers src/logic.py %v, want [5]", got)
	}
	if got := covByTest["tests/test_b.py::test_mul"]["src/logic.py"]; !reflect.DeepEqual(got, []int{9}) {
		t.Errorf("test_mul covers src/logic.py %v, want [9]", got)
	}
	// Parametrised ids must survive the context round-trip verbatim.
	if _, ok := covByTest["tests/test_a.py::test_param[1-one two]"]; !ok {
		t.Errorf("no coverage entry for the space-containing parametrised id; got %v", keysOfStr(covByTest))
	}
	if _, ok := covByTest["tests/test_a.py::test_param[2-a-b]"]; !ok {
		t.Errorf("no coverage entry for the hyphen-containing parametrised id; got %v", keysOfStr(covByTest))
	}
}

func TestIntegrationRunSubsetAgainstRealPytest(t *testing.T) {
	repo, a := realPythonRepo(t)

	ids := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
	}
	res, err := Run(a, repo, ids, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	var got []string
	for _, o := range res.Outcomes {
		got = append(got, o.Test)
	}
	sort.Strings(got)
	want := append([]string(nil), ids...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Outcomes = %q, want exactly the selected ids %q — the ids did not round-trip", got, want)
	}
	if _, err := os.Stat(filepath.Join(repo, ".coverage")); err != nil {
		t.Errorf(".coverage was not written to the repo root: %v", err)
	}
}

func TestIntegrationRunSubsetMatchesTheSeedAnswer(t *testing.T) {
	repo, a := realPythonRepo(t)

	seed, err := Seed(a, repo)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	seedStatus := map[string]string{}
	for _, o := range seed.Outcomes {
		seedStatus[o.Test] = o.Status
	}
	seedCov := map[string]map[string][]int{}
	for _, tc := range seed.Coverage.PerTest {
		seedCov[tc.Test] = tc.Files
	}

	ids := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_b.py::test_mul",
	}
	sub, err := Run(a, repo, ids, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	subStatus := map[string]string{}
	for _, o := range sub.Outcomes {
		subStatus[o.Test] = o.Status
	}
	subCov := map[string]map[string][]int{}
	for _, tc := range sub.Coverage.PerTest {
		subCov[tc.Test] = tc.Files
	}

	for _, id := range ids {
		if subStatus[id] != seedStatus[id] {
			t.Errorf("Run outcome for %q = %q, seed said %q", id, subStatus[id], seedStatus[id])
		}
		if !reflect.DeepEqual(subCov[id], seedCov[id]) {
			t.Errorf("Run coverage for %q =\n  %v\nseed said\n  %v", id, subCov[id], seedCov[id])
		}
	}
}

func TestIntegrationRunWithABadSelectorIsFatal(t *testing.T) {
	repo, a := realPythonRepo(t)
	_, err := Run(a, repo, []string{"tests/test_a.py::test_does_not_exist"}, false)
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatalf("Run = %v, want *FatalExitError; pytest exits 4 on a bad selector and that must not look like a pass", err)
	}
	if fe.Code != 4 {
		t.Fatalf("FatalExitError.Code = %d, want 4", fe.Code)
	}
}

func TestIntegrationListAgainstRealPytest(t *testing.T) {
	repo, a := realPythonRepo(t)
	got, err := List(a, repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_a.py::test_const",
		"tests/test_b.py::test_mul",
		"tests/test_b.py::test_fail",
		"tests/test_b.py::test_skipped",
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List =\n  %q\nwant\n  %q", got, want)
	}
}

func keysOf(m map[string][]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func keysOfStr(m map[string]map[string][]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
