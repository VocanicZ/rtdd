package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// #578 rule 1: a changed line code() blanks (a docstring, a comment) maps to no node, so a
// comment-only edit selects nothing; a code line next to it still selects its tests.
func TestWhichIgnoresChangedLinesThatAreNotCode(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "lib/w.py", "\"\"\"Doc line one.\nDoc line two.\n\"\"\"\n# note\n\n\ndef f():\n    return 1\n")
	gittest.Write(t, dir, "tests/test_w.py", "from lib.w import f\n\n\ndef test_f():\n    assert f() == 1\n")
	gittest.Commit(t, dir, "base")

	gittest.Write(t, dir, "lib/w.py", "\"\"\"Doc line ONE.\nDoc line TWO.\n\"\"\"\n# NOTE\n\n\ndef f():\n    return 1\n")
	w, _ := runE2EWhich(t, dir)
	assertIDs(t, "changed_nodes", w.changed(), []string{})
	assertIDs(t, "Round 1", w.round(1), []string{})

	gittest.Write(t, dir, "lib/w.py", "\"\"\"Doc line ONE.\nDoc line TWO.\n\"\"\"\n# NOTE\n\n\ndef f():\n    return 2\n")
	w, _ = runE2EWhich(t, dir)
	assertIDs(t, "changed_nodes", w.changed(), []string{"lib/w.py::f"})
	assertIDs(t, "Round 1", w.round(1), []string{"tests/test_w.py::test_f"})
}
