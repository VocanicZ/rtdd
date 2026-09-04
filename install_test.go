// Package installtest exercises install.sh end to end: it serves a GoReleaser-shaped
// fixture (an archive plus checksums.txt) over a local HTTP server, runs the script
// against an isolated PATH prefix and HOME, and asserts the installed binary works and
// that a corrupted checksum aborts the install cleanly.
package installtest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testVersion is the release tag install.sh is told to install. Its numeric form (no
// leading "v") appears in the archive name, matching .goreleaser.yaml's name_template.
const testVersion = "v0.0.0-test"

func versionNum() string { return strings.TrimPrefix(testVersion, "v") }

// archiveName mirrors .goreleaser.yaml: name_template: "rtdd_{{.Version}}_{{.Os}}_{{.Arch}}".
func archiveName(osName, arch string) string {
	return fmt.Sprintf("rtdd_%s_%s_%s.tar.gz", versionNum(), osName, arch)
}

// buildFixtureBinary compiles cmd/rtdd for the host OS/arch with a known version baked in
// via ldflags, the same -X mechanism GoReleaser uses in production.
func buildFixtureBinary(t *testing.T, dir string) string {
	t.Helper()
	out := filepath.Join(dir, "rtdd")
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version="+versionNum(), "-o", out, "./cmd/rtdd")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build cmd/rtdd: %v\n%s", err, b)
	}
	return out
}

// buildArchive tars the given binary as "rtdd" and gzips it, matching the layout
// install.sh expects to extract a single named member from.
func buildArchive(t *testing.T, binPath string) []byte {
	t.Helper()
	bin, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "rtdd", Mode: 0o755, Size: int64(len(bin))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(bin); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// serveFixture starts a server that serves /<version>/<file>, exactly like GitHub release
// asset download URLs, for the one archive+checksums pair given.
func serveFixture(t *testing.T, archiveFilename string, archive []byte, checksums string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/"+testVersion+"/"+archiveFilename, func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/"+testVersion+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// isolatedPATH symlinks exactly the external commands install.sh can call into a fresh
// directory and returns it: the child process gets an "empty PATH prefix" carrying
// nothing except what the script itself checks for up front.
func isolatedPATH(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	tools := []string{
		"uname", "tar", "gzip", "mktemp", "curl", "wget", "sha256sum", "shasum",
		"grep", "awk", "sed", "chmod", "mv", "mkdir", "rm",
	}
	for _, name := range tools {
		p, err := exec.LookPath(name)
		if err != nil {
			continue // optional: only one of curl/wget and one of sha256sum/shasum must exist
		}
		if err := os.Symlink(p, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	return bin
}

// runInstall runs install.sh with an isolated PATH and HOME plus the given env overrides,
// and reports its exit code, combined output, and the install directory it was told to use.
func runInstall(t *testing.T, extraEnv ...string) (code int, output, installDir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found on PATH")
	}
	installDir = filepath.Join(t.TempDir(), "bin")

	env := []string{
		"PATH=" + isolatedPATH(t),
		"HOME=" + t.TempDir(),
		"TMPDIR=" + t.TempDir(),
		"RTDD_VERSION=" + testVersion,
		"RTDD_INSTALL_DIR=" + installDir,
	}
	env = append(env, extraEnv...)

	cmd := exec.Command(shPath, filepath.Join(wd, "install.sh"))
	cmd.Env = env
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			return ee.ExitCode(), string(out), installDir
		}
		t.Fatalf("running install.sh: %v", runErr)
	}
	return 0, string(out), installDir
}

// fixtureOSArch skips the test on a host install.sh does not support, so this file only
// asserts what install.sh actually claims to do (AC4: linux/darwin, amd64/arm64).
func fixtureOSArch(t *testing.T) (osName, arch string) {
	t.Helper()
	osName = runtime.GOOS
	if osName != "linux" && osName != "darwin" {
		t.Skip("install.sh only supports linux/darwin")
	}
	arch = runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		t.Skip("install.sh only supports amd64/arm64")
	}
	return osName, arch
}

func TestInstallOnACleanPath(t *testing.T) {
	osName, arch := fixtureOSArch(t)
	bin := buildFixtureBinary(t, t.TempDir())
	archiveFilename := archiveName(osName, arch)
	archive := buildArchive(t, bin)
	checksums := sha256Hex(archive) + "  " + archiveFilename + "\n"

	baseURL := serveFixture(t, archiveFilename, archive, checksums)

	code, output, installDir := runInstall(t, "RTDD_BASE_URL="+baseURL)
	if code != 0 {
		t.Fatalf("install.sh exited %d:\n%s", code, output)
	}

	installed := filepath.Join(installDir, "rtdd")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("rtdd was not installed at %s: %v\noutput:\n%s", installed, err, output)
	}

	out, err := exec.Command(installed, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version failed: %v\n%s", installed, err, out)
	}
	if !strings.Contains(string(out), versionNum()) {
		t.Errorf("%s --version = %q, want it to name the installed version %q", installed, out, versionNum())
	}
}

func TestInstallAbortsOnACorruptedChecksum(t *testing.T) {
	osName, arch := fixtureOSArch(t)
	bin := buildFixtureBinary(t, t.TempDir())
	archiveFilename := archiveName(osName, arch)
	archive := buildArchive(t, bin)
	// Deliberately wrong: 64 zeros never matches a real sha256 of a non-empty archive.
	checksums := strings.Repeat("0", 64) + "  " + archiveFilename + "\n"

	baseURL := serveFixture(t, archiveFilename, archive, checksums)

	code, output, installDir := runInstall(t, "RTDD_BASE_URL="+baseURL)
	if code == 0 {
		t.Fatalf("install.sh exited 0 on a corrupted checksum:\n%s", output)
	}
	if !strings.Contains(output, "checksum mismatch") {
		t.Errorf("output = %q, want it to name a checksum mismatch", output)
	}
	if _, err := os.Stat(filepath.Join(installDir, "rtdd")); err == nil {
		t.Errorf("rtdd was installed at %s despite a corrupted checksum", installDir)
	}
}
