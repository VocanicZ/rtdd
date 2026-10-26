package install

import (
	"encoding/json"
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

// #509: ~/.codex/AGENTS.md and ~/.gemini/GEMINI.md are the user's shared memory files.
// An install writes only into rtdd's own directories and leaves both byte-identical.
func TestPlanGlobalWritesOnlyRtddOwnedDirectories(t *testing.T) {
	home := homeWith(t, ".claude", ".codex/skills/.system", ".gemini")
	shared := map[string]string{
		".codex/AGENTS.md":  "# My instructions\n\nAlways use tabs.\n",
		".gemini/GEMINI.md": "# Gemini notes\n",
	}
	for rel, body := range shared {
		if err := os.WriteFile(filepath.Join(home, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := map[string]bool{}
	for _, f := range repoFiles(t, home) {
		before[f] = true
	}

	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatalf("PlanGlobal: %v", err)
	}
	if err := Apply(home, steps); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for rel, body := range shared {
		got, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != body {
			t.Errorf("~/%s = %q after install, want it byte-identical %q", rel, got, body)
		}
	}
	owned := []string{".claude/skills/rtdd/", ".codex/skills/rtdd/", ".gemini/extensions/rtdd/"}
	var created []string
	for _, f := range repoFiles(t, home) {
		if before[f] {
			continue
		}
		created = append(created, f)
		ok := false
		for _, dir := range owned {
			ok = ok || strings.HasPrefix(f, dir)
		}
		if !ok {
			t.Errorf("install created ~/%s, outside rtdd's own directories %v", f, owned)
		}
	}
	want := []string{
		".claude/skills/rtdd/SKILL.md",
		".codex/skills/rtdd/SKILL.md",
		".gemini/extensions/rtdd/GEMINI.md",
		".gemini/extensions/rtdd/gemini-extension.json",
	}
	if strings.Join(created, ",") != strings.Join(want, ",") {
		t.Errorf("install created %v, want %v", created, want)
	}
}

// A Codex without a skills directory reads no skills; rtdd skips it, says why, and never
// falls back to the shared ~/.codex/AGENTS.md.
func TestPlanGlobalSkipsACodexWithNoSkillsDirectory(t *testing.T) {
	home := homeWith(t, ".codex")
	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	s := stepFor(t, steps, ".codex/skills/rtdd/")
	if s.Action != Skip || !strings.Contains(s.Note, "~/.codex/skills/") || !strings.Contains(s.Note, "never writes ~/.codex/AGENTS.md") {
		t.Errorf("codex step = %v %q, want a skip naming the missing skills directory", s.Action, s.Note)
	}
	if err := Apply(home, steps); err != nil {
		t.Fatal(err)
	}
	if got := repoFiles(t, home); len(got) != 0 {
		t.Errorf("install wrote %v into a home whose Codex reads no skills", got)
	}
}

// The Gemini CLI surface is an extension: a manifest naming its own context file, which
// carries the short machine-wide block.
func TestPlanGlobalWritesAGeminiExtension(t *testing.T) {
	home := homeWith(t, ".gemini")
	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(home, steps); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(home, ".gemini", "extensions", "rtdd", "gemini-extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct{ Name, Version, ContextFileName string }
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatalf("gemini-extension.json is not JSON: %v\n%s", err, b)
	}
	if manifest.Name != "rtdd" || manifest.Version == "" || manifest.ContextFileName != "GEMINI.md" {
		t.Errorf("manifest = %+v, want name rtdd, a version and contextFileName GEMINI.md", manifest)
	}
	ctx, err := os.ReadFile(filepath.Join(home, ".gemini", "extensions", "rtdd", manifest.ContextFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(ctx) != globalFiles()[protocol.GlobalAgentsPath] {
		t.Errorf("context file = %q, want the machine-wide block", ctx)
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

// An install followed by an uninstall leaves the home directory as it was found: rtdd's
// own directories go whole, and the user's shared files were never touched.
func TestPlanGlobalUninstallRestoresTheHostsFiles(t *testing.T) {
	home := homeWith(t, ".claude", ".codex/skills", ".gemini")
	existing := "# My instructions\n\nAlways use tabs.\n"
	codex := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.WriteFile(codex, []byte(existing), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}
	before := repoFiles(t, home)

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

	for _, dir := range []string{".claude/skills/rtdd", ".codex/skills/rtdd", ".gemini/extensions/rtdd"} {
		if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(dir))); !os.IsNotExist(err) {
			t.Errorf("uninstall left ~/%s behind", dir)
		}
	}
	if after := repoFiles(t, home); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Errorf("home after uninstall holds %v, want %v", after, before)
	}
	got, err := os.ReadFile(codex)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(got) != existing {
		t.Errorf("AGENTS.md after uninstall = %q, want the original %q", got, existing)
	}
}

// A file the user put in rtdd's directory is not rtdd's: only rtdd's files go, and the
// directory stays.
func TestPlanGlobalUninstallKeepsADirectoryHoldingAForeignFile(t *testing.T) {
	home := homeWith(t, ".claude/skills/rtdd")
	writeAt(t, home, ".claude/skills/rtdd/SKILL.md", "skill\n")
	writeAt(t, home, ".claude/skills/rtdd/notes.md", "mine\n")
	undo, err := PlanGlobalUninstall(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyUninstall(home, undo); err != nil {
		t.Fatal(err)
	}
	if got := repoFiles(t, home); strings.Join(got, ",") != ".claude/skills/rtdd/notes.md" {
		t.Errorf("home after uninstall holds %v, want only the user's notes.md", got)
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
