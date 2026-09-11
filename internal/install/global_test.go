package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

// globalFiles is what install.Files() returns, reduced to the two keys PlanGlobal reads.
func globalFiles() map[string]string {
	return map[string]string{
		protocol.GlobalSkillPath: "GLOBAL SKILL BODY\n",
		protocol.GlobalAgentsPath: protocol.BeginMarker + "\n## rtdd\n\nglobal block\n" +
			protocol.EndMarker + "\n",
	}
}

// homeWith builds a fake $HOME holding exactly the named agent directories.
func homeWith(t *testing.T, dirs ...string) string {
	t.Helper()
	home := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", d, err)
		}
	}
	return home
}

// Installing rtdd must not create config directories for agents the user does not have.
// A ~/.codex conjured up by a test-selector's installer is litter in someone's home
// directory, and it makes `ls ~` lie about which tools are installed.
func TestPlanGlobalSkipsAgentsThatAreNotInstalled(t *testing.T) {
	home := homeWith(t) // no agent directories at all
	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatalf("PlanGlobal: %v", err)
	}
	if len(steps) == 0 {
		t.Fatal("PlanGlobal produced no steps; it must report every surface it considered")
	}
	for _, s := range steps {
		if s.Action != Skip {
			t.Errorf("%s: action = %v, want skip in a home with no agent installed", s.Path, s.Action)
		}
	}
	if err := Apply(home, steps); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, dir := range []string{".claude", ".codex", ".gemini"} {
		if _, err := os.Stat(filepath.Join(home, dir)); !os.IsNotExist(err) {
			t.Errorf("Apply created %s for an agent that is not installed", dir)
		}
	}
}

// The skill lands at the path Claude Code actually reads.
func TestPlanGlobalWritesTheClaudeSkillWhenClaudeIsInstalled(t *testing.T) {
	home := homeWith(t, ".claude")
	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatalf("PlanGlobal: %v", err)
	}
	const rel = ".claude/skills/rtdd/SKILL.md"
	s := stepFor(t, steps, rel)
	if s.Action != Create {
		t.Fatalf("%s: action = %v, want create", rel, s.Action)
	}
	if err := Apply(home, steps); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(got) != globalFiles()[protocol.GlobalSkillPath] {
		t.Errorf("installed skill = %q, want the rendered global skill", got)
	}
}

// A global AGENTS.md is the user's file. rtdd owns what is between its markers and must
// leave every other byte alone — the same contract the repo-scoped merge already honours.
func TestPlanGlobalMergesIntoAnExistingCodexAgentsFile(t *testing.T) {
	home := homeWith(t, ".codex")
	existing := "# My instructions\n\nAlways use tabs.\n"
	path := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}

	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatalf("PlanGlobal: %v", err)
	}
	if err := Apply(home, steps); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read merged AGENTS.md: %v", err)
	}
	if !strings.Contains(string(got), "Always use tabs.") {
		t.Error("merge dropped the user's own instructions")
	}
	if !strings.Contains(string(got), protocol.BeginMarker) {
		t.Error("merge did not add the rtdd block")
	}
}

// A skill the user has edited by hand is not rtdd's to overwrite silently.
func TestPlanGlobalConflictsOnAnEditedSkillAndForceOverwrites(t *testing.T) {
	home := homeWith(t, ".claude")
	rel := filepath.FromSlash(".claude/skills/rtdd/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(home, rel)), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, rel), []byte("hand-edited\n"), 0o644); err != nil {
		t.Fatalf("seed skill: %v", err)
	}

	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatalf("PlanGlobal: %v", err)
	}
	s := stepFor(t, steps, ".claude/skills/rtdd/SKILL.md")
	if s.Action != Conflict {
		t.Fatalf("action = %v, want conflict for a hand-edited skill", s.Action)
	}

	forced, err := PlanGlobal(home, globalFiles(), true)
	if err != nil {
		t.Fatalf("PlanGlobal(force): %v", err)
	}
	s = stepFor(t, forced, ".claude/skills/rtdd/SKILL.md")
	if s.Action != Create {
		t.Errorf("action = %v with --force, want create", s.Action)
	}
}

// An install followed by an uninstall leaves the home directory as it was found.
func TestPlanGlobalUninstallRestoresTheHostsFiles(t *testing.T) {
	home := homeWith(t, ".claude", ".codex")
	existing := "# My instructions\n\nAlways use tabs.\n"
	codex := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.WriteFile(codex, []byte(existing), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}

	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatalf("PlanGlobal: %v", err)
	}
	if err := Apply(home, steps); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	undo, err := PlanGlobalUninstall(home)
	if err != nil {
		t.Fatalf("PlanGlobalUninstall: %v", err)
	}
	if err := ApplyUninstall(home, undo); err != nil {
		t.Fatalf("ApplyUninstall: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(".claude/skills/rtdd/SKILL.md"))); !os.IsNotExist(err) {
		t.Error("uninstall left the global skill behind")
	}
	got, err := os.ReadFile(codex)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(got) != existing {
		t.Errorf("AGENTS.md after uninstall = %q, want the original %q", got, existing)
	}
}

// `rtdd update` re-runs the machine-wide install after every upgrade, and the new binary
// renders a different skill than the old one did. If rtdd treated its OWN previous output
// as a conflict, the normal upgrade path would fail on every machine and leave everyone
// pinned to the skill their first install happened to write. A file carrying the generated
// marker is rtdd's to replace; a file without it has been hand-written and is not.
func TestPlanGlobalReplacesItsOwnPreviousSkillWithoutForce(t *testing.T) {
	home := homeWith(t, ".claude")
	rel := filepath.FromSlash(".claude/skills/rtdd/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(home, rel)), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	stale := "---\nname: rtdd\n---\n\n" + protocol.Generated + "\n\n# rtdd\n\nan older release's skill\n"
	if err := os.WriteFile(filepath.Join(home, rel), []byte(stale), 0o644); err != nil {
		t.Fatalf("seed skill: %v", err)
	}

	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatalf("PlanGlobal: %v", err)
	}
	s := stepFor(t, steps, ".claude/skills/rtdd/SKILL.md")
	if s.Action != Create {
		t.Fatalf("action = %v, want create: rtdd must replace a skill it generated itself", s.Action)
	}
	if err := Apply(home, steps); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(home, rel))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != globalFiles()[protocol.GlobalSkillPath] {
		t.Errorf("skill was not refreshed: got %q", got)
	}
}
