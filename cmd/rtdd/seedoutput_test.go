package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// fakeRepo is a repository served only by a host adapter whose units are run by run.sh:
// a unit named *good* passes and writes an lcov file, any other prints and exits `bad`.
func fakeRepo(t *testing.T, bad string, units ...string) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "fake.marker", "")
	gittest.Write(t, dir, ".gitignore", ".rtdd/map.jsonl\n.rtdd/meta.json\n")
	gittest.Write(t, dir, "run.sh", "case \"$1\" in *good*) printf 'SF:run.sh\\nDA:1,1\\nend_of_record\\n' > \"$2/lcov.info\"; exit 0;;\n"+
		"*) echo \"boom from $1\"; exit "+bad+";; esac\n")
	for _, u := range units {
		gittest.Write(t, dir, u, "x\n")
	}
	writeHostAdapter(t, dir, "fake.yaml", `name: fake
detect: ["fake.marker"]
unit_cmd: "sh run.sh {unit} {tmp}"
coverage_file: "{tmp}/lcov.info"
coverage_format: lcov
test_globs: ["t/*.txt"]
source_globs: ["*.sh"]
`)
	gittest.Commit(t, dir, "init")
	chdir(t, dir)
	return dir
}

func TestSeedPrintsAFailedUnitsOutput(t *testing.T) {
	dir := fakeRepo(t, "1", "t/good.txt", "t/bad.txt")
	var out, errb strings.Builder
	if code := run([]string{"seed"}, &out, &errb); code != 1 {
		t.Fatalf("seed = %d, want 1\n%s\n%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String()+errb.String(), "boom from t/bad.txt") {
		t.Errorf("seed does not show the failed unit's output:\n%s\n%s", out.String(), errb.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".rtdd", "map.jsonl")); err != nil {
		t.Errorf("seed with a passing unit wrote no map: %v", err)
	}
}

// An adapter whose every unit errored recorded nothing: saving that map would read as
// seeded, so seed exits 3 and writes neither map nor meta.
func TestSeedWhereEveryUnitErroredWritesNoMap(t *testing.T) {
	dir := fakeRepo(t, "3", "t/a.txt", "t/b.txt")
	var out, errb strings.Builder
	if code := run([]string{"seed"}, &out, &errb); code != 3 {
		t.Fatalf("seed = %d, want 3\n%s\n%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String()+errb.String(), "boom from t/a.txt") {
		t.Errorf("seed does not show the errored unit's output:\n%s\n%s", out.String(), errb.String())
	}
	for _, f := range []string{"map.jsonl", "meta.json"} {
		if _, err := os.Stat(filepath.Join(dir, ".rtdd", f)); err == nil {
			t.Errorf("seed wrote .rtdd/%s although every unit errored", f)
		}
	}
}
