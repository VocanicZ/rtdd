package selfupdate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up to the module root so these tests can reach the installer.
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

// installerFacts is what the Node installer says about the release layout.
type installerFacts struct {
	Repo    string `json:"repo"`
	Archive string `json:"archive"`
	Base    string `json:"base"`
	API     string `json:"api"`
}

// askInstaller runs the installer's own modules and returns the values they produce.
//
// This EXECUTES the other implementation rather than grepping its source. The previous
// version of this guard matched literals in install.sh, which could only ever prove the
// script contained a string — not that it derived the same answer. Calling the functions
// compares behaviour, so a refactor that keeps the old literal in a comment while computing
// something else now fails here.
func askInstaller(t *testing.T) installerFacts {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found on PATH")
	}
	const script = `
import { archiveName } from './installer/platform.js';
import { baseUrl, apiUrl, REPO } from './installer/release.js';
process.stdout.write(JSON.stringify({
  repo: REPO,
  archive: archiveName('1.2.3', 'linux', 'amd64'),
  base: baseUrl({}),
  api: apiUrl({}),
}));
`
	cmd := exec.Command(node, "--input-type=module", "-e", script)
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the installer modules: %v\n%s", err, out)
	}
	var facts installerFacts
	if err := json.Unmarshal(out, &facts); err != nil {
		t.Fatalf("parsing installer output %q: %v", out, err)
	}
	return facts
}

// TestArchiveNameAgreesWithTheInstaller is the drift guard. The installer and this package
// are two implementations of one protocol, and the repo's rule elsewhere is that a gate
// exercises the real thing rather than a copy that can drift. A copy is unavoidable here —
// the installer cannot be Go, because it runs before any rtdd binary is on the machine — so
// the next best thing is a test that fails when the two stop agreeing.
func TestArchiveNameAgreesWithTheInstaller(t *testing.T) {
	got := askInstaller(t).Archive
	want := archiveName("1.2.3", "linux", "amd64")
	if got != want {
		t.Errorf("the installer builds %q and this package builds %q; the two have drifted", got, want)
	}
}

// TestReleaseURLsAgreeWithTheInstaller pins the other half: the same repository, the same
// two endpoints, and the same checksum file.
func TestReleaseURLsAgreeWithTheInstaller(t *testing.T) {
	facts := askInstaller(t)
	if facts.Repo != Repo {
		t.Errorf("the installer targets repository %q, this package targets %q", facts.Repo, Repo)
	}
	if facts.Base != defaultBaseURL {
		t.Errorf("the installer downloads from %q, this package from %q", facts.Base, defaultBaseURL)
	}
	if facts.API != defaultAPIURL {
		t.Errorf("the installer resolves releases at %q, this package at %q", facts.API, defaultAPIURL)
	}
}

// installerSource returns the installer's sources concatenated, for the two facts that are
// about what the code names rather than what it computes.
func installerSource(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, name := range []string{"index.js", "release.js", "platform.js"} {
		body, err := os.ReadFile(filepath.Join(repoRoot(t), "installer", name))
		if err != nil {
			t.Fatalf("read installer/%s: %v", name, err)
		}
		b.Write(body)
	}
	return b.String()
}

// TestInstallerResolvesTheSameFieldFromTheReleaseAPI — both read tag_name, and a release
// that is still a draft is invisible to that endpoint for both of them.
func TestInstallerResolvesTheSameFieldFromTheReleaseAPI(t *testing.T) {
	if !strings.Contains(installerSource(t), "tag_name") {
		t.Error("the installer no longer resolves the release through tag_name")
	}
}

// TestInstallerFetchesTheSameChecksumFile — one published checksums.txt covers every asset,
// so both implementations must look for it under the same name.
func TestInstallerFetchesTheSameChecksumFile(t *testing.T) {
	if !strings.Contains(installerSource(t), "checksums.txt") {
		t.Error("the installer no longer verifies against checksums.txt")
	}
}
