package runner

import "fmt"

// FatalExitError is a unit that exited with a code the adapter maps in its exit_codes
// table — 4 for pytest, say, which is what it exits when pytest-cov is missing and --cov
// is rejected. It looks like a test failure to a naive caller and is not: the run cannot
// be trusted, so the CLI exits 2 instead.
type FatalExitError struct {
	Unit  string
	Code  int
	Label string
	// Output is the tail of the unit's combined output: the runner's own explanation.
	Output string
	// Requires are the adapter's `requires` reasons, for the CLI's hint.
	Requires []string
}

func (e *FatalExitError) Error() string {
	msg := fmt.Sprintf("runner: %s exited %d (%s); this is a fatal error, not a test failure", e.Unit, e.Code, e.Label)
	if e.Output != "" {
		msg += "\n" + e.Output
	}
	return msg
}
