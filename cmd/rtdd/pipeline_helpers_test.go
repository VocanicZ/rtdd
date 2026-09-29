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
// hitOffset is the 1-based line within appended that must be reported uncovered.
func pipelineCheck(t *testing.T, dir, storeFile, appended string, hitOffset int, wantTest string) {
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

	before, err := os.ReadFile(filepath.Join(dir, storeFile))
	if err != nil {
		t.Fatal(err)
	}
	wantLine := strings.Count(string(before), "\n") + hitOffset

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

	// cmdRun writes to os.Stdout, not to run's writer.
	var code int
	runOut := captureStdout(t, func() { code = run([]string{"run", "--json"}, &out, &errb) })
	if code != 0 {
		t.Fatalf("run = %d\nstdout:\n%s", code, runOut)
	}
	out.Reset()
	out.WriteString(runOut)
	var r struct {
		Uncovered struct {
			Files []JSONFileReport `json:"files"`
		} `json:"uncovered"`
	}
	if err := json.Unmarshal([]byte(out.String()), &r); err != nil {
		t.Fatalf("run json: %v\n%s", err, out.String())
	}
	for _, fr := range r.Uncovered.Files {
		if fr.Path != storeFile {
			continue
		}
		for _, rg := range fr.Ranges {
			if rg.Class == "uncovered" && rg.Start <= wantLine && wantLine <= rg.End {
				return
			}
		}
	}
	t.Fatalf("no uncovered range in %s covers appended line %d\n%s", storeFile, wantLine, out.String())
}
