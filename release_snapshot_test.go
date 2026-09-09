// release_snapshot_test.go closes the gap release_artifacts_test.go leaves open. That file
// cross-builds every binary in .goreleaser.yaml's matrix and inspects its executable
// format, and it reads the archive's `files:` list straight out of the config - but
// nothing has ever looked inside an archive GoReleaser actually produced. A config can
// name five files and still ship four: a `before` hook can wipe a path, a
// `format_overrides` typo can hand Windows a tarball, an `ignore` entry can quietly drop a
// platform. This file runs the real thing - scripts/release-snapshot.sh, which is
// `goreleaser release --snapshot --clean` at a pinned version - and asserts on what lands
// in build/dist.
//
// Why --skip=before: .goreleaser.yaml's before hooks end with `go test ./...`, and this
// test IS one of those tests. Running the hooks from here would re-enter this file inside
// the hook, which would run goreleaser again, without bound. The hooks are the repo's own
// CI steps (go mod tidy, rtdd-gen check, rtdd-gen verify, go test ./...); scripts/ci-local.sh
// and .github/workflows/ci.yml already run every one of them, so skipping them here drops
// no coverage. Everything after the hooks - the build matrix, the archiving, the
// format overrides, the file list - runs for real.
//
// --snapshot is what makes this safe to run from a unit test: GoReleaser refuses to
// publish in snapshot mode, so no tag is pushed, no release and no draft is created.
//
// Why the script rather than a `goreleaser` binary: #372 shipped this file guarding AC7
// with `exec.LookPath("goreleaser")` in front of it. Nothing installs GoReleaser on this
// host or on any CI runner, so the gate skipped everywhere and the package still said
// `ok`. scripts/release-snapshot.sh fetches a pinned GoReleaser with `go run`, which the
// repo can always do for itself, so the gate has no reason left to skip except a cold
// module cache with no network.
package installtest

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// shippedArchivePaths is the file list PRD-level acceptance names by hand: the binary plus
// the five documents. The binary aside, these are read back out of .goreleaser.yaml by
// archiveShippedFiles so the assertion tracks the config, and requiredShippedFiles then
// insists the config still names every one of them.
var requiredShippedFiles = []string{
	"README.md",
	"LICENSE",
	"dist/SKILL.md",
	"dist/AGENTS.md",
	"dist/cursor/rules/rtdd.mdc",
}

// snapshotDistDir is the `dist:` GoReleaser is configured with. It is deliberately not
// ./dist - that is the tracked generated-front-end tree - and it must stay untracked.
const snapshotDistDir = "build/dist"

// archiveShippedFiles returns the documents .goreleaser.yaml's archive ships alongside the
// binary, and fails if the config has stopped naming any of the required ones.
func archiveShippedFiles(t *testing.T, cfg goreleaserConfig) []string {
	t.Helper()
	if len(cfg.Archives) != 1 {
		t.Fatalf(".goreleaser.yaml declares %d archives, this gate assumes exactly 1", len(cfg.Archives))
	}
	files := cfg.Archives[0].Files
	have := map[string]bool{}
	for _, f := range files {
		have[f] = true
	}
	for _, w := range requiredShippedFiles {
		if !have[w] {
			t.Fatalf("archive %q no longer ships %q, has %v", cfg.Archives[0].ID, w, files)
		}
	}
	return files
}

// binaryNameFor is the name the binary carries inside the archive: GoReleaser appends .exe
// on Windows and nothing anywhere else.
func binaryNameFor(binary, goos string) string {
	if goos == "windows" {
		return binary + ".exe"
	}
	return binary
}

