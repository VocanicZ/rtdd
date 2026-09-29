package runner

import (
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
