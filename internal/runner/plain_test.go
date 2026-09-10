package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// A plain run writes no .coverage, because `subset_plain` carries no --cov. Reading one
// anyway failed the whole cycle with `.coverage is unreadable` and reported exit 3 over
// zero executed tests — which is worse than the slow path it was meant to replace, and
// worse still for looking like a broken toolchain. Measured against a real flask clone
// before the guard existed.
func plainAdapter(script string) *adapter.Adapter {
	return &adapter.Adapter{
		Name:        "fake",
		Subset:      script + " {tests}",
		SubsetPlain: script + " {tests}",
		Coverage:    "sqlite",
		Report:      "pytest-reportlog",
	}
}

// writeStub writes an executable that produces a report log and NO coverage store,
// which is exactly what an uninstrumented pytest invocation leaves behind.
func writeStub(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "stub.sh")
	body := "#!/usr/bin/env bash\nfor a in \"$@\"; do case \"$a\" in --report-log=*) echo '{\"$report_type\":\"TestReport\",\"when\":\"call\",\"nodeid\":\"t::a\",\"outcome\":\"passed\",\"duration\":0.01}' > \"${a#--report-log=}\";; esac; done\nexit 0\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunPlainDoesNotDemandACoverageStoreItNeverWrote(t *testing.T) {
	root := t.TempDir()
	a := plainAdapter(writeStub(t, root))
	a.Subset = a.Subset + " --report-log={log}"
	a.SubsetPlain = a.SubsetPlain + " --report-log={log}"

	res, err := RunPlain(a, root, []string{"t::a"}, false)
	if err != nil {
		t.Fatalf("RunPlain: %v — a plain run must not read a store it never wrote", err)
	}
	if res == nil {
		t.Fatal("RunPlain returned no result")
	}
	if _, statErr := os.Stat(filepath.Join(root, ".coverage")); statErr == nil {
		t.Fatal("the stub wrote a .coverage; the test no longer exercises the missing-store path")
	}
}

func TestRunPlainRefusesAnAdapterThatDeclaresNoPlainCommand(t *testing.T) {
	root := t.TempDir()
	a := plainAdapter(writeStub(t, root))
	a.SubsetPlain = ""

	_, err := RunPlain(a, root, []string{"t::a"}, false)
	if err == nil {
		t.Fatal("RunPlain accepted an adapter with no subset_plain")
	}
	if !strings.Contains(err.Error(), "subset_plain") {
		t.Errorf("error = %q, want it to name the missing key", err)
	}
}
