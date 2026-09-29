package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// nodeFixture copies a committed fixture into a temp git repo, runs npm install there
// (skipping when it cannot, e.g. offline) and then drives the real pipeline.
func nodeFixture(t *testing.T, fixture, store, appended string, hit int, wantTest string) {
	t.Helper()
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not on PATH")
	}
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/fixtures/"+fixture)); err != nil {
		t.Fatal(err)
	}
	install := exec.Command("npm", "install", "--no-audit", "--no-fund", "--ignore-scripts")
	install.Dir = dir
	if out, err := install.CombinedOutput(); err != nil {
		t.Skipf("npm install failed (offline?): %v\n%s", err, out)
	}
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	pipelineCheck(t, dir, store, appended, hit, wantTest)
}

func TestPipelineJest(t *testing.T) {
	nodeFixture(t, "jest", "src/store.js", "\nfunction extra(k) {\n  return k + \"!\";\n}\nmodule.exports.extra = extra;\n", 3, "src/api.test.js")
}

func TestPipelineVitest(t *testing.T) {
	nodeFixture(t, "vitest", "src/store.ts", "\nexport function extra(k: string): string {\n  return k + \"!\";\n}\n", 3, "src/api.test.ts")
}
