package adapter

import (
	"reflect"
	"strings"
	"testing"
)

func v3(t *testing.T, body string) *Adapter {
	t.Helper()
	a, err := parse([]byte(body), "test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

const goV3 = `name: go
detect: ["go.mod"]
unit_cmd: "go test -count=1 -coverpkg=./... -coverprofile={tmp}/cover.out -run {names} ./{dir}"
unit_names: '^func (Test\w+)\('
coverage_file: "{tmp}/cover.out"
coverage_format: gocover
env: { GOFLAGS: "-mod=mod", CACHE: "{tmp}/c" }
test_globs: ["**/*_test.go"]
source_globs: ["**/*.go"]
`

func TestUnitArgvSubstitutesEveryPlaceholder(t *testing.T) {
	a := v3(t, goV3)
	got, err := a.UnitArgv("calc/calc_test.go", "/tmp/u1", "^(TestAdd|TestSub)$")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go", "test", "-count=1", "-coverpkg=./...", "-coverprofile=/tmp/u1/cover.out",
		"-run", "^(TestAdd|TestSub)$", "./calc"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnitArgv = %q, want %q", got, want)
	}
	if p := a.CoveragePath("/tmp/u1"); p != "/tmp/u1/cover.out" {
		t.Errorf("CoveragePath = %q", p)
	}
	if env := a.UnitEnv("/tmp/u1"); env["CACHE"] != "/tmp/u1/c" || env["GOFLAGS"] != "-mod=mod" {
		t.Errorf("UnitEnv = %v", env)
	}
}

func TestUnitArgvRootDirIsDot(t *testing.T) {
	a := v3(t, goV3)
	got, _ := a.UnitArgv("calc_test.go", "/t", "^(TestA)$")
	if got[len(got)-1] != "./." {
		t.Errorf("root-level unit dir = %q, want ./.", got[len(got)-1])
	}
}

func TestUnitNamesOf(t *testing.T) {
	a := v3(t, goV3)
	names, ok := a.UnitNamesOf([]byte("package c\nfunc TestAdd(t *testing.T) {}\nfunc helper() {}\nfunc TestSub(t *testing.T) {}\n"))
	if !ok || names != "^(TestAdd|TestSub)$" {
		t.Errorf("UnitNamesOf = %q, %v", names, ok)
	}
	if _, ok := a.UnitNamesOf([]byte("package c\nfunc helper() {}\n")); ok {
		t.Error("a file with no Test funcs reported runnable names")
	}
}

func TestV3Validation(t *testing.T) {
	for _, c := range []struct{ name, body, want string }{
		{"unknown format", strings.Replace(goV3, "gocover", "sqlite", 1), "coverage_format"},
		{"no unit placeholder", strings.Replace(goV3, "./{dir}", "./...", 1) + "", ""},
		{"coverage file outside tmp", strings.Replace(goV3, `coverage_file: "{tmp}/cover.out"`, `coverage_file: "cover.out"`, 1), "coverage_file"},
		{"unknown placeholder", strings.Replace(goV3, "{names}", "{tests}", 1), "{tests}"},
		{"bad names regex", strings.Replace(goV3, `'^func (Test\w+)\('`, `'^func (Test'`, 1), "unit_names"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := parse([]byte(c.body), "x.yaml")
			if c.want == "" {
				if err != nil {
					t.Fatalf("valid adapter rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want mention of %q", err, c.want)
			}
		})
	}
}
