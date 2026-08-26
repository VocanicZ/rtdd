package initrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func put(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMergeManagedBlockIntoAbsentFile(t *testing.T) {
	got := MergeManagedBlock("", "BLOCK")
	if got != "BLOCK\n" {
		t.Fatalf("MergeManagedBlock() = %q, want %q", got, "BLOCK\n")
	}
}

func TestMergeManagedBlockAppendsWithoutClobbering(t *testing.T) {
	existing := "# My project\n\nRules the team wrote.\n"
	got := MergeManagedBlock(existing, BeginMarker+"\nrtdd\n"+EndMarker)
	if !strings.HasPrefix(got, existing) {
		t.Fatalf("existing content must be preserved byte for byte and come first:\n%q", got)
	}
	want := existing + "\n" + BeginMarker + "\nrtdd\n" + EndMarker + "\n"
	if got != want {
		t.Fatalf("MergeManagedBlock()\n got: %q\nwant: %q", got, want)
	}
}

// A file the host repo left without a trailing newline must still gain one before the
// managed block is appended, and must not lose its final line.
func TestMergeManagedBlockAppendsToAFileWithNoTrailingNewline(t *testing.T) {
	existing := "# My project\n\nno trailing newline"
	got := MergeManagedBlock(existing, BeginMarker+"\nrtdd\n"+EndMarker)
	want := existing + "\n\n" + BeginMarker + "\nrtdd\n" + EndMarker + "\n"
	if got != want {
		t.Fatalf("MergeManagedBlock()\n got: %q\nwant: %q", got, want)
	}
}

func TestMergeManagedBlockReplacesBetweenMarkersOnly(t *testing.T) {
	existing := "HEADER\n" + BeginMarker + "\nold rtdd text\n" + EndMarker + "\nFOOTER\n"
	block := BeginMarker + "\nnew rtdd text\n" + EndMarker
	got := MergeManagedBlock(existing, block)
	want := "HEADER\n" + BeginMarker + "\nnew rtdd text\n" + EndMarker + "\nFOOTER\n"
	if got != want {
		t.Fatalf("MergeManagedBlock()\n got: %q\nwant: %q", got, want)
	}
	if !strings.Contains(got, "HEADER") || !strings.Contains(got, "FOOTER") {
		t.Fatal("content outside the markers must survive")
	}
}

// An already-identical block is a no-op at the string level too: the fourth merge rule
// ("unchanged, write nothing") is only reachable when the merge is a fixed point.
func TestMergeManagedBlockIsAFixedPoint(t *testing.T) {
	block := BeginMarker + "\nrtdd\n" + EndMarker
	once := MergeManagedBlock("# House rules\n", block)
	twice := MergeManagedBlock(once, block)
	if twice != once {
		t.Fatalf("MergeManagedBlock is not idempotent\n once: %q\ntwice: %q", once, twice)
	}
}

func TestEnsureFrontEndOnExistingAgentsMD(t *testing.T) {
	root := t.TempDir()
	original := "# AGENTS\n\nDo not run destructive commands.\n"
	put(t, root, "AGENTS.md", original)

	act, err := EnsureFrontEnd(root, "AGENTS.md", Block())
	if err != nil {
		t.Fatalf("EnsureFrontEnd() error = %v", err)
	}
	if act.Kind != "updated" {
		t.Fatalf("Kind = %q, want \"updated\"", act.Kind)
	}
	got := read(t, root, "AGENTS.md")
	if !strings.HasPrefix(got, original) {
		t.Fatalf("the host repo's AGENTS.md was clobbered:\n%s", got)
	}
	if !strings.Contains(got, BeginMarker) || !strings.Contains(got, EndMarker) {
		t.Fatalf("managed block missing:\n%s", got)
	}

	// Second run is idempotent.
	act2, err := EnsureFrontEnd(root, "AGENTS.md", Block())
	if err != nil {
		t.Fatalf("EnsureFrontEnd() error = %v", err)
	}
	if act2.Kind != "unchanged" {
		t.Fatalf("second Kind = %q, want \"unchanged\"", act2.Kind)
	}
	if read(t, root, "AGENTS.md") != got {
		t.Fatal("second run must be byte-identical")
	}
}

// The third merge rule, exercised through the filesystem: a stale managed block is
// rewritten in place and every byte outside the markers survives.
func TestEnsureFrontEndReplacesAStaleBlockInPlace(t *testing.T) {
	root := t.TempDir()
	head := "# AGENTS\n\nTeam rules.\n\n"
	tail := "\n\n## After\n\nMore team rules.\n"
	put(t, root, "AGENTS.md", head+BeginMarker+"\nan old rtdd block\n"+EndMarker+tail)

	act, err := EnsureFrontEnd(root, "AGENTS.md", Block())
	if err != nil {
		t.Fatalf("EnsureFrontEnd() error = %v", err)
	}
	if act.Kind != "updated" {
		t.Fatalf("Kind = %q, want \"updated\"", act.Kind)
	}
	got := read(t, root, "AGENTS.md")
	if !strings.HasPrefix(got, head) || !strings.HasSuffix(got, tail) {
		t.Fatalf("content outside the markers was not preserved byte for byte:\n%q", got)
	}
	if strings.Contains(got, "an old rtdd block") {
		t.Fatalf("the stale managed block survived:\n%s", got)
	}
	if strings.Count(got, BeginMarker) != 1 {
		t.Fatalf("the block was appended instead of replaced:\n%s", got)
	}
}

func TestEnsureFrontEndCreatesAnAbsentFile(t *testing.T) {
	root := t.TempDir()
	act, err := EnsureFrontEnd(root, ".cursor/rules/rtdd.mdc", Block())
	if err != nil {
		t.Fatalf("EnsureFrontEnd() error = %v", err)
	}
	if act.Kind != "created" {
		t.Fatalf("Kind = %q, want \"created\"", act.Kind)
	}
	if act.Path != ".cursor/rules/rtdd.mdc" {
		t.Fatalf("Path = %q, want the repo-relative path", act.Path)
	}
	if got := read(t, root, ".cursor/rules/rtdd.mdc"); got != Block()+"\n" {
		t.Fatalf("a created front-end must contain only the managed block, got:\n%s", got)
	}
}

func TestEnsureGitAttributes(t *testing.T) {
	root := t.TempDir()
	act, err := EnsureGitAttributes(root)
	if err != nil {
		t.Fatalf("EnsureGitAttributes() error = %v", err)
	}
	if act.Kind != "created" {
		t.Fatalf("Kind = %q, want \"created\"", act.Kind)
	}
	if got := read(t, root, ".gitattributes"); got != ".rtdd/map.jsonl merge=union\n" {
		t.Fatalf(".gitattributes = %q", got)
	}
}

func TestEnsureGitAttributesAppendsToExisting(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".gitattributes", "*.png binary\n")
	act, err := EnsureGitAttributes(root)
	if err != nil {
		t.Fatalf("EnsureGitAttributes() error = %v", err)
	}
	if act.Kind != "updated" {
		t.Fatalf("Kind = %q, want \"updated\"", act.Kind)
	}
	want := "*.png binary\n.rtdd/map.jsonl merge=union\n"
	if got := read(t, root, ".gitattributes"); got != want {
		t.Fatalf(".gitattributes = %q, want %q", got, want)
	}
}

