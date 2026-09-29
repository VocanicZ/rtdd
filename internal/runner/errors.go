package runner

import "fmt"

// FatalExitError is a unit that exited with a code the adapter maps in its exit_codes
// table — 4 (bad-selector) for pytest, say. It looks like a test failure to a naive
// caller and is not: the run cannot be trusted, so the CLI exits 2 instead.
type FatalExitError struct {
	Unit  string
	Code  int
	Label string
}

func (e *FatalExitError) Error() string {
	return fmt.Sprintf("runner: %s exited %d (%s); this is a fatal error, not a test failure", e.Unit, e.Code, e.Label)
}
