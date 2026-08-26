package contract

import (
	"os"
	"os/exec"
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

// internal/gitctx is the only ENGINE package permitted to invoke git. A second shell-out
// site would bypass the error wrapping and the real-repo test discipline this package
// carries.
//
// internal/pytestfixture/fixture.go is the single amended exception (00-interfaces.md,
// "gittest — internal/pytestfixture shells out to git inline"). It is a non-test file that
// builds a fixture repository, so it can either shell out or import gittest, and gittest
// imports `testing` — which would put `testing` on a non-test dependency graph. The
// allowance is one named file, not a package prefix, and TestOnlyTestFilesImportGittest
// plus TestTestOnlyHelpersDoNotCompileInTesting are what it is paid for with.
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
		if strings.HasPrefix(rel, "internal/gitctx/") || strings.HasPrefix(rel, "internal/contract/") ||
			rel == "internal/pytestfixture/fixture.go" {
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

// gittest imports `testing`, which is why the contract says nothing outside a _test.go
// file may import it: one non-test importer puts the testing package — its flag
// registration, its exit behaviour — on a production dependency graph. Benign today only
// because cmd/ happens not to reach the offending package; nothing guarded it.
func TestOnlyTestFilesImportGittest(t *testing.T) {
	root := repoRoot(t)
	const pkg = `"github.com/VocanicZ/rtdd/internal/gitctx/gittest"`

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
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(b), pkg) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s is not a _test.go file and imports internal/gitctx/gittest, which imports testing", filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// The import guard above is the rule; this is the consequence it exists to prevent.
// internal/pytestfixture is the package that reached for gittest, and the cheapest
// statement of "it did not reach again" is its own dependency graph.
func TestTestOnlyHelpersDoNotCompileInTesting(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}
	root := repoRoot(t)

	for _, pkg := range []string{"./internal/pytestfixture", "./cmd/rtdd", "./internal/adapter"} {
		cmd := exec.Command("go", "list", "-deps", pkg)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.TrimSpace(line) == "testing" {
				t.Errorf("go list -deps %s contains `testing`: a non-test package pulled in a test-only helper", pkg)
			}
		}
	}
}
