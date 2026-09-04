package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
)

func TestRenderInit(t *testing.T) {
	steps := []install.Step{
		{Path: ".gitattributes", Action: install.Create},
		{Path: ".rtdd/config.yaml", Action: install.Create},
		{Path: "AGENTS.md", Action: install.Create},
		{Path: "CLAUDE.md", Action: install.AppendBlock},
		{Path: ".cursor/rules/rtdd.mdc", Action: install.Skip, Note: "already current"},
	}
	got := RenderInit(steps)
	want := "" +
		"create         .gitattributes\n" +
		"create         .rtdd/config.yaml\n" +
		"create         AGENTS.md\n" +
		"append-block   CLAUDE.md\n" +
		"skip           .cursor/rules/rtdd.mdc  — already current\n"
	if got != want {
		t.Fatalf("RenderInit()\n got:\n%s\nwant:\n%s", got, want)
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
	for _, want := range []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", ".cursor/rules/rtdd.mdc", ".claude/skills/rtdd/SKILL.md"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("init output is missing %q:\n%s", want, stdout)
		}
	}

	agents := readRepoFileForTest(t, dir, "AGENTS.md")
	if !strings.HasPrefix(agents, "# AGENTS\n\nHouse rules.\n") {
		t.Errorf("the host repo's AGENTS.md was clobbered:\n%s", agents)
	}
	if !strings.Contains(agents, protocol.BeginMarker) {
		t.Errorf("AGENTS.md did not gain the managed block:\n%s", agents)
	}
	if got := readRepoFileForTest(t, dir, ".gitattributes"); !strings.Contains(got, ".rtdd/map.jsonl merge=union") {
		t.Errorf(".gitattributes = %q, want the union merge driver", got)
	}
	// The host repo never had a CLAUDE.md, so init must not introduce one — the
	// generated Claude Code skill under .claude/skills/rtdd/ is the right surface.
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err == nil {
		t.Error("init must not create CLAUDE.md when the host repo has none")
	}

	// Re-running reports skip for every step.
	code, stdout, stderr = rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("second init exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "create") || strings.Contains(stdout, "replace-block") || strings.Contains(stdout, "append-block") {
		t.Errorf("re-running init must report skip for every file:\n%s", stdout)
	}
}

// A host repo that already has a CLAUDE.md gets the block merged into it, same as
// AGENTS.md.
func TestInitMergesClaudeMdWhenTheHostRepoAlreadyHasOne(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "CLAUDE.md", "# Our Claude notes\n\nDo not touch prod.\n")

	code, _, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	claude := readRepoFileForTest(t, dir, "CLAUDE.md")
	if !strings.HasPrefix(claude, "# Our Claude notes\n\nDo not touch prod.\n") {
		t.Errorf("the host repo's CLAUDE.md was clobbered:\n%s", claude)
	}
	if !strings.Contains(claude, protocol.BeginMarker) {
		t.Errorf("CLAUDE.md did not gain the managed block:\n%s", claude)
	}
}

func TestInitDryRunPrintsThePlanAndWritesNothing(t *testing.T) {
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "init", "--dry-run")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "AGENTS.md") {
		t.Errorf("dry-run output is missing the plan:\n%s", stdout)
	}
	for _, rel := range []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", ".cursor/rules/rtdd.mdc", ".claude/skills/rtdd/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil {
			t.Errorf("--dry-run must not write %s", rel)
		}
	}
}

// A whole-file target that exists and differs is a conflict, and init refuses to
// write anything until --force is passed.
func TestInitConflictsOnAHandEditedSkillFileAndForceOverridesIt(t *testing.T) {
	dir := newTestRepo(t)
	skillDir := filepath.Join(dir, ".claude", "skills", "rtdd")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".claude/skills/rtdd/SKILL.md", "hand written, not ours\n")

	code, _, stderr := rtdd(t, dir, "init")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 on conflict (stderr: %s)", code, stderr)
	}
	if got := readRepoFileForTest(t, dir, ".claude/skills/rtdd/SKILL.md"); got != "hand written, not ours\n" {
		t.Errorf("conflicting file was written without --force: %q", got)
	}

	code, _, stderr = rtdd(t, dir, "init", "--force")
	if code != 0 {
		t.Fatalf("--force exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if got := readRepoFileForTest(t, dir, ".claude/skills/rtdd/SKILL.md"); got == "hand written, not ours\n" {
		t.Error("--force did not overwrite the conflicting file")
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

	for _, rel := range []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", ".cursor/rules/rtdd.mdc", ".claude/skills/rtdd/SKILL.md"} {
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
	for _, rel := range []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", ".cursor/rules/rtdd.mdc", ".claude/skills/rtdd/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not installed into the working directory: %v", rel, err)
		}
		if !strings.Contains(stdout, rel) {
			t.Errorf("init output is missing %q:\n%s", rel, stdout)
		}
	}
}
