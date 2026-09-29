package runner

import (
	"errors"
	"fmt"
	"testing"
)

func TestFatalExitError(t *testing.T) {
	cases := []struct {
		err  *FatalExitError
		want string
	}{
		{&FatalExitError{Unit: "tests/test_a.py", Code: 4, Label: "bad-selector"},
			"runner: tests/test_a.py exited 4 (bad-selector); this is a fatal error, not a test failure"},
		{&FatalExitError{Unit: "calc/calc_test.go", Code: 5, Label: "no-tests-collected"},
			"runner: calc/calc_test.go exited 5 (no-tests-collected); this is a fatal error, not a test failure"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
}

// The CLI recovers it with errors.As to choose exit 2, even when wrapped.
func TestFatalExitErrorIsRecoverableWithErrorsAs(t *testing.T) {
	err := fmt.Errorf("seed: %w", &FatalExitError{Unit: "tests/test_a.py", Code: 4, Label: "bad-selector"})
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatal("errors.As failed to recover *FatalExitError")
	}
	if fe.Unit != "tests/test_a.py" || fe.Code != 4 || fe.Label != "bad-selector" {
		t.Fatalf("recovered %+v", fe)
	}
}
