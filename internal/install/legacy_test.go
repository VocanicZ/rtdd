package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

func readAt(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// #509 acceptance: an older init's block between the user's own text goes, and the user's
// text is byte-identical afterwards.
func TestLegacyFilesStripTheOldBlockAndLeaveTheUsersTextByteIdentical(t *testing.T) {
	root := t.TempDir()
	above, below := "# Rules\n\nHouse rules.\n", "## More\n\nTail.\n"
	writeAt(t, root, "AGENTS.md", above+"\n"+block("an older init's text")+"\n"+below)

	steps, err := PlanLegacyFiles(root, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, "AGENTS.md"); s.Action != StripBlock {
		t.Fatalf("AGENTS.md step = %v, want StripBlock", s.Action)
	}
	if err := ApplyUninstall(root, steps); err != nil {
		t.Fatal(err)
	}
	if got := readAt(t, root, "AGENTS.md"); got != above+"\n"+below {
		t.Errorf("AGENTS.md = %q, want the user's text alone %q", got, above+"\n"+below)
	}
}

// What an opt-in wrote is asked for, not legacy; a file rtdd did not write is not rtdd's.
func TestLegacyFilesLeaveOptInsAndForeignFilesAlone(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, "AGENTS.md", "# Rules\n\n"+markOptIn(block("opted in"), protocol.BeginMarker))
	writeAt(t, root, "CLAUDE.md", "# notes, no rtdd here\n")
	writeAt(t, root, ".claude/skills/rtdd/SKILL.md", markOptIn(protocol.Generated+"\n\nskill\n", protocol.Generated))
	writeAt(t, root, ".cursor/rules/rtdd.mdc", "a rule the host wrote itself\n")
	writeAt(t, root, ".gitignore", "node_modules/\n")

	steps, err := PlanLegacyFiles(root, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 0 {
		t.Errorf("PlanLegacyFiles = %+v, want nothing: every file is an opt-in or the user's", steps)
	}
}

// An older init's whole-file front-ends are deleted, the skill with rtdd's own directory
// when nothing else is in it.
func TestLegacyFilesDeleteTheOldWholeFileFrontEnds(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, ".claude/skills/rtdd/SKILL.md", protocol.Generated+"\n\nskill\n")
	writeAt(t, root, ".cursor/rules/rtdd.mdc", protocol.Generated+"\n\nrule\n")
	writeAt(t, root, ".cursor/rules/house.mdc", "the host's own rule\n")

	steps, err := PlanLegacyFiles(root, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".claude/skills/rtdd/"); s.Action != Delete {
		t.Errorf("skill step = %v, want Delete of rtdd's directory", s.Action)
	}
	if s := stepFor(t, steps, ".cursor/rules/rtdd.mdc"); s.Action != Delete {
		t.Errorf("mdc step = %v, want Delete", s.Action)
	}
	if err := ApplyUninstall(root, steps); err != nil {
		t.Fatal(err)
	}
	if got := repoFiles(t, root); strings.Join(got, ",") != ".cursor/rules/house.mdc" {
		t.Errorf("repository holds %v, want only the host's own rule", got)
	}

	// --project-skill owns those paths, so an init run with it reports neither.
	writeAt(t, root, ".claude/skills/rtdd/SKILL.md", protocol.Generated+"\n\nskill\n")
	steps, err = PlanLegacyFiles(root, PlanOptions{ProjectSkill: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 0 {
		t.Errorf("with ProjectSkill: %+v, want nothing", steps)
	}
}

// The root .gitignore line goes, every other line stays, and .rtdd/.gitignore takes over
// first so the graph cache is never left unignored.
func TestLegacyFilesMoveTheGraphCacheLineIntoRtdd(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, ".gitignore", "node_modules/\n.rtdd/graph.json\n*.log\n")
	steps, err := PlanLegacyFiles(root, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || steps[0].Path != ".rtdd/.gitignore" || steps[1].Path != ".gitignore" {
		t.Fatalf("steps = %+v, want .rtdd/.gitignore then .gitignore", steps)
	}
	if err := ApplyUninstall(root, steps); err != nil {
		t.Fatal(err)
	}
	if got := readAt(t, root, ".gitignore"); got != "node_modules/\n*.log\n" {
		t.Errorf(".gitignore = %q, want the user's lines alone", got)
	}
	if got := readAt(t, root, ".rtdd/.gitignore"); got != "graph.json\n" {
		t.Errorf(".rtdd/.gitignore = %q, want %q", got, "graph.json\n")
	}

	// A .gitignore an older init created holding only that line goes whole.
	root = t.TempDir()
	writeAt(t, root, ".gitignore", "/.rtdd/graph.json\n")
	writeAt(t, root, ".rtdd/.gitignore", "graph.json\n")
	steps, err = PlanLegacyFiles(root, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Path != ".gitignore" || steps[0].Action != Delete {
		t.Errorf("steps = %+v, want only a Delete of .gitignore", steps)
	}
}

// An older `rtdd skill install` merged its block into the shared machine-wide files.
func TestGlobalLegacyFilesStripTheBlockFromTheSharedHomeFiles(t *testing.T) {
	home := t.TempDir()
	user := "# My instructions\n\nAlways use tabs.\n"
	writeAt(t, home, ".codex/AGENTS.md", user+"\n"+block("old global"))
	writeAt(t, home, ".gemini/GEMINI.md", block("old global"))

	steps, err := PlanGlobalLegacyFiles(home)
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".codex/AGENTS.md"); s.Action != StripBlock {
		t.Errorf("codex step = %v, want StripBlock", s.Action)
	}
	if s := stepFor(t, steps, ".gemini/GEMINI.md"); s.Action != Delete {
		t.Errorf("gemini step = %v, want Delete: it held nothing but the block", s.Action)
	}
	if err := ApplyUninstall(home, steps); err != nil {
		t.Fatal(err)
	}
	if got := readAt(t, home, ".codex/AGENTS.md"); got != user {
		t.Errorf("~/.codex/AGENTS.md = %q, want %q", got, user)
	}
}

// v0.3.0 wrote Codex's skill to ~/.codex/skills/rtdd/, a directory the codex row has
// Retired. Until an install removes it, it is footprint — but only rtdd's renders are; a
// file the user put beside them is neither reported nor removed.
func TestGlobalLegacyFilesReportRtddsFilesInARetiredDirectory(t *testing.T) {
	home := t.TempDir()
	writeAt(t, home, ".codex/skills/rtdd/SKILL.md", protocol.Generated+"\nv0.3.0's skill\n")
	steps, err := PlanGlobalLegacyFiles(home)
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".codex/skills/rtdd/"); s.Action != Delete || !strings.Contains(s.Note, "Codex skill") {
		t.Errorf("retired directory step = %v %q, want a Delete naming the Codex skill", s.Action, s.Note)
	}

	writeAt(t, home, ".codex/skills/rtdd/notes.md", "mine\n")
	steps, err = PlanGlobalLegacyFiles(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Path != ".codex/skills/rtdd/SKILL.md" || steps[0].Action != Delete {
		t.Errorf("steps = %+v, want only a Delete of rtdd's SKILL.md", steps)
	}
	if err := ApplyUninstall(home, steps); err != nil {
		t.Fatal(err)
	}
	if got := repoFiles(t, home); strings.Join(got, ",") != ".codex/skills/rtdd/notes.md" {
		t.Errorf("home after --legacy-files holds %v, want only the user's notes.md", got)
	}

	if err := os.Remove(filepath.Join(home, ".codex", "skills", "rtdd", "notes.md")); err != nil {
		t.Fatal(err)
	}
	writeAt(t, home, ".codex/skills/rtdd/notes.md", "mine\n")
	steps, err = PlanGlobalLegacyFiles(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 0 {
		t.Errorf("steps = %+v for a retired directory holding only the user's file, want none", steps)
	}
}

// The report init, doctor and update print: one Legacy line per piece of footprint, each
// naming the command that removes it, and nothing Apply would write.
func TestReportLegacyIsAReadOnlyLinePerPiece(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, "AGENTS.md", "# Rules\n\n"+block("old"))
	writeAt(t, root, ".gitignore", ".rtdd/graph.json\n")
	steps, err := PlanLegacyFiles(root, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	report := ReportLegacy(steps, "")
	if len(report) != 2 {
		t.Fatalf("report = %+v, want AGENTS.md and .gitignore only", report)
	}
	for _, s := range report {
		if s.Action != Legacy || s.Content != "" || !strings.Contains(s.Note, "rtdd uninstall --legacy-files") {
			t.Errorf("report step = %+v, want a contentless Legacy step naming --legacy-files", s)
		}
	}
	before := repoFiles(t, root)
	if err := Apply(root, report); err != nil {
		t.Fatal(err)
	}
	if after := repoFiles(t, root); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Errorf("applying a report changed the repository: %v -> %v", before, after)
	}
	if got := ReportLegacy(steps, "~/")[0].Path; got != "~/AGENTS.md" {
		t.Errorf("prefixed path = %q, want ~/AGENTS.md", got)
	}
}
