package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// copyTree copies every file under src into dst, keeping relative paths.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		gittest.Write(t, dst, filepath.ToSlash(rel), string(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// goldenWhichRepo commits testdata/which-golden/repo, then applies the known edit:
// testdata/which-golden/edit over it (total and unused in src/calc.py, the import line of
// src/report.py, Parse in money/money.go) and src/legacy.py deleted.
func goldenWhichRepo(t *testing.T) string {
	t.Helper()
	fixture, err := filepath.Abs(filepath.Join("testdata", "which-golden"))
	if err != nil {
		t.Fatal(err)
	}
	dir := gittest.Init(t)
	copyTree(t, filepath.Join(fixture, "repo"), dir)
	gittest.Commit(t, dir, "fixture")
	copyTree(t, filepath.Join(fixture, "edit"), dir)
	if err := os.Remove(filepath.Join(dir, "src", "legacy.py")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// PRD #410 AC3: the whole schema-3 document for a fixture repository, byte for byte.
// HEAD's sha is the one moving part; it is written as <HEAD>. Regenerate, after
// reading the diff, with:
//
//	RTDD_UPDATE_GOLDEN=1 go test ./cmd/rtdd/ -run '^TestWhichJSONGolden$'
func TestWhichJSONGolden(t *testing.T) {
	golden, err := filepath.Abs(filepath.Join("testdata", "which-golden", "want.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := goldenWhichRepo(t)
	code, out, errOut := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("rtdd which --json = %d, stderr %q", code, errOut)
	}
	got := strings.ReplaceAll(out, `"`+gittest.HeadShort(t, dir)+`"`, `"<HEAD>"`)
	if os.Getenv("RTDD_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("rtdd which --json moved from %s:\n got: %s\nwant: %s", golden, got, want)
	}
}

// PRD #410 AC3: exactly spec §9's nine top-level keys, schema 3, and [] — never null —
// for every empty array.
func TestWhichJSONHasExactlyTheSchema3KeysAndNoNulls(t *testing.T) {
	dir := whichRepo(t) // nothing changed: every array is empty
	code, out, errOut := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("rtdd which --json = %d, stderr %q", code, errOut)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	var keys []string
	for k := range doc {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := []string{"base", "changed", "changed_nodes", "command", "graph", "rounds", "schema", "untested", "warnings"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("top-level keys = %v, want exactly %v", keys, want)
	}
	if string(doc["schema"]) != "3" || string(doc["command"]) != `"which"` || string(doc["base"]) != `"HEAD"` {
		t.Errorf("schema/command/base = %s/%s/%s, want 3/\"which\"/\"HEAD\"", doc["schema"], doc["command"], doc["base"])
	}
	for _, k := range []string{"changed", "changed_nodes", "untested", "warnings"} {
		if string(doc[k]) != "[]" {
			t.Errorf("%s = %s, want []", k, doc[k])
		}
	}
	var rounds []map[string]json.RawMessage
	if err := json.Unmarshal(doc["rounds"], &rounds); err != nil || len(rounds) != 3 {
		t.Fatalf("rounds = %s, want three round objects", doc["rounds"])
	}
	for i, r := range rounds[:2] {
		if string(r["tests"]) != "[]" || string(r["files"]) != "[]" {
			t.Errorf("round %d tests/files = %s/%s, want []/[]", i+1, r["tests"], r["files"])
		}
	}
	if string(rounds[2]["round"]) != "3" || string(rounds[2]["full_suite"]) != "true" || len(rounds[2]) != 2 {
		t.Errorf("round 3 = %v, want exactly {\"round\": 3, \"full_suite\": true}", rounds[2])
	}
}

// PRD #410 AC3: a round's files are its tests' files, de-duplicated, in test order.
func TestWhichJSONRoundFilesAreTheDeduplicatedTestFiles(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	_, out, _ := rtdd(t, dir, "which", "--json")
	var doc struct {
		Rounds []struct {
			Round int `json:"round"`
			Tests []struct {
				ID string `json:"id"`
			} `json:"tests"`
			Files []string `json:"files"`
		} `json:"rounds"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	r2 := doc.Rounds[1]
	if len(r2.Tests) != 2 {
		t.Fatalf("round 2 tests = %v, want test_add and test_report", r2.Tests)
	}
	if want := []string{"tests/test_calc.py", "tests/test_report.py"}; !reflect.DeepEqual(r2.Files, want) {
		t.Errorf("round 2 files = %v, want %v", r2.Files, want)
	}
	if want := []string{"tests/test_calc.py"}; !reflect.DeepEqual(doc.Rounds[0].Files, want) {
		t.Errorf("round 1 files = %v, want %v", doc.Rounds[0].Files, want)
	}
}

// PRD #410 AC6: --json changes the output, never the exit code.
func TestWhichJSONExitCodesMatchTextMode(t *testing.T) {
	dir := whichRepo(t)
	for _, c := range []struct {
		dir  string
		args []string
		want int
	}{
		{dir, []string{"which", "--json"}, 0},
		{dir, []string{"which", "--json", "--base", "no-such-ref"}, 2},
		{t.TempDir(), []string{"which", "--json"}, 3},
	} {
		if code, _, _ := rtdd(t, c.dir, c.args...); code != c.want {
			t.Errorf("rtdd %v: exit %d, want %d", c.args, code, c.want)
		}
	}
}
