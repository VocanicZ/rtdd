package runner

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSeedRunsOnceWithNoTestIDs(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile,
		testReportEntry("tests/test_a.py::test_add", "setup", "passed", 0.001),
		testReportEntry("tests/test_a.py::test_add", "call", "passed", 0.412),
		testReportEntry("tests/test_a.py::test_add", "teardown", "passed", 0.001),
	)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"),
		"tests/test_a.py::test_add|run", []byte{0x20})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	res, err := Seed(a, repo)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if n := invocations(t, argsFile); n != 1 {
		t.Fatalf("%d invocations, want exactly 1 — seed is one full instrumented run", n)
	}
	b, _ := os.ReadFile(argsFile)
	if strings.Contains(string(b), "\ntests/") {
		t.Fatalf("seed passed test ids to the runner; it must run the whole suite. argv:\n%s", b)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Status != "pass" {
		t.Fatalf("Outcomes = %+v, want one passing entry", res.Outcomes)
	}
	if len(res.Coverage.PerTest) != 1 {
		t.Fatalf("Coverage.PerTest = %+v, want one entry", res.Coverage.PerTest)
	}
}

func TestSeedSysmonWarningIsFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_STDOUT": "CoverageWarning: ... (no-sysmon-context); see ...",
	})
	if _, err := Seed(a, repo); !errors.Is(err, ErrSysmonContext) {
		t.Fatalf("Seed = %v, want ErrSysmonContext — seeding from a 90%%-empty map is the worst case", err)
	}
}

func TestSeedExit5IsFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "5"})
	var fe *FatalExitError
	if _, err := Seed(a, repo); !errors.As(err, &fe) {
		t.Fatalf("Seed = %v, want *FatalExitError; seeding an empty suite must not write an empty map", err)
	}
}

func TestListParsesCollectOnlyOutput(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_STDOUT": strings.Join([]string{
			"tests/test_a.py::test_add",
			"tests/test_a.py::test_param[1-one two]",
			"tests/test_a.py::test_param[2-a-b]",
			"tests/test_a.py::test_const",
			"tests/test_b.py::test_mul",
			"tests/test_b.py::test_fail",
			"tests/test_b.py::test_skipped",
			"",
			"9 tests collected in 0.03s",
		}, "\n"),
	})
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
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List =\n  %q\nwant\n  %q", got, want)
	}
}

// Collection order is the suite's own order and carries information — pytest runs
// tests in it. Sorting the ids here would silently reorder every T2 run.
func TestListPreservesCollectionOrder(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_STDOUT": strings.Join([]string{
			"tests/test_z.py::test_zeta",
			"tests/test_a.py::test_alpha",
			"tests/test_m.py::test_mu",
			"",
			"3 tests collected in 0.01s",
		}, "\n"),
	})
	got, err := List(a, repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"tests/test_z.py::test_zeta",
		"tests/test_a.py::test_alpha",
		"tests/test_m.py::test_mu",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List =\n  %q\nwant collection order\n  %q", got, want)
	}
}

// The exit-5 asymmetry, both sides in one test: for List an empty suite is empty,
// for a subset Run the same code means the ids RTDD produced selected nothing.
func TestExit5IsEmptyForListAndFatalForRun(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_EXIT":   "5",
		"RTDD_STUB_STDOUT": "\nno tests ran in 0.01s",
	})

	got, err := List(a, repo)
	if err != nil {
		t.Fatalf("List on an empty suite = %v, want nil error", err)
	}
	if len(got) != 0 {
		t.Fatalf("List = %q, want empty", got)
	}

	var fe *FatalExitError
	if _, err := Run(a, repo, []string{"tests/test_a.py::test_gone"}, false); !errors.As(err, &fe) {
		t.Fatalf("Run = %v, want *FatalExitError — exit 5 on a subset means the selection matched nothing", err)
	}
	if fe.Code != 5 || fe.Label != "no-tests-collected" {
		t.Fatalf("FatalExitError = %+v, want code 5 no-tests-collected", fe)
	}
}

func TestListEmptySuiteExit5IsNotFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_EXIT":   "5",
		"RTDD_STUB_STDOUT": "\nno tests ran in 0.01s",
	})
	got, err := List(a, repo)
	if err != nil {
		t.Fatalf("List on an empty suite = %v, want nil error", err)
	}
	if len(got) != 0 {
		t.Fatalf("List = %q, want empty", got)
	}
}

func TestListExit4IsFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "4"})
	var fe *FatalExitError
	if _, err := List(a, repo); !errors.As(err, &fe) {
		t.Fatalf("List = %v, want *FatalExitError for exit 4", err)
	}
}

func TestListUnmappedNonzeroExitIsAnError(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "3"})
	if _, err := List(a, repo); err == nil {
		t.Fatal("List with an unmapped nonzero exit = nil error; an interpreter crash is not an empty suite")
	}
}

func TestListSysmonWarningIsFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_STDOUT": "CoverageWarning: ... (no-sysmon-context); see ...",
	})
	if _, err := List(a, repo); !errors.Is(err, ErrSysmonContext) {
		t.Fatalf("List = %v, want ErrSysmonContext", err)
	}
}

func TestListWithoutAListCommandIsAnError(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, nil)
	a.List = ""
	if _, err := List(a, repo); err == nil {
		t.Fatal("List with no list command = nil error, want error")
	}
}

// Same enforcement as Run: {src} was dropped by the M1b amendment, and the var map
// the caller supplies is the only thing keeping it out of expanded argv.
func TestListRejectsAnAdapterThatNamesSrc(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, nil)
	a.List = filepath.Join(repo, "stubpytest") + " --collect-only --cov={src}"

	_, err := List(a, repo)
	if err == nil {
		t.Fatal("List with an adapter naming {src} = nil error; the placeholder must be unresolvable")
	}
	if !strings.Contains(err.Error(), "{src}") {
		t.Errorf("error = %v, want it to name the unresolved {src} placeholder", err)
	}
}

// The seed template carries {log}; the list template may too. Both must resolve to a
// real writable path rather than an empty string, or the child gets `--report-log=`.
func TestListResolvesTheLogPlaceholder(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":   argsFile,
		"RTDD_STUB_STDOUT": "tests/test_a.py::test_add",
	})
	a.List = filepath.Join(repo, "stubpytest") + " --collect-only --report-log={log}"

	got, err := List(a, repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0] != "tests/test_a.py::test_add" {
		t.Fatalf("List = %q, want the single collected id", got)
	}
	b, _ := os.ReadFile(argsFile)
	if strings.Contains(string(b), "--report-log=\n") {
		t.Fatalf("{log} expanded to an empty path; argv:\n%s", b)
	}
}
