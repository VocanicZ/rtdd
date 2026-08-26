package coverage

import "strings"

// phases are the dynamic-context suffixes pytest-cov appends to a nodeid.
// Measured on real data: `|run`, `|setup`, `|teardown`.
var phases = map[string]bool{"run": true, "setup": true, "teardown": true}

// NormalizeContext splits "tests/test_a.py::test_x|run" into
// ("tests/test_a.py::test_x", "run", true).
//
// An empty context returns ok=false — that is import-time coverage, executed
// during collection before any dynamic context is set, and attributed to no test
// (spec §6, audit A1).
//
// The split is on the LAST '|', and only when the suffix is a known phase: a
// parametrised id can itself contain '|' (measured:
// "tests/test_pipe.py::test_pipe[a|b]|run"), and a static context set by the host
// repo has no phase suffix at all.
func NormalizeContext(ctx string) (testID, phase string, ok bool) {
	if ctx == "" {
		return "", "", false
	}
	i := strings.LastIndex(ctx, "|")
	if i < 0 {
		return ctx, "", true
	}
	if !phases[ctx[i+1:]] {
		return ctx, "", true
	}
	id := ctx[:i]
	if id == "" {
		return "", "", false
	}
	return id, ctx[i+1:], true
}
