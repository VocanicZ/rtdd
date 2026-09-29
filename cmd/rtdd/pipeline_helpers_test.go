package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pipelineCheck drives the real CLI through seed -> edit store -> which -> run in dir and
// asserts that only wantTest is selected although the edit is reached only through it.
func pipelineCheck(t *testing.T, dir, storeFile, appended, wantTest string) {
	t.Helper()
	chdir(t, dir)

	var out, errb strings.Builder
	if code := run([]string{"seed"}, &out, &errb); code != 0 {
		t.Fatalf("seed = %d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errb.String())
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".rtdd", "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		V int `json:"v"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil || meta.V != 2 {
		t.Fatalf("meta.json v = %d (err %v), want 2: %s", meta.V, err, raw)
	}

	f, err := os.OpenFile(filepath.Join(dir, storeFile), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(appended); err != nil {
		t.Fatal(err)
	}
	f.Close()

	out.Reset()
	errb.Reset()
	if code := run([]string{"which", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("which = %d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errb.String())
	}
	var w struct {
		Tier      string `json:"tier"`
		Selection struct {
			Tests []string `json:"tests"`
		} `json:"selection"`
	}
	if err := json.Unmarshal([]byte(out.String()), &w); err != nil {
		t.Fatalf("which json: %v\n%s", err, out.String())
	}
	if w.Tier != "T0" || len(w.Selection.Tests) != 1 || w.Selection.Tests[0] != wantTest {
		t.Fatalf("which tier=%q tests=%v, want T0 [%s]\n%s", w.Tier, w.Selection.Tests, wantTest, out.String())
	}

	out.Reset()
	errb.Reset()
	if code := run([]string{"run", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("run = %d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errb.String())
	}
	var r struct {
		Uncovered struct {
			Summary struct {
				UncoveredLines int `json:"uncovered_lines"`
			} `json:"summary"`
		} `json:"uncovered"`
	}
	if err := json.Unmarshal([]byte(out.String()), &r); err != nil {
		t.Fatalf("run json: %v\n%s", err, out.String())
	}
	if r.Uncovered.Summary.UncoveredLines < 1 {
		t.Fatalf("uncovered_lines = %d, want the appended line counted\n%s", r.Uncovered.Summary.UncoveredLines, out.String())
	}
}
