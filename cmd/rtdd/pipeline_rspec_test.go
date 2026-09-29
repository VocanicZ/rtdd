package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// Unverified on a real toolchain: it skips unless bundle is present and the fixture's
// gems (rspec, simplecov, simplecov-lcov) install.
func TestPipelineRSpec(t *testing.T) {
	if _, err := exec.LookPath("bundle"); err != nil {
		t.Skip("bundle not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/fixtures/rspec")); err != nil {
		t.Fatal(err)
	}
	install := exec.Command("bundle", "install", "--quiet")
	install.Dir = dir
	if out, err := install.CombinedOutput(); err != nil {
		t.Skipf("bundle install failed (offline?): %v\n%s", err, out)
	}
	gittest.Write(t, dir, ".gitignore", ".bundle/\nvendor/\ncoverage/\n.rtdd/\nGemfile.lock\n")
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	// Appended after the module, the method body is line 3.
	pipelineCheck(t, dir, "lib/store.rb", "\ndef extra(k)\n  k + \"!\"\nend\n", 3, "spec/api_spec.rb")
}
