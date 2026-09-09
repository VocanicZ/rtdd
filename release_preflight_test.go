// release_preflight_test.go exercises scripts/release-preflight.sh (issue #201, plan Task 24's
// agent half) end to end, the way install_test.go exercises install.sh: by running the real
// script as a subprocess against a fixture repo. go/uv/gh are replaced by tiny stubs on PATH so
// every branch is reachable without a real toolchain, network, or a nested `go test ./...` (the
// script runs `go test ./...` as one of its own steps, so a real `go` there would make this test
// invoke itself recursively). git is never stubbed and this file never shells out to it directly
// - internal/contract's TestNoGitMockExistsInTheTree and TestOnlyGitctxShellsOutToGit forbid
// both, repo-wide - so the fixture is a real repository built through internal/gitctx/gittest,
// and the prereg-m4 tag is a real tag or genuinely absent.
package installtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

const toolStub = `#!/bin/sh
set -eu
prog="$(basename "$0")"
case "$prog" in
  go)
    if [ "$1" = "run" ]; then
      case "$3" in
        check) exit "${STUB_CHECK_EXIT:-0}" ;;
        verify) exit "${STUB_VERIFY_EXIT:-0}" ;;
      esac
    fi
    if [ "$1" = "test" ]; then
      # The release-artifact linkage check is the only go test the script runs with
      # -run, so it gets its own exit control: a repo can have a green suite and still
      # produce a dynamically linked artifact, and the report has to say so separately.
      for arg in "$@"; do
        if [ "$arg" = "-run" ]; then
          exit "${STUB_LINKAGE_EXIT:-0}"
        fi
      done
      exit "${STUB_GOTEST_EXIT:-0}"
    fi
    exit 0
    ;;
  uv)
    if [ "$2" = "pytest" ]; then
      exit "${STUB_PYTEST_EXIT:-0}"
    fi
    if [ "$2" = "python" ] && [ "$3" = "preflight.py" ]; then
      exit "${STUB_PREFLIGHT_EXIT:-0}"
    fi
    exit 0
    ;;
  gh)
    if [ "${STUB_GH_FAIL:-0}" = "1" ]; then
      echo "gh: could not determine visibility" >&2
      exit 1
    fi
    echo "${STUB_GH_VISIBILITY:-PRIVATE}"
    exit 0
    ;;
esac
exit 0
`

