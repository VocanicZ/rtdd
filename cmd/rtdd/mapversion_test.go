package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// v1Repo is a python repository whose map was written by an older rtdd: meta "v":1 and a
// test-case-id row covering the file that is then changed. The process is chdir'd into it.
func v1Repo(t *testing.T) {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "pytest.ini", "[pytest]\n")
	gittest.Write(t, dir, "src/a.py", "X = 1\n")
	gittest.Write(t, dir, "tests/test_a.py", "def test_x():\n    pass\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n__pycache__/\n")
	gittest.Commit(t, dir, "init")
	sha := gittest.HeadShort(t, dir)
	gittest.Write(t, dir, ".rtdd/meta.json", `{"v":1,"adapter":"python","seeded_at":"`+sha+`","cycles":0}`+"\n")
	gittest.Write(t, dir, ".rtdd/map.jsonl", `{"t":"tests/test_a.py::test_x","f":["src/a.py"],"c":"`+sha+`"}`+"\n")
	gittest.Write(t, dir, "src/a.py", "X = 2\n")
	chdir(t, dir)
}

// v1Doc is the part of the --json document the version rule decides.
type v1Doc struct {
	Tier      string `json:"tier"`
	Reason    string `json:"reason"`
	Selection struct {
		Tests []string `json:"tests"`
	} `json:"selection"`
}

func assertUnseededT2(t *testing.T, raw string) {
	t.Helper()
	var d v1Doc
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("json: %v\n%s", err, raw)
	}
	if d.Tier != "T2" || !strings.Contains(d.Reason, "rtdd seed") {
		t.Fatalf("tier=%q reason=%q, want T2 with a reason naming rtdd seed\n%s", d.Tier, d.Reason, raw)
	}
	if len(d.Selection.Tests) != 1 || d.Selection.Tests[0] != "tests/test_a.py" {
		t.Errorf("tests = %v, want the unit tests/test_a.py", d.Selection.Tests)
	}
}

// A v1 map holds test-case ids, not units. It is not migrated: it reads as unseeded, so
// the answer is the full suite with the reason telling the user to run `rtdd seed`.
func TestAV1MapIsTreatedAsUnseeded(t *testing.T) {
	v1Repo(t)
	var out, errb strings.Builder
	if code := run([]string{"which", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("which = %d\n%s\n%s", code, out.String(), errb.String())
	}
	assertUnseededT2(t, out.String())
}
