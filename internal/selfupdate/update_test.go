package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const (
	oldBinary = "the binary that is already installed"
	newBinary = "the binary the release publishes"
)

// fixture is a published release served over loopback: the latest-release API, the archive
// for this host, and checksums.txt. No test in this package reaches github.com, and none
// of them knows what rtdd's real version is - the tag here is one no release will carry.
type fixture struct {
	t        *testing.T
	tag      string
	body     string
	corrupt  bool
	mu       sync.Mutex
	requests []string
	srv      *httptest.Server
	// archive is built once: tarGz walks a map, so building it per request would serve
	// bytes that do not match the checksum published for them.
	archive []byte
}

func newFixture(t *testing.T, tag, body string) *fixture {
	t.Helper()
	f := &fixture{t: t, tag: tag, body: body}
	f.archive = f.buildArchive()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.URL.Path)
		f.mu.Unlock()

		archive := archiveName(f.tag, runtime.GOOS, runtime.GOARCH)
		switch r.URL.Path {
		case "/api/releases/latest":
			fmt.Fprintf(w, `{"tag_name": %q, "name": "rtdd %s"}`, f.tag, f.tag)
		case "/download/" + f.tag + "/" + archive:
			w.Write(f.archiveBytes())
		case "/download/" + f.tag + "/checksums.txt":
			sum := sha256.Sum256(f.archiveBytes())
			if f.corrupt {
				sum = sha256.Sum256([]byte("a different archive entirely"))
			}
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), archive)
		default:
			http.NotFound(w, r)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fixture) archiveBytes() []byte { return f.archive }

func (f *fixture) buildArchive() []byte {
	if runtime.GOOS == "windows" {
		return zipArchive(f.t, map[string]string{binaryName(runtime.GOOS): f.body})
	}
	return tarGz(f.t, map[string]string{"README.md": "docs", binaryName(runtime.GOOS): f.body})
}

// options returns Options pointed at this fixture, with a target file standing in for the
// installed binary.
func (f *fixture) options(target string) Options {
	return Options{
		Target:  target,
		APIURL:  f.srv.URL + "/api/releases/latest",
		BaseURL: f.srv.URL + "/download",
		Client:  f.srv.Client(),
	}
}

func (f *fixture) fetched(substr string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.requests {
		if strings.Contains(p, substr) {
			return true
		}
	}
	return false
}

// installedBinary writes a stand-in for the currently installed rtdd and returns its path.
func installedBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), binaryName(runtime.GOOS))
	if err := os.WriteFile(path, []byte(oldBinary), 0o755); err != nil {
		t.Fatalf("write the stand-in binary: %v", err)
	}
	return path
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestUpdateReplacesTheBinaryWhenTheReleaseIsNewer is the headline behaviour.
func TestUpdateReplacesTheBinaryWhenTheReleaseIsNewer(t *testing.T) {
	f := newFixture(t, "v9.9.9", newBinary)
	target := installedBinary(t)

	res, err := Update("0.1.1", f.options(target))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !res.Updated {
		t.Error("Update reported no update for a release newer than the running version")
	}
	if res.Latest != "v9.9.9" {
		t.Errorf("Result.Latest = %q, want v9.9.9", res.Latest)
	}
	if got := mustRead(t, target); got != newBinary {
		t.Errorf("the installed binary is %q, want the published one", got)
	}
}

// TestUpdateLeavesTheBinaryExecutable - a replacement that lands without the execute bit
// is indistinguishable from a broken install the next time the user types rtdd.
func TestUpdateLeavesTheBinaryExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits do not apply")
	}
	f := newFixture(t, "v9.9.9", newBinary)
	target := installedBinary(t)

	if _, err := Update("0.1.1", f.options(target)); err != nil {
		t.Fatalf("Update: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat %s: %v", target, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("the replaced binary has mode %v, which is not executable", info.Mode().Perm())
	}
}

// TestUpdateDoesNothingWhenAlreadyOnTheLatestRelease asserts both halves: no replacement,
// and no download at all. Fetching a several-megabyte archive to discover it is the one
// already installed is the behaviour this test exists to prevent.
func TestUpdateDoesNothingWhenAlreadyOnTheLatestRelease(t *testing.T) {
	f := newFixture(t, "v9.9.9", newBinary)
	target := installedBinary(t)

	res, err := Update("9.9.9", f.options(target))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if res.Updated || res.Outdated {
		t.Errorf("Update reported Updated=%v Outdated=%v for the version already installed", res.Updated, res.Outdated)
	}
	if got := mustRead(t, target); got != oldBinary {
		t.Errorf("the installed binary changed to %q", got)
	}
	if f.fetched("checksums.txt") || f.fetched(".tar.gz") || f.fetched(".zip") {
		t.Errorf("Update downloaded a release it had already decided not to install: %v", f.requests)
	}
}

