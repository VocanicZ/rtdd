package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// newDetectableRepo is the repo the documented setup path produces: a python toolchain
// marker, source, tests, and NO .rtdd/adapter.yaml. `rtdd init` writes no adapter file,
// so this — not the fixture-installed state — is what `which` actually meets.
func newDetectableRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	gittest.Write(t, dir, "src/constants.py", "MAX_RETRIES = 3\n")
	gittest.Write(t, dir, "src/logic.py", "def add(a, b):\n    return a + b\n")
	gittest.Write(t, dir, "tests/test_logic.py", "def test_add():\n    pass\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	return dir
}

func containsString(hay []string, want string) bool {
	for _, s := range hay {
		if s == want {
			return true
		}
	}
	return false
}
