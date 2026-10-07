package graphbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

// perfShapes are one small source file per fixture-corpus language. %[1]d makes every
// generated name distinct: a tree where 1 250 files each define `run` measures spec
// §4.3's accepted over-linking (n² calls edges), not the scanner.
var perfShapes = []struct{ ext, src string }{
	{".py", "class C%[1]d:\n    def run%[1]d(self, x):\n        return helper%[1]d(x)\n\n\ndef helper%[1]d(x):\n    if x:\n        return x + 1\n    return 0\n"},
	{".go", "package p\n\nfunc Helper%[1]d(x int) int {\n\tif x > 0 {\n\t\treturn x + 1\n\t}\n\treturn 0\n}\n\nfunc Run%[1]d() int { return Helper%[1]d(1) }\n"},
	{".ts", "export function helper%[1]d(x: number): number {\n  return x + 1;\n}\n\nexport const run%[1]d = (x: number) => {\n  return helper%[1]d(x);\n};\n"},
	{".java", "class C%[1]d {\n    int helper%[1]d(int x) {\n        return x + 1;\n    }\n\n    int run%[1]d() {\n        return helper%[1]d(1);\n    }\n}\n"},
	{".rs", "pub fn helper%[1]d(x: i32) -> i32 {\n    x + 1\n}\n\npub fn run%[1]d() -> i32 {\n    helper%[1]d(1)\n}\n"},
	{".rb", "class C%[1]d\n  def helper%[1]d(x)\n    x + 1\n  end\n\n  def run%[1]d\n    helper%[1]d(1)\n  end\nend\n"},
	{".lua", "local M = {}\n\nfunction M.helper%[1]d(x)\n  return x + 1\nend\n\nfunction M.run%[1]d()\n  return M.helper%[1]d(1)\nend\n\nreturn M\n"},
	{".sh", "helper%[1]d() {\n  echo $(( $1 + 1 ))\n}\n\nrun%[1]d() {\n  helper%[1]d 1\n}\n"},
}

// PRD #409 AC10, first half: a cold scanner build of 10 000 files in under 10 s.
func TestColdBuildOf10000FilesIsUnder10s(t *testing.T) {
	root := gittest.Init(t)
	for i := 0; i < 10000; i++ {
		s := perfShapes[i%len(perfShapes)]
		p := filepath.Join(root, fmt.Sprintf("pkg%03d", i/100), fmt.Sprintf("f%05d%s", i, s.ext))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(fmt.Sprintf(s.src, i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gittest.Commit(t, root, "10k")

	start := time.Now()
	res, err := Build(root, graph.DefaultConfig(), Options{CachePath: filepath.Join(t.TempDir(), "graph.json")})
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cold build: %d files, %d nodes, %d edges in %v", len(res.Scanned), len(res.Graph.Nodes), len(res.Graph.Edges), took)
	if len(res.Scanned) != 10000 {
		t.Fatalf("scanned %d files, want 10000", len(res.Scanned))
	}
	if took >= 10*time.Second {
		t.Errorf("cold build took %v, want < 10s", took)
	}
}

// PRD #409 AC10, second half: on this repository, a warm build in under 1 s — the
// Build that `rtdd graph` runs, minus printing six counts. The cache lives in a temp dir
// so the test never writes into the checkout. In a working tree with edits, the second
// Build still re-reads the edited files (they are in the changed set); that it need not
// rewrite the cache for them is part of what is measured.
func TestWarmBuildOfThisRepositoryIsUnder1s(t *testing.T) {
	wd, _ := os.Getwd()
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	cfg, err := graph.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	opt := Options{CachePath: filepath.Join(t.TempDir(), "graph.json")}
	if _, err := Build(root, cfg, opt); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	res, err := Build(root, cfg, opt)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("warm build: %d nodes, %d edges, %d re-scanned, in %v", len(res.Graph.Nodes), len(res.Graph.Edges), len(res.Scanned), took)
	if took >= time.Second {
		t.Errorf("warm build took %v, want < 1s", took)
	}
}
