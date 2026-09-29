package runner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

const goAdapter = `name: go
detect: ["go.mod"]
unit_cmd: "go test -count=1 -coverpkg=./... -coverprofile={tmp}/cover.out -run {names} ./{dir}"
unit_names: '^func (Test\w+)\('
coverage_file: "{tmp}/cover.out"
coverage_format: gocover
test_globs: ["**/*_test.go"]
source_globs: ["**/*.go"]
`

// gofixRepo copies testdata/gofix into a fresh git repo, because units are enumerated
// through git.
func gofixRepo(t *testing.T) (string, *adapter.Adapter) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/gofix")); err != nil {
		t.Fatal(err)
	}
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "go.yaml")
	if err := os.WriteFile(p, []byte(goAdapter), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := adapter.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return dir, a
}

func TestUnitsAreTheTestFiles(t *testing.T) {
	dir, a := gofixRepo(t)
	got, err := Units(a, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api/api_test.go", "calc/calc_test.go", "calc/helpers_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Units = %v, want %v", got, want)
	}
}

func TestSeedRecordsWhatEachUnitExecuted(t *testing.T) {
	dir, a := gofixRepo(t)
	res, err := Seed(a, dir)
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, o := range res.Outcomes {
		status[o.Test] = o.Status
	}
	wantStatus := map[string]string{"api/api_test.go": "pass", "calc/calc_test.go": "pass", "calc/helpers_test.go": "skip"}
	if !reflect.DeepEqual(status, wantStatus) {
		t.Errorf("statuses = %v, want %v", status, wantStatus)
	}
	files := map[string][]string{}
	for _, pt := range res.Coverage.PerTest {
		for f := range pt.Files {
			files[pt.Test] = append(files[pt.Test], f)
		}
	}
	// The case the static tier missed: store.go is covered by api_test.go.
	if !contains(files["api/api_test.go"], "store/store.go") {
		t.Errorf("api_test.go files = %v, want store/store.go among them", files["api/api_test.go"])
	}
	// Isolation: calc's unit never saw store.go.
	if contains(files["calc/calc_test.go"], "store/store.go") {
		t.Errorf("calc_test.go files = %v leaked another unit's coverage", files["calc/calc_test.go"])
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestAFailingUnitFailsTheRunAndKeepsItsOutput(t *testing.T) {
	dir, a := gofixRepo(t)
	p := filepath.Join(dir, "calc/calc_test.go")
	body, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(body), "!= 3", "!= 4", 1)), 0o644)
	res, err := Run(a, dir, []string{"calc/calc_test.go"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 1 || len(res.Failed) != 1 || res.Failed[0] != "calc/calc_test.go" {
		t.Errorf("ExitCode=%d Failed=%v, want 1 [calc/calc_test.go]", res.ExitCode, res.Failed)
	}
	if !strings.Contains(res.Output["calc/calc_test.go"], "add") {
		t.Errorf("failure output not kept: %q", res.Output["calc/calc_test.go"])
	}
}

func TestAPassWithNoCoverageFileIsAnError(t *testing.T) {
	dir, a := gofixRepo(t)
	a.CoverageFile = "{tmp}/never-written.out"
	res, err := Run(a, dir, []string{"calc/calc_test.go"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcomes[0].Status != "error" || res.ExitCode == 0 {
		t.Errorf("status=%s exit=%d, want error and a non-zero exit", res.Outcomes[0].Status, res.ExitCode)
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestFailFastStopsSchedulingAfterTheFirstFailure(t *testing.T) {
	dir, a := gofixRepo(t)
	a.Jobs = 1
	p := filepath.Join(dir, "calc/calc_test.go")
	body, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(body), "!= 3", "!= 4", 1)), 0o644)
	res, err := Run(a, dir, []string{"calc/calc_test.go", "api/api_test.go"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Test != "calc/calc_test.go" {
		t.Errorf("outcomes = %v, want only calc/calc_test.go", res.Outcomes)
	}
}

func TestAFatalMappedExitStopsSchedulingAndIsReturned(t *testing.T) {
	dir, a := gofixRepo(t)
	a.Jobs = 1
	os.WriteFile(filepath.Join(dir, "exit3.sh"), []byte("exit 3\n"), 0o644)
	a.UnitCmd = "sh exit3.sh"
	a.ExitCodes = map[int]string{3: "bad"}
	_, err := Run(a, dir, []string{"calc/calc_test.go", "api/api_test.go"}, false)
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Errorf("err = %v, want FatalExitError", err)
	}
}

// The inherited entry is REPLACED, not appended after, so a child that reads the first
// match still gets the adapter's value.
func TestMergeEnvReplacesRatherThanAppends(t *testing.T) {
	got := mergeEnv([]string{"PATH=/bin", "COVERAGE_FILE=/x", "HOME=/h"}, map[string]string{"COVERAGE_FILE": "/t/.coverage"})
	want := []string{"PATH=/bin", "HOME=/h", "COVERAGE_FILE=/t/.coverage"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("mergeEnv = %v, want %v", got, want)
	}
}

// A unit_cmd that reads its unit_files: `go test` runs a test that reads the file whose
// path arrives through env, and the run only passes (with coverage) when it is there.
func TestUnitFilesAreWrittenIntoTheUnitsTmp(t *testing.T) {
	dir, _ := gofixRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "calc", "unitfile_test.go"), []byte(`package calc

import (
	"os"
	"testing"
)

func TestUnitFile(t *testing.T) {
	b, err := os.ReadFile(os.Getenv("RTDD_UF"))
	if err != nil || string(b) != "hi "+os.Getenv("RTDD_TMP") {
		t.Fatalf("unit file = %q, %v", b, err)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gittest.InitRepo(dir, "again"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "go.yaml")
	yaml := goAdapter + "env: { RTDD_UF: \"{tmp}/sub/x.txt\", RTDD_TMP: \"{tmp}\" }\nunit_files:\n  sub/x.txt: \"hi {tmp}\"\n"
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := adapter.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	r := runUnit(a, dir, "calc/unitfile_test.go", []string{"calc/calc.go", "calc/unitfile_test.go"})
	if r.fatal != nil || r.outcome.Status != "pass" {
		t.Fatalf("status %q fatal %v\n%s", r.outcome.Status, r.fatal, r.output)
	}
}

func TestAnUnwritableUnitFileIsAnErrorUnit(t *testing.T) {
	dir, a := gofixRepo(t)
	// "x" is both a file and a parent directory: the second write cannot succeed.
	a.UnitFiles = map[string]string{"x": "a", "x/y": "b"}
	r := runUnit(a, dir, "calc/calc_test.go", nil)
	if r.outcome.Status != "error" || !strings.Contains(r.output, "unit_files") {
		t.Fatalf("status %q output %q, want error naming unit_files", r.outcome.Status, r.output)
	}
}

// A runner that rejects its arguments (pytest without pytest-cov exits 4) says why on
// stderr; the fatal error must carry that text, or the user sees only an exit code.
func TestFatalExitCarriesTheUnitsOutput(t *testing.T) {
	dir, a := gofixRepo(t)
	a.Jobs = 1
	os.WriteFile(filepath.Join(dir, "exit4.sh"), []byte("echo 'error: unrecognized arguments: --cov=.' >&2\nexit 4\n"), 0o644)
	a.UnitCmd = "sh exit4.sh"
	a.ExitCodes = map[int]string{4: "bad-selector"}
	a.Requires = []adapter.Requirement{{Bin: "pytest", Reason: "needs the pytest-cov plugin"}}
	_, err := Run(a, dir, []string{"calc/calc_test.go"}, false)
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v, want FatalExitError", err)
	}
	if !strings.Contains(err.Error(), "unrecognized arguments: --cov=.") {
		t.Errorf("error does not carry the unit's output: %q", err.Error())
	}
	if len(fe.Requires) != 1 || fe.Requires[0] != "needs the pytest-cov plugin" {
		t.Errorf("Requires = %v, want the adapter's requires reasons", fe.Requires)
	}
}
