package main

import (
	"strings"
	"testing"
)

func TestDoctorWithAPositionalArgumentIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "doctor", "src/db.py")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage") {
		t.Errorf("stderr = %q, want usage text", stderr)
	}
}

func TestDoctorWithAnUnknownFlagIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "doctor", "--frobnicate")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("stderr is empty; a usage error must say what was wrong")
	}
}

func TestUsageDocumentsDoctor(t *testing.T) {
	dir := newTestRepo(t)
	code, stdout, _ := rtdd(t, dir, "help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "rtdd doctor") {
		t.Errorf("usage text does not document doctor:\n%s", stdout)
	}
}
