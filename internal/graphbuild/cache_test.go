package graphbuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

// PRD #409 AC8: a file is re-scanned only when its blob id changed or it is in the
// working-tree changed set.
func TestCacheRescansOnlyChangedBlobsAndTheWorkingTreeChangedSet(t *testing.T) {
	root := repo(t, calcProject)
	if got, want := build(t, root).Scanned, []string{"README.md", "src/calc.py", "tests/test_calc.py"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cold build scanned %v, want %v", got, want)
	}
	if got := build(t, root).Scanned; len(got) != 0 {
		t.Errorf("warm build with no change scanned %v, want nothing", got)
	}
	gittest.Write(t, root, "src/calc.py", calcProject["src/calc.py"]+"\n\ndef sub(a, b):\n    return a - b\n")
	if got, want := build(t, root).Scanned, []string{"src/calc.py"}; !reflect.DeepEqual(got, want) {
		t.Errorf("after an uncommitted edit scanned %v, want %v", got, want)
	}
	gittest.Commit(t, root, "sub")
	build(t, root) // the cache now holds src/calc.py at its new blob
	if got := build(t, root).Scanned; len(got) != 0 {
		t.Errorf("after committing and rebuilding scanned %v, want nothing", got)
	}
}

func TestCacheRescansACommittedChange(t *testing.T) {
	root := repo(t, calcProject)
	build(t, root)
	gittest.Write(t, root, "tests/test_calc.py", calcProject["tests/test_calc.py"]+"\n\ndef test_more():\n    assert add(2, 2) == 4\n")
	gittest.Commit(t, root, "more") // committed: not in the changed set, but a new blob
	if got, want := build(t, root).Scanned, []string{"tests/test_calc.py"}; !reflect.DeepEqual(got, want) {
		t.Errorf("after a committed change scanned %v, want %v", got, want)
	}
}

// A warm build from the cache is the same graph as a cold build of the same tree.
func TestWarmBuildEqualsColdBuild(t *testing.T) {
	root := repo(t, calcProject)
	cold := build(t, root)
	warm := build(t, root)
	if !reflect.DeepEqual(warm.Graph, cold.Graph) {
		t.Errorf("warm graph differs from cold:\n warm %+v\n cold %+v", warm.Graph, cold.Graph)
	}
}

// A class's method edges survive the cache: a warm build that re-reads nothing still
// has every class -> method edge a cold build had.
func TestWarmBuildKeepsMethodEdges(t *testing.T) {
	root := repo(t, map[string]string{
		"pkg/shapes.py": "class Shape:\n    def area(self):\n        return 0\n\n    def name(self):\n        return self.area()\n",
		"pkg/other.py":  "class Other:\n    def area(self):\n        return 1\n",
	})
	cold := build(t, root)
	warm := build(t, root)
	if len(warm.Scanned) != 0 {
		t.Fatalf("warm build scanned %v, want nothing", warm.Scanned)
	}
	methods := 0
	for _, e := range warm.Graph.Edges {
		if e.Relation == graph.RelMethod {
			methods++
		}
	}
	if methods != 3 {
		t.Errorf("warm graph has %d method edges, want 3: %+v", methods, warm.Graph.Edges)
	}
	if !reflect.DeepEqual(warm.Graph, cold.Graph) {
		t.Errorf("warm graph differs from cold:\n warm %+v\n cold %+v", warm.Graph, cold.Graph)
	}
}

// While a file is being edited it is re-read on every build (it is in the changed set),
// but a re-read that finds what the cache already holds leaves the cache file alone;
// a further edit rewrites it.
func TestRescanOfAnUnchangedEditDoesNotRewriteTheCache(t *testing.T) {
	root := repo(t, calcProject)
	cachePath := filepath.Join(root, ".rtdd", "graph.json")
	build(t, root)
	gittest.Write(t, root, "src/calc.py", calcProject["src/calc.py"]+"\n\ndef sub(a, b):\n    return a - b\n")
	build(t, root) // records the edit
	before, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := build(t, root).Scanned, []string{"src/calc.py"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("scanned %v, want %v", got, want)
	}
	after, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Errorf("the cache was rewritten though the re-read file is unchanged since it was cached")
	}

	gittest.Write(t, root, "src/calc.py", calcProject["src/calc.py"]+"\n\ndef mul(a, b):\n    return a * b\n")
	res := build(t, root)
	again, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(after, again) {
		t.Errorf("the cache was not rewritten after a further edit")
	}
	names := map[string]bool{}
	for _, n := range readCache(cachePath).results["src/calc.py"].Nodes {
		names[n.Name] = true
	}
	if !names["mul"] || names["sub"] || len(res.Graph.Nodes) != 4 {
		t.Errorf("cache holds %v for src/calc.py and the graph %d nodes; want mul, not sub, and 4 nodes", names, len(res.Graph.Nodes))
	}
}

