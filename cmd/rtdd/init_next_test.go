package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// newPolyglotRepo detects BOTH the built-in python adapter (pyproject.toml) and the
// host-authored vitest one (package.json). It is the case that stops the seed guidance
// from being a global flag flip: the repository still has a coverage adapter to seed.
func newPolyglotRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	gittest.Write(t, dir, "src/logic.py", "def add(a, b):\n    return a + b\n")
	gittest.Write(t, dir, "tests/test_logic.py", "def test_add():\n    pass\n")
	gittest.Write(t, dir, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b;\n")
	gittest.Write(t, dir, "src/logic.test.ts", "it('adds', () => {});\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// The Python-only path is unchanged, word for word: seeding really is the next step for
// an adapter that records coverage.
func TestInitInACoverageRepoKeepsTheSeedNextStep(t *testing.T) {
	dir := newDetectableRepo(t)
	fixLookPath(t)

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	const want = "Next: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it."
	if !strings.Contains(stdout, want) {
		t.Errorf("stdout does not contain %q:\n%s", want, stdout)
	}
}
