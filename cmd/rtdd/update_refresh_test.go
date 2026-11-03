package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
	"github.com/VocanicZ/rtdd/internal/selfupdate"
)

// #566: an upgrade from an older rtdd must install the skill for every agent the NEW release
// supports, so the refresh after a replacement is the new binary's to run; and every
// `rtdd update`, already-current included, re-checks for an agent that lacks the skill.

// TestHelperProcessRtdd is not a test: run by a fake release's binary with
// RTDD_TEST_AS_BINARY set, it is rtdd — this tree's code, the "new" release.
func TestHelperProcessRtdd(t *testing.T) {
	if os.Getenv("RTDD_TEST_AS_BINARY") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	os.Exit(run(args, os.Stdout, os.Stderr))
}

// newRelease writes the binary a fake `rtdd update` installs: a script that logs its
// arguments to the file it returns and, when asRtdd, runs as rtdd.
func newRelease(t *testing.T, asRtdd bool) (bin, argv string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake release is a shell script")
	}
	dir := t.TempDir()
	bin, argv = filepath.Join(dir, "rtdd"), filepath.Join(dir, "argv")
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\n", argv)
	if asRtdd {
		script += fmt.Sprintf("RTDD_TEST_AS_BINARY=1 exec %q -test.run='^TestHelperProcessRtdd$' -- \"$@\"\n", os.Args[0])
	}
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, argv
}

// withRelease makes `rtdd update` report res instead of downloading anything.
func withRelease(t *testing.T, res selfupdate.Result) {
	t.Helper()
	saved := selfUpdate
	selfUpdate = func(string, selfupdate.Options) (selfupdate.Result, error) { return res, nil }
	t.Cleanup(func() { selfUpdate = saved })
}

// v030Home is a home directory as rtdd v0.3.0 left it — Claude Code's and Codex's skills,
// nothing else of rtdd's — on a machine where Hermes Agent is also installed.
func v030Home(t *testing.T) string {
	home := fakeHome(t, ".claude", ".codex", ".hermes")
	old := "---\nname: rtdd\n---\n\n" + protocol.Generated + "\n\nv0.3.0's skill\n"
	writeFile(t, home, ".claude/skills/rtdd/SKILL.md", old)
	writeFile(t, home, ".codex/skills/rtdd/SKILL.md", old)
	return home
}

const hermesSkill = ".hermes/skills/rtdd/SKILL.md"

func exists(t *testing.T, home, rel string) bool {
	t.Helper()
	_, err := os.Lstat(filepath.Join(home, filepath.FromSlash(rel)))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

// The acceptance test: v0.3.0's install plus ~/.hermes, `rtdd update` to a newer build, and
// Hermes has the skill — written by the new binary, which the update ran.
func TestUpdateFromV030InstallsTheSkillForAnAgentTheNewReleaseSupports(t *testing.T) {
	home := v030Home(t)
	bin, argv := newRelease(t, true)
	withVersion(t, "0.3.0")
	withRelease(t, selfupdate.Result{Current: "0.3.0", Latest: "v9.9.9", Target: bin, Outdated: true, Updated: true})

	code, stdout, stderr := rtdd(t, newTestRepo(t), "update")
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr)
	}
	if got := strings.TrimSpace(readRepoFileForTest(t, filepath.Dir(argv), "argv")); got != "skill install --refresh" {
		t.Errorf("the new binary ran with %q, want `skill install --refresh`", got)
	}
	files, err := install.Files()
	if err != nil {
		t.Fatal(err)
	}
	want := files[protocol.GlobalSkillPath]
	for _, rel := range []string{hermesSkill, ".claude/skills/rtdd/SKILL.md", ".agents/skills/rtdd/SKILL.md"} {
		if !exists(t, home, rel) {
			t.Errorf("after the update %s does not exist:\n%s", rel, stdout)
			continue
		}
		if got := homeFile(t, home, rel); got != want {
			t.Errorf("%s is not the new release's skill:\n%s", rel, got)
		}
	}
	if exists(t, home, ".codex/skills/rtdd") {
		t.Error("v0.3.0's ~/.codex/skills/rtdd/ survived the update")
	}
	if !strings.Contains(stdout, "linked the agent skill at "+hermesSkill) {
		t.Errorf("the output does not say Hermes's skill was installed:\n%s", stdout)
	}
}

// The refresh is the new binary's alone: the old process, whose agent table is the one being
// replaced, writes nothing itself. With a new binary that only logs, nothing is installed.
func TestUpdateLeavesTheRefreshToTheNewBinary(t *testing.T) {
	home := v030Home(t)
	bin, argv := newRelease(t, false)
	withVersion(t, "0.3.0")
	withRelease(t, selfupdate.Result{Current: "0.3.0", Latest: "v9.9.9", Target: bin, Outdated: true, Updated: true})

	if code, _, stderr := rtdd(t, newTestRepo(t), "update"); code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr)
	}
	if !exists(t, filepath.Dir(argv), "argv") {
		t.Fatal("the new binary was never run")
	}
	if exists(t, home, hermesSkill) {
		t.Error("the old process installed Hermes's skill from its own plan")
	}
	if got := homeFile(t, home, ".claude/skills/rtdd/SKILL.md"); !strings.Contains(got, "v0.3.0's skill") {
		t.Error("the old process refreshed Claude Code's skill from its own renders")
	}
}

