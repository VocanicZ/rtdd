package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeFiles() map[string]string {
	return map[string]string{
		"dist/SKILL.md":              "# skill v1\n",
		"dist/cursor/rules/rtdd.mdc": "# mdc v1\n",
		"dist/AGENTS.md":             block("agents v1"),
	}
}

func stepFor(t *testing.T, steps []Step, path string) Step {
	t.Helper()
	for _, s := range steps {
		if s.Path == path {
			return s
		}
	}
	t.Fatalf("no step for %s in %+v", path, steps)
	return Step{}
}

func TestPlanOnACleanRepoCreatesEverything(t *testing.T) {
	root := t.TempDir()
	steps, err := Plan(root, fakeFiles(), false)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]Action{
		".claude/skills/rtdd/SKILL.md": Create,
		".cursor/rules/rtdd.mdc":       Create,
		"AGENTS.md":                    Create,
		".gitattributes":               AppendBlock,
		".rtdd/config.yaml":            Create,
	}
	for path, action := range want {
		s := stepFor(t, steps, path)
		if s.Action != action {
			t.Errorf("%s: action = %v, want %v", path, s.Action, action)
		}
	}
	// CLAUDE.md is not introduced into a repo that never had one.
	for _, s := range steps {
		if s.Path == "CLAUDE.md" {
			t.Fatalf("CLAUDE.md must not be planned when the host has none: %+v", s)
		}
	}
}

func TestPlanMergesClaudeMdOnlyWhenHostAlreadyHasOne(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# our claude notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(root, fakeFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	s := stepFor(t, steps, "CLAUDE.md")
	if s.Action != AppendBlock {
		t.Fatalf("CLAUDE.md action = %v, want AppendBlock", s.Action)
	}
	if !strings.HasPrefix(s.Content, "# our claude notes\n") {
		t.Fatalf("CLAUDE.md content lost the host's own notes: %q", s.Content)
	}
}

func TestPlanSkipsWholeFileTargetsWhenContentIsUnchanged(t *testing.T) {
	root := t.TempDir()
	files := fakeFiles()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "skills", "rtdd", "SKILL.md"), []byte(files["dist/SKILL.md"]), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(root, files, false)
	if err != nil {
		t.Fatal(err)
	}
	s := stepFor(t, steps, ".claude/skills/rtdd/SKILL.md")
	if s.Action != Skip {
		t.Fatalf("action = %v, want Skip", s.Action)
	}
}

func TestPlanConflictsOnADifferingWholeFileWithoutForce(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "skills", "rtdd", "SKILL.md"), []byte("hand-edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(root, fakeFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	s := stepFor(t, steps, ".claude/skills/rtdd/SKILL.md")
	if s.Action != Conflict {
		t.Fatalf("action = %v, want Conflict", s.Action)
	}
}

func TestPlanForceOverwritesADifferingWholeFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "skills", "rtdd", "SKILL.md"), []byte("hand-edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(root, fakeFiles(), true)
	if err != nil {
		t.Fatal(err)
	}
	s := stepFor(t, steps, ".claude/skills/rtdd/SKILL.md")
	if s.Action != Create {
		t.Fatalf("action = %v, want Create under --force", s.Action)
	}
}

func TestPlanGitattributesAppendsTheUnionLineOnceThenSkips(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.png binary\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(root, fakeFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	s := stepFor(t, steps, ".gitattributes")
	if s.Action != AppendBlock {
		t.Fatalf("action = %v, want AppendBlock", s.Action)
	}
	if !strings.Contains(s.Content, "*.png binary") || !strings.Contains(s.Content, "merge=union") {
		t.Fatalf("gitattributes content = %q, want both the original line and the union merge driver", s.Content)
	}

	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	again, err := Plan(root, fakeFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	s = stepFor(t, again, ".gitattributes")
	if s.Action != Skip {
		t.Fatalf("second Plan action = %v, want Skip", s.Action)
	}
}

func TestPlanConfigIsCreatedOnceThenNeverOverwritten(t *testing.T) {
	root := t.TempDir()
	steps, err := Plan(root, fakeFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rtdd", "config.yaml"), []byte("stale_commits: 999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := Plan(root, fakeFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	s := stepFor(t, again, ".rtdd/config.yaml")
	if s.Action != Skip {
		t.Fatalf("action = %v, want Skip — a tuned config must never be overwritten", s.Action)
	}
}

func TestApplyIsANoOpOnAllSkipStepsAndWritesEverythingElse(t *testing.T) {
	root := t.TempDir()
	steps, err := Plan(root, fakeFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc", "AGENTS.md", ".gitattributes", ".rtdd/config.yaml"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not written: %v", rel, err)
		}
	}

	again, err := Plan(root, fakeFiles(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range again {
		if s.Action != Skip {
			t.Errorf("second Plan: %s action = %v, want Skip (idempotent)", s.Path, s.Action)
		}
	}
}

func TestApplyRefusesToWriteOnConflict(t *testing.T) {
	root := t.TempDir()
	steps := []Step{{Path: "AGENTS.md", Action: Conflict, Note: "differs"}}
	if err := Apply(root, steps); err == nil {
		t.Fatal("want an error when a step is a Conflict")
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err == nil {
		t.Fatal("AGENTS.md must not be written when the plan has a conflict")
	}
}

func TestFilesReturnsTheEmbeddedGeneratedFrontEnds(t *testing.T) {
	files, err := Files()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dist/SKILL.md", "dist/AGENTS.md", "dist/cursor/rules/rtdd.mdc"} {
		if _, ok := files[key]; !ok {
			t.Errorf("Files() is missing %q", key)
		}
	}
}
