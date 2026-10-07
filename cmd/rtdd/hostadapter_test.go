package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// newHostAdapterRepo is a python repo — the built-in detects it — that also carries host
// adapter files.
func newHostAdapterRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	gittest.Write(t, dir, "src/logic.py", "def add(a, b):\n    return a + b\n")
	gittest.Write(t, dir, "tests/test_logic.py", "def test_add():\n    pass\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// A repo with no .rtdd/adapters/ is every repo that exists today. The resolved set is the
// shipped set, so the fidelity block reports built-ins only: nothing is host-authored,
// nothing overrides anything, and no file failed to load.
func TestDoctorOnARepoWithNoHostAdaptersSaysNothingAboutAdapters(t *testing.T) {
	dir := newHostAdapterRepo(t)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, unwanted := range []string{"host-authored", "overrides built-in", "not loaded"} {
		if strings.Contains(stdout, unwanted) {
			t.Errorf("doctor said %q on a repo with no host adapters:\n%s", unwanted, stdout)
		}
	}
}