// A new binary that cannot refresh — an older release rolled back to, without --refresh —
// does not fail an update that happened; the note says what to run.
func TestUpdateNotesARefreshTheNewBinaryCouldNotDo(t *testing.T) {
	fakeHome(t, ".hermes")
	withVersion(t, "0.4.0")
	withRelease(t, selfupdate.Result{Current: "0.4.0", Latest: "v0.3.0", Target: filepath.Join(t.TempDir(), "missing"), Updated: true})
	code, _, stderr := rtdd(t, newTestRepo(t), "update")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "rtdd skill install") {
		t.Errorf("stderr does not say how to refresh by hand: %s", stderr)
	}
}

// An already-current `rtdd update` restores an agent skill that was deleted, and installs
// one for an agent added since.
func TestAlreadyCurrentUpdateRestoresADeletedAgentSkill(t *testing.T) {
	home := fakeHome(t, ".claude", ".hermes")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("skill install: %d %s", code, stderr)
	}
	if err := os.RemoveAll(filepath.Join(home, ".hermes", "skills", "rtdd")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".openclaw"), 0o755); err != nil {
		t.Fatal(err)
	}
	withVersion(t, "0.4.0")
	withRelease(t, selfupdate.Result{Current: "0.4.0", Latest: "v0.4.0", Target: "/nonexistent/rtdd"})

	code, stdout, stderr := rtdd(t, newTestRepo(t), "update")
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr)
	}
	for _, rel := range []string{hermesSkill, ".openclaw/skills/rtdd/SKILL.md"} {
		if !exists(t, home, rel) {
			t.Errorf("an already-current update did not restore %s:\n%s", rel, stdout)
		}
	}
}

// --check writes nothing, the re-check included.
func TestUpdateCheckInstallsNoMissingSkill(t *testing.T) {
	home := fakeHome(t, ".hermes")
	withVersion(t, "0.4.0")
	withRelease(t, selfupdate.Result{Current: "0.4.0", Latest: "v0.4.0"})
	if code, _, stderr := rtdd(t, newTestRepo(t), "update", "--check"); code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr)
	}
	if exists(t, home, hermesSkill) {
		t.Error("update --check installed a skill")
	}
}

// `rtdd skill install --refresh` is what the new binary runs: it never fails on a
// hand-written front-end — it leaves it alone, says so, and installs the rest.
func TestSkillInstallRefreshLeavesAHandWrittenSkillAloneAndExitsZero(t *testing.T) {
	home := fakeHome(t, ".claude", ".hermes")
	writeFile(t, home, ".claude/skills/rtdd/SKILL.md", "my own\n")
	code, _, stderr := rtdd(t, t.TempDir(), "skill", "install", "--refresh")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if got := homeFile(t, home, ".claude/skills/rtdd/SKILL.md"); got != "my own\n" {
		t.Errorf("--refresh overwrote a hand-written skill: %q", got)
	}
	if !strings.Contains(stderr, "left .claude/skills/rtdd/SKILL.md alone") {
		t.Errorf("stderr does not say the hand-written skill was left alone: %s", stderr)
	}
	if !exists(t, home, hermesSkill) {
		t.Error("--refresh did not install Hermes's skill")
	}
}

func TestSkillInstallRefreshRejectsDryRunAndForce(t *testing.T) {
	fakeHome(t)
	for _, extra := range []string{"--dry-run", "--force"} {
		if code, _, _ := rtdd(t, t.TempDir(), "skill", "install", "--refresh", extra); code != 2 {
			t.Errorf("--refresh %s: exit code = %d, want 2", extra, code)
		}
	}
}

// `rtdd init` installs the skill for an agent that lacks it — and only that: a stale skill
// is the update's to refresh, never init's.
func TestInitInstallsAMissingAgentSkillAndRefreshesNothing(t *testing.T) {
	home := fakeHome(t, ".claude", ".hermes")
	stale := protocol.Generated + "\nan older release\n"
	writeFile(t, home, ".claude/skills/rtdd/SKILL.md", stale)

	code, stdout, stderr := rtdd(t, newTestRepo(t), "init")
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr)
	}
	if !exists(t, home, hermesSkill) || !exists(t, home, install.CanonicalSkill) {
		t.Errorf("init did not install Hermes's missing skill:\n%s", stdout)
	}
	if got := homeFile(t, home, ".claude/skills/rtdd/SKILL.md"); got != stale {
		t.Errorf("init refreshed an existing skill: %q", got)
	}
}

func TestInitDryRunInstallsNoMissingSkill(t *testing.T) {
	home := fakeHome(t, ".hermes")
	if code, _, stderr := rtdd(t, newTestRepo(t), "init", "--dry-run"); code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr)
	}
	if exists(t, home, hermesSkill) {
		t.Error("init --dry-run installed a skill")
	}
}

// A linked agent whose canonical copy was deleted has a dangling link: its skill is missing,
// and `rtdd init` restores the copy the link resolves to.
func TestInitRestoresTheCanonicalCopyADanglingLinkNeeds(t *testing.T) {
	home := fakeHome(t, ".hermes")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("skill install: %d %s", code, stderr)
	}
	if err := os.RemoveAll(filepath.Join(home, ".local", "share", "rtdd")); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := rtdd(t, newTestRepo(t), "init"); code != 0 {
		t.Fatalf("init: %d %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(hermesSkill))); err != nil {
		t.Errorf("Hermes's skill still does not resolve after init: %v", err)
	}
}
