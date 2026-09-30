// install_snapshot_test.go closes the last gap in the install story. install_test.go
// already drives the installer end to end against an httptest server, but every byte it
// serves is a fixture this repo tarred itself: a hand-rolled archive holding one `go
// build` binary. Nothing has ever proven the script installs an archive the *release
// pipeline* produced. That is the "fresh machine" path - `curl … | sh` against real
// GitHub release assets - and the difference is not cosmetic. The real archive is built
// by GoReleaser with its own ldflags, carries five extra documents next to the binary,
// is named by .goreleaser.yaml's name_template, and is listed in a checksums.txt
// GoReleaser wrote. Any one of those could drift out from under the installer's
// `rtdd_${VERSION_NUM}_${OS}_${ARCH}.tar.gz`, its single-member `tar -xzf … rtdd`, or its
// `grep " ${ARCHIVE}$" checksums.txt` and the fixture tests would stay green.
//
// So this file builds the real archives - scripts/release-snapshot.sh, which is
// `goreleaser release --snapshot --clean` at a pinned version, into build/dist - serves
// those exact files plus GoReleaser's own checksums.txt and a
// GitHub-shaped `releases/latest` body from the same local fixture-server technique
// install_test.go uses, and installs from them into a temp dir.
//
// Nothing is tagged, released or published: --snapshot makes GoReleaser refuse to
// publish, and every URL the installer is handed points at 127.0.0.1, so no fetch reaches
// github.com. The assertions below prove that last part rather than assuming it - the
// server records every path it is asked for.
//
// Why --skip=before: .goreleaser.yaml's before hooks end with `go test ./...`, and this
// test IS one of those tests; running the hooks here would re-enter this file inside the
// hook without bound. scripts/ci-local.sh and .github/workflows/ci.yml run every one of
// those hooks already. This is the same trade release_snapshot_test.go makes.
package installtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// snapshotMetadata is the subset of GoReleaser's build/dist/metadata.json this file needs.
// Version is where the snapshot version string comes from - `{{ incpatch .Version }}-next`
// per .goreleaser.yaml's snapshot.version_template - so the assertions track whatever
// GoReleaser stamped into the binary instead of hardcoding a literal that would rot the
// moment the repo is tagged.
type snapshotMetadata struct {
	ProjectName string `json:"project_name"`
	Tag         string `json:"tag"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
}

// snapshotRelease is one `goreleaser release --snapshot` run's output.
type snapshotRelease struct {
	distDir string
	meta    snapshotMetadata
}

// releaseTag is the directory a real GitHub release serves its assets under. GoReleaser
// names archives from .Version (no leading v) but publishes them beneath the tag (with
// one), so a real download URL reads …/releases/download/v1.2.3/rtdd_1.2.3_linux_amd64.tar.gz.
// The fixture server reproduces that shape rather than a flattened one, because the installer
// builds the two halves of that URL from the same string and a test that served both
// halves identically would not notice if it stopped.
func (r snapshotRelease) releaseTag() string { return "v" + r.meta.Version }

// archiveFor is the archive name .goreleaser.yaml's name_template produces for a target.
func (r snapshotRelease) archiveFor(goos, goarch string) string {
	return fmt.Sprintf("rtdd_%s_%s_%s.tar.gz", r.meta.Version, goos, goarch)
}

var (
	snapshotOnce   sync.Once
	snapshotResult *snapshotRelease
	snapshotErr    error
)

// requireSnapshotRelease builds the real release archives once per test binary and returns
// them, through scripts/release-snapshot.sh - the same command release_snapshot_test.go's
// gate runs, and the only place this repo invokes GoReleaser. It used to look a
// `goreleaser` binary up on PATH and skip when it found none, which meant this file's
// end-to-end install proof never ran anywhere (issue #385). The script fetches a pinned
// GoReleaser with `go run`, so the only thing left that can stop it is a cold module cache
// with no network.
func requireSnapshotRelease(t *testing.T) *snapshotRelease {
	t.Helper()
	root := findRepoRootForTest(t)

	// The tracked dist/ front-end tree lives next to GoReleaser's build/dist, and
	// GoReleaser cleans its dist dir before building. Fingerprint the tracked tree so a
	// misconfigured `dist:` cannot quietly delete it and leave the working tree dirty.
	frontEndBefore := trackedTreeDigest(t, filepath.Join(root, "dist"))

	snapshotOnce.Do(func() {
		snapshotResult, snapshotErr = buildSnapshotRelease(root)
	})
	if snapshotErr != nil {
		if moduleFetchFailure(snapshotErr.Error()) {
			t.Skipf("SKIP: %s could not fetch the pinned GoReleaser module - a cold module cache with no network is the one thing this repo cannot provide for itself: %v", snapshotScript, snapshotErr)
		}
		t.Fatalf("building the snapshot release: %v", snapshotErr)
	}

	if got := trackedTreeDigest(t, filepath.Join(root, "dist")); got != frontEndBefore {
		t.Errorf("the snapshot run modified the tracked dist/ front-end tree; digest %s -> %s", frontEndBefore, got)
	}
	if !buildDistIsGitIgnored(t, root) {
		t.Errorf("%s holds this test's build output but .gitignore does not ignore it: the run would dirty the working tree", snapshotDistDir)
	}
	return snapshotResult
}

func buildSnapshotRelease(root string) (*snapshotRelease, error) {
	cmd := exec.Command(filepath.Join(root, snapshotScript), "--skip=before")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s --skip=before: %w\n%s", snapshotScript, err, out)
	}
	distDir := filepath.Join(root, snapshotDistDir)
	raw, err := os.ReadFile(filepath.Join(distDir, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("read the build's own metadata: %w", err)
	}
	var meta snapshotMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("parse %s/metadata.json: %w", snapshotDistDir, err)
	}
	if meta.Version == "" {
		return nil, fmt.Errorf("%s/metadata.json carries no version: %s", snapshotDistDir, raw)
	}
	if !strings.HasSuffix(meta.Version, "-next") {
		return nil, fmt.Errorf("metadata version %q does not look like .goreleaser.yaml's snapshot.version_template output", meta.Version)
	}
	return &snapshotRelease{distDir: distDir, meta: meta}, nil
}

// snapshotFixtureServer serves the real archives over HTTP in GitHub's asset layout and
// records every path it is asked for, so the tests can prove the installer talked to nobody
// else.
type snapshotFixtureServer struct {
	url string

	mu        sync.Mutex
	requested []string
}

func (s *snapshotFixtureServer) record(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requested = append(s.requested, path)
}

func (s *snapshotFixtureServer) paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requested...)
}

func (s *snapshotFixtureServer) sawRequestFor(suffix string) bool {
	for _, p := range s.paths() {
		if strings.HasSuffix(p, suffix) {
			return true
		}
	}
	return false
}

// serveSnapshotRelease publishes build/dist over httptest at /<tag>/<file>, the shape
// GitHub release asset URLs take. checksums is the checksums.txt body to serve - callers
// pass GoReleaser's own, or a corrupted copy of it. apiBody, when non-empty, is served at
// apiPath so one server can back both RTDD_BASE_URL and RTDD_API_URL.
func serveSnapshotRelease(t *testing.T, rel *snapshotRelease, checksums, apiBody string) *snapshotFixtureServer {
	t.Helper()
	fx := &snapshotFixtureServer{}
	prefix := "/" + rel.releaseTag() + "/"

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fx.record(r.URL.Path)
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, prefix)
		if name == "checksums.txt" {
			w.Write([]byte(checksums))
			return
		}
		// Only real release assets are servable, and only by base name: a request that
		// escaped build/dist would be a hole in the fixture, not a download.
		if name == "" || strings.Contains(name, "/") {
			http.NotFound(w, r)
			return
		}
		if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".zip") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(rel.distDir, name))
	})
	if apiBody != "" {
		mux.HandleFunc(apiPath, func(w http.ResponseWriter, r *http.Request) {
			fx.record(r.URL.Path)
			w.Write([]byte(apiBody))
		})
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	fx.url = srv.URL
	return fx
}

// goreleaserChecksums is the checksums.txt GoReleaser wrote for this build, read from disk
// rather than recomputed: the point of the checksum assertions is that the installer agrees
// with the release pipeline's file, so recomputing it here would test nothing.
func goreleaserChecksums(t *testing.T, rel *snapshotRelease) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(rel.distDir, "checksums.txt"))
	if err != nil {
		t.Fatalf("read the checksums.txt GoReleaser produced: %v", err)
	}
	return string(b)
}

// corruptChecksumFor returns GoReleaser's checksums.txt with one archive's digest replaced
// by 64 zeros, which no non-empty file hashes to. It fails when the named archive has no
// entry, so a rename in name_template shows up as a broken fixture rather than a test that
// silently corrupts nothing.
func corruptChecksumFor(t *testing.T, checksums, archive string) string {
	t.Helper()
	lines := strings.Split(checksums, "\n")
	found := false
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == archive {
			lines[i] = strings.Repeat("0", 64) + "  " + archive
			found = true
		}
	}
	if !found {
		t.Fatalf("checksums.txt has no entry for %s; it holds:\n%s", archive, checksums)
	}
	return strings.Join(lines, "\n")
}

// assertInstalledSnapshotBinary runs the installed binary and insists it reports the
// version GoReleaser stamped into this build.
func assertInstalledSnapshotBinary(t *testing.T, installDir string, rel *snapshotRelease, installOutput string) {
	t.Helper()
	installed := filepath.Join(installDir, "rtdd")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("rtdd was not installed at %s: %v\ninstall output:\n%s", installed, err, installOutput)
	}
	out, err := exec.Command(installed, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version failed: %v\n%s\ninstall output:\n%s", installed, err, out, installOutput)
	}
	if !strings.Contains(string(out), rel.meta.Version) {
		t.Errorf("%s --version = %q, want it to report the snapshot version %q GoReleaser stamped in",
			installed, out, rel.meta.Version)
	}
}

// assertOnlyTheFixtureServerWasAsked proves the install fetched the archive and the
// checksums from the local server, which is the only evidence available in-process that no
// request went to github.com - the installer is handed 127.0.0.1 URLs and its PATH carries
// nothing that could reach the network on its own.
func assertOnlyTheFixtureServerWasAsked(t *testing.T, fx *snapshotFixtureServer, tag, archive string) {
	t.Helper()
	for _, want := range []string{"/" + tag + "/" + archive, "/" + tag + "/checksums.txt"} {
		if !fx.sawRequestFor(want) {
			t.Errorf("the fixture server was never asked for %s; it served %v", want, fx.paths())
		}
	}
}

// TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion is the fresh-machine path
// with a version pinned: the installer is pointed at the archives `goreleaser release
// --snapshot` just produced and must install the one for this host.
func TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion(t *testing.T) {
	osName, arch := fixtureOSArch(t)
	rel := requireSnapshotRelease(t)

	archive := rel.archiveFor(osName, arch)
	if _, err := os.Stat(filepath.Join(rel.distDir, archive)); err != nil {
		t.Fatalf("the snapshot build produced no archive for this host (%s/%s): %v", osName, arch, err)
	}
	fx := serveSnapshotRelease(t, rel, goreleaserChecksums(t, rel), "")

	code, output, installDir := runInstaller(t,
		"RTDD_BASE_URL="+fx.url,
		"RTDD_VERSION="+rel.releaseTag(),
	)
	if code != 0 {
		t.Fatalf("the installer exited %d installing the real snapshot archive:\n%s", code, output)
	}
	if !strings.Contains(output, "downloading "+archive) {
		t.Errorf("output = %q, want it to download the GoReleaser-named archive %q", output, archive)
	}
	assertInstalledSnapshotBinary(t, installDir, rel, output)
	assertOnlyTheFixtureServerWasAsked(t, fx, rel.releaseTag(), archive)
	assertNothingLeakedOutsideTheTempInstallDir(t, installDir)
}

// TestInstallFromRealSnapshotArchivesResolvesTheLatestRelease is the same path every real
// `curl … | sh` user takes: no RTDD_VERSION, so the installer reads the tag off the releases
// API and installs that release's archive. The API body is GitHub's shape, carrying the
// tag the snapshot archives are named for.
func TestInstallFromRealSnapshotArchivesResolvesTheLatestRelease(t *testing.T) {
	osName, arch := fixtureOSArch(t)
	rel := requireSnapshotRelease(t)

	archive := rel.archiveFor(osName, arch)
	tag := rel.releaseTag()
	apiBody := fmt.Sprintf("{\n  \"id\": 1,\n  \"tag_name\": %q,\n  \"name\": %q\n}\n", tag, tag)
	fx := serveSnapshotRelease(t, rel, goreleaserChecksums(t, rel), apiBody)

	code, output, installDir := runInstaller(t,
		"RTDD_BASE_URL="+fx.url,
		"RTDD_API_URL="+fx.url+apiPath,
		"RTDD_VERSION=",
	)
	if code != 0 {
		t.Fatalf("the installer exited %d resolving and installing the latest release:\n%s", code, output)
	}
	if !strings.Contains(output, "resolving the latest rtdd release") {
		t.Errorf("output = %q, want it to report resolving the latest release", output)
	}
	// The archive name is derived from the resolved tag, so naming it proves which
	// version was resolved without re-implementing the installer's parser here.
	if !strings.Contains(output, "downloading "+archive+" ("+tag+")") {
		t.Errorf("output = %q, want it to download %q for resolved tag %q", output, archive, tag)
	}
	if !fx.sawRequestFor(apiPath) {
		t.Errorf("the releases API was never queried; the server served %v", fx.paths())
	}
	assertInstalledSnapshotBinary(t, installDir, rel, output)
	assertOnlyTheFixtureServerWasAsked(t, fx, tag, archive)
	assertNothingLeakedOutsideTheTempInstallDir(t, installDir)
}

// TestInstallFromRealSnapshotArchivesAbortsOnACorruptedChecksum is the other half of the
// checksum claim. The pinned test above proves the installer accepts the digest GoReleaser
// wrote; without this one that would be indistinguishable from the installer not checking at
// all. Same real archive, same real checksums.txt with this host's entry corrupted.
func TestInstallFromRealSnapshotArchivesAbortsOnACorruptedChecksum(t *testing.T) {
	osName, arch := fixtureOSArch(t)
	rel := requireSnapshotRelease(t)

	archive := rel.archiveFor(osName, arch)
	corrupted := corruptChecksumFor(t, goreleaserChecksums(t, rel), archive)
	fx := serveSnapshotRelease(t, rel, corrupted, "")

	code, output, installDir := runInstaller(t,
		"RTDD_BASE_URL="+fx.url,
		"RTDD_VERSION="+rel.releaseTag(),
	)
	if code == 0 {
		t.Fatalf("the installer exited 0 on a corrupted checksum for the real archive:\n%s", output)
	}
	if !strings.Contains(output, "checksum mismatch") {
		t.Errorf("output = %q, want it to name a checksum mismatch", output)
	}
	if _, err := os.Stat(filepath.Join(installDir, "rtdd")); err == nil {
		t.Errorf("rtdd was installed at %s despite a corrupted checksum", installDir)
	}
	assertOnlyTheFixtureServerWasAsked(t, fx, rel.releaseTag(), archive)
}

// assertNothingLeakedOutsideTheTempInstallDir pins the blast radius of a test that runs a
// real installer: the binary must land in a temp dir, never in the repo working tree and
// never anywhere under the developer's own $HOME. runInstall already hands the installer a
// throwaway HOME and TMPDIR; this checks the destination it was given is as disposable as
// those, so a future edit to the env cannot quietly start writing to ~/.local/bin.
func assertNothingLeakedOutsideTheTempInstallDir(t *testing.T, installDir string) {
	t.Helper()
	abs, err := filepath.Abs(installDir)
	if err != nil {
		t.Fatal(err)
	}
	root := findRepoRootForTest(t)
	if rel, err := filepath.Rel(root, abs); err == nil && !strings.HasPrefix(rel, "..") {
		t.Errorf("the installer installed into %s, which is inside the repo working tree at %s", abs, root)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, abs); err == nil && !strings.HasPrefix(rel, "..") {
			t.Errorf("the installer installed into %s, which is inside the real $HOME at %s", abs, home)
		}
	}
	if _, err := os.Stat(filepath.Join(abs, "rtdd")); err != nil {
		t.Fatalf("the temp install dir %s holds no rtdd: %v", abs, err)
	}
}
