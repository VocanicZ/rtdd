package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestPipelineCargo(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not on PATH")
	}
	if _, err := exec.LookPath("cargo-llvm-cov"); err != nil {
		t.Skip("cargo-llvm-cov not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/fixtures/cargo")); err != nil {
		t.Fatal(err)
	}
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	pipelineCheck(t, dir, "src/store.rs", "\npub fn extra(k: &str) -> String {\n    format!(\"{}!\", k)\n}\n", 3, "tests/api.rs")
}
