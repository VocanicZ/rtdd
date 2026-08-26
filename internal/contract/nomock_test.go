package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Git is never mocked in this engine. internal/gitctx is the only package allowed to
// shell out, and every git-dependent test runs against a real `git init` in t.TempDir().
// A mock would let gitctx's tests agree with a fiction instead of with git's real output,
// which is precisely where the bugs this package exists to prevent live.
func TestNoGitMockExistsInTheTree(t *testing.T) {
	root := repoRoot(t)
	mock := regexp.MustCompile(`(?i)\b(fake|mock|stub)[_a-z0-9]*git|git[_a-z0-9]*(fake|mock|stub)\b`)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "docs":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if m := mock.FindString(string(b)); m != "" {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s mentions %q: git is never mocked, use a real repo in t.TempDir()", rel, m)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// internal/gitctx is the only package permitted to invoke git. A second shell-out site
// would bypass the error wrapping and the real-repo test discipline this package carries.
func TestOnlyGitctxShellsOutToGit(t *testing.T) {
	root := repoRoot(t)
	execGit := regexp.MustCompile(`exec\.Command\(\s*"git"`)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "docs":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/gitctx/") || strings.HasPrefix(rel, "internal/contract/") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if execGit.MatchString(string(b)) {
			t.Errorf("%s shells out to git: internal/gitctx is the only package allowed to", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}
