package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
)

// v02Repository is a repository as v0.2 left it: seeded map, its meta file, a host adapter,
// the adapter-era config, the merge driver beside an unrelated .gitattributes line, and the
// skill v0.2 rendered.
func v02Repository(t *testing.T) string {
	t.Helper()
	dir := graphRepo(t)
	writeFile(t, dir, ".rtdd/map.jsonl", `{"t":"tests/test_calc.py","f":["src/calc.py"],"c":"a3f21e0","d":12,"s":"pass","a":"python"}`+"\n")
	writeFile(t, dir, ".rtdd/meta.json", `{"v":2,"seeded_at":"a3f21e0"}`+"\n")
	writeFile(t, dir, ".rtdd/adapters/python.yaml", "name: python\n")
	writeFile(t, dir, ".rtdd/config.yaml", "stale_commits: 50\ndrift_guard: 100\nhub_threshold: 0.40\nadapters:\n  - name: python\n")
	writeFile(t, dir, ".gitattributes", "*.png binary\n.rtdd/map.jsonl merge=union\n")
	writeFile(t, dir, ".claude/skills/rtdd/SKILL.md", "---\nname: rtdd\n---\n\n"+protocol.Generated+"\n\nRun `rtdd seed` once.\n")
	gittest.Commit(t, dir, "v0.2 setup")
	return dir
}

// removals are the lines of init's output that remove something.
func removals(out string) []string {
	return regexp.MustCompile(`(?m)^(delete|strip-block)\s+\S+.*$`).FindAllString(out, -1)
}

// PRD #411 AC6: init on a v0.2 repository removes all four artefacts, says so once each,
// keeps the unrelated .gitattributes line, and leaves the repository set up for v0.3.0.
func TestInitMigratesAV02Repository(t *testing.T) {
	dir := v02Repository(t)
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	for _, rel := range []string{".rtdd/map.jsonl", ".rtdd/meta.json", ".rtdd/adapters"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s survived init: %v", rel, err)
		}
	}
	if got := readRepoFileForTest(t, dir, ".gitattributes"); got != "*.png binary\n" {
		t.Errorf(".gitattributes = %q, want only the unrelated line left", got)
	}
	for _, re := range []string{`(?m)^delete\s+\.rtdd/map\.jsonl\b`, `(?m)^delete\s+\.rtdd/meta\.json\b`,
		`(?m)^delete\s+\.rtdd/adapters/`, `(?m)^strip-block\s+\.gitattributes\b`} {
		if !regexp.MustCompile(re).MatchString(out) {
			t.Errorf("init output does not report the removal %s:\n%s", re, out)
		}
	}
	if n := len(removals(out)); n != 4 {
		t.Errorf("init reported %d removals, want 4:\n%s", n, out)
	}
	cfg, err := graph.LoadConfig(dir)
	if err != nil || !reflect.DeepEqual(cfg, graph.DefaultConfig()) {
		t.Errorf("after migration the config loads as %+v (%v), want graph.DefaultConfig()", cfg, err)
	}
	if raw := readRepoFileForTest(t, dir, ".rtdd/config.yaml"); strings.Contains(raw, "stale_commits") || strings.Contains(raw, "adapters") {
		t.Errorf("the v0.2 config survived:\n%s", raw)
	}
	if !strings.Contains(readRepoFileForTest(t, dir, ".gitignore"), ".rtdd/graph.json\n") {
		t.Error("the migrated repository does not ignore .rtdd/graph.json")
	}
	files, err := install.Files()
	if err != nil {
		t.Fatal(err)
	}
	if got := readRepoFileForTest(t, dir, ".claude/skills/rtdd/SKILL.md"); got != files["dist/SKILL.md"] {
		t.Errorf("the v0.2 skill was not replaced by the v0.3.0 render:\n%s", got)
	}
}

// PRD #411 AC6: a second init on the migrated repository removes and reports nothing.
func TestInitTwiceOnAMigratedRepositoryRemovesNothing(t *testing.T) {
	dir := v02Repository(t)
	if code, _, errOut := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("first rtdd init = %d, stderr %q", code, errOut)
	}
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("second rtdd init = %d, stderr %q", code, errOut)
	}
	if r := removals(out); len(r) != 0 {
		t.Errorf("the second init reported removals %q", r)
	}
}

// PRD #411 AC6: a repository v0.2 never touched gets no removal line.
func TestInitOnAFreshRepositoryPrintsNoRemoval(t *testing.T) {
	dir := graphRepo(t)
	writeFile(t, dir, ".gitattributes", "*.png binary\n")
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, stderr %q", code, errOut)
	}
	if r := removals(out); len(r) != 0 {
		t.Errorf("init on a fresh repository reported removals %q", r)
	}
}
