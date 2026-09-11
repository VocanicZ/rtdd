package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

func testBlock() string {
	return protocol.BeginMarker + "\n## rtdd\n\nwhat the tool does.\n\n" + protocol.EndMarker + "\n"
}

// TestMergeThenRemoveReturnsTheFileUnchanged is the property that matters: AGENTS.md and
// CLAUDE.md belong to the host project, so installing and then uninstalling must give the
// file back byte for byte. Anything less means `rtdd uninstall` edits someone else's
// document, which is the thing the markers exist to prevent.
func TestMergeThenRemoveReturnsTheFileUnchanged(t *testing.T) {
	for name, original := range map[string]string{
		"empty file":            "",
		"prose only":            "# Project\n\nSome prose.\n",
		"prose with no newline": "# Project\n\nSome prose.",
		"several sections":      "# Project\n\n## One\n\ntext\n\n## Two\n\nmore text\n",
	} {
		merged, _, err := MergeBlock(original, testBlock())
		if err != nil {
			t.Fatalf("%s: MergeBlock: %v", name, err)
		}
		if !strings.Contains(merged, protocol.BeginMarker) {
			t.Fatalf("%s: MergeBlock produced no block to remove", name)
		}
		got, action, err := RemoveBlock(merged)
		if err != nil {
			t.Fatalf("%s: RemoveBlock: %v", name, err)
		}
		if action != StripBlock {
			t.Errorf("%s: action = %v, want StripBlock", name, action)
		}
		want := original
		if want != "" && !strings.HasSuffix(want, "\n") {
			want += "\n" // MergeBlock normalises the trailing newline on the way in
		}
		if got != want {
			t.Errorf("%s: round trip returned %q, want %q", name, got, want)
		}
	}
}

// TestRemoveBlockLeavesAFileWithoutMarkersAlone - uninstalling twice, or uninstalling from
// a repo where someone deleted the block by hand, must not be an error and must not edit.
func TestRemoveBlockLeavesAFileWithoutMarkersAlone(t *testing.T) {
	original := "# Project\n\nNothing of rtdd's in here.\n"
	got, action, err := RemoveBlock(original)
	if err != nil {
		t.Fatalf("RemoveBlock: %v", err)
	}
	if action != Skip {
		t.Errorf("action = %v, want Skip", action)
	}
	if got != original {
		t.Errorf("RemoveBlock edited a file that carried no block: %q", got)
	}
}

// TestRemoveBlockRefusesAnAmbiguousFile mirrors MergeBlock: anything that cannot be read
// unambiguously is an error rather than a guess, because guessing destroys someone's file.
func TestRemoveBlockRefusesAnAmbiguousFile(t *testing.T) {
	for name, body := range map[string]string{
		"two blocks":       testBlock() + "\nprose\n\n" + testBlock(),
		"begin only":       protocol.BeginMarker + "\nhalf a block\n",
		"end only":         "prose\n" + protocol.EndMarker + "\n",
		"end before begin": protocol.EndMarker + "\nprose\n" + protocol.BeginMarker + "\n",
	} {
		if _, action, err := RemoveBlock(body); err == nil {
			t.Errorf("%s: RemoveBlock returned action %v and no error; an ambiguous file must refuse", name, action)
		}
	}
}

// --- PlanUninstall -------------------------------------------------------------------

// installedRepo lays out a repository as `rtdd init` leaves it, plus a host AGENTS.md that
// had content of its own before rtdd ever ran.
func installedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write(".claude/skills/rtdd/SKILL.md", "# rtdd\n\nskill body\n")
	write(".cursor/rules/rtdd.mdc", "cursor rule body\n")
	merged, _, err := MergeBlock("# House rules\n\nWritten by the project.\n", testBlock())
	if err != nil {
		t.Fatalf("MergeBlock: %v", err)
	}
	write("AGENTS.md", merged)
	write(".gitattributes", "*.py text\n"+gitattributesLine+"\n")
	write(".rtdd/config.yaml", "stale_commits: 50\n")
	write(".rtdd/map.jsonl", `{"t":"tests/test_a.py::test_a"}`+"\n")
	return root
}

// stepFor lives in install_test.go — uninstall asserts on the same Step list.

