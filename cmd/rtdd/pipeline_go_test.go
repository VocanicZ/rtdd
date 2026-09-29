package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestPipelineGo(t *testing.T) {
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
	pipelineCheck(t, dir, "store/store.go", "\nfunc Extra(k string) string {\n\treturn k + \"!\"\n}\n", 3, "api/api_test.go")
}
