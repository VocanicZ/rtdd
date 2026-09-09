// release_snapshot_script_test.go guards scripts/release-snapshot.sh — the one place this
// repo invokes GoReleaser (plan 07 Task 5's File Structure).
//
// #372 shipped release_snapshot_test.go with the right assertions and no way to run them:
// the gate looked for a `goreleaser` binary on PATH, found none on this host and none on
// any CI runner, and reported `ok`. A gate that reports ok without executing is worse than
// no gate, because it also reports that PRD #368 AC7 is discharged. The script removes the
// PATH dependency entirely — GoReleaser is fetched by `go run` at a pinned version, which
// the repo can do for itself — and these tests hold the script to the two properties the
// gate depends on: it is runnable, and it publishes nothing.
package installtest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// snapshotScript is the path, relative to the repo root, of the script that builds the
// release archives. release_snapshot_test.go's gate runs this exact file, so a change here
// is a change to what the gate proves.
const snapshotScript = "scripts/release-snapshot.sh"

// pinnedGoreleaser matches a fully pinned module version: `@v2.12.7`, never `@latest`. An
// unpinned toolchain makes the archives a moving target, so the gate would be asserting
// over output nobody chose.
var pinnedGoreleaser = regexp.MustCompile(`github\.com/goreleaser/goreleaser/v2@v\d+\.\d+\.\d+`)

// withoutComments drops whole-line comments so the forbidden-token checks below read the
// script's and the test file's CODE. Prose that names a shape in order to forbid it - this
// file's own comments do it repeatedly - is not that shape occurring.
func withoutComments(src, marker string) string {
	var kept []string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), marker) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// funcBody returns the source of one top-level function, from its declaration to the next
// one, and "" when the file declares no such function.
func funcBody(src, decl string) string {
	i := strings.Index(src, decl)
	if i < 0 {
		return ""
	}
	rest := src[i:]
	if j := strings.Index(rest[1:], "\nfunc "); j >= 0 {
		return rest[:j+1]
	}
	return rest
}

func readSnapshotScript(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(findRepoRootForTest(t), snapshotScript))
	if err != nil {
		t.Fatalf("read %s: %v", snapshotScript, err)
	}
	return string(b)
}

// TestReleaseSnapshotScriptIsExecutable is issue #222's precedent applied here: a script
// committed without its exec bit is a script every caller has to remember to prefix with
// `bash`, and the gate below calls it directly.
//
// The assertion is on the exec bits rather than on the literal 0755 the script is
// committed with. Git records one bit, not a mode: a 100755 blob checks out as 0777 minus
// the checkout umask, so the same commit is 0755 under umask 022 and 0775 under umask 002.
// Pinning the literal would fail on the second host while the committed file is correct.
func TestReleaseSnapshotScriptIsExecutable(t *testing.T) {
	path := filepath.Join(findRepoRootForTest(t), snapshotScript)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", snapshotScript, err)
	}
	if mode := info.Mode().Perm(); mode&0o555 != 0o555 {
		t.Errorf("%s is mode %04o; it must be readable and executable by everyone (committed 0755)", snapshotScript, mode)
	}
}

// TestReleaseSnapshotScriptFetchesGoreleaserItselfAtAPinnedVersion is the whole point of
// the script: the repo supplies its own GoReleaser, so the gate cannot be turned off by an
// unprovisioned host.
func TestReleaseSnapshotScriptFetchesGoreleaserItselfAtAPinnedVersion(t *testing.T) {
	src := readSnapshotScript(t)

	if !strings.Contains(src, "go run") {
		t.Errorf("%s does not run GoReleaser through `go run`, so it needs the binary provisioned for it", snapshotScript)
	}
	if !pinnedGoreleaser.MatchString(src) {
		t.Errorf("%s names no pinned github.com/goreleaser/goreleaser/v2@vX.Y.Z; source:\n%s", snapshotScript, src)
	}
	if strings.Contains(src, "goreleaser/v2@latest") {
		t.Errorf("%s tracks @latest; the archives the gate asserts over must come from a version this repo chose", snapshotScript)
	}
	code := withoutComments(src, "#")
	for _, forbidden := range []string{"command -v goreleaser", "which goreleaser", "LookPath"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("%s consults %q: the script must not depend on a goreleaser binary on PATH", snapshotScript, forbidden)
		}
	}
}

