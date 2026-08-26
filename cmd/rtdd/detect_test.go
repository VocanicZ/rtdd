package main

import (
	"bytes"
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

// status reads the same env, so it reports the detected adapter by name rather than
// "none", and says the name came from detection rather than from a file that is absent.
func TestStatusDetectsTheAdapterWhenNoAdapterFileExists(t *testing.T) {
	dir := newDetectableRepo(t)

	code, stdout, stderr := rtdd(t, dir, "status")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "adapter: python") {
		t.Errorf("status must name the detected adapter:\n%s", stdout)
	}
	if strings.Contains(stdout, "adapter: none") {
		t.Errorf("status reports no adapter on a repo detection resolves:\n%s", stdout)
	}
	if !strings.Contains(stdout, "detected") {
		t.Errorf("status must say the adapter came from detection, not from a file:\n%s", stdout)
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

// The mirror of TestCmdRunFiresTheStaticImportFallbackAndAgreesWithWhich, on the repo
// state that has no adapter file at all: `which` advises and `run` executes, so they
// must classify the same repo with the same adapter.
func TestWhichAndRunAgreeOnTheDetectedAdapter(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if _, err := os.Stat(filepath.Join(repo, ".rtdd", "adapter.yaml")); err == nil {
		t.Fatalf("precondition: this test covers the repo state with no .rtdd/adapter.yaml")
	}
	makeSuiteGreen(t, repo)
	gitRun(t, repo, "commit", "-am", "green suite")

	if code := cmdSeed(nil); code != 0 {
		t.Fatalf("cmdSeed = %d, want 0 once the suite is green", code)
	}
	touchLogic(t, repo)

	var wout, werr bytes.Buffer
	if code := cmdWhich([]string{"--json"}, &wout, &werr); code != 0 {
		t.Fatalf("cmdWhich --json = %d, want 0 (stderr: %s)", code, werr.String())
	}
	want := decodeOutput(t, wout.String())

	var code int
	raw := captureStdout(t, func() { code = cmdRun([]string{"--json"}) })
	if code != 0 {
		t.Fatalf("cmdRun --json = %d, want 0.\n%s", code, raw)
	}
	got := decodeOutput(t, raw)

	if want.Adapter == "" {
		t.Errorf("which adapter = %q, want the detected name", want.Adapter)
	}
	if got.Adapter != want.Adapter {
		t.Errorf("run adapter = %q, which adapter = %q — the two commands must agree",
			got.Adapter, want.Adapter)
	}
	for _, c := range got.Changed {
		for _, w := range want.Changed {
			if c.Path == w.Path && c.Instrumentable != w.Instrumentable {
				t.Errorf("%s instrumentable: run = %v, which = %v", c.Path, c.Instrumentable, w.Instrumentable)
			}
		}
	}
	if !containsString(instrumentablePaths(got.Changed), "src/logic.py") {
		t.Errorf("run classified src/logic.py as not instrumentable:\n%s", raw)
	}
	if !containsString(instrumentablePaths(want.Changed), "src/logic.py") {
		t.Errorf("which classified src/logic.py as not instrumentable:\n%s", wout.String())
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