// The cache is graphify's graph.json shape plus built_at_commit and per-file blob ids.
func TestCacheIsGraphifyShapedWithBuiltAtCommitAndBlobIDs(t *testing.T) {
	root := repo(t, calcProject)
	build(t, root)
	b, err := os.ReadFile(filepath.Join(root, ".rtdd", "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Directed      *bool             `json:"directed"`
		Nodes         []map[string]any  `json:"nodes"`
		Links         []map[string]any  `json:"links"`
		BuiltAtCommit string            `json:"built_at_commit"`
		Files         map[string]string `json:"rtdd_files"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Directed == nil || len(doc.Nodes) != 3 || len(doc.Links) != 2 {
		t.Errorf("cache has directed=%v, %d nodes, %d links; want a node-link graph of 3 and 2", doc.Directed, len(doc.Nodes), len(doc.Links))
	}
	for _, n := range doc.Nodes {
		for _, k := range []string{"id", "label", "file_type", "source_file", "source_location"} {
			if _, ok := n[k]; !ok {
				t.Errorf("cache node %v has no graphify key %q", n["id"], k)
			}
		}
	}
	if doc.BuiltAtCommit != gittest.HeadShort(t, root) {
		t.Errorf("built_at_commit = %q, want HEAD", doc.BuiltAtCommit)
	}
	blobs, err := gitctx.BlobIDs(root)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Files["src/calc.py"] != blobs["src/calc.py"] || doc.Files["src/calc.py"] == "" {
		t.Errorf("rtdd_files[src/calc.py] = %q, want HEAD's blob %q", doc.Files["src/calc.py"], blobs["src/calc.py"])
	}
	if _, ok := doc.Files["README.md"]; !ok {
		t.Errorf("rtdd_files = %v, want README.md listed though it has no nodes", doc.Files)
	}
}

func TestCorruptOrForeignCacheIsRebuiltNotAnError(t *testing.T) {
	for name, body := range map[string]string{
		"corrupt":       "{\"nodes\": [",
		"other version": "{\"rtdd_cache\": 99, \"nodes\": []}",
		"other scanner": "{\"rtdd_cache\": " + strconv.Itoa(cacheVersion) + ", \"rtdd_scanner\": \"not-this-one\", \"nodes\": []}",
	} {
		t.Run(name, func(t *testing.T) {
			root := repo(t, calcProject)
			build(t, root)
			if err := os.WriteFile(filepath.Join(root, ".rtdd", "graph.json"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			res := build(t, root)
			if len(res.Scanned) != 3 || len(res.Graph.Nodes) != 3 {
				t.Errorf("scanned %v into %d nodes, want a full rebuild of 3 files / 3 nodes", res.Scanned, len(res.Graph.Nodes))
			}
		})
	}
}

// A cached file's call into a re-scanned file is re-linked, never left dangling.
func TestCachedCallerOfARenamedFunctionIsNotLeftDangling(t *testing.T) {
	root := repo(t, calcProject)
	build(t, root)
	gittest.Write(t, root, "src/calc.py", "def plus(a, b):\n    return a + b\n")
	res := build(t, root)
	ids := map[string]bool{}
	for _, n := range res.Graph.Nodes {
		ids[n.ID] = true
	}
	for _, e := range res.Graph.Edges {
		if !ids[e.From] || !ids[e.To] {
			t.Errorf("dangling edge %+v", e)
		}
	}
}

// On the graphify path the scanner reads only the stale files, and the cache spares even
// those once a committed stale file has been scanned at its blob.
func TestCacheServesGraphifysCommittedStaleFiles(t *testing.T) {
	root := repo(t, overlayProject)
	overlayGraphify(t, root)
	gittest.Write(t, root, "src/calc.py", "# moved down two lines\n\n"+overlayProject["src/calc.py"])
	gittest.Run(t, root, "add", "src/calc.py")
	gittest.Run(t, root, "commit", "-q", "-m", "shift")

	cold := build(t, root)
	if cold.Source != SourceGraphifyScanner || !reflect.DeepEqual(cold.Scanned, []string{"src/calc.py"}) {
		t.Fatalf("cold build: Source %q, scanned %v; want graphify+scanner reading src/calc.py", cold.Source, cold.Scanned)
	}
	warm := build(t, root)
	if len(warm.Scanned) != 0 {
		t.Errorf("warm build scanned %v, want nothing", warm.Scanned)
	}
	if !reflect.DeepEqual(warm.Graph, cold.Graph) {
		t.Errorf("warm graph differs from cold:\n warm %+v\n cold %+v", warm.Graph, cold.Graph)
	}
}
