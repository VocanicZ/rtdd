package main

import (
	"os"
	"path/filepath"
	"strings"
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

// Detection replaces the file default (docs/plans/00-interfaces.md:912). `rtdd init`
// writes no .rtdd/adapter.yaml, so a stock post-init repo had every advisory command
// running with no adapter: adapter "", nothing instrumentable, unmapped_files empty
// exactly when a user first reaches for it.
func TestWhichDetectsTheAdapterOnAStockPostInitRepo(t *testing.T) {
	dir := newDetectableRepo(t)

	if code, _, stderr := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".rtdd", "adapter.yaml")); err == nil {
		t.Fatalf("precondition: rtdd init now writes .rtdd/adapter.yaml; this test covers the state where it does not")
	}
	writeFile(t, dir, "src/constants.py", "MAX_RETRIES = 4\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)

	if got.Adapter != "python" {
		t.Errorf("adapter = %q, want %q: detection replaces the file default", got.Adapter, "python")
	}
	var found bool
	for _, c := range got.Changed {
		if c.Path != "src/constants.py" {
			continue
		}
		found = true
		if !c.Instrumentable {
			t.Errorf("src/constants.py instrumentable = false; a detected adapter classifies it as source")
		}
	}
	if !found {
		t.Fatalf("src/constants.py missing from changed:\n%s", stdout)
	}
	if !containsString(got.UnmappedFiles, "src/constants.py") {
		t.Errorf("unmapped_files = %#v, want src/constants.py: no map row covers it", got.UnmappedFiles)
	}
	if anyWarningContains(got.Warnings, "classification is disabled") {
		t.Errorf("warnings = %#v, want no missing-adapter warning once detection succeeds", got.Warnings)
	}
}

// An explicit --adapter path is an override, not a hint. A path that does not exist is a
// configuration error the user asked for, never a silent fall back to detection.
func TestExplicitAdapterPathThatDoesNotExistIsAConfigError(t *testing.T) {
	dir := newDetectableRepo(t)

	code, _, stderr := rtdd(t, dir, "which", "--adapter", ".rtdd/nope.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 for an --adapter path that does not exist (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "nope.yaml") {
		t.Errorf("stderr = %q, want it to name the missing path", stderr)
	}
}

// Detection finding nothing still says so. A repo with no recognisable toolchain must
// not read as a classified one.
func TestWhichStillWarnsWhenDetectionFindsNoAdapter(t *testing.T) {
	dir := newTestRepo(t) // no toolchain marker of any kind
	writeFile(t, dir, "src/auth.py", "def login():\n    return 9\n")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "classification is disabled") {
		t.Errorf("which must still warn when detection resolves no adapter:\n%s", stdout)
	}
}

func instrumentablePaths(changed []JSONChange) []string {
	var out []string
	for _, c := range changed {
		if c.Instrumentable {
			out = append(out, c.Path)
		}
	}
	return out
}

func containsString(hay []string, want string) bool {
	for _, s := range hay {
		if s == want {
			return true
		}
	}
	return false
}
