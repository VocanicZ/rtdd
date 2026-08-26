package runner

import (
	"bytes"
	"errors"
	"fmt"
)

// ErrSysmonContext is returned when the run emitted coverage.py's
// "no-sysmon-context" warning. Callers MUST exit 3. Never proceed with the map.
//
// Measured (audit A7): under COVERAGE_CORE=sysmon — the default on Python 3.14+
// where supported — 4 tests ran and 1 non-empty context was recorded, and the
// process exited 0. A warning is the only signal there is.
var ErrSysmonContext = errors.New("dynamic contexts unavailable: COVERAGE_CORE=sysmon")

// sysmonToken is coverage.py's stable warning id. The surrounding prose and the
// version-pinned documentation URL both change between releases; this does not.
const sysmonToken = "no-sysmon-context"

// hasSysmonWarning reports whether the run's COMBINED stdout+stderr carries the
// no-sysmon-context warning.
//
// Measured: pytest prints it in its warnings summary on STDOUT. Stderr was empty.
// Scanning stderr alone finds nothing and the corrupted map gets written.
func hasSysmonWarning(combined []byte) bool {
	return bytes.Contains(combined, []byte(sysmonToken))
}

// FatalExitError is a chunk that exited with a code the adapter maps in its
// exit_codes table — 4 (bad-selector) or 5 (no-tests-collected) for pytest.
//
// These look like test failures to a naive caller and are not: 4 means the
// selector RTDD produced was not accepted, 5 means nothing ran at all. Either
// one silently turns a subset run into "no tests failed".
type FatalExitError struct {
	Chunk int
	Code  int
	Label string
}

func (e *FatalExitError) Error() string {
	return fmt.Sprintf("runner: chunk %d exited %d (%s); this is a fatal error, not a test failure",
		e.Chunk, e.Code, e.Label)
}