// archiveExtensionFor is the extension the archive carries, derived from the config's
// format_overrides: zip for Windows, the default tarball everywhere else.
func archiveExtensionFor(goos string) string {
	if goos == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

// archiveFileList reads the names an archive contains without extracting it, so the
// assertion costs a directory read rather than five binary writes. The reader is chosen by
// extension, which is exactly the format_overrides claim under test: handing a .zip to the
// tar reader is a failure, not a fallback.
func archiveFileList(path string) ([]string, error) {
	switch {
	case strings.HasSuffix(path, ".zip"):
		return zipFileList(path)
	case strings.HasSuffix(path, ".tar.gz"):
		return tarGzFileList(path)
	default:
		return nil, fmt.Errorf("%s has no archive extension this gate can read", filepath.Base(path))
	}
}

func zipFileList(path string) ([]string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open as zip: %w", err)
	}
	defer r.Close()
	var names []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		names = append(names, filepath.ToSlash(f.Name))
	}
	return names, nil
}

func tarGzFileList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("open as gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		names = append(names, filepath.ToSlash(h.Name))
	}
	return names, nil
}

// verifyArchiveShipsEveryPath returns nil when the listing carries every wanted path, and
// an error naming what is missing otherwise. It returns an error instead of calling
// t.Error so TestArchiveContentCheckRejectsAnArchiveMissingAShippedPath can prove the
// check actually rejects something - a gate that cannot fail is not a gate, which is the
// state the archives were in before this file existed.
func verifyArchiveShipsEveryPath(names, want []string) error {
	have := map[string]bool{}
	for _, n := range names {
		have[n] = true
	}
	var missing []string
	for _, w := range want {
		if !have[w] {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		sorted := append([]string(nil), names...)
		sort.Strings(sorted)
		return fmt.Errorf("does not ship %v; it contains %v", missing, sorted)
	}
	return nil
}

// buildDistIsGitIgnored reports whether .gitignore keeps GoReleaser's dist dir out of the
// working tree's status. The test asserts this instead of shelling out to `git status`:
// internal/contract's TestOnlyGitctxShellsOutToGit forbids any package but internal/gitctx
// from invoking git, and an ignored output dir is the property that makes the status clean
// in the first place.
func buildDistIsGitIgnored(t *testing.T, root string) bool {
	t.Helper()
	f, err := os.Open(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		switch strings.TrimSpace(s.Text()) {
		case "/build/", "/build", "build/", "build", "/build/dist/", "/build/dist":
			return true
		}
	}
	return false
}

// trackedTreeDigest fingerprints the tracked generated-front-end tree, so the snapshot run
// can be shown not to have touched it. GoReleaser cleans its own dist dir before building,
// and the whole reason .goreleaser.yaml points `dist:` at build/dist is that cleaning
// ./dist would delete these files.
func trackedTreeDigest(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\n%x\n", filepath.ToSlash(rel), sha256.Sum256(b))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// moduleFetchFailure reports whether a failed snapshot run failed because the pinned
// GoReleaser module could not be fetched. That is the one capability this repo cannot
// provide for itself - a cold module cache on a host with no network - and so the only
// thing left that may legitimately skip this gate. Everything else, the goreleaser binary
// very much included, the repo fetches on demand.
func moduleFetchFailure(out string) bool {
	for _, marker := range []string{
		"dial tcp",
		"no such host",
		"i/o timeout",
		"connection refused",
		"network is unreachable",
		"TLS handshake timeout",
		"module lookup disabled",
		"proxy.golang.org",
		"unrecognized import path",
	} {
		if strings.Contains(out, marker) {
			return true
		}
	}
	return false
}

// runSnapshotBuild builds the release archives into build/dist by running the repo's own
// script, so the gate exercises the same command a human and CI run rather than a copy of
// it that could drift.
//
// --skip=before is passed on top of the script's own flags: .goreleaser.yaml's before
// hooks end with `go test ./...`, and this test IS one of those tests, so running them
// from here would re-enter this file inside the hook without bound. The hooks are the
// repo's own CI steps and scripts/ci-local.sh runs every one of them already.
func runSnapshotBuild(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command(filepath.Join(root, snapshotScript), "--skip=before")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		return
	}
	if moduleFetchFailure(string(out)) {
		t.Skipf("SKIP: %s could not fetch the pinned GoReleaser module - a cold module cache with no network is the one thing this repo cannot provide for itself.\n%s", snapshotScript, out)
	}
	t.Fatalf("%s --skip=before: %v\n%s", snapshotScript, err, out)
}

// goreleaserBinaryName is the binary this repo deliberately does NOT depend on. It is a
// constant rather than a literal so that the source scan in release_snapshot_script_test.go
// can forbid `LookPath("goreleaser")` outright, with no exemption for the one place that
// legitimately asks - the check below, which asserts the sanitised PATH resolves nothing.
const goreleaserBinaryName = "goreleaser"

// pathWithoutGoreleaser returns a PATH that resolves `go` but not `goreleaser`: every
// directory holding a goreleaser executable is dropped, and a shim directory carrying a
// symlink to the real `go` is prepended so dropping one cannot take the toolchain with it.
func pathWithoutGoreleaser(t *testing.T) string {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go is not on PATH: %v", err)
	}
	shim := t.TempDir()
	if err := os.Symlink(goBin, filepath.Join(shim, "go")); err != nil {
		t.Fatalf("link go into the shim dir: %v", err)
	}
	kept := []string{shim}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, goreleaserBinaryName)); err == nil && !info.IsDir() {
			continue
		}
		kept = append(kept, dir)
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

