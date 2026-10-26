package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

func writeAt(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// PRD #411 AC5, as #509 amends it: a new repository gets the v0.3.0 config, and the graph
// cache is ignored by a nested .rtdd/.gitignore rather than a line in the host's own.
func TestPlanWritesTheConfigAndIgnoresTheGraphCache(t *testing.T) {
	root := t.TempDir()
	steps, err := Plan(root, fakeFiles(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".rtdd/config.yaml"); s.Action != Create || s.Content != DefaultConfig() {
		t.Errorf("config step = %v %q, want Create DefaultConfig()", s.Action, s.Content)
	}
	if s := stepFor(t, steps, ".rtdd/.gitignore"); s.Action != Create || s.Content != "graph.json\n" {
		t.Errorf(".rtdd/.gitignore step = %v %q, want Create %q", s.Action, s.Content, "graph.json\n")
	}
}

// #509: the host's root .gitignore is never planned, whatever it holds. A line an older
// init appended is reported as legacy, never rewritten.
func TestPlanNeverPlansTheRootGitignore(t *testing.T) {
	for _, existing := range []string{"node_modules/\n", "node_modules/", "node_modules/\n.rtdd/graph.json\n", "/.rtdd/graph.json\n"} {
		root := t.TempDir()
		writeAt(t, root, ".gitignore", existing)
		steps, err := Plan(root, fakeFiles(), PlanOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range steps {
			if s.Path == ".gitignore" && s.Action != Legacy {
				t.Errorf(".gitignore %q: step = %v, want no step but a legacy report", existing, s.Action)
			}
		}
	}
}

// PRD #411 AC5: `--force` is removed. With --project-skill, a whole-file front-end an
// earlier rtdd rendered — it carries protocol.Generated — is replaced; one rtdd did not
// write is a conflict.
func TestPlanReplacesAnEarlierRenderAndConflictsOnAForeignFile(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, ".claude/skills/rtdd/SKILL.md", protocol.Generated+"\n\nan earlier release's text\n")
	writeAt(t, root, ".cursor/rules/rtdd.mdc", "a rule the host wrote itself\n")
	steps, err := Plan(root, fakeFiles(), PlanOptions{ProjectSkill: true})
	if err != nil {
		t.Fatal(err)
	}
	want := markOptIn(fakeFiles()["dist/SKILL.md"], protocol.Generated)
	if s := stepFor(t, steps, ".claude/skills/rtdd/SKILL.md"); s.Action != Replace || s.Content != want {
		t.Errorf("skill step = %v %q, want Replace with the new render", s.Action, s.Content)
	}
	if s := stepFor(t, steps, ".cursor/rules/rtdd.mdc"); s.Action != Conflict {
		t.Errorf("mdc step = %v, want Conflict: rtdd did not write that file", s.Action)
	}
}
