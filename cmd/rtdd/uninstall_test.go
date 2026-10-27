package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
)

// initialisedRepo runs the real `rtdd init`, with every opt-in, so uninstall is tested against what install
// actually writes, rather than against a fixture that could drift from it.
func initialisedRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	gittest.Write(t, dir, "tests/test_a.py", "def test_a():\n    pass\n")
	gittest.Write(t, dir, "AGENTS.md", "# House rules\n\nWritten by the project.\n")
	if code, _, stderr := rtdd(t, dir, "init", "--project-skill", "--append-to", "AGENTS.md"); code != 0 {
		t.Fatalf("rtdd init: exit %d (stderr: %s)", code, stderr)
	}
	return dir
}

// TestUninstallRemovesWhatInitWrote is the round trip through the real commands.
func TestUninstallRemovesWhatInitWrote(t *testing.T) {
	dir := initialisedRepo(t)
	for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("init did not write %s: %v", rel, err)
		}
	}

	code, stdout, stderr := rtdd(t, dir, "uninstall")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, ".claude/skills/rtdd/SKILL.md") {
		t.Errorf("stdout does not report what it removed: %q", stdout)
	}
	for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s survived uninstall", rel)
		}
	}
	agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(agents) != "# House rules\n\nWritten by the project.\n" {
		t.Errorf("AGENTS.md = %q, want the project's own content back", agents)
	}
}

// TestUninstallKeepsTheMapAndTheBinaryByDefault - the two things a user has to ask for.
func TestUninstallKeepsTheMapAndTheBinaryByDefault(t *testing.T) {
	dir := initialisedRepo(t)
	if code, _, stderr := rtdd(t, dir, "uninstall"); code != 0 {
		t.Fatalf("exit code (stderr: %s)", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".rtdd", "config.yaml")); err != nil {
		t.Errorf(".rtdd/config.yaml was removed without --state: %v", err)
	}
}

func TestUninstallStateRemovesTheRecordedMap(t *testing.T) {
	dir := initialisedRepo(t)
	if code, _, stderr := rtdd(t, dir, "uninstall", "--state"); code != 0 {
		t.Fatalf("exit code (stderr: %s)", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".rtdd")); !os.IsNotExist(err) {
		t.Errorf(".rtdd/ survived --state (stat err: %v)", err)
	}
}

// TestUninstallDryRunChangesNothing - the plan is a promise, so it must not be one the
// command has already carried out.
func TestUninstallDryRunChangesNothing(t *testing.T) {
	dir := initialisedRepo(t)
	code, stdout, stderr := rtdd(t, dir, "uninstall", "--dry-run")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "SKILL.md") {
		t.Errorf("--dry-run printed no plan: %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude/skills/rtdd/SKILL.md")); err != nil {
		t.Errorf("--dry-run removed the skill file: %v", err)
	}
}

// TestUninstallIsIdempotent - running it twice reports nothing left to do and exits 0.
func TestUninstallIsIdempotent(t *testing.T) {
	dir := initialisedRepo(t)
	if code, _, stderr := rtdd(t, dir, "uninstall"); code != 0 {
		t.Fatalf("first run (stderr: %s)", stderr)
	}
	code, stdout, stderr := rtdd(t, dir, "uninstall")
	if code != 0 {
		t.Fatalf("second run: exit %d (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "delete ") {
		t.Errorf("a second uninstall still wants to delete something: %q", stdout)
	}
}

// TestUninstallStopsOnAFileItCannotRead - an ambiguous AGENTS.md is a conflict, and the
// command exits 2 without editing anything.
func TestUninstallStopsOnAFileItCannotRead(t *testing.T) {
	dir := initialisedRepo(t)
	agentsPath := filepath.Join(dir, "AGENTS.md")
	original, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	doubled := string(original) + "\n" + string(original)
	if err := os.WriteFile(agentsPath, []byte(doubled), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	code, _, stderr := rtdd(t, dir, "uninstall")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "AGENTS.md") {
		t.Errorf("stderr does not name the file it refused to edit: %q", stderr)
	}
	got, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(got) != doubled {
		t.Errorf("a refused uninstall still edited AGENTS.md")
	}
}

func TestRenderUninstallListsEveryStep(t *testing.T) {
	out := RenderUninstall([]install.Step{
		{Path: ".cursor/rules/rtdd.mdc", Action: install.Delete},
		{Path: "AGENTS.md", Action: install.StripBlock},
		{Path: ".rtdd/", Action: install.Skip, Note: "kept"},
	})
	for _, want := range []string{"delete", ".cursor/rules/rtdd.mdc", "strip-block", "AGENTS.md", "skip", "kept"} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderUninstall output %q does not carry %q", out, want)
		}
	}
}

