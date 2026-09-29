package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// seededGoFixture is the go pipeline fixture, committed and seeded.
func seededGoFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../../internal/runner/testdata/gofix")); err != nil {
		t.Fatal(err)
	}
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)
	var out, errb strings.Builder
	if code := run([]string{"seed"}, &out, &errb); code != 0 {
		t.Fatalf("seed = %d\n%s\n%s", code, out.String(), errb.String())
	}
	return dir
}

func appendTo(t *testing.T, p, s string) {
	t.Helper()
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

// A deleted test file keeps its map row until a re-seed. It must never be selected — a
// unit that does not exist errors every run — and `run` must drop its row when it saves.
func TestDeletedTestFileIsNeverSelectedAndItsRowIsPruned(t *testing.T) {
	dir := seededGoFixture(t)
	if err := os.Remove(filepath.Join(dir, "api", "api_test.go")); err != nil {
		t.Fatal(err)
	}
	appendTo(t, filepath.Join(dir, "store", "store.go"), "\nfunc Extra() int { return 1 }\n")

	var out, errb strings.Builder
	if code := run([]string{"which", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("which = %d\n%s\n%s", code, out.String(), errb.String())
	}
	var w struct {
		Selection struct {
			Tests  []string `json:"tests"`
			Reason string   `json:"reason"`
		} `json:"selection"`
	}
	if err := json.Unmarshal([]byte(out.String()), &w); err != nil {
		t.Fatalf("which json: %v\n%s", err, out.String())
	}
	for _, id := range w.Selection.Tests {
		if id == "api/api_test.go" {
			t.Fatalf("which selected the deleted api/api_test.go: %s", out.String())
		}
	}
	if !strings.Contains(out.String(), "stale row: api/api_test.go") {
		t.Errorf("which does not name the dropped stale row:\n%s", out.String())
	}

	var code int
	runOut := captureStdout(t, func() { code = run([]string{"run"}, &out, &errb) })
	if code != 0 {
		t.Fatalf("run = %d, want 0\n%s", code, runOut)
	}

	// A run that executes something saves the map, and the deleted file's row goes.
	appendTo(t, filepath.Join(dir, "calc", "calc_test.go"), "\nfunc TestSub(t *testing.T) { _ = Sub(2, 1) }\n")
	runOut = captureStdout(t, func() { code = run([]string{"run"}, &out, &errb) })
	if code != 0 {
		t.Fatalf("second run = %d, want 0\n%s", code, runOut)
	}
	if _, ok := readMapJSONL(t, dir)["api/api_test.go"]; ok {
		t.Errorf("map still holds a row for the deleted api/api_test.go after run\n%s", runOut)
	}
}