// TestUpdateRefusesAnArchiveWhoseChecksumDoesNotMatch is the security-relevant case: the
// bytes are wrong, so nothing is written and the installed binary is exactly as it was.
func TestUpdateRefusesAnArchiveWhoseChecksumDoesNotMatch(t *testing.T) {
	f := newFixture(t, "v9.9.9", newBinary)
	f.corrupt = true
	target := installedBinary(t)

	if _, err := Update("0.1.1", f.options(target)); err == nil {
		t.Fatal("Update installed an archive whose checksum did not match")
	}
	if got := mustRead(t, target); got != oldBinary {
		t.Errorf("a failed verification still replaced the binary: %q", got)
	}
	if entries, err := os.ReadDir(filepath.Dir(target)); err == nil && len(entries) != 1 {
		t.Errorf("a failed verification left %d files beside the binary, want only the binary itself", len(entries))
	}
}

// TestCheckOnlyReportsWithoutWriting - the flag that exists so a user can ask the question
// without answering it.
func TestCheckOnlyReportsWithoutWriting(t *testing.T) {
	f := newFixture(t, "v9.9.9", newBinary)
	target := installedBinary(t)

	opts := f.options(target)
	opts.CheckOnly = true
	res, err := Update("0.1.1", opts)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if res.Updated {
		t.Error("--check reported Updated; it must never write")
	}
	if !res.Outdated {
		t.Error("--check did not report that a newer release exists")
	}
	if got := mustRead(t, target); got != oldBinary {
		t.Errorf("--check replaced the binary: %q", got)
	}
}

// TestUpdateRefusesToGuessFromASourceBuild covers `go build` with no ldflags, which leaves
// version at "dev" (cmd/rtdd/version.go). Replacing someone's own build with a published
// one because the two cannot be compared is the wrong guess to make silently.
func TestUpdateRefusesToGuessFromASourceBuild(t *testing.T) {
	f := newFixture(t, "v9.9.9", newBinary)
	target := installedBinary(t)

	_, err := Update("dev", f.options(target))
	if err == nil {
		t.Fatal("Update compared a source build against a release instead of refusing")
	}
	if !strings.Contains(err.Error(), "dev") {
		t.Errorf("the refusal does not name the version it could not order: %v", err)
	}
	// The sentinel is how the command tells a configuration problem (exit 2) from a
	// broken environment (exit 3). Matching on message text would make the exit code a
	// property of the wording.
	if !errors.Is(err, ErrNotComparable) {
		t.Errorf("error is not ErrNotComparable, so the command cannot pick an exit code for it: %v", err)
	}
	if got := mustRead(t, target); got != oldBinary {
		t.Errorf("a refused update still replaced the binary: %q", got)
	}
}

// TestAnExplicitTagOverridesTheComparison - naming a tag is a decision already taken, so
// it installs that tag even when it is older than what is running. This is how a source
// build installs a release, and how a bad release is rolled back.
func TestAnExplicitTagOverridesTheComparison(t *testing.T) {
	f := newFixture(t, "v0.0.1", newBinary)
	target := installedBinary(t)

	opts := f.options(target)
	opts.Tag = "v0.0.1"
	res, err := Update("9.9.9", opts)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !res.Updated {
		t.Error("an explicitly named tag was not installed")
	}
	if got := mustRead(t, target); got != newBinary {
		t.Errorf("the installed binary is %q, want the tag that was named", got)
	}
	if f.fetched("/api/") {
		t.Error("Update asked which release is latest despite being handed a tag")
	}
}

// TestUpdateReportsAnUnwritableDestinationWithoutTouchingIt is the /usr/local/bin case for
// a user who is not root.
func TestUpdateReportsAnUnwritableDestinationWithoutTouchingIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to a read-only directory")
	}
	f := newFixture(t, "v9.9.9", newBinary)
	target := installedBinary(t)
	dir := filepath.Dir(target)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	_, err := Update("0.1.1", f.options(target))
	if err == nil {
		t.Fatal("Update claimed success writing into a read-only directory")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("the error does not name the directory that could not be written: %v", err)
	}
	if !errors.Is(err, ErrNotWritable) {
		t.Errorf("error is not ErrNotWritable, so the command cannot pick an exit code for it: %v", err)
	}
	if got := mustRead(t, target); got != oldBinary {
		t.Errorf("the installed binary changed to %q", got)
	}
}
