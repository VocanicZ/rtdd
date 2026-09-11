package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/install"
)

// initialisedRepo runs the real `rtdd init` so uninstall is tested against what install
// actually writes, rather than against a fixture that could drift from it.
func initialisedRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	gittest.Write(t, dir, "tests/test_a.py", "def test_a():\n    pass\n")
	gittest.Write(t, dir, "AGENTS.md", "# House rules\n\nWritten by the project.\n")
	if code, _, stderr := rtdd(t, dir, "init"); code != 0 {
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
