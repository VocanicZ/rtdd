package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
)

// markDetectable gives a fixture repo the python toolchain marker a real python repository
// carries. v0.3.0's init gates on nothing, so the marker only makes the fixture realistic.
func markDetectable(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
}

func TestRenderInit(t *testing.T) {
	steps := []install.Step{
		{Path: ".claude/skills/rtdd/SKILL.md", Action: install.Create},
		{Path: ".rtdd/config.yaml", Action: install.Create},
		{Path: "AGENTS.md", Action: install.Create},
		{Path: "CLAUDE.md", Action: install.AppendBlock},
		{Path: ".cursor/rules/rtdd.mdc", Action: install.Skip, Note: "already current"},
	}
	got := RenderInit(steps)
	want := "" +
		"create         .claude/skills/rtdd/SKILL.md\n" +
		"create         .rtdd/config.yaml\n" +
		"create         AGENTS.md\n" +
		"append-block   CLAUDE.md\n" +
		"skip           .cursor/rules/rtdd.mdc  — already current\n"
	if got != want {
		t.Fatalf("RenderInit()\n got:\n%s\nwant:\n%s", got, want)
	}
}

// worktreeFiles lists every file under dir outside .git/, slash-separated and relative.
func worktreeFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		out = append(out, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// #509 acceptance: `rtdd init` in a repository with the user's own AGENTS.md and .gitignore
// leaves both byte-identical, and creates nothing outside .rtdd/.
func TestInitInstallsIntoTheWorkingDirectory(t *testing.T) {
	dir := newTestRepo(t)
	markDetectable(t, dir)
	host := map[string]string{
		"AGENTS.md":  "# AGENTS\n\nHouse rules.\n",
		".gitignore": "node_modules/\n*.log\n",
	}
	for rel, body := range host {
		writeFile(t, dir, rel, body)
	}
	before := map[string]bool{}
	for _, f := range worktreeFiles(t, dir) {
		before[f] = true
	}

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{".rtdd/config.yaml", ".rtdd/.gitignore"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("init output is missing %q:\n%s", want, stdout)
		}
	}
	for rel, body := range host {
		if got := readRepoFileForTest(t, dir, rel); got != body {
			t.Errorf("%s = %q after init, want it byte-identical %q", rel, got, body)
		}
	}
	var created []string
	for _, f := range worktreeFiles(t, dir) {
		if !before[f] {
			created = append(created, f)
		}
	}
	if strings.Join(created, ",") != ".rtdd/.gitignore,.rtdd/config.yaml" {
		t.Errorf("init created %v, want only .rtdd/.gitignore and .rtdd/config.yaml", created)
	}
	if got := readRepoFileForTest(t, dir, ".rtdd/.gitignore"); got != "graph.json\n" {
		t.Errorf(".rtdd/.gitignore = %q, want %q", got, "graph.json\n")
	}

	// Re-running reports skip for every step.
	code, stdout, stderr = rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("second init exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "create") || strings.Contains(stdout, "append") {
		t.Errorf("re-running init must report skip for every file:\n%s", stdout)
	}
}

// --project-skill opts in to the committed front-ends: rtdd's own whole files, never a
// block in AGENTS.md.
func TestInitProjectSkillWritesTheWholeFileFrontEndsOnly(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "AGENTS.md", "# AGENTS\n\nHouse rules.\n")

	code, _, stderr := rtdd(t, dir, "init", "--project-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc"} {
		if got := readRepoFileForTest(t, dir, rel); !strings.Contains(got, install.OptInMarker) {
			t.Errorf("%s carries no opt-in marker:\n%s", rel, got)
		}
	}
	if got := readRepoFileForTest(t, dir, "AGENTS.md"); got != "# AGENTS\n\nHouse rules.\n" {
		t.Errorf("--project-skill touched AGENTS.md:\n%s", got)
	}
}

// --append-to is the explicit way into a shared file: the block is merged into the file
// named, and every byte of the host's own text stays.
func TestInitAppendToMergesTheNamedSharedFile(t *testing.T) {
	dir := newTestRepo(t)
	markDetectable(t, dir)
	writeFile(t, dir, "CLAUDE.md", "# Our Claude notes\n\nDo not touch prod.\n")

	code, _, stderr := rtdd(t, dir, "init", "--append-to", "CLAUDE.md")
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
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
		t.Error("init created AGENTS.md though only CLAUDE.md was named")
	}
}