// writeToolStubs drops go/uv/gh shims into dir, all sharing toolStub's dispatch-by-basename
// body. Deliberately excludes git: this repo's policy is that git is never mocked.
func writeToolStubs(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"go", "uv", "gh"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(toolStub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func findRepoRootForTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

// prereqCommitDate is the date gittest.InitRepo's pinned GIT_AUTHOR_DATE/GIT_COMMITTER_DATE
// renders as with `--date=short`, which is what scripts/release-preflight.sh asks git for.
const prereqCommitDate = "2026-01-01"

// buildFixtureRepo lays out the minimum tree scripts/release-preflight.sh reads - the real
// script (so a fix to it is exercised, not a copy that could drift), the four placeholder-grep
// targets, and docs/outcomes/SELECTED - as a real git repository (via gittest, never a mock),
// and returns the repo dir plus its HEAD short sha.
func buildFixtureRepo(t *testing.T, extraReadme string, selected string, taggedPrereg bool) (dir, headSHA string) {
	t.Helper()
	repoRoot := findRepoRootForTest(t)
	scriptSrc, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "release-preflight.sh"))
	if err != nil {
		t.Fatal(err)
	}

	dir = t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Issue #222: the copy inherits the mode of the file it was read from. Chmodding it to
	// 0o755 unconditionally manufactured the very bit the repo was missing, so these tests
	// stayed green while `scripts/release-preflight.sh` was unrunnable in a fresh clone.
	mustWrite("scripts/release-preflight.sh", string(scriptSrc))
	scriptInfo, err := os.Stat(filepath.Join(repoRoot, "scripts", "release-preflight.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "scripts", "release-preflight.sh"), scriptInfo.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	mustWrite("README.md", "# rtdd\n"+extraReadme)
	mustWrite("protocol/PROTOCOL.md", "# protocol\n")
	mustWrite("dist/SKILL.md", "# skill\n")
	mustWrite("bench/PREREGISTRATION.md", "status: UNSIGNED\n")
	mustWrite("docs/outcomes/SELECTED", selected+"\n")
	if err := os.MkdirAll(filepath.Join(dir, "bench", "swebench"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := gittest.InitRepo(dir, "fixture"); err != nil {
		t.Fatal(err)
	}
	headSHA = gittest.HeadShort(t, dir)
	if taggedPrereg {
		gittest.Run(t, dir, "tag", "prereg-m4")
	}
	return dir, headSHA
}

// runPreflight runs the fixture repo's copy of the script with the given stub-controlling env
// vars layered on top of a PATH that resolves go/uv/gh to stubs before any real toolchain, while
// leaving the real `git` on PATH untouched.
func runPreflight(t *testing.T, repoDir string, extraEnv map[string]string) (stdout string, exitCode int) {
	t.Helper()
	stubDir := t.TempDir()
	writeToolStubs(t, stubDir)

	cmd := exec.Command(filepath.Join(repoDir, "scripts", "release-preflight.sh"))
	cmd.Dir = repoDir
	env := append(os.Environ(), "PATH="+stubDir+":"+os.Getenv("PATH"))
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	err := cmd.Run()
	exitCode = 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running release-preflight.sh: %v", err)
	}
	return outBuf.String(), exitCode
}

func TestReleasePreflightReportsUnreadyRepoAndRefusesToExitZero(t *testing.T) {
	repo, _ := buildFixtureRepo(t, "", "positive", false)
	out, code := runPreflight(t, repo, map[string]string{
		"STUB_CHECK_EXIT":     "0",
		"STUB_VERIFY_EXIT":    "0",
		"STUB_GOTEST_EXIT":    "0",
		"STUB_PYTEST_EXIT":    "0",
		"STUB_PREFLIGHT_EXIT": "3",
		"STUB_GH_VISIBILITY":  "PRIVATE",
	})

	if code == 0 {
		t.Fatalf("expected nonzero exit for an unsigned pre-registration, got 0. stdout:\n%s", out)
	}
	want := map[string]string{
		"Branch taken at Task 14:": "positive",
		"Front-end checks:":        "pass",
		"Placeholders:":            "none",
		"Pre-registration tag:":    "prereg-m4 -> missing",
		"Current visibility:":      "private",
	}
	for prefix, mustContain := range want {
		line := findLine(t, out, prefix)
		if !strings.Contains(line, mustContain) {
			t.Errorf("line %q: want to contain %q", line, mustContain)
		}
	}
	if !strings.Contains(out, "DECISION REQUIRED") {
		t.Errorf("missing DECISION REQUIRED block:\n%s", out)
	}
}

func TestReleasePreflightReportsFullyGreenRepo(t *testing.T) {
	repo, headSHA := buildFixtureRepo(t, "", "positive", true)
	if err := os.MkdirAll(filepath.Join(repo, "bench", "results", "swebench"), 0o755); err != nil {
		t.Fatal(err)
	}
	tables := "## Pre-registered kill criterion\n\nResult: **MET**.\n"
	if err := os.WriteFile(filepath.Join(repo, "bench", "results", "swebench", "tables.md"), []byte(tables), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runPreflight(t, repo, map[string]string{
		"STUB_CHECK_EXIT":     "0",
		"STUB_VERIFY_EXIT":    "0",
		"STUB_GOTEST_EXIT":    "0",
		"STUB_PYTEST_EXIT":    "0",
		"STUB_PREFLIGHT_EXIT": "0",
		"STUB_GH_VISIBILITY":  "PUBLIC",
	})

	if code != 0 {
		t.Fatalf("expected exit 0 for a fully green repo, got %d. stdout:\n%s", code, out)
	}
	want := map[string]string{
		"Kill criterion:":       "MET",
		"Front-end checks:":     "pass",
		"Test suite:":           "pass",
		"Release binaries:":     "pass",
		"Pre-registration tag:": "prereg-m4 -> " + headSHA + " " + prereqCommitDate,
		"Current visibility:":   "public",
	}
	for prefix, mustContain := range want {
		line := findLine(t, out, prefix)
		if !strings.Contains(line, mustContain) {
			t.Errorf("line %q: want to contain %q", line, mustContain)
		}
	}
}

// TestReleasePreflightFailsOnAnUnverifiedReleaseArtifact is issue #212's half of the
// pre-flight: the script is meant to be a pre-flight for the binaries, so an artifact that
// is not self-contained has to stop it and be named in the decision block, even when every
// other check is green.
func TestReleasePreflightFailsOnAnUnverifiedReleaseArtifact(t *testing.T) {
	repo, _ := buildFixtureRepo(t, "", "positive", true)
	out, code := runPreflight(t, repo, map[string]string{
		"STUB_CHECK_EXIT":     "0",
		"STUB_VERIFY_EXIT":    "0",
		"STUB_GOTEST_EXIT":    "0",
		"STUB_LINKAGE_EXIT":   "1",
		"STUB_PYTEST_EXIT":    "0",
		"STUB_PREFLIGHT_EXIT": "0",
		"STUB_GH_VISIBILITY":  "PUBLIC",
	})

	if code == 0 {
		t.Fatalf("expected nonzero exit when a release artifact is not statically linked, got 0. stdout:\n%s", out)
	}
	line := findLine(t, out, "Release binaries:")
	if !strings.Contains(line, "fail") {
		t.Errorf("line %q: want to contain %q", line, "fail")
	}
	// The failure must be attributed to the artifacts, not blamed on the test suite, which
	// is green here.
	line = findLine(t, out, "Test suite:")
	if !strings.Contains(line, "pass") {
		t.Errorf("line %q: want to contain %q", line, "pass")
	}
}

func TestReleasePreflightReportsPlaceholdersAndMissingUvExplicitly(t *testing.T) {
	repo, _ := buildFixtureRepo(t, "TODO: fill this in\n", "negative", false)

	stubDir := t.TempDir()
	// No uv stub written: STUB_PYTEST_EXIT is irrelevant because `command -v uv` must fail.
	for _, name := range []string{"go", "gh"} {
		if err := os.WriteFile(filepath.Join(stubDir, name), []byte(toolStub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(filepath.Join(repo, "scripts", "release-preflight.sh"))
	cmd.Dir = repo
	// A real uv may be installed elsewhere on the host PATH (this dev machine has one at
	// ~/.local/bin/uv); to actually simulate "uv absent" the PATH must exclude it, not just
	// omit a stub for it, so it's rebuilt from only the stub dir plus the base system dirs
	// (which is also where the real `git` this test relies on lives).
	cmd.Env = append(os.Environ(),
		"PATH="+stubDir+":/usr/bin:/bin",
		"STUB_CHECK_EXIT=1",
		"STUB_VERIFY_EXIT=0",
		"STUB_GH_FAIL=1",
	)
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	err := cmd.Run()
	out := outBuf.String()
	if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("expected the script to exit nonzero, got err=%v\n%s", err, out)
	}

	if !strings.Contains(out, "TODO: fill this in") {
		t.Errorf("placeholder hit not reported in output:\n%s", out)
	}
	line := findLine(t, out, "Branch taken at Task 14:")
	if !strings.Contains(line, "negative") {
		t.Errorf("line %q: want to contain %q", line, "negative")
	}
	line = findLine(t, out, "Front-end checks:")
	if !strings.Contains(line, "fail") {
		t.Errorf("line %q: want to contain %q", line, "fail")
	}
	line = findLine(t, out, "Test suite:")
	if !strings.Contains(strings.ToLower(line), "uv") {
		t.Errorf("line %q: missing uv absent should be called out explicitly, not silently passed", line)
	}
	line = findLine(t, out, "Current visibility:")
	if !strings.Contains(line, "unknown") {
		t.Errorf("line %q: want to contain %q", line, "unknown")
	}
}

func findLine(t *testing.T, text, prefix string) string {
	t.Helper()
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, prefix) {
			return l
		}
	}
	t.Fatalf("no line found matching %q in:\n%s", prefix, text)
	return ""
}

func TestReleasePreflightNeverMutatesRepositoryState(t *testing.T) {
	repoRoot := findRepoRootForTest(t)
	src, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "release-preflight.sh"))
	if err != nil {
		t.Fatal(err)
	}
	var codeLines []string
	for _, l := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue // explanatory comments (including this guarantee's own description) don't count
		}
		codeLines = append(codeLines, l)
	}
	text := strings.Join(codeLines, "\n")
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`gh\s+repo\s+edit`),
		regexp.MustCompile(`git\s+tag`),
		regexp.MustCompile(`git\s+push`),
		regexp.MustCompile(`gh\s+release`),
	}
	for _, re := range forbidden {
		if re.MatchString(text) {
			t.Errorf("scripts/release-preflight.sh contains a mutating command matching %s", re)
		}
	}
}

