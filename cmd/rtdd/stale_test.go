package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// seededGoFixture is the go pipeline fixture, committed and seeded.
func seededGoFixture(t *testing.T) string {
	t.Helper()
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
	chdir(t, dir)
	var out, errb strings.Builder
	if code := run([]string{"seed"}, &out, &errb); code != 0 {
		t.Fatalf("seed = %d\n%s\n%s", code, out.String(), errb.String())
	}
	return dir
}

func appendTo(t *testing.T, p, s string) {
	t.Helper()
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}