// TestTheArchiveGateRunsWithNoGoreleaserBinaryOnPATH is the assertion that the gate above
// is not silently skipping. It runs that one test in a child `go test` with a PATH that
// resolves no goreleaser binary - the exact condition under which #372's gate reported
// `ok` without executing - and insists on a PASS. A SKIP here is a failure, not a pass.
func TestTheArchiveGateRunsWithNoGoreleaserBinaryOnPATH(t *testing.T) {
	root := findRepoRootForTest(t)
	t.Setenv("PATH", pathWithoutGoreleaser(t))
	if p, err := exec.LookPath(goreleaserBinaryName); err == nil {
		t.Fatalf("the sanitised PATH still resolves goreleaser at %s", p)
	}

	cmd := exec.Command("go", "test", "-count=1", "-v",
		"-run", "^"+snapshotGateName+"$", ".")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	got := string(out)
	if err != nil {
		if moduleFetchFailure(got) {
			t.Skipf("SKIP: the child run could not fetch the pinned GoReleaser module - a cold module cache with no network is the one thing this repo cannot provide for itself.\n%s", got)
		}
		t.Fatalf("go test -run %s with no goreleaser on PATH: %v\n%s", snapshotGateName, err, got)
	}
	if strings.Contains(got, "--- SKIP") {
		t.Errorf("%s skipped with no goreleaser binary on PATH; the gate must build the archives itself:\n%s", snapshotGateName, got)
	}
	if !strings.Contains(got, "--- PASS: "+snapshotGateName) {
		t.Errorf("%s did not report a PASS with no goreleaser binary on PATH:\n%s", snapshotGateName, got)
	}
}

// snapshotGateName is the gate below, named once so the child run above and the -run
// pattern that selects it cannot drift apart.
const snapshotGateName = "TestGoreleaserSnapshotShipsFiveArchivesWithEveryShippedPath"

// trackedDirs are the directories a snapshot run must leave exactly as it found them.
// dist/ is the one actually at risk - GoReleaser cleans its output dir, and pointing it at
// ./dist would delete the generated front-ends - but the release also runs `go mod tidy`
// and builds from source, so the rest are covered too rather than assumed safe.
var trackedDirs = []string{"dist", "scripts", "docs", "protocol", "adapters", "cmd", "internal"}

