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

// PRD #411 AC5: a new repository gets the v0.3.0 config and a .gitignore naming the cache.
func TestPlanWritesTheConfigAndIgnoresTheGraphCache(t *testing.T) {
	root := t.TempDir()
	steps, err := Plan(root, fakeFiles())
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".rtdd/config.yaml"); s.Action != Create || s.Content != DefaultConfig() {
		t.Errorf("config step = %v %q, want Create DefaultConfig()", s.Action, s.Content)
	}
	if s := stepFor(t, steps, ".gitignore"); s.Action != Create || s.Content != ".rtdd/graph.json\n" {
		t.Errorf(".gitignore step = %v %q, want Create %q", s.Action, s.Content, ".rtdd/graph.json\n")
	}
}

// PRD #411 AC5: a host .gitignore gains the line once, after its own lines, and a
// repository that already ignores the cache — either spelling — is left alone.
func TestPlanAppendsTheGraphCacheLineOnlyWhenItIsMissing(t *testing.T) {
	for _, c := range []struct {
		existing string
		action   Action
		content  string
	}{
		{"node_modules/\n", AppendLine, "node_modules/\n.rtdd/graph.json\n"},
		{"node_modules/", AppendLine, "node_modules/\n.rtdd/graph.json\n"},
		{"node_modules/\n.rtdd/graph.json\n", Skip, ""},
		{"/.rtdd/graph.json\n", Skip, ""},
	} {
		root := t.TempDir()
		writeAt(t, root, ".gitignore", c.existing)
		steps, err := Plan(root, fakeFiles())
		if err != nil {
			t.Fatal(err)
		}
		s := stepFor(t, steps, ".gitignore")
		if s.Action != c.action || (c.action != Skip && s.Content != c.content) {
			t.Errorf(".gitignore %q: step = %v %q, want %v %q", c.existing, s.Action, s.Content, c.action, c.content)
		}
	}
}

// PRD #411 AC5: `--force` is removed. A whole-file front-end an earlier rtdd rendered —
// it carries protocol.Generated — is replaced; one rtdd did not write is a conflict.
func TestPlanReplacesAnEarlierRenderAndConflictsOnAForeignFile(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, ".claude/skills/rtdd/SKILL.md", protocol.Generated+"\n\nan earlier release's text\n")
	writeAt(t, root, ".cursor/rules/rtdd.mdc", "a rule the host wrote itself\n")
	steps, err := Plan(root, fakeFiles())
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".claude/skills/rtdd/SKILL.md"); s.Action != Replace || s.Content != fakeFiles()["dist/SKILL.md"] {
		t.Errorf("skill step = %v %q, want Replace with the new render", s.Action, s.Content)
	}
	if s := stepFor(t, steps, ".cursor/rules/rtdd.mdc"); s.Action != Conflict {
		t.Errorf("mdc step = %v, want Conflict: rtdd did not write that file", s.Action)
	}
}