func TestEnsureGitAttributesIdempotent(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".gitattributes", "*.png binary\n.rtdd/map.jsonl merge=union\n")
	act, err := EnsureGitAttributes(root)
	if err != nil {
		t.Fatalf("EnsureGitAttributes() error = %v", err)
	}
	if act.Kind != "unchanged" {
		t.Fatalf("Kind = %q, want \"unchanged\"", act.Kind)
	}
}

func TestEnsureConfigNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	act, err := EnsureConfig(root)
	if err != nil {
		t.Fatalf("EnsureConfig() error = %v", err)
	}
	if act.Kind != "created" {
		t.Fatalf("Kind = %q, want \"created\"", act.Kind)
	}
	got := read(t, root, ".rtdd/config.yaml")
	for _, needle := range []string{"stale_commits: 50", "drift_guard: 100", "hub_threshold: 0.40"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("config.yaml missing %q:\n%s", needle, got)
		}
	}

	put(t, root, ".rtdd/config.yaml", "stale_commits: 7\n")
	act2, err := EnsureConfig(root)
	if err != nil {
		t.Fatalf("EnsureConfig() error = %v", err)
	}
	if act2.Kind != "unchanged" {
		t.Fatalf("Kind = %q, want \"unchanged\"", act2.Kind)
	}
	if read(t, root, ".rtdd/config.yaml") != "stale_commits: 7\n" {
		t.Fatal("an existing config must never be overwritten")
	}
}