// trackedTreeState fingerprints every tracked directory plus the repo's root files, so the
// snapshot run can be shown to have written nothing outside build/dist. It reads the tree
// rather than asking git: internal/contract's TestOnlyGitctxShellsOutToGit forbids any
// package but internal/gitctx from invoking git.
func trackedTreeState(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	for _, d := range trackedDirs {
		fmt.Fprintf(h, "%s=%s\n", d, trackedTreeDigest(t, filepath.Join(root, d)))
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		fmt.Fprintf(h, "%s=%x\n", e.Name(), sha256.Sum256(b))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestGoreleaserSnapshotShipsFiveArchivesWithEveryShippedPath is the gate: it runs the
// release for real in snapshot mode and asserts on the archives that come out.
func TestGoreleaserSnapshotShipsFiveArchivesWithEveryShippedPath(t *testing.T) {
	root := findRepoRootForTest(t)
	cfg := loadGoreleaserConfig(t)
	shipped := archiveShippedFiles(t, cfg)
	binary := cfg.Builds[0].Binary
	if binary == "" {
		t.Fatal(".goreleaser.yaml's build declares no binary name")
	}

	if !buildDistIsGitIgnored(t, root) {
		t.Errorf("%s is GoReleaser's output dir but .gitignore does not ignore it: a snapshot run would dirty the working tree", snapshotDistDir)
	}
	treeBefore := trackedTreeState(t, root)

	// --clean wipes build/dist first, so the assertions below see this run's output only.
	runSnapshotBuild(t, root)

	if got := trackedTreeState(t, root); got != treeBefore {
		t.Errorf("the snapshot run modified the tracked tree; digest %s -> %s. Build output belongs in %s, which .gitignore ignores", treeBefore, got, snapshotDistDir)
	}

	distDir := filepath.Join(root, snapshotDistDir)
	entries, err := os.ReadDir(distDir)
	if err != nil {
		t.Fatalf("read %s: %v", snapshotDistDir, err)
	}
	var archives []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".tar.gz") || strings.HasSuffix(e.Name(), ".zip") {
			archives = append(archives, e.Name())
		}
	}
	sort.Strings(archives)

	// The expected set is the .goreleaser.yaml matrix minus its ignore list - the same
	// expansion release_artifacts_test.go uses - so adding a platform to the release
	// config cannot quietly add an archive nobody looked inside.
	matrix := releaseMatrix(cfg)
	if len(matrix) != 5 {
		t.Fatalf(".goreleaser.yaml's matrix expands to %d targets, want 5: %v", len(matrix), matrix)
	}
	wantTargets := map[string]bool{
		"linux/amd64":   false,
		"linux/arm64":   false,
		"darwin/amd64":  false,
		"darwin/arm64":  false,
		"windows/amd64": false,
	}
	for _, tg := range matrix {
		if _, ok := wantTargets[tg.String()]; !ok {
			t.Errorf(".goreleaser.yaml builds %s, which is not one of the five archives this gate covers", tg)
			continue
		}
		wantTargets[tg.String()] = true
	}
	for _, name := range []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64"} {
		if !wantTargets[name] {
			t.Errorf(".goreleaser.yaml no longer builds %s", name)
		}
	}
	for _, tg := range matrix {
		if tg.String() == "windows/arm64" {
			t.Error("windows/arm64 is in the build matrix; .goreleaser.yaml's ignore list is meant to exclude it")
		}
	}

	if len(archives) != 5 {
		t.Fatalf("%s holds %d archives, want exactly 5: %v", snapshotDistDir, len(archives), archives)
	}

	// Each matrix target must own exactly one archive, named for its os/arch and carrying
	// the extension format_overrides gives it.
	matched := map[string]bool{}
	for _, tg := range matrix {
		suffix := "_" + tg.goos + "_" + tg.goarch + archiveExtensionFor(tg.goos)
		var found string
		for _, a := range archives {
			if strings.HasSuffix(a, suffix) {
				if found != "" {
					t.Errorf("two archives end in %q: %s and %s", suffix, found, a)
				}
				found = a
			}
		}
		if found == "" {
			t.Errorf("no archive in %s is named for %s with extension %s; got %v",
				snapshotDistDir, tg, archiveExtensionFor(tg.goos), archives)
			continue
		}
		matched[found] = true

		t.Run(tg.goos+"_"+tg.goarch, func(t *testing.T) {
			names, err := archiveFileList(filepath.Join(distDir, found))
			if err != nil {
				t.Fatalf("read %s: %v", found, err)
			}
			want := append([]string{binaryNameFor(binary, tg.goos)}, shipped...)
			if err := verifyArchiveShipsEveryPath(names, want); err != nil {
				t.Errorf("the %s archive %s %v", tg, found, err)
			}
		})
	}
	for _, a := range archives {
		if !matched[a] {
			t.Errorf("%s is an archive no matrix target claims", a)
		}
	}

	// windows/arm64 is excluded by the ignore list, so nothing may be named for it.
	for _, a := range archives {
		if strings.Contains(a, "_windows_arm64") {
			t.Errorf("%s exists, but windows/arm64 is on .goreleaser.yaml's ignore list", a)
		}
	}
}

