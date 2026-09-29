package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// Unverified on a real toolchain: it skips unless composer and a coverage driver exist.
func TestPipelinePHPUnit(t *testing.T) {
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("php not on PATH")
	}
	if _, err := exec.LookPath("composer"); err != nil {
		t.Skip("composer not on PATH (vendor/bin/phpunit needs composer install)")
	}
	mods, _ := exec.Command("php", "-m").Output()
	if m := strings.ToLower(string(mods)); !strings.Contains(m, "pcov") && !strings.Contains(m, "xdebug") {
		t.Skip("neither pcov nor xdebug is loaded; phpunit cannot record coverage")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/fixtures/phpunit")); err != nil {
		t.Fatal(err)
	}
	install := exec.Command("composer", "install", "--no-interaction", "--quiet")
	install.Dir = dir
	if out, err := install.CombinedOutput(); err != nil {
		t.Skipf("composer install failed (offline?): %v\n%s", err, out)
	}
	gittest.Write(t, dir, ".gitignore", "vendor/\n.rtdd/\ncomposer.lock\n")
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	// Appended after the class, so the file stays valid; the return is line 4.
	pipelineCheck(t, dir, "src/Store.php", "\nfunction extra(string $k): string\n{\n    return $k . \"!\";\n}\n", 4, "tests/ApiTest.php")
}