func TestRunInstallsEverything(t *testing.T) {
	root := t.TempDir()
	put(t, root, "CLAUDE.md", "# House rules\n")

	acts, err := Run(root)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", "CLAUDE.md", ".cursor/rules/rtdd.mdc"}
	if len(acts) != len(want) {
		t.Fatalf("Run() returned %d actions, want %d: %#v", len(acts), len(want), acts)
	}
	for i, w := range want {
		if acts[i].Path != w {
			t.Fatalf("acts[%d].Path = %q, want %q", i, acts[i].Path, w)
		}
	}
	if !strings.HasPrefix(read(t, root, "CLAUDE.md"), "# House rules\n") {
		t.Fatal("an existing CLAUDE.md must not be clobbered")
	}
	if !strings.Contains(read(t, root, "CLAUDE.md"), BeginMarker) {
		t.Fatal("CLAUDE.md must gain the managed block")
	}
	if !strings.Contains(read(t, root, "AGENTS.md"), BeginMarker) {
		t.Fatal("AGENTS.md must be created with the managed block")
	}
	if !strings.Contains(read(t, root, ".cursor/rules/rtdd.mdc"), BeginMarker) {
		t.Fatal(".cursor/rules/rtdd.mdc must be created with the managed block")
	}
	if read(t, root, ".gitattributes") != ".rtdd/map.jsonl merge=union\n" {
		t.Fatal(".gitattributes must carry the union merge driver")
	}
}

// Re-running init reports `unchanged` for every file and writes nothing: the whole tree
// must be byte-identical, mtimes included would be overreach but content is the promise.
func TestRunIsIdempotent(t *testing.T) {
	root := t.TempDir()
	put(t, root, "CLAUDE.md", "# House rules\n")
	put(t, root, ".gitattributes", "*.png binary\n")

	if _, err := Run(root); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	before := map[string]string{}
	rels := []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", "CLAUDE.md", ".cursor/rules/rtdd.mdc"}
	for _, rel := range rels {
		before[rel] = read(t, root, rel)
	}

	acts, err := Run(root)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	for _, a := range acts {
		if a.Kind != "unchanged" {
			t.Errorf("second Run(): %s = %q, want \"unchanged\"", a.Path, a.Kind)
		}
	}
	for _, rel := range rels {
		if got := read(t, root, rel); got != before[rel] {
			t.Errorf("second Run() rewrote %s:\n got: %q\nwant: %q", rel, got, before[rel])
		}
	}
}

// Block() is the text installed into every front-end. It must carry both markers, and
// exactly once each, or a second init would append rather than replace.
func TestBlockIsDelimitedByExactlyOnePairOfMarkers(t *testing.T) {
	b := Block()
	if strings.Count(b, BeginMarker) != 1 || strings.Count(b, EndMarker) != 1 {
		t.Fatalf("Block() must carry exactly one marker pair:\n%s", b)
	}
	if !strings.HasPrefix(b, BeginMarker) {
		t.Errorf("Block() must start with %q:\n%s", BeginMarker, b)
	}
	if !strings.HasSuffix(b, EndMarker) {
		t.Errorf("Block() must end with %q:\n%s", EndMarker, b)
	}
}