func TestUsageListsUninstall(t *testing.T) {
	if !strings.Contains(usage, "rtdd uninstall") {
		t.Error("the usage block does not list `rtdd uninstall`")
	}
}

// legacyRepo is a repository and home directory as an rtdd init and skill install before
// #509 left them: front-ends outside .rtdd/, a block between the user's own text, a root
// .gitignore line, and a block in the shared ~/.codex/AGENTS.md.
func legacyRepo(t *testing.T) (dir, home string) {
	t.Helper()
	dir = gittest.Init(t)
	home = t.TempDir()
	t.Setenv("RTDD_HOME", home)
	old := protocol.BeginMarker + "\n## rtdd\n\nan older init's text\n" + protocol.EndMarker + "\n"
	gittest.Write(t, dir, "AGENTS.md", "# House rules\n\nWritten by the project.\n\n"+old+"\n## Later\n\nMore of ours.\n")
	gittest.Write(t, dir, ".claude/skills/rtdd/SKILL.md", "---\nname: rtdd\n---\n\n"+protocol.Generated+"\n\nold skill\n")
	gittest.Write(t, dir, ".cursor/rules/rtdd.mdc", protocol.Generated+"\n\nold rule\n")
	gittest.Write(t, dir, ".gitignore", "node_modules/\n.rtdd/graph.json\n")
	gittest.Write(t, dir, ".rtdd/config.yaml", "max_stale_ratio: 0.3\n")
	writeFile(t, home, ".codex/AGENTS.md", "# Mine\n\n"+old)
	return dir, home
}

// #509 acceptance: --legacy-files removes exactly the old footprint — by markers, so the
// user's text around the old block is byte-identical — and prints each change.
func TestUninstallLegacyFilesRemovesExactlyTheOldFootprint(t *testing.T) {
	dir, home := legacyRepo(t)

	code, stdout, stderr := rtdd(t, dir, "uninstall", "--legacy-files")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, line := range []string{
		"delete         .claude/skills/rtdd/",
		"delete         .cursor/rules/rtdd.mdc",
		"strip-block    AGENTS.md",
		"create         .rtdd/.gitignore",
		"strip-block    .gitignore",
		"strip-block    ~/.codex/AGENTS.md",
	} {
		if !strings.Contains(stdout, line) {
			t.Errorf("output does not report %q:\n%s", line, stdout)
		}
	}
	for rel, want := range map[string]string{
		"AGENTS.md":         "# House rules\n\nWritten by the project.\n\n## Later\n\nMore of ours.\n",
		".gitignore":        "node_modules/\n",
		".rtdd/.gitignore":  "graph.json\n",
		".rtdd/config.yaml": "max_stale_ratio: 0.3\n",
	} {
		if got := readRepoFileForTest(t, dir, rel); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	for _, rel := range []string{".claude/skills/rtdd", ".cursor/rules/rtdd.mdc"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s survived --legacy-files", rel)
		}
	}
	if got := readRepoFileForTest(t, home, ".codex/AGENTS.md"); got != "# Mine\n" {
		t.Errorf("~/.codex/AGENTS.md = %q, want the user's text alone", got)
	}

	// Nothing is left: a second run, and init, report nothing legacy.
	_, again, _ := rtdd(t, dir, "uninstall", "--legacy-files")
	if !strings.Contains(again, "no legacy files found") {
		t.Errorf("second --legacy-files run:\n%s", again)
	}
	if _, out, _ := rtdd(t, dir, "init"); strings.Contains(out, "legacy") {
		t.Errorf("init after --legacy-files still reports legacy files:\n%s", out)
	}
}

