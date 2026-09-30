// main_test.go holds this package's TestMain. Its only job is cleanup, and the reason it
// exists is written out below because the failure it prevents is five minutes and one
// pushed tag away from being invisible.
package installtest

import (
	"os"
	"testing"
)

// pkgDir is this package's directory relative to the repo root, for the tests that read
// their own source or re-run this package in a child `go test`.
const pkgDir = "internal/installtest"

// TestMain runs the suite and then removes GoReleaser's dist directory, so the package
// leaves the working tree as it found it.
//
// Why: .goreleaser.yaml's `before` hooks end with `go test ./...`, and the snapshot gates
// in this package build the real release into build/dist. GoReleaser cleans its dist
// directory FIRST, then runs the hooks, then requires that directory to be empty — so a
// real release wiped build/dist, spent five minutes running this suite, found the suite's
// own five archives sitting in build/dist and died:
//
//	⨯ release failed after 5m17s
//	  error=build/dist is not empty, remove it before running goreleaser or use the --clean flag
//
// That is what happened to the v0.1.0 tag, with `--clean` already set and every check
// green. --clean had simply run before the thing that dirtied the directory.
//
// Guarded by TestSuiteLeavesNoDistDirForGoreleaserToBuildInto.
//
// It also moves the working directory to the repo root first: every path in this package
// (README.md, scripts/, build/dist, ./cmd/rtdd) is written relative to the root, which is
// where these tests lived before they moved to pkgDir.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	code := m.Run()
	// snapshotDistDir is relative to the repo root, which is now this binary's working
	// directory. Cleanup failure must not mask the suite's own verdict.
	_ = os.RemoveAll(snapshotDistDir)
	os.Exit(code)
}
