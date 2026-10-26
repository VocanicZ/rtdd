package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

func fakeFiles() map[string]string {
	return map[string]string{
		"dist/SKILL.md":              "---\nname: rtdd\n---\n\n" + protocol.Generated + "\n\n# skill v1\n",
		"dist/cursor/rules/rtdd.mdc": "---\nglobs: **/*\n---\n\n" + protocol.Generated + "\n\n# mdc v1\n",
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

// repoFiles lists every file under root, slash-separated and relative to it.
func repoFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		out = append(out, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// #509: rtdd is self-contained. On a clean repository init plans .rtdd/config.yaml and
// .rtdd/.gitignore and nothing else, and applying the plan creates nothing outside .rtdd/.
func TestPlanOnACleanRepoWritesOnlyInsideRtdd(t *testing.T) {
	root := t.TempDir()
	steps, err := Plan(root, fakeFiles(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range steps {
		if !strings.HasPrefix(s.Path, ".rtdd/") {
			t.Errorf("Plan touches %s (%v) — init must write inside .rtdd/ only", s.Path, s.Action)
		}
	}
	if s := stepFor(t, steps, ".rtdd/.gitignore"); s.Action != Create || s.Content != "graph.json\n" {
		t.Errorf(".rtdd/.gitignore = %v %q, want Create %q", s.Action, s.Content, "graph.json\n")
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	got := repoFiles(t, root)
	want := []string{".rtdd/.gitignore", ".rtdd/config.yaml"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("after Apply the repository holds %v, want %v", got, want)
	}
}

// #509: a user's AGENTS.md, CLAUDE.md and .gitignore are theirs; init plans none of them.
func TestPlanNeverTouchesTheHostsSharedFiles(t *testing.T) {
	root := t.TempDir()
	host := map[string]string{
		"AGENTS.md":  "# AGENTS\n\nHouse rules.\n",
		"CLAUDE.md":  "# our claude notes\n",
		".gitignore": "node_modules/\n",
	}
	for rel, body := range host {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	steps, err := Plan(root, fakeFiles(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	for rel, body := range host {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != body {
			t.Errorf("%s = %q after init, want it byte-identical %q", rel, b, body)
		}
	}
}

// --project-skill opts the repository into rtdd's own whole-file front-ends, each marked as
// asked for so it is never reported as legacy.
func TestPlanProjectSkillWritesTheWholeFileFrontEndsMarkedOptIn(t *testing.T) {
	root := t.TempDir()
	steps, err := Plan(root, fakeFiles(), PlanOptions{ProjectSkill: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc"} {
		s := stepFor(t, steps, rel)
		if s.Action != Create {
			t.Errorf("%s: action = %v, want Create", rel, s.Action)
		}
		if !strings.Contains(s.Content, protocol.Generated+"\n"+OptInMarker+"\n") {
			t.Errorf("%s does not carry the opt-in marker after the generated line:\n%s", rel, s.Content)
		}
	}
	for _, s := range steps {
		if s.Path == "AGENTS.md" || s.Path == "CLAUDE.md" {
			t.Errorf("--project-skill planned the shared file %s: %+v", s.Path, s)
		}
	}
}

// --append-to is the only way rtdd's block enters a shared file, and only the file named.
func TestPlanAppendToMergesTheBlockIntoTheNamedFileOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# our claude notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(root, fakeFiles(), PlanOptions{AppendTo: []string{"CLAUDE.md"}})
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
	if !strings.Contains(s.Content, protocol.BeginMarker+"\n"+OptInMarker+"\n") {
		t.Errorf("the appended block carries no opt-in marker:\n%s", s.Content)
	}
	for _, s := range steps {
		if s.Path == "AGENTS.md" {
			t.Errorf("AGENTS.md was planned though only CLAUDE.md was named: %+v", s)
		}
	}
}

func TestPlanAppendToRefusesAPathOutsideTheRepositoryOrInsideRtdd(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../elsewhere.md", ".rtdd/notes.md", filepath.Join(t.TempDir(), "x.md")} {
		steps, err := Plan(root, fakeFiles(), PlanOptions{AppendTo: []string{name}})
		if err != nil {
			t.Fatal(err)
		}
		if s := stepFor(t, steps, name); s.Action != Conflict {
			t.Errorf("--append-to %s: action = %v, want Conflict", name, s.Action)
		}
	}
}

func TestPlanSkipsWholeFileTargetsWhenContentIsUnchanged(t *testing.T) {
	root := t.TempDir()
	files := fakeFiles()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	current := markOptIn(files["dist/SKILL.md"], protocol.Generated)
	if err := os.WriteFile(filepath.Join(root, ".claude", "skills", "rtdd", "SKILL.md"), []byte(current), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(root, files, PlanOptions{ProjectSkill: true})
	if err != nil {
		t.Fatal(err)
	}
	s := stepFor(t, steps, ".claude/skills/rtdd/SKILL.md")
	if s.Action != Skip {
		t.Fatalf("action = %v, want Skip", s.Action)
	}
}

// An existing .rtdd/.gitignore is rtdd's own: the graph line is added once, after whatever
// it holds, and never twice.
func TestPlanRtddGitignoreAppendsTheGraphLineOnce(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rtdd", ".gitignore"), []byte("swarm/"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(root, fakeFiles(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".rtdd/.gitignore"); s.Action != AppendLine || s.Content != "swarm/\ngraph.json\n" {
		t.Fatalf(".rtdd/.gitignore = %v %q, want AppendLine %q", s.Action, s.Content, "swarm/\ngraph.json\n")
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	again, err := Plan(root, fakeFiles(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, again, ".rtdd/.gitignore"); s.Action != Skip {
		t.Errorf("second plan: .rtdd/.gitignore = %v, want Skip", s.Action)
	}
}

// PRD #410 AC5 / #452: init writes no merge driver for a coverage map nothing writes any
// more; a host's own .gitattributes is neither planned nor touched.
func TestPlanLeavesGitattributesAlone(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.png binary\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(root, fakeFiles(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range steps {
		if s.Path == ".gitattributes" {
			t.Fatalf("Plan still plans .gitattributes: %+v", s)
		}
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "*.png binary\n" {
		t.Errorf(".gitattributes = %q, want it untouched", b)
	}
}

func TestPlanConfigIsCreatedOnceThenNeverOverwritten(t *testing.T) {
	root := t.TempDir()
	steps, err := Plan(root, fakeFiles(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rtdd", "config.yaml"), []byte("max_stale_ratio: 0.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := Plan(root, fakeFiles(), PlanOptions{})
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
	opts := PlanOptions{ProjectSkill: true, AppendTo: []string{"AGENTS.md"}}
	steps, err := Plan(root, fakeFiles(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc", "AGENTS.md", ".rtdd/config.yaml", ".rtdd/.gitignore"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not written: %v", rel, err)
		}
	}

	again, err := Plan(root, fakeFiles(), opts)
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

// TestEmbeddedProtocolMatchesTheSource pins internal/install/protocol.md to
// protocol/PROTOCOL.md. The embed is what `rtdd init` writes, so a stale copy
// ships a stale protocol; until now the only thing reading the two files
// against each other was a single step in .github/workflows/ci.yml, which no
// local run of the suite exercises.
func TestEmbeddedProtocolMatchesTheSource(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "protocol", "PROTOCOL.md"))
	if err != nil {
		t.Fatalf("read protocol/PROTOCOL.md: %v", err)
	}
	if embeddedProtocol != string(src) {
		t.Errorf("internal/install/protocol.md (%d bytes) differs from protocol/PROTOCOL.md (%d bytes); "+
			"copy the source over the embed and commit the result",
			len(embeddedProtocol), len(src))
	}
}