func TestUninstallLegacyFilesDryRunChangesNothing(t *testing.T) {
	dir, home := legacyRepo(t)
	before := readRepoFileForTest(t, dir, "AGENTS.md")
	code, stdout, stderr := rtdd(t, dir, "uninstall", "--legacy-files", "--dry-run")
	if code != 0 || !strings.Contains(stdout, "strip-block    AGENTS.md") || !strings.Contains(stdout, "dry run") {
		t.Fatalf("exit %d, stdout:\n%s\nstderr: %s", code, stdout, stderr)
	}
	if got := readRepoFileForTest(t, dir, "AGENTS.md"); got != before {
		t.Errorf("--dry-run edited AGENTS.md")
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "AGENTS.md")); err != nil {
		t.Errorf("--dry-run touched ~/.codex/AGENTS.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "rtdd", "SKILL.md")); err != nil {
		t.Errorf("--dry-run deleted the old skill: %v", err)
	}
}

// What the opt-ins wrote is not legacy, and --legacy-files leaves it.
func TestUninstallLegacyFilesLeavesTheOptIns(t *testing.T) {
	dir := initialisedRepo(t)
	agents := readRepoFileForTest(t, dir, "AGENTS.md")
	code, stdout, stderr := rtdd(t, dir, "uninstall", "--legacy-files")
	if code != 0 || !strings.Contains(stdout, "no legacy files found") {
		t.Fatalf("exit %d, stdout:\n%s\nstderr: %s", code, stdout, stderr)
	}
	if got := readRepoFileForTest(t, dir, "AGENTS.md"); got != agents {
		t.Errorf("--legacy-files edited an opted-in AGENTS.md")
	}
}

// A legacy file rtdd cannot read unambiguously stops the run before anything is written.
func TestUninstallLegacyFilesStopsOnAConflictWithNothingChanged(t *testing.T) {
	dir, _ := legacyRepo(t)
	writeFile(t, dir, "CLAUDE.md", protocol.BeginMarker+"\n"+protocol.BeginMarker+"\n")
	code, _, stderr := rtdd(t, dir, "uninstall", "--legacy-files")
	if code != 2 || !strings.Contains(stderr, "conflict") {
		t.Fatalf("exit %d, stderr %q; want 2 reporting the conflict", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "rules", "rtdd.mdc")); err != nil {
		t.Errorf("a refused run still deleted the old rule: %v", err)
	}
}

func TestUninstallLegacyFilesRefusesTheOtherFlags(t *testing.T) {
	dir := newTestRepo(t)
	for _, flag := range []string{"--state", "--binary", "--global"} {
		if code, _, _ := rtdd(t, dir, "uninstall", "--legacy-files", flag); code != 2 {
			t.Errorf("uninstall --legacy-files %s = %d, want 2", flag, code)
		}
	}
}

// homeFiles maps every file under home to its content.
func homeFiles(t *testing.T, home string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(home, p)
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", home, err)
	}
	return out
}

// #553: plain `rtdd uninstall` run from the home directory — no repository there, or the
// home directory is itself a dotfiles repository — must leave the user-level installs
// `rtdd skill install` wrote. Their paths are ProjectSkillFiles paths too, and they carry
// the same marker, so only knowing the root is the home directory tells them apart.
func TestUninstallFromHomeKeepsTheUserLevelInstalls(t *testing.T) {
	for name, seed := range map[string]func(t *testing.T) string{
		"no repository": func(t *testing.T) string { return t.TempDir() },
		"dotfiles repo": func(t *testing.T) string { return gittest.Init(t) },
	} {
		t.Run(name, func(t *testing.T) {
			home := seed(t)
			t.Setenv("RTDD_HOME", home)
			for _, dir := range []string{".claude", ".codex"} {
				if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if code, _, stderr := rtdd(t, home, "skill", "install"); code != 0 {
				t.Fatalf("rtdd skill install: exit %d (stderr: %s)", code, stderr)
			}
			before := homeFiles(t, home)
			for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".agents/skills/rtdd/SKILL.md"} {
				if _, ok := before[rel]; !ok {
					t.Fatalf("skill install did not write ~/%s", rel)
				}
			}

			code, stdout, stderr := rtdd(t, home, "uninstall")
			if code != 0 {
				t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
			}
			if !strings.Contains(stdout, "--global") {
				t.Errorf("output does not say --global removes the user-level installs:\n%s", stdout)
			}
			after := homeFiles(t, home)
			for rel, want := range before {
				if got, ok := after[rel]; !ok {
					t.Errorf("~/%s was deleted by a plain uninstall", rel)
				} else if got != want {
					t.Errorf("~/%s changed by a plain uninstall", rel)
				}
			}
		})
	}
}
