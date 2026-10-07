package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// Skipped where gradle is not on PATH; first run for real on Gradle 8.10.2 (issue #423).
func TestPipelineGradle(t *testing.T) {
	if _, err := exec.LookPath("gradle"); err != nil {
		t.Skip("gradle not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/fixtures/gradle")); err != nil {
		t.Fatal(err)
	}
	// The adapter runs ./gradlew; the fixture ships none, so make one.
	wrap := exec.Command("gradle", "-q", "wrapper")
	wrap.Dir = dir
	if out, err := wrap.CombinedOutput(); err != nil {
		t.Skipf("gradle wrapper failed: %v\n%s", err, out)
	}
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	pipelineCheck(t, dir, "src/main/java/calc/Store.java", "\nclass Extra {\n    static String extra(String k) {\n        return k + \"!\";\n    }\n}\n", 4, "src/test/java/calc/ApiTest.java")
}