func TestReleasePreflightIsDocumentedInDevelopmentMd(t *testing.T) {
	repoRoot := findRepoRootForTest(t)
	doc, err := os.ReadFile(filepath.Join(repoRoot, "DEVELOPMENT.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "release-preflight.sh") {
		t.Errorf("DEVELOPMENT.md does not mention scripts/release-preflight.sh")
	}
}

// realPreregContent is the repository's actual bench/PREREGISTRATION.md. The fixture repo
// uses it verbatim so these tests describe the state main is really in — three fields
// deliberately empty, `status: UNSIGNED` — rather than a hand-written stand-in that could
// drift from it.
func realPreregContent(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(findRepoRootForTest(t), "bench", "PREREGISTRATION.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// mustMatchLine asserts that some line of text matches re, and returns it. Matching is by
// regexp rather than by literal substring so the assertions are tolerant of the column
// widths in the decision block's heredoc.
func mustMatchLine(t *testing.T, text string, re *regexp.Regexp) string {
	t.Helper()
	for _, l := range strings.Split(text, "\n") {
		if re.MatchString(l) {
			return l
		}
	}
	t.Errorf("no line matching %s in:\n%s", re, text)
	return ""
}

// TestReleasePreflightReportsFourPassesOnTheRepoAsItStandsToday is issue #375's assertion.
//
// The fixture is main as it actually stands: every automated check green, the real
// (unsigned) bench/PREREGISTRATION.md, no prereg-m4 tag, and therefore preflight.py
// refusing with exit 3 — the exact refusal scripts/ci-prereg.sh treats as PASS for an
// unsigned pre-registration. In that state all four automated verdicts must read as passes.
// A launch gate correctly declining to launch a run that has not been authorised is not a
// failing test suite, and reporting it as one made the release checklist blame `go test`
// and pytest for a human decision that has simply not been taken yet.
func TestReleasePreflightReportsFourPassesOnTheRepoAsItStandsToday(t *testing.T) {
	repo, _ := buildFixtureRepo(t, "", "positive", false)
	if err := os.WriteFile(filepath.Join(repo, "bench", "PREREGISTRATION.md"), []byte(realPreregContent(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Commit(t, repo, "real pre-registration")

	out, _ := runPreflight(t, repo, map[string]string{
		"STUB_CHECK_EXIT":     "0",
		"STUB_VERIFY_EXIT":    "0",
		"STUB_GOTEST_EXIT":    "0",
		"STUB_LINKAGE_EXIT":   "0",
		"STUB_PYTEST_EXIT":    "0",
		"STUB_PREFLIGHT_EXIT": "3",
		"STUB_GH_VISIBILITY":  "PRIVATE",
	})

	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`Front-end checks:\s+pass\s*$`),
		regexp.MustCompile(`Test suite:\s+pass\s*$`),
		regexp.MustCompile(`Release binaries:\s+pass\s*$`),
		regexp.MustCompile(`Placeholders:\s+none\s*$`),
	} {
		mustMatchLine(t, out, re)
	}

	// The two verdicts that are correctly not green today must stay exactly as they are:
	// neither may be manufactured to make the block look finished.
	mustMatchLine(t, out, regexp.MustCompile(`Pre-registration tag:\s+prereg-m4 -> missing`))
	mustMatchLine(t, out, regexp.MustCompile(`Kill criterion:\s+unknown`))
}

// TestReleasePreflightStillRefusesTheTwoIrreversibleActions guards the feature the script
// exists for. Making the four automated verdicts green must not turn the checklist into
// something that acts: both irreversible calls stay with a human, named and numbered.
func TestReleasePreflightStillRefusesTheTwoIrreversibleActions(t *testing.T) {
	repo, _ := buildFixtureRepo(t, "", "positive", false)
	out, _ := runPreflight(t, repo, map[string]string{"STUB_PREFLIGHT_EXIT": "3"})

	if !strings.Contains(out, "DECISION REQUIRED — repository visibility and release") {
		t.Errorf("the DECISION REQUIRED heading is gone:\n%s", out)
	}
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`1\.\s+Make the repository public`),
		regexp.MustCompile(`2\.\s+Push a v\* tag`),
		regexp.MustCompile(`I will not do either of these`),
	} {
		mustMatchLine(t, out, want)
	}
}

// TestReleasePreflightLeavesTheRepositoryUntouched is the runtime half of the guarantee
// TestReleasePreflightNeverMutatesRepositoryState makes by reading the source: after a full
// run against a real repository, the working tree is clean, no tag was created, and
// bench/PREREGISTRATION.md is byte-for-byte what it was.
func TestReleasePreflightLeavesTheRepositoryUntouched(t *testing.T) {
	repo, _ := buildFixtureRepo(t, "", "positive", false)
	preregPath := filepath.Join(repo, "bench", "PREREGISTRATION.md")
	if err := os.WriteFile(preregPath, []byte(realPreregContent(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Commit(t, repo, "real pre-registration")
	before, err := os.ReadFile(preregPath)
	if err != nil {
		t.Fatal(err)
	}

	runPreflight(t, repo, map[string]string{"STUB_PREFLIGHT_EXIT": "3"})

	if status := gittest.Run(t, repo, "status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Errorf("the script dirtied the working tree:\n%s", status)
	}
	if tags := strings.TrimSpace(gittest.Run(t, repo, "tag", "--list")); tags != "" {
		t.Errorf("the script created a tag: %q", tags)
	}
	after, err := os.ReadFile(preregPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("bench/PREREGISTRATION.md was rewritten by the script")
	}
}

// TestReleasePreflightOnThisRepoReportsFourPasses runs the script against THIS repository
// with only `go` stubbed — the one substitution the test cannot avoid, because the script
// runs `go test ./...` as a step and a real `go` there would make this test invoke itself.
// uv, python and the real bench/swebench/preflight.py are the genuine articles, so this is
// the assertion that reads the actual pre-registration state on disk rather than a fixture's
// copy of it. It skips where uv is absent (the CI `test` job has no uv; the `prereg` job and
// scripts/ci-local.sh do).
func TestReleasePreflightOnThisRepoReportsFourPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the real bench/swebench suite")
	}
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv is not on PATH — see DEVELOPMENT.md")
	}
	repoRoot := findRepoRootForTest(t)
	stubDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stubDir, "go"), []byte(toolStub), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(filepath.Join(repoRoot, "scripts", "release-preflight.sh"))
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "PATH="+stubDir+":"+os.Getenv("PATH"))
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("running release-preflight.sh: %v\n%s", err, outBuf.String())
		}
	}
	out := outBuf.String()

	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`Front-end checks:\s+pass\s*$`),
		regexp.MustCompile(`Test suite:\s+pass\s*$`),
		regexp.MustCompile(`Release binaries:\s+pass\s*$`),
		regexp.MustCompile(`Placeholders:\s+none\s*$`),
	} {
		mustMatchLine(t, out, re)
	}
}
