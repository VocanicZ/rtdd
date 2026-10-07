package graphbuild

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

// pyGraphify records one function per .py file at HEAD, graphify's view of a project it
// only ever parsed Python in. manifest lists every file graphify saw.
func pyGraphify(t *testing.T, root string, py []string, manifest []string) {
	t.Helper()
	var nodes []gfNode
	for _, f := range py {
		nodes = append(nodes, gfNode{f, strings.TrimSuffix(f, ".py") + "()", f, 1})
	}
	writeGraphify(t, root, fullHead(t, root), nodes, nil, manifest)
}

func nodeIDs(g graph.Graph) map[string]bool {
	ids := map[string]bool{}
	for _, n := range g.Nodes {
		ids[n.ID] = true
	}
	return ids
}

// PRD #409 AC5 (#441): in graphify+scanner mode every changed file is scanned and
// overlaid, whatever its extension — an untracked .sh, a modified committed .js and an
// extensionless executable script, though graphify only ever held .py code nodes.
func TestEveryChangedFileIsScannedWhateverItsExtension(t *testing.T) {
	root := repo(t, map[string]string{
		"a.py": "def a():\n    return 1\n", "b.py": "def b():\n    return 2\n",
		"c.py": "def c():\n    return 3\n", "web.js": "function web() {\n  return 1\n}\n",
	})
	pyGraphify(t, root, []string{"a.py", "b.py", "c.py"}, []string{"a.py", "b.py", "c.py", "web.js"})
	gittest.Write(t, root, "tool.sh", "helper() {\n  echo hi\n}\n")
	gittest.Write(t, root, "web.js", "function web() {\n  return 2\n}\n\nfunction more() {\n  return 3\n}\n")
	gittest.Write(t, root, "bin/greet", "#!/usr/bin/env bash\ngreet() {\n  echo hi\n}\n")
	if err := os.Chmod(filepath.Join(root, "bin", "greet"), 0o755); err != nil {
		t.Fatal(err)
	}

	res := build(t, root)
	if res.Source != SourceGraphifyScanner || res.GraphifyIgnored != "" {
		t.Fatalf("Source %q, GraphifyIgnored %q; want graphify+scanner", res.Source, res.GraphifyIgnored)
	}
	if want := []string{"bin/greet", "tool.sh", "web.js"}; !reflect.DeepEqual(res.StaleFiles, want) {
		t.Errorf("StaleFiles = %v, want %v", res.StaleFiles, want)
	}
	ids := nodeIDs(res.Graph)
	for _, id := range []string{"tool.sh::helper", "web.js::web", "web.js::more", "bin/greet::greet", "a.py::a", "b.py::b", "c.py::c"} {
		if !ids[id] {
			t.Errorf("graph lacks %s; nodes %v", id, ids)
		}
	}
}

// PRD #409 AC5 (#441): the max_stale_ratio numerator counts code files only — a README
// edit or a new language overlaid from the scanner does not push graphify over 50 %.
func TestNonCodeAndNewLanguageFilesDoNotCountTowardTheRatio(t *testing.T) {
	root := repo(t, map[string]string{
		"a.py": "def a():\n    return 1\n", "b.py": "def b():\n    return 2\n", "README.md": "# r\n",
	})
	pyGraphify(t, root, []string{"a.py", "b.py"}, []string{"a.py", "b.py", "README.md"})
	gittest.Write(t, root, "README.md", "# changed\n")
	for _, f := range []string{"x.sh", "y.sh", "z.sh"} {
		gittest.Write(t, root, f, strings.TrimSuffix(f, ".sh")+"() {\n  echo hi\n}\n")
	}

	res := build(t, root)
	if res.Source != SourceGraphifyScanner || res.GraphifyIgnored != "" {
		t.Fatalf("Source %q, GraphifyIgnored %q; want graphify+scanner — 3 .sh files against 2 .py code files is not stale", res.Source, res.GraphifyIgnored)
	}
	if res.StaleCodeFiles != 0 {
		t.Errorf("StaleCodeFiles = %d, want 0", res.StaleCodeFiles)
	}
	if want := []string{"README.md", "x.sh", "y.sh", "z.sh"}; !reflect.DeepEqual(res.StaleFiles, want) {
		t.Errorf("StaleFiles = %v, want %v", res.StaleFiles, want)
	}
	if ids := nodeIDs(res.Graph); !ids["x.sh::x"] || !ids["a.py::a"] {
		t.Errorf("graph lacks x.sh::x or a.py::a: %v", ids)
	}
}

// The too_stale count is the code-only numerator, not every overlaid file.
func TestTooStaleCountsCodeFilesOnly(t *testing.T) {
	root := repo(t, fourFiles)
	fourGraphify(t, root, fullHead(t, root))
	edit(t, root, "a.py", "b.py", "c.py")
	gittest.Write(t, root, "notes.txt", "n\n")
	res := build(t, root)
	if res.GraphifyIgnored != IgnoredTooStale || res.StaleCodeFiles != 3 {
		t.Errorf("GraphifyIgnored %q, StaleCodeFiles %d; want too_stale with 3", res.GraphifyIgnored, res.StaleCodeFiles)
	}
}

// §4.1: a changed file the filter rejects — binary, over 1 MiB, scan_exclude — is still
// never scanned, nor overlaid.
func TestAFilteredChangedFileIsNeverScanned(t *testing.T) {
	root := repo(t, map[string]string{"a.py": "def a():\n    return 1\n", "b.py": "def b():\n    return 2\n"})
	pyGraphify(t, root, []string{"a.py", "b.py"}, []string{"a.py", "b.py"})
	gittest.Write(t, root, "blob.sh", "bin() {\n  echo \x00\n}\n")
	gittest.Write(t, root, "big.sh", "big() {\n  echo hi\n}\n"+strings.Repeat("#\n", 1<<19+1))
	gittest.Write(t, root, "vendor/lib.sh", "lib() {\n  echo hi\n}\n")
	gittest.Write(t, root, "ok.sh", "ok() {\n  echo hi\n}\n")

	res := build(t, root)
	if res.Source != SourceGraphifyScanner {
		t.Fatalf("Source %q, want graphify+scanner", res.Source)
	}
	if want := []string{"ok.sh"}; !reflect.DeepEqual(res.StaleFiles, want) || !reflect.DeepEqual(res.Scanned, want) {
		t.Errorf("StaleFiles %v, Scanned %v; want only ok.sh", res.StaleFiles, res.Scanned)
	}
	ids := nodeIDs(res.Graph)
	for _, id := range []string{"blob.sh::bin", "big.sh::big", "vendor/lib.sh::lib"} {
		if ids[id] {
			t.Errorf("filtered file's node %s is in the graph", id)
		}
	}
}
