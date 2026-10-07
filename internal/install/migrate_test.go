package install

import (
	"os"
	"path/filepath"
	"testing"
)

// v02Layout lays out what a v0.2 `rtdd init` and `rtdd seed` left in a repository.
func v02Layout(t *testing.T, gitattributes string) string {
	t.Helper()
	root := t.TempDir()
	writeAt(t, root, ".rtdd/map.jsonl", `{"t":"tests/test_a.py","f":["src/a.py"],"c":"a3f21e0","d":12,"s":"pass","a":"python"}`+"\n")
	writeAt(t, root, ".rtdd/meta.json", `{"v":2,"seeded_at":"a3f21e0"}`+"\n")
	writeAt(t, root, ".rtdd/adapters/python.yaml", "name: python\n")
	writeAt(t, root, ".rtdd/config.yaml", "stale_commits: 50\ndrift_guard: 100\nhub_threshold: 0.40\nadapters:\n  - name: python\n")
	if gitattributes != "" {
		writeAt(t, root, ".gitattributes", gitattributes)
	}
	return root
}

// PRD #411 AC6: each of the four v0.2 artefacts is planned for removal, and a
// .gitattributes keeps every line but rtdd's.
func TestPlanMigrationRemovesEveryV02Artefact(t *testing.T) {
	root := v02Layout(t, "*.png binary\n.rtdd/map.jsonl merge=union\n")
	steps, err := PlanMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".rtdd/map.jsonl", ".rtdd/meta.json", ".rtdd/adapters/"} {
		if s := stepFor(t, steps, rel); s.Action != Delete || s.Note == "" {
			t.Errorf("%s: step = %v %q, want Delete with a note saying what it was", rel, s.Action, s.Note)
		}
	}
	if s := stepFor(t, steps, ".gitattributes"); s.Action != StripBlock || s.Content != "*.png binary\n" {
		t.Errorf(".gitattributes: step = %v %q, want StripBlock leaving %q", s.Action, s.Content, "*.png binary\n")
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".rtdd/map.jsonl", ".rtdd/meta.json", ".rtdd/adapters"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s survived Apply: %v", rel, err)
		}
	}
}

// PRD #411 AC6: a .gitattributes that held nothing but the v0.2 line goes with it, as
// uninstall already does.
func TestPlanMigrationDeletesAGitattributesHoldingOnlyTheV02Line(t *testing.T) {
	root := v02Layout(t, ".rtdd/map.jsonl merge=union\n")
	steps, err := PlanMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".gitattributes"); s.Action != Delete {
		t.Errorf(".gitattributes: step = %v, want Delete", s.Action)
	}
}

// PRD #411 AC6: a repository v0.2 never touched plans no migration step at all.
func TestPlanMigrationOnAFreshRepositoryPlansNothing(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, ".gitattributes", "*.png binary\n")
	writeAt(t, root, ".rtdd/config.yaml", DefaultConfig())
	steps, err := PlanMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 0 {
		t.Errorf("PlanMigration on a fresh repository = %+v, want no step", steps)
	}
}

// PRD #411 AC6: a config holding only v0.2 keys is replaced by the v0.3.0 defaults; a
// config that sets any v0.3.0 key is the user's and is kept.
func TestPlanReplacesAV02OnlyConfigAndKeepsAV030One(t *testing.T) {
	root := v02Layout(t, "")
	steps, err := Plan(root, fakeFiles())
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".rtdd/config.yaml"); s.Action != Replace || s.Content != DefaultConfig() {
		t.Errorf("v0.2 config: step = %v %q, want Replace with DefaultConfig()", s.Action, s.Content)
	}
	root = t.TempDir()
	writeAt(t, root, ".rtdd/config.yaml", "max_stale_ratio: 0.3\nstale_commits: 50\n")
	steps, err = Plan(root, fakeFiles())
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".rtdd/config.yaml"); s.Action != Skip {
		t.Errorf("a config setting max_stale_ratio: step = %v, want Skip", s.Action)
	}
}