// TestReleaseSnapshotScriptCreatesNoTagReleaseOrDraft is PRD #368's global constraint. The
// script exists to be run casually, from a test and from CI, so it has to be incapable of
// touching GitHub state — not merely unlikely to.
func TestReleaseSnapshotScriptCreatesNoTagReleaseOrDraft(t *testing.T) {
	src := readSnapshotScript(t)
	code := withoutComments(src, "#")

	for _, forbidden := range []string{"git tag", "git push", "gh release", "gh repo edit"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("%s contains %q; a snapshot build must create no tag, no release and no draft", snapshotScript, forbidden)
		}
	}
	if !strings.Contains(src, "--snapshot") {
		t.Errorf("%s does not pass --snapshot, so GoReleaser would demand a tag and publish against it", snapshotScript)
	}
	if !strings.Contains(src, "--skip=publish") {
		t.Errorf("%s does not pass --skip=publish", snapshotScript)
	}
	if !strings.Contains(src, "--clean") {
		t.Errorf("%s does not pass --clean, so the gate could inspect a previous run's archives", snapshotScript)
	}
}

// TestReleaseSnapshotScriptWritesOnlyToTheIgnoredDistDir keeps the script from being the
// thing that dirties the tree: .goreleaser.yaml points `dist:` at build/dist precisely
// because ./dist is the tracked generated front-end tree, and the script must not override
// that back to the default.
func TestReleaseSnapshotScriptWritesOnlyToTheIgnoredDistDir(t *testing.T) {
	if strings.Contains(withoutComments(readSnapshotScript(t), "#"), "--dist") {
		t.Errorf("%s overrides --dist; .goreleaser.yaml's `dist: %s` is what keeps the tracked dist/ tree alive", snapshotScript, snapshotDistDir)
	}
}

// TestNoOtherScriptInvokesGoreleaser holds the plan's "the ONLY place this repo invokes
// GoReleaser" claim. Two entry points drift; one does not.
func TestNoOtherScriptInvokesGoreleaser(t *testing.T) {
	dir := filepath.Join(findRepoRootForTest(t), "scripts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read scripts/: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Join("scripts", e.Name()) == snapshotScript {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read scripts/%s: %v", e.Name(), err)
		}
		// A reference to the config file by name is documentation, not an invocation;
		// what must stay unique is the command that runs the tool.
		body := strings.ReplaceAll(string(b), ".goreleaser.yaml", "")
		if strings.Contains(body, "goreleaser") {
			t.Errorf("scripts/%s invokes goreleaser; %s is meant to be the only place this repo does", e.Name(), snapshotScript)
		}
	}
}

// goreleaserDependentTests are the two files whose assertions only mean anything if a
// GoReleaser run actually happened: the archive gate, and install.sh proven against the
// archives that run produced.
var goreleaserDependentTests = []string{"release_snapshot_test.go", "install_snapshot_test.go"}

// TestTheArchiveGateNeverSkipsForAMissingBinary reads those files' own source. The
// regression this milestone is fixing was invisible in a green run: the gate skipped, the
// package said ok, and nothing in the suite objected. The source is where that shape is
// cheapest to forbid outright.
func TestTheArchiveGateNeverSkipsForAMissingBinary(t *testing.T) {
	root := findRepoRootForTest(t)
	for _, name := range goreleaserDependentTests {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		src := string(b)
		code := withoutComments(src, "//")

		if strings.Contains(code, `LookPath("goreleaser")`) {
			t.Errorf("%s looks a goreleaser binary up on PATH; %s fetches it, so absence from PATH is not a reason to skip", name, snapshotScript)
		}

		// Every skip has to name a capability this repo genuinely cannot provide for
		// itself. There is exactly one: fetching the pinned module with a cold cache and no
		// network. A skip for anything the repo can install is the bug, not a concession to it.
		rest := src
		for {
			i := strings.Index(rest, "t.Skip")
			if i < 0 {
				break
			}
			rest = rest[i+len("t.Skip"):]
			msg := rest
			if len(msg) > 400 {
				msg = msg[:400]
			}
			if !strings.Contains(msg, "cold module cache") {
				t.Errorf("a t.Skip in %s does not name the cold module cache as its reason: %.200q", name, msg)
			}
		}
	}

	// The gate's own body may neither look the binary up nor skip: it builds the archives
	// through the script, and the only skip it can reach is runSnapshotBuild's, which the
	// message scan above holds to the cold module cache.
	b, err := os.ReadFile(filepath.Join(root, "release_snapshot_test.go"))
	if err != nil {
		t.Fatalf("read release_snapshot_test.go: %v", err)
	}
	gate := funcBody(withoutComments(string(b), "//"), "func "+snapshotGateName+"(")
	if gate == "" {
		t.Fatalf("release_snapshot_test.go declares no %s", snapshotGateName)
	}
	for _, forbidden := range []string{"LookPath", "t.Skip"} {
		if strings.Contains(gate, forbidden) {
			t.Errorf("%s calls %s in its own body; %s fetches GoReleaser, so absence from PATH is not a reason to skip", snapshotGateName, forbidden, snapshotScript)
		}
	}
}
