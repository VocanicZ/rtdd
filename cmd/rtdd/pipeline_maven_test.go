package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestPipelineMaven(t *testing.T) {
	if _, err := exec.LookPath("mvn"); err != nil {
		t.Skip("mvn not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/fixtures/maven")); err != nil {
		t.Fatal(err)
	}
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	// The appended class sits in Store.java so the file still compiles; its return is line 4.
	pipelineCheck(t, dir, "src/main/java/calc/Store.java", "\nclass Extra {\n    static String extra(String k) {\n        return k + \"!\";\n    }\n}\n", 4, "src/test/java/calc/ApiTest.java")
}
