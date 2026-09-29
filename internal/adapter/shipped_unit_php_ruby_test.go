package adapter

import (
	"reflect"
	"strings"
	"testing"
)

func shippedByName(t *testing.T, name string) *Adapter {
	t.Helper()
	all, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range all {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("no shipped adapter %s", name)
	return nil
}

// Neither toolchain runs where this was written, so pin what the YAML renders.
func TestPHPUnitRendersItsUnit(t *testing.T) {
	a := shippedByName(t, "phpunit")
	got, err := a.UnitArgv("tests/ApiTest.php", "/t/u1", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"vendor/bin/phpunit", "--coverage-cobertura", "/t/u1/cobertura.xml", "tests/ApiTest.php"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnitArgv = %q, want %q", got, want)
	}
	if p := a.CoveragePath("/t/u1"); p != "/t/u1/cobertura.xml" || a.CoverageFormat != "cobertura" {
		t.Errorf("coverage = %s (%s)", p, a.CoverageFormat)
	}
	if env := a.UnitEnv("/t/u1"); env["XDEBUG_MODE"] != "coverage" {
		t.Errorf("UnitEnv = %v", env)
	}
}

func TestRSpecRendersItsUnitAndLoader(t *testing.T) {
	a := shippedByName(t, "rspec")
	got, err := a.UnitArgv("spec/api_spec.rb", "/t/u1", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bundle", "exec", "rspec", "spec/api_spec.rb"}; !reflect.DeepEqual(got, want) {
		t.Errorf("UnitArgv = %q, want %q", got, want)
	}
	if env := a.UnitEnv("/t/u1"); env["RUBYOPT"] != "-r/t/u1/rtdd_simplecov.rb" {
		t.Errorf("UnitEnv = %v", env)
	}
	loader := a.UnitFileContents("/t/u1")["rtdd_simplecov.rb"]
	for _, w := range []string{`require "bundler/setup"`, `c.single_report_path = "/t/u1/lcov.info"`, "SimpleCov.start"} {
		if !strings.Contains(loader, w) {
			t.Errorf("loader lacks %q:\n%s", w, loader)
		}
	}
	if strings.Contains(loader, "{tmp}") {
		t.Errorf("loader has an unsubstituted {tmp}:\n%s", loader)
	}
	if p := a.CoveragePath("/t/u1"); p != "/t/u1/lcov.info" || a.CoverageFormat != "lcov" {
		t.Errorf("coverage = %s (%s)", p, a.CoverageFormat)
	}
}
