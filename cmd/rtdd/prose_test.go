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
// never scanned as code — `rtdd graph --json` counts the code file's node, and since #504
// one file node per file: no definition of a prose file's code sample.
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
	if want := 1 + 1 + len(proseFiles); doc.Graph.Nodes != want || doc.Graph.Edges != 0 {
		t.Errorf("graph = %d nodes, %d edges; want %d nodes (src/calc.py::add and a file node per file), 0 edges — prose was scanned", doc.Graph.Nodes, doc.Graph.Edges, want)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".rtdd", "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range proseFiles {
		if strings.Contains(string(b), f+"::") {
			t.Errorf(".rtdd/graph.json holds a definition of prose file %s", f)
		}
	}
}

// Issue #465: a changed prose file produces no changed definition. Since #504 it changes
// as its file node, and one nothing names is untested, never reported as none.
func TestWhichMapsAChangedProseFileToItsFileNodeAlone(t *testing.T) {
	dir := proseRepo(t)
	for _, f := range proseFiles {
		e2eEdit(t, dir, f, "add(1, 2)", "add(2, 3)")
	}
	w, text := runE2EWhich(t, dir)
	want := []string{"docs/guide.rst", "docs/manual.adoc", "docs/notes.markdown", "docs/plan.md", "notes.txt"}
	assertIDs(t, "changed_nodes", w.changed(), want)
	assertIDs(t, "untested", w.Untested, want)
	for _, f := range proseFiles {
		if strings.Contains(text, f+"::") {
			t.Errorf("text output names a node in %s:\n%s", f, text)
		}
	}
}
