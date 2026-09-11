// main_test.go holds this package's TestMain. Its only job is cleanup, and the reason it
// exists is written out below because the failure it prevents is five minutes and one
// pushed tag away from being invisible.
package installtest

import (
	"os"
	"testing"
)

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
func TestMain(m *testing.M) {
	code := m.Run()
	// snapshotDistDir is relative to the package directory, which is this binary's
	// working directory. Cleanup failure must not mask the suite's own verdict.
	_ = os.RemoveAll(snapshotDistDir)
	os.Exit(code)
}
