package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestPipelineDotnet(t *testing.T) {
	if _, err := exec.LookPath("dotnet"); err != nil {
		t.Skip("dotnet not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/fixtures/dotnet")); err != nil {
		t.Fatal(err)
	}
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	// The appended class sits in Store.cs so the file still compiles; its return is line 6.
	pipelineCheck(t, dir, "Calc/Store.cs", "\npublic static class Extra\n{\n    public static string Tag(string k)\n    {\n        return k + \"!\";\n    }\n}\n", 6, "tests/Calc.Tests/ApiTests.cs")
}
