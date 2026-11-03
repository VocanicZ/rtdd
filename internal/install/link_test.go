package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

// #566: where an agent loads a SKILL.md through a link, its install is a symbolic link to
// the one canonical copy rtdd owns, so no agent's copy goes stale on its own.

func readHome(t *testing.T, home, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func isSymlink(t *testing.T, home, rel string) bool {
	t.Helper()
	fi, err := os.Lstat(filepath.Join(home, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("lstat %s: %v", rel, err)
	}
	return fi.Mode()&os.ModeSymlink != 0
}

func findStep(steps []Step, path string) (Step, bool) {
	for _, s := range steps {
		if s.Path == path {
			return s, true
		}
	}
	return Step{}, false
}

func installGlobal(t *testing.T, home string, force bool) []Step {
	t.Helper()
	steps, err := PlanGlobal(home, globalFiles(), force)
	if err != nil {
		t.Fatalf("PlanGlobal: %v", err)
	}
	if err := Apply(home, steps); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return steps
}

const hermesSkill = ".hermes/skills/rtdd/SKILL.md"

func TestPlanGlobalLinksHermesToTheCanonicalSkill(t *testing.T) {
	home := homeWith(t, ".hermes")
	steps := installGlobal(t, home, false)

	if steps[0].Path != CanonicalSkill || steps[0].Action != Create {
		t.Errorf("first step = %+v, want the canonical copy created before any link to it", steps[0])
	}
	if !isSymlink(t, home, hermesSkill) {
		t.Fatalf("%s is not a symbolic link", hermesSkill)
	}
	dest, err := os.Readlink(filepath.Join(home, filepath.FromSlash(hermesSkill)))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, filepath.FromSlash(CanonicalSkill)); dest != want {
		t.Errorf("%s links to %s, want %s", hermesSkill, dest, want)
	}
	want := globalFiles()[protocol.GlobalSkillPath]
	if got := readHome(t, home, hermesSkill); got != want {
		t.Errorf("%s resolves to %q, want the canonical skill %q", hermesSkill, got, want)
	}

	// A second install finds the link in place and writes nothing.
	again, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := findStep(again, hermesSkill); s.Action != Skip {
		t.Errorf("re-install plans %+v for a link already in place, want skip", s)
	}
}

// An agent not verified to follow links keeps a copy, and no canonical copy is written for
// a machine where nothing would link to it.
func TestPlanGlobalWritesNoCanonicalCopyWhenNothingLinks(t *testing.T) {
	home := homeWith(t, ".claude")
	steps := installGlobal(t, home, false)
	if _, ok := findStep(steps, CanonicalSkill); ok {
		t.Errorf("planned the canonical copy with no linking agent present: %+v", steps)
	}
	if isSymlink(t, home, ".claude/skills/rtdd/SKILL.md") {
		t.Error("Claude Code's skill is a link; it is not verified to follow one")
	}
}

// A refresh rewrites one file, and every linked agent reads the new text.
func TestRefreshingTheCanonicalCopyRefreshesEveryLinkedAgent(t *testing.T) {
	home := homeWith(t, ".hermes", ".openclaw")
	installGlobal(t, home, false)
	files := globalFiles()
	files[protocol.GlobalSkillPath] = protocol.Generated + "\nA NEWER RELEASE\n"
	steps, err := PlanGlobal(home, files, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(home, steps); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{hermesSkill, ".openclaw/skills/rtdd/SKILL.md"} {
		if got := readHome(t, home, rel); !strings.Contains(got, "A NEWER RELEASE") {
			t.Errorf("%s reads %q after the refresh", rel, got)
		}
	}
}

// v0.4.0 wrote a copy; the next install replaces rtdd's own copy with the link.
func TestPlanGlobalReplacesAnEarlierCopyWithALink(t *testing.T) {
	home := homeWith(t, ".hermes")
	writeHome(t, home, hermesSkill, protocol.Generated+"\nan earlier release's copy\n")
	installGlobal(t, home, false)
	if !isSymlink(t, home, hermesSkill) {
		t.Errorf("%s is still a copy", hermesSkill)
	}
}

// A hand-written file at a linked path is never overwritten without --force, and the
// refused install writes no link.
func TestPlanGlobalLeavesAHandWrittenFileAtALinkedPathAlone(t *testing.T) {
	home := homeWith(t, ".hermes")
	writeHome(t, home, hermesSkill, "my own rtdd notes\n")
	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := findStep(steps, hermesSkill); s.Action != Conflict {
		t.Fatalf("plan for a hand-written %s = %+v, want conflict", hermesSkill, s)
	}
	installGlobal(t, home, true)
	if !isSymlink(t, home, hermesSkill) {
		t.Errorf("--force did not replace the hand-written file with the link")
	}
}

// A link the user made to somewhere else is theirs.
func TestPlanGlobalLeavesAForeignLinkAlone(t *testing.T) {
	home := homeWith(t, ".hermes/skills/rtdd")
	mine := filepath.Join(home, "mine.md")
	if err := os.WriteFile(mine, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(mine, filepath.Join(home, filepath.FromSlash(hermesSkill))); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	steps, err := PlanGlobal(home, globalFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := findStep(steps, hermesSkill); s.Action != Conflict {
		t.Errorf("plan for a link to the user's own file = %+v, want conflict", s)
	}
	uninst, err := PlanGlobalUninstall(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyUninstall(home, uninst); err != nil {
		t.Fatal(err)
	}
	if got := readHome(t, home, hermesSkill); got != "mine\n" {
		t.Errorf("uninstall removed or changed the user's own link: %q", got)
	}
}

// A canonical copy rtdd did not write is not linked to: agents get copies instead.
func TestPlanGlobalCopiesWhenTheCanonicalPathIsNotRtdds(t *testing.T) {
	home := homeWith(t, ".hermes")
	writeHome(t, home, CanonicalSkill, "not rtdd's\n")
	steps := installGlobal(t, home, false)
	if s, _ := findStep(steps, CanonicalSkill); s.Action != Skip {
		t.Errorf("canonical step = %+v, want skip", s)
	}
	if isSymlink(t, home, hermesSkill) {
		t.Error("linked to a canonical copy rtdd did not write")
	}
	if got := readHome(t, home, CanonicalSkill); got != "not rtdd's\n" {
		t.Errorf("the canonical path was overwritten: %q", got)
	}
}

// Uninstall removes the links and the canonical copy, and leaves no directory of rtdd's.
func TestPlanGlobalUninstallRemovesLinksAndTheCanonicalCopy(t *testing.T) {
	home := homeWith(t, ".hermes", ".config/opencode")
	installGlobal(t, home, false)
	steps, err := PlanGlobalUninstall(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyUninstall(home, steps); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".hermes/skills/rtdd", ".config/opencode/skills/rtdd", canonicalRoot} {
		if _, err := os.Lstat(filepath.Join(home, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("uninstall left %s (err %v)", rel, err)
		}
	}
	for _, rel := range []string{".hermes", ".config/opencode"} {
		if _, err := os.Stat(filepath.Join(home, rel)); err != nil {
			t.Errorf("uninstall removed the agent's own %s: %v", rel, err)
		}
	}
}

func writeHome(t *testing.T, home, rel, body string) {
	t.Helper()
	abs := filepath.Join(home, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
