package coverage

import "testing"

// Every input below is a verbatim `context.context` value read out of a real
// .coverage on this machine.
func TestNormalizeContext(t *testing.T) {
	cases := []struct {
		name   string
		ctx    string
		wantID string
		wantPh string
		wantOK bool
	}{
		{
			name:   "empty context is import-time, not a test",
			ctx:    "",
			wantID: "", wantPh: "", wantOK: false,
		},
		{
			name:   "plain run phase",
			ctx:    "tests/test_a.py::test_add|run",
			wantID: "tests/test_a.py::test_add", wantPh: "run", wantOK: true,
		},
		{
			name:   "parametrised id containing a space",
			ctx:    "tests/test_a.py::test_param[1-one two]|run",
			wantID: "tests/test_a.py::test_param[1-one two]", wantPh: "run", wantOK: true,
		},
		{
			name:   "parametrised id containing a hyphen",
			ctx:    "tests/test_a.py::test_param[2-a-b]|run",
			wantID: "tests/test_a.py::test_param[2-a-b]", wantPh: "run", wantOK: true,
		},
		{
			name:   "parametrised id containing a pipe: split on the LAST pipe",
			ctx:    "tests/test_pipe.py::test_pipe[a|b]|run",
			wantID: "tests/test_pipe.py::test_pipe[a|b]", wantPh: "run", wantOK: true,
		},
		{
			name:   "setup phase",
			ctx:    "tests/test_c.py::test_with_fixture|setup",
			wantID: "tests/test_c.py::test_with_fixture", wantPh: "setup", wantOK: true,
		},
		{
			name:   "teardown phase",
			ctx:    "tests/test_c.py::test_with_fixture|teardown",
			wantID: "tests/test_c.py::test_with_fixture", wantPh: "teardown", wantOK: true,
		},
		{
			name:   "a static context with no phase suffix is kept whole",
			ctx:    "mystaticcontext",
			wantID: "mystaticcontext", wantPh: "", wantOK: true,
		},
		{
			name:   "a pipe that is not a known phase is part of the id",
			ctx:    "tests/test_x.py::test_y[a|b]",
			wantID: "tests/test_x.py::test_y[a|b]", wantPh: "", wantOK: true,
		},
		{
			name:   "an unknown suffix after a pipe is part of the id",
			ctx:    "tests/test_x.py::test_y|call",
			wantID: "tests/test_x.py::test_y|call", wantPh: "", wantOK: true,
		},
		{
			name:   "a phase suffix with no id is not a test",
			ctx:    "|teardown",
			wantID: "", wantPh: "", wantOK: false,
		},
		{
			name:   "class-based id",
			ctx:    "tests/test_k.py::TestThing::test_method|run",
			wantID: "tests/test_k.py::TestThing::test_method", wantPh: "run", wantOK: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ph, ok := NormalizeContext(tc.ctx)
			if id != tc.wantID || ph != tc.wantPh || ok != tc.wantOK {
				t.Fatalf("NormalizeContext(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.ctx, id, ph, ok, tc.wantID, tc.wantPh, tc.wantOK)
			}
		})
	}
}

// The ids that carry a space, a hyphen or a pipe are exactly the ones a naive
// splitter mangles, and a mangled id no longer re-selects its test. Rebuilding
// "<id>|<phase>" must give back the byte-identical measured context.
func TestNormalizeContextRoundTripsAwkwardIDs(t *testing.T) {
	for _, ctx := range []string{
		"tests/test_a.py::test_param[1-one two]|run",
		"tests/test_a.py::test_param[2-a-b]|run",
		"tests/test_pipe.py::test_pipe[a|b]|run",
		"tests/test_c.py::test_with_fixture|setup",
		"tests/test_c.py::test_with_fixture|teardown",
	} {
		id, ph, ok := NormalizeContext(ctx)
		if !ok {
			t.Fatalf("NormalizeContext(%q) ok = false, want true", ctx)
		}
		if got := id + "|" + ph; got != ctx {
			t.Fatalf("round trip of %q = %q, want the original", ctx, got)
		}
	}
}
