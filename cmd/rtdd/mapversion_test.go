package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// A v1 map holds test-case ids, not units. It is not migrated: it reads as unseeded, so
// the answer is the full suite with the reason telling the user to run `rtdd seed`.
func TestAV1MapIsTreatedAsUnseeded(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "pytest.ini", "[pytest]\n")
	gittest.Write(t, dir, "src/a.py", "X = 1\n")
	gittest.Write(t, dir, "tests/test_a.py", "def test_x():\n    pass\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	sha := gittest.HeadShort(t, dir)
	gittest.Write(t, dir, ".rtdd/meta.json", `{"v":1,"adapter":"python","seeded_at":"`+sha+`","cycles":0}`+"\n")
	gittest.Write(t, dir, ".rtdd/map.jsonl", `{"t":"tests/test_a.py::test_x","f":["src/a.py"],"c":"`+sha+`"}`+"\n")
	gittest.Write(t, dir, "src/a.py", "X = 2\n")
	chdir(t, dir)

	var out, errb strings.Builder
	if code := run([]string{"which", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("which = %d\n%s\n%s", code, out.String(), errb.String())
	}
	var w struct {
		Tier      string `json:"tier"`
		Selection struct {
			Tests  []string `json:"tests"`
			Reason string   `json:"reason"`
		} `json:"selection"`
	}
	if err := json.Unmarshal([]byte(out.String()), &w); err != nil {
		t.Fatalf("which json: %v\n%s", err, out.String())
	}
	if w.Tier != "T2" || !strings.Contains(out.String(), "rtdd seed") {
		t.Fatalf("tier=%q, want T2 with a reason naming rtdd seed\n%s", w.Tier, out.String())
	}
	if len(w.Selection.Tests) != 1 || w.Selection.Tests[0] != "tests/test_a.py" {
		t.Errorf("tests = %v, want the unit tests/test_a.py", w.Selection.Tests)
	}
}
