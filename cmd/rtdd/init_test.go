package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/initrepo"
)

func TestRenderInit(t *testing.T) {
	acts := []initrepo.Action{
		{Path: ".gitattributes", Kind: "created"},
		{Path: ".rtdd/config.yaml", Kind: "created"},
		{Path: "AGENTS.md", Kind: "created"},
		{Path: "CLAUDE.md", Kind: "updated"},
		{Path: ".cursor/rules/rtdd.mdc", Kind: "unchanged"},
	}
	got := RenderInit(acts)
	want := "" +
		"rtdd init\n" +
		"  created    .gitattributes\n" +
		"  created    .rtdd/config.yaml\n" +
		"  created    AGENTS.md\n" +
		"  updated    CLAUDE.md\n" +
		"  unchanged  .cursor/rules/rtdd.mdc\n" +
		"\n" +
		"Next: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it.\n"
	if got != want {
		t.Fatalf("RenderInit()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestInitBlockDocumentsTheThreeClasses(t *testing.T) {
	b := initrepo.Block()
	for _, needle := range []string{"import-time", "uncovered", "covered", "This is not a gap"} {
		if !strings.Contains(b, needle) {
			t.Fatalf("agent front-end block is missing %q:\n%s", needle, b)
		}
	}
}

// `rtdd init` is dispatched from main, installs into the working directory, and exits 0.
func TestInitInstallsIntoTheWorkingDirectory(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "AGENTS.md", "# AGENTS\n\nHouse rules.\n")

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", "CLAUDE.md", ".cursor/rules/rtdd.mdc"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("init output is missing %q:\n%s", want, stdout)
		}
	}

	agents := readRepoFileForTest(t, dir, "AGENTS.md")
	if !strings.HasPrefix(agents, "# AGENTS\n\nHouse rules.\n") {
		t.Errorf("the host repo's AGENTS.md was clobbered:\n%s", agents)
	}
	if !strings.Contains(agents, initrepo.BeginMarker) {
		t.Errorf("AGENTS.md did not gain the managed block:\n%s", agents)
	}
	if got := readRepoFileForTest(t, dir, ".gitattributes"); !strings.Contains(got, ".rtdd/map.jsonl merge=union") {
		t.Errorf(".gitattributes = %q, want the union merge driver", got)
	}

	// Re-running reports unchanged for every file.
	code, stdout, stderr = rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("second init exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "created") || strings.Contains(stdout, "updated") {
		t.Errorf("re-running init must report unchanged for every file:\n%s", stdout)
	}
}

func TestInitRejectsExtraArguments(t *testing.T) {
	dir := newTestRepo(t)
	code, _, stderr := rtdd(t, dir, "init", "--frobnicate")
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr is empty; a usage error must say what was wrong")
	}
}

// The command must be discoverable: an agent that cannot find `init` in the help text
// will never run it.
func TestUsageDocumentsInit(t *testing.T) {
	dir := newTestRepo(t)
	code, stdout, _ := rtdd(t, dir, "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "rtdd init") {
		t.Errorf("usage text does not document `rtdd init`:\n%s", stdout)
	}
}

func readRepoFileForTest(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// `rtdd init` run from a subdirectory installs at the REPO ROOT, not at the working
// directory. Every other command resolves the root with findRepoRoot; init must agree.
//
// The `.gitattributes` assertion is deliberately made through `git check-attr` rather
// than by reading the file: attribute patterns are directory-scoped, so a
// `.gitattributes` written into sub/deep/ binds `merge=union` to a path that does not
// exist and leaves the real map at the root with no union merge driver at all. Reading
// file contents cannot see that; asking git can.
func TestInitInstallsAtTheRepoRootFromASubdirectory(t *testing.T) {
	dir := newTestRepo(t)
	deep := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := rtdd(t, deep, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}

	for _, rel := range []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", "CLAUDE.md", ".cursor/rules/rtdd.mdc"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not installed at the repo root: %v", rel, err)
		}
		if _, err := os.Stat(filepath.Join(deep, filepath.FromSlash(rel))); err == nil {
			t.Errorf("%s was installed into the working directory sub/deep, not the repo root", rel)
		}
	}

	got := gitRun(t, dir, "check-attr", "merge", "--", ".rtdd/map.jsonl")
	if !strings.Contains(got, "merge: union") {
		t.Errorf("git check-attr merge -- .rtdd/map.jsonl = %q, want the union merge driver", got)
	}
}

// Outside a git repository there is no root to find, and `rtdd init` must still work:
// installing before `git init` is a legitimate order. It falls back to the working
// directory.
func TestInitFallsBackToTheWorkingDirectoryOutsideAGitRepository(t *testing.T) {
	dir := t.TempDir()

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, rel := range []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", "CLAUDE.md", ".cursor/rules/rtdd.mdc"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not installed into the working directory: %v", rel, err)
		}
		if !strings.Contains(stdout, rel) {
			t.Errorf("init output is missing %q:\n%s", rel, stdout)
		}
	}
}