// TestPlanUninstallRemovesExactlyWhatPlanWrites is the symmetry claim: uninstall covers
// the front-ends, the marker block and the .gitattributes line, which is the whole of what
// Plan writes outside .rtdd/.
func TestPlanUninstallRemovesExactlyWhatPlanWrites(t *testing.T) {
	root := installedRepo(t)
	steps, err := PlanUninstall(root, UninstallOptions{})
	if err != nil {
		t.Fatalf("PlanUninstall: %v", err)
	}
	for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc"} {
		if got := stepFor(t, steps, rel).Action; got != Delete {
			t.Errorf("%s: action = %v, want Delete", rel, got)
		}
	}
	if got := stepFor(t, steps, "AGENTS.md").Action; got != StripBlock {
		t.Errorf("AGENTS.md: action = %v, want StripBlock — the host's own content stays", got)
	}
	if got := stepFor(t, steps, ".gitattributes").Action; got != StripBlock {
		t.Errorf(".gitattributes: action = %v, want StripBlock", got)
	}
}

// TestPlanUninstallKeepsTheRecordedMapUnlessAsked - the map is the expensive thing to
// rebuild, and removing the agent front-ends is not a request to throw away a seed run.
func TestPlanUninstallKeepsTheRecordedMapUnlessAsked(t *testing.T) {
	root := installedRepo(t)

	steps, err := PlanUninstall(root, UninstallOptions{})
	if err != nil {
		t.Fatalf("PlanUninstall: %v", err)
	}
	if got := stepFor(t, steps, ".rtdd/").Action; got != Skip {
		t.Errorf(".rtdd/: action = %v, want Skip by default", got)
	}

	steps, err = PlanUninstall(root, UninstallOptions{State: true})
	if err != nil {
		t.Fatalf("PlanUninstall --state: %v", err)
	}
	if got := stepFor(t, steps, ".rtdd/").Action; got != Delete {
		t.Errorf(".rtdd/ with State: action = %v, want Delete", got)
	}
}

// TestApplyUninstallLeavesTheHostsOwnContent is the end-to-end version of the property
// RemoveBlock is tested for: after uninstalling, AGENTS.md is exactly what the project
// wrote and .gitattributes keeps every line that was not rtdd's.
func TestApplyUninstallLeavesTheHostsOwnContent(t *testing.T) {
	root := installedRepo(t)
	steps, err := PlanUninstall(root, UninstallOptions{})
	if err != nil {
		t.Fatalf("PlanUninstall: %v", err)
	}
	if err := ApplyUninstall(root, steps); err != nil {
		t.Fatalf("ApplyUninstall: %v", err)
	}

	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(agents) != "# House rules\n\nWritten by the project.\n" {
		t.Errorf("AGENTS.md = %q, want the project's own content back", agents)
	}
	ga, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		t.Fatalf("read .gitattributes: %v", err)
	}
	if !strings.Contains(string(ga), "*.py text") {
		t.Errorf(".gitattributes = %q, want the host's own rules kept", ga)
	}
	if strings.Contains(string(ga), gitattributesLine) {
		t.Errorf(".gitattributes still carries rtdd's line: %q", ga)
	}
	for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s survived the uninstall", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".rtdd", "map.jsonl")); err != nil {
		t.Errorf("the recorded map was removed without --state: %v", err)
	}
}

// TestUninstallingTwiceIsNotAnError - the second run finds nothing to do and says so.
func TestUninstallingTwiceIsNotAnError(t *testing.T) {
	root := installedRepo(t)
	steps, err := PlanUninstall(root, UninstallOptions{})
	if err != nil {
		t.Fatalf("first PlanUninstall: %v", err)
	}
	if err := ApplyUninstall(root, steps); err != nil {
		t.Fatalf("first ApplyUninstall: %v", err)
	}

	steps, err = PlanUninstall(root, UninstallOptions{})
	if err != nil {
		t.Fatalf("second PlanUninstall: %v", err)
	}
	if err := ApplyUninstall(root, steps); err != nil {
		t.Fatalf("second ApplyUninstall: %v", err)
	}
	for _, s := range steps {
		if s.Action != Skip {
			t.Errorf("a second uninstall still wants to %v %s", s.Action, s.Path)
		}
	}
}

// TestPlanUninstallRefusesAnAmbiguousAgentsFile - a file rtdd cannot read unambiguously is
// reported as a conflict and left alone, exactly as install does.
func TestPlanUninstallRefusesAnAmbiguousAgentsFile(t *testing.T) {
	root := installedRepo(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(testBlock()+"\nprose\n\n"+testBlock()), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	steps, err := PlanUninstall(root, UninstallOptions{})
	if err != nil {
		t.Fatalf("PlanUninstall: %v", err)
	}
	if got := stepFor(t, steps, "AGENTS.md").Action; got != Conflict {
		t.Errorf("AGENTS.md: action = %v, want Conflict", got)
	}
	if err := ApplyUninstall(root, steps); err == nil {
		t.Error("ApplyUninstall proceeded through a conflict instead of stopping")
	}
}