// What an older init wrote outside .rtdd/ — in the repository and in the home directory —
// is reported, each line naming the command that removes it, and nothing is changed.
func TestInitReportsAnOlderInitsFootprintAndLeavesItAlone(t *testing.T) {
	dir := newTestRepo(t)
	home := t.TempDir()
	t.Setenv("RTDD_HOME", home)
	agents := "# AGENTS\n\n" + protocol.BeginMarker + "\nold\n" + protocol.EndMarker + "\n"
	writeFile(t, dir, "AGENTS.md", agents)
	writeFile(t, dir, ".gitignore", ".rtdd/graph.json\n")
	writeFile(t, home, ".codex/AGENTS.md", agents)

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, path := range []string{"AGENTS.md", ".gitignore", "~/.codex/AGENTS.md"} {
		found := false
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "legacy ") && strings.Contains(line, " "+path+"  — ") && strings.Contains(line, "rtdd uninstall --legacy-files") {
				found = true
			}
		}
		if !found {
			t.Errorf("init did not report %s as legacy:\n%s", path, stdout)
		}
	}
	if got := readRepoFileForTest(t, dir, "AGENTS.md"); got != agents {
		t.Errorf("init changed AGENTS.md:\n%s", got)
	}
	if got := readRepoFileForTest(t, dir, ".gitignore"); got != ".rtdd/graph.json\n" {
		t.Errorf("init changed .gitignore: %q", got)
	}
	if got := readRepoFileForTest(t, home, ".codex/AGENTS.md"); got != agents {
		t.Errorf("init changed ~/.codex/AGENTS.md:\n%s", got)
	}
}

func TestInitDryRunPrintsThePlanAndWritesNothing(t *testing.T) {
	dir := newTestRepo(t)
	markDetectable(t, dir)

	code, stdout, stderr := rtdd(t, dir, "init", "--dry-run")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, ".rtdd/config.yaml") {
		t.Errorf("dry-run output is missing the plan:\n%s", stdout)
	}
	for _, rel := range []string{".rtdd/config.yaml", ".rtdd/.gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil {
			t.Errorf("--dry-run must not write %s", rel)
		}
	}
}

// With --project-skill, a whole-file target rtdd did not write is a conflict: init writes
// nothing and exits 2, and no flag overrides it (PRD #411 AC5 removes --force).
func TestInitConflictsOnASkillFileRtddDidNotWrite(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, ".claude/skills/rtdd/SKILL.md", "hand written, not ours\n")

	code, _, stderr := rtdd(t, dir, "init", "--project-skill")
	if code != 2 || !strings.Contains(stderr, "move the named file(s) aside") {
		t.Fatalf("exit code = %d, stderr %q; want 2 telling the user to move the file aside", code, stderr)
	}
	if got := readRepoFileForTest(t, dir, ".claude/skills/rtdd/SKILL.md"); got != "hand written, not ours\n" {
		t.Errorf("init overwrote a file rtdd did not write: %q", got)
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
// directory. Every other command resolves the root from git; init must agree.
func TestInitInstallsAtTheRepoRootFromASubdirectory(t *testing.T) {
	dir := newTestRepo(t)
	markDetectable(t, dir)
	deep := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := rtdd(t, deep, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}

	for _, rel := range []string{".rtdd/config.yaml", ".rtdd/.gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not installed at the repo root: %v", rel, err)
		}
		if _, err := os.Stat(filepath.Join(deep, filepath.FromSlash(rel))); err == nil {
			t.Errorf("%s was installed into the working directory sub/deep, not the repo root", rel)
		}
	}
}

// Outside a git repository there is no root to find, and `rtdd init` must still work:
// installing before `git init` is a legitimate order. It falls back to the working
// directory.
func TestInitFallsBackToTheWorkingDirectoryOutsideAGitRepository(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"demo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, rel := range []string{".rtdd/config.yaml", ".rtdd/.gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not installed into the working directory: %v", rel, err)
		}
		if !strings.Contains(stdout, rel) {
			t.Errorf("init output is missing %q:\n%s", rel, stdout)
		}
	}
}

// Issue #369: a stray `.git` — an empty directory left above the working tree by an
// unrelated process — used to be accepted as the repository root, so `rtdd init`
// installed into that ancestor instead of the tree the user was standing in. git
// rejects such a marker; init must fall back to the working directory.
func TestInitIgnoresAStrayGitAncestorAndInstallsIntoTheWorkingDirectory(t *testing.T) {
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir stray .git: %v", err)
	}
	dir := filepath.Join(outer, "project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"demo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, rel := range []string{".rtdd/config.yaml", ".rtdd/.gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not installed into the working directory: %v", rel, err)
		}
		if _, err := os.Stat(filepath.Join(outer, filepath.FromSlash(rel))); err == nil {
			t.Errorf("%s was installed into the stray-.git ancestor, not the working directory", rel)
		}
	}
}