// TestArchiveContentCheckRejectsAnArchiveMissingAShippedPath proves the reader and the
// assertion above can fail, in both archive formats. Without it the gate could be green
// because it read nothing, which is indistinguishable from green because it read
// everything.
func TestArchiveContentCheckRejectsAnArchiveMissingAShippedPath(t *testing.T) {
	want := append([]string{"rtdd"}, requiredShippedFiles...)
	complete := append([]string(nil), want...)
	incomplete := []string{"rtdd", "README.md", "dist/SKILL.md", "dist/AGENTS.md", "dist/cursor/rules/rtdd.mdc"}

	dir := t.TempDir()
	for _, tc := range []struct {
		name  string
		write func(path string, entries []string) error
	}{
		{"tar_gz", writeTarGzWithEntries},
		{"zip", writeZipWithEntries},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ext := ".tar.gz"
			if tc.name == "zip" {
				ext = ".zip"
			}
			good := filepath.Join(dir, tc.name+"_good"+ext)
			bad := filepath.Join(dir, tc.name+"_bad"+ext)
			if err := tc.write(good, complete); err != nil {
				t.Fatal(err)
			}
			if err := tc.write(bad, incomplete); err != nil {
				t.Fatal(err)
			}

			names, err := archiveFileList(good)
			if err != nil {
				t.Fatalf("read the complete archive: %v", err)
			}
			if err := verifyArchiveShipsEveryPath(names, want); err != nil {
				t.Errorf("a complete archive was rejected: %v", err)
			}

			names, err = archiveFileList(bad)
			if err != nil {
				t.Fatalf("read the incomplete archive: %v", err)
			}
			err = verifyArchiveShipsEveryPath(names, want)
			if err == nil {
				t.Fatal("an archive shipping no LICENSE was accepted: the check is vacuous")
			}
			if !strings.Contains(err.Error(), "LICENSE") {
				t.Errorf("expected the rejection to name LICENSE, got: %v", err)
			}
		})
	}
}

// TestArchiveReaderRejectsTheWrongFormat is the format_overrides half of the same
// argument: the gate reads each archive with the reader its extension implies, so a
// Windows archive that is secretly a tarball has to be an error rather than a silent pass.
func TestArchiveReaderRejectsTheWrongFormat(t *testing.T) {
	dir := t.TempDir()
	tarballNamedZip := filepath.Join(dir, "rtdd_0.0.0_windows_amd64.zip")
	if err := writeTarGzWithEntries(tarballNamedZip, []string{"rtdd.exe"}); err != nil {
		t.Fatal(err)
	}
	if _, err := archiveFileList(tarballNamedZip); err == nil {
		t.Error("a tarball named .zip was read as a zip: the format assertion is vacuous")
	}

	zipNamedTarball := filepath.Join(dir, "rtdd_0.0.0_linux_amd64.tar.gz")
	if err := writeZipWithEntries(zipNamedTarball, []string{"rtdd"}); err != nil {
		t.Fatal(err)
	}
	if _, err := archiveFileList(zipNamedTarball); err == nil {
		t.Error("a zip named .tar.gz was read as a tarball: the format assertion is vacuous")
	}

	if _, err := archiveFileList(filepath.Join(dir, "checksums.txt")); err == nil {
		t.Error("a non-archive was accepted by archiveFileList")
	}
}

func writeTarGzWithEntries(path string, entries []string) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		body := []byte("x")
		if err := tw.WriteHeader(&tar.Header{Name: e, Mode: 0o644, Size: int64(len(body))}); err != nil {
			return err
		}
		if _, err := tw.Write(body); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func writeZipWithEntries(path string, entries []string) error {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte("x")); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
