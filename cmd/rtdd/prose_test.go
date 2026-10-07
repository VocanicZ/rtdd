package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// proseSample is a code sample of the kind a plan document quotes: it defines a function
// and calls the project's real one, so a scanner reading prose would make a node of it.
const proseSample = "Example:\n\ndef test_prose():\n    assert add(1, 2) == 3\n\nfunc TestProse(t *testing.T) {\n\tadd(1, 2)\n}\n"

var proseFiles = []string{"docs/plan.md", "docs/notes.markdown", "docs/guide.rst", "notes.txt", "docs/manual.adoc"}

// proseRepo is one code file and one prose file of each prose extension, every prose file
// holding a code sample.
func proseRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n")
	for _, f := range proseFiles {
		gittest.Write(t, dir, f, proseSample)
	}
	gittest.Commit(t, dir, "init")
	return dir
}

// Issue #465: with the default config, prose files (Markdown, reST, text, AsciiDoc) are
// never scanned as code — `rtdd graph --json` counts the code file's node alone.
func TestGraphNeverScansProseFilesAsCode(t *testing.T) {
	dir := proseRepo(t)
	code, out, errOut := rtdd(t, dir, "graph", "--json")
	if code != 0 {
		t.Fatalf("rtdd graph --json = %d, stderr %q", code, errOut)
	}
	var doc struct {
		Graph struct {
			Nodes int `json:"nodes"`
			Edges int `json:"edges"`
		} `json:"graph"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if doc.Graph.Nodes != 1 || doc.Graph.Edges != 0 {
		t.Errorf("graph = %d nodes, %d edges; want 1 node (src/calc.py::add), 0 edges — prose was scanned", doc.Graph.Nodes, doc.Graph.Edges)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".rtdd", "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range proseFiles {
		if strings.Contains(string(b), f) {
			t.Errorf(".rtdd/graph.json mentions prose file %s", f)
		}
	}
}

// Issue #465: a changed prose file produces no changed node and is not reported untested.
func TestWhichIgnoresAChangedProseFile(t *testing.T) {
	dir := proseRepo(t)
	for _, f := range proseFiles {
		e2eEdit(t, dir, f, "add(1, 2)", "add(2, 3)")
	}
	w, text := runE2EWhich(t, dir)
	assertIDs(t, "changed_nodes", w.changed(), []string{})
	assertIDs(t, "untested", w.Untested, []string{})
	for _, f := range proseFiles {
		if strings.Contains(text, f+"::") {
			t.Errorf("text output names a node in %s:\n%s", f, text)
		}
	}
}
