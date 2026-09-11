package selfupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up to the module root so this test can read install.sh.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}

func installScript(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	return string(b)
}

// TestArchiveNameAgreesWithInstallSh is the drift guard. install.sh and this package are
// two implementations of one protocol, and the repo's rule elsewhere is that a gate
// exercises the real thing rather than a copy that can drift. A copy is unavoidable here -
// the installer cannot be Go, because it runs before Go is on the machine - so the next
// best thing is a test that fails when the two stop agreeing.
//
// Feeding the Go function the shell's own variable names renders its format string as the
// literal install.sh must contain, so this compares derivation rules and not one version's
// answer.
func TestArchiveNameAgreesWithInstallSh(t *testing.T) {
	script := installScript(t)
	want := archiveName("${VERSION_NUM}", "${OS}", "${ARCH}")
	if !strings.Contains(script, want) {
		t.Errorf("install.sh does not build the archive name this package builds (%s); the two have drifted", want)
	}
}

// TestReleaseURLsAgreeWithInstallSh pins the other half: the same two endpoints, the same
// path shape under them, and the same checksum file.
func TestReleaseURLsAgreeWithInstallSh(t *testing.T) {
	script := installScript(t)
	// install.sh composes both URLs from REPO, so the literals to look for are this
	// package's constants with the repository put back behind that variable. Comparing
	// the expanded strings would pass a script that had hardcoded a different repo into
	// one of the two URLs.
	if !strings.Contains(script, `REPO="`+Repo+`"`) {
		t.Fatalf("install.sh does not set REPO to %q", Repo)
	}
	for _, want := range []string{
		strings.Replace(defaultAPIURL, Repo, "${REPO}", 1),
		strings.Replace(defaultBaseURL, Repo, "${REPO}", 1),
		"${BASE_URL}/${VERSION}/${ARCHIVE}",
		"checksums.txt",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("install.sh does not carry %q; it and this package no longer describe the same release layout", want)
		}
	}
}

// TestInstallShResolvesTheSameFieldFromTheReleaseAPI - both read tag_name, and a release
// that is still a draft is invisible to that endpoint for both of them.
func TestInstallShResolvesTheSameFieldFromTheReleaseAPI(t *testing.T) {
	if !strings.Contains(installScript(t), "tag_name") {
		t.Error("install.sh no longer resolves the release through tag_name")
	}
}
