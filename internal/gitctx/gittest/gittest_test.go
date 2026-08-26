package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// InitRepo is this package's *testing.T-free entry point: it returns an error instead
// of failing the test binary. Nothing outside a _test.go file may reach it — see
// internal/contract's TestOnlyTestFilesImportGittest.
func TestInitRepoCommitsTheTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InitRepo(dir, "fixture"); err != nil {
		t.Fatalf("InitRepo: %v", err)
	}
	if got := HeadShort(t, dir); len(got) < 4 {
		t.Fatalf("HEAD = %q, want a short SHA", got)
	}
	if got := strings.TrimSpace(Run(t, dir, "status", "--porcelain")); got != "" {
		t.Errorf("worktree dirty after InitRepo: %q", got)
	}
	if got := strings.TrimSpace(Run(t, dir, "log", "-1", "--pretty=%s")); got != "fixture" {
		t.Errorf("commit subject = %q, want %q", got, "fixture")
	}
}

func TestInitRepoReportsGitFailures(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	if err := InitRepo(filepath.Join(t.TempDir(), "nope"), "fixture"); err == nil {
		t.Fatal("InitRepo into a missing directory succeeded, want an error")
	}
}
