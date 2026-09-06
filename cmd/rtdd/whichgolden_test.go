package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A single-adapter repository's `rtdd which` output is frozen, byte for byte.
//
// Spec §4.1 makes "a seeded Python repository's selection is byte-identical to today" a
// regression REQUIREMENT, and per-adapter selection is exactly the change that could
// break it without anyone noticing: a heading printed for one adapter, a map filtered
// one row too far, a tier that moved because the selector saw a different map. The
// golden is the whole of stdout, so any of those fails here first.
//
// The golden was captured from the commit before per-adapter selection existed. It is
// not to be regenerated to make a change pass — a diff here means the change is visible
// to every existing Python repository, which is the thing being ruled out.
func TestWhichOutputForASeededPythonRepositoryIsByteIdentical(t *testing.T) {
	// Resolved BEFORE rtdd() chdirs into the fixture repository: a relative testdata
	// path would then name a file inside the fixture, not this package's.
	golden, err := filepath.Abs(filepath.Join("testdata", "which-python-golden.txt"))
	if err != nil {
		t.Fatal(err)
	}

	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}

	if os.Getenv("RTDD_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, []byte(stdout), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if stdout != string(want) {
		t.Errorf("rtdd which output moved for a single-adapter repository.\n got: %q\nwant: %q", stdout, want)
	}
}
