// distcleanup_test.go guards the one thing that broke the v0.1.0 release after every
// other check had passed: this package's snapshot gates build the real release into
// build/dist, and a `goreleaser release` run cannot build into a dist directory that
// something else has filled.
package installtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestSuiteLeavesNoDistDirForGoreleaserToBuildInto runs this package's test binary with no
// tests selected, so the only thing that executes is TestMain, and asserts the dist
// directory it found is gone afterwards.
//
// The sentinel is what makes the assertion mean something: without a file placed there
// first, a missing build/dist proves nothing, because the directory may simply never have
// existed in this checkout.
func TestSuiteLeavesNoDistDirForGoreleaserToBuildInto(t *testing.T) {
	root := findRepoRootForTest(t)
	distDir := filepath.Join(root, snapshotDistDir)

	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatalf("create %s: %v", snapshotDistDir, err)
	}
	sentinel := filepath.Join(distDir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("left behind by a previous run\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", sentinel, err)
	}

	// -run '^$' selects no test, so this child compiles the package, runs TestMain, and
	// exits. It cannot re-enter this test.
	cmd := exec.Command("go", "test", "-count=1", "-run", "^$", ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test -run '^$' .: %v\n%s", err, out)
	}

	if _, err := os.Stat(distDir); !os.IsNotExist(err) {
		t.Errorf("%s survived a run of this package's tests (stat err: %v). GoReleaser cleans its dist dir BEFORE its before hooks, and `go test ./...` is the last of those hooks, so anything this suite leaves there makes the next `goreleaser release` fail with \"build/dist is not empty\"", snapshotDistDir, err)
	}
}
