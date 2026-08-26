package runner

import (
	"errors"
	"strings"
	"testing"
)

// realWarningStdout is the verbatim stdout of
// `COVERAGE_CORE=sysmon pytest --cov --cov-context=test -q` on this machine
// (coverage.py 7.15.4, pytest 9.0.3). Note where the warning appears: in pytest's
// warnings summary, on STDOUT. Stderr was EMPTY. The process exited 1 only
// because a test failed; with all tests passing it exits 0.
const realWarningStdout = `.....Fs                                                                  [100%]
=============================== warnings summary ===============================
tests/test_a.py::test_add
  /home/claude/.local/lib/python3.13/site-packages/coverage/control.py:799: CoverageWarning: Dynamic contexts aren't supported with core=sysmon; context data may be incomplete (no-sysmon-context); see https://coverage.readthedocs.io/en/7.15.4/messages.html#warning-no-sysmon-context
    self._warn(

-- Docs: https://docs.pytest.org/en/stable/how-to/capture-warnings.html
================================ tests coverage ================================
`

const cleanRun = `.....Fs                                                                  [100%]
=========================== short test summary info ============================
FAILED tests/test_b.py::test_fail - assert 6 == 7
==================== 1 failed, 5 passed, 1 skipped in 0.14s ====================
`

func TestHasSysmonWarning(t *testing.T) {
	if !hasSysmonWarning([]byte(realWarningStdout)) {
		t.Fatal("hasSysmonWarning missed the real measured warning; the map would be ~90% empty with exit 0")
	}
	if hasSysmonWarning([]byte(cleanRun)) {
		t.Fatal("hasSysmonWarning fired on a clean run")
	}
	if hasSysmonWarning(nil) {
		t.Fatal("hasSysmonWarning fired on empty output")
	}
}

// The warning is printed on STDOUT and stderr is EMPTY (measured: stdout hits 1,
// stderr hits 0). A detector fed only the stderr half sees nothing and the
// corrupted map ships; the scan must be of the COMBINED stream.
func TestHasSysmonWarningScansTheCombinedStreamNotStderrAlone(t *testing.T) {
	stdout := []byte(realWarningStdout)
	var stderr []byte

	if hasSysmonWarning(stderr) {
		t.Fatal("fixture is wrong: stderr must be empty, that is the whole point of audit A7")
	}
	combined := append(append([]byte{}, stdout...), stderr...)
	if !hasSysmonWarning(combined) {
		t.Fatal("hasSysmonWarning missed the warning in a combined stream whose stderr half is empty")
	}
}

func TestHasSysmonWarningMatchesTheStableToken(t *testing.T) {
	// coverage.py's message prose and its version-pinned URL both change between
	// releases; the parenthesised warning id does not. Match on that.
	if !hasSysmonWarning([]byte("CoverageWarning: something totally reworded (no-sysmon-context); see https://example/9.9.9/x")) {
		t.Fatal("hasSysmonWarning must key off the no-sysmon-context token, not the surrounding prose")
	}
}

func TestErrSysmonContextMessage(t *testing.T) {
	if ErrSysmonContext == nil {
		t.Fatal("ErrSysmonContext is nil")
	}
	if !strings.Contains(ErrSysmonContext.Error(), "sysmon") {
		t.Fatalf("ErrSysmonContext.Error() = %q, want it to name sysmon", ErrSysmonContext.Error())
	}
	if !errors.Is(ErrSysmonContext, ErrSysmonContext) {
		t.Fatal("errors.Is does not match ErrSysmonContext against itself")
	}
}

func TestFatalExitError(t *testing.T) {
	cases := []struct {
		err  *FatalExitError
		want []string
	}{
		{&FatalExitError{Chunk: 0, Code: 4, Label: "bad-selector"}, []string{"4", "bad-selector"}},
		{&FatalExitError{Chunk: 2, Code: 5, Label: "no-tests-collected"}, []string{"2", "5", "no-tests-collected"}},
	}
	for _, tc := range cases {
		msg := tc.err.Error()
		for _, w := range tc.want {
			if !strings.Contains(msg, w) {
				t.Errorf("FatalExitError.Error() = %q, want it to contain %q", msg, w)
			}
		}
		if !strings.Contains(msg, "not a test failure") {
			t.Errorf("FatalExitError.Error() = %q, want it to say this is not a test failure", msg)
		}
	}
}

func TestFatalExitErrorIsRecoverableWithErrorsAs(t *testing.T) {
	var err error = &FatalExitError{Chunk: 1, Code: 4, Label: "bad-selector"}
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatal("errors.As failed to recover *FatalExitError; the CLI needs it to choose exit 2")
	}
	if fe.Code != 4 {
		t.Fatalf("fe.Code = %d, want 4", fe.Code)
	}
}
