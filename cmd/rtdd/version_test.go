package main

import (
	"strings"
	"testing"
)

func TestVersionPrintsTheBuildIdentity(t *testing.T) {
	dir := newTestRepo(t)
	code, stdout, stderr := rtdd(t, dir, "--version")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "rtdd") || !strings.Contains(stdout, version) {
		t.Errorf("stdout = %q, want it to name rtdd and the version %q", stdout, version)
	}
}
