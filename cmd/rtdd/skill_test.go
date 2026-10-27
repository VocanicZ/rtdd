package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
)

// fakeHome points the machine-wide install at a temporary directory holding exactly the
// named agent config directories, and returns its path.
func fakeHome(t *testing.T, agents ...string) string {
	t.Helper()
	home := t.TempDir()
	for _, a := range agents {
		if err := os.MkdirAll(filepath.Join(home, a), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", a, err)
		}
	}
	t.Setenv("RTDD_HOME", home)
	return home
}

func homeFile(t *testing.T, home, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// This is the whole point of the feature: installing the binary must leave a skill an
// agent can find, without the user having to run anything inside a repository first.
func TestSkillInstallWritesTheClaudeSkill(t *testing.T) {
	home := fakeHome(t, ".claude")
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "skill", "install")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, ".claude/skills/rtdd/SKILL.md") {
		t.Errorf("output does not name the installed skill:\n%s", stdout)
	}

	body := homeFile(t, home, ".claude/skills/rtdd/SKILL.md")
	if !strings.HasPrefix(body, "---\n") || !strings.Contains(body, "name: rtdd") {
		t.Errorf("installed skill has no usable frontmatter:\n%s", body[:min(200, len(body))])
	}
	if !strings.Contains(body, "rtdd init") {
		t.Error("the machine-wide skill never mentions `rtdd init`, so it cannot bootstrap a new repo")
	}
}

// `rtdd skill install` runs from install.sh on machines that have none of these agents.
// It must succeed and say what it skipped, not fail and not create the directories.
func TestSkillInstallSucceedsAndCreatesNothingWhenNoAgentIsInstalled(t *testing.T) {
	home := fakeHome(t)
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "skill", "install")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 on a machine with no agent installed (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "skip") {
		t.Errorf("output does not report what it skipped:\n%s", stdout)
	}
	for _, d := range []string{".claude", ".codex", ".gemini"} {
		if _, err := os.Stat(filepath.Join(home, d)); !os.IsNotExist(err) {
			t.Errorf("created %s for an agent that is not installed", d)
		}
	}
}

// #509 acceptance: with a fake HOME, `rtdd skill install` creates only rtdd-owned
// directories, and ~/.codex/AGENTS.md and ~/.gemini/GEMINI.md are untouched.
func TestSkillInstallCreatesOnlyRtddOwnedDirectories(t *testing.T) {
	home := fakeHome(t, ".claude", ".codex/skills/.system")
	writeFile(t, home, ".gemini/settings.json", "{}\n")
	shared := map[string]string{".codex/AGENTS.md": "# Mine\n\nkeep me\n", ".gemini/GEMINI.md": "# Gemini, mine\n"}
	for rel, body := range shared {
		writeFile(t, home, rel, body)
	}
	before := map[string]bool{}
	for _, f := range worktreeFiles(t, home) {
		before[f] = true
	}
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "skill", "install")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for rel, body := range shared {
		if got := homeFile(t, home, rel); got != body {
			t.Errorf("~/%s = %q after install, want it untouched %q", rel, got, body)
		}
	}
	var created []string
	for _, f := range worktreeFiles(t, home) {
		if !before[f] {
			created = append(created, f)
		}
	}
	want := ".agents/skills/rtdd/SKILL.md,.claude/skills/rtdd/SKILL.md,.gemini/extensions/rtdd/GEMINI.md,.gemini/extensions/rtdd/gemini-extension.json"
	if strings.Join(created, ",") != want {
		t.Errorf("install created %v, want %s", created, want)
	}
	if strings.Contains(stdout, "legacy") {
		t.Errorf("install reported legacy files in a home with none:\n%s", stdout)
	}
}

// A block an older `rtdd skill install` merged into a shared file is reported, not edited.
func TestSkillInstallReportsAnOlderInstallsBlock(t *testing.T) {
	home := fakeHome(t, ".codex")
	old := "# Mine\n\n" + protocol.BeginMarker + "\nold\n" + protocol.EndMarker + "\n"
	writeFile(t, home, ".codex/AGENTS.md", old)
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "skill", "install")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "legacy         ~/.codex/AGENTS.md") || !strings.Contains(stdout, "rtdd uninstall --legacy-files") {
		t.Errorf("install did not report the old block:\n%s", stdout)
	}
	if got := homeFile(t, home, ".codex/AGENTS.md"); got != old {
		t.Errorf("install edited ~/.codex/AGENTS.md:\n%s", got)
	}
}

func TestSkillInstallDryRunWritesNothing(t *testing.T) {
	home := fakeHome(t, ".claude")
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "skill", "install", "--dry-run")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, ".claude/skills/rtdd/SKILL.md") {
		t.Errorf("dry run did not print the plan:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(".claude/skills/rtdd/SKILL.md"))); !os.IsNotExist(err) {
		t.Error("--dry-run wrote the skill")
	}
}

func TestSkillUninstallRemovesWhatInstallWrote(t *testing.T) {
	home := fakeHome(t, ".claude")
	dir := newTestRepo(t)

	if code, _, stderr := rtdd(t, dir, "skill", "install"); code != 0 {
		t.Fatalf("install: code = %d, stderr = %s", code, stderr)
	}
	if code, _, stderr := rtdd(t, dir, "skill", "uninstall"); code != 0 {
		t.Fatalf("uninstall: code = %d, stderr = %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(".claude/skills/rtdd/SKILL.md"))); !os.IsNotExist(err) {
		t.Error("uninstall left the global skill behind")
	}
}

// `rtdd skill prompt` is the answer for every agent rtdd cannot write to itself — Cursor's
// User Rules in its app settings, and anything else the user happens to run. It has
// to be self-contained, because it gets pasted into a chat window with no shell behind it,
// and it must work on a machine where no supported agent is installed at all.
func TestSkillPromptIsSelfContainedAndNeedsNoInstalledAgent(t *testing.T) {
	fakeHome(t)
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "skill", "prompt")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, protocol.GlobalSkillDescription) {
		t.Error("prompt does not carry the skill's description, so a pasted copy has no trigger")
	}
	if !strings.Contains(stdout, "rtdd init") {
		t.Error("prompt does not carry the setup instructions")
	}
	if !strings.Contains(stdout, "rtdd which") {
		t.Error("prompt does not carry the command reference")
	}
}

// The prompt's own preamble, outside the rendered skill, is the first thing the agent reads.
// It said v0.2's "runs only the tests that cover the code you changed, using coverage" after
// v0.3.0 shipped, contradicting the skill below it (PRD #411 AC7).
func TestSkillPromptPreambleDescribesV030(t *testing.T) {
	preamble, _, ok := strings.Cut(RenderSkillPrompt("body\n"), "````markdown")
	if !ok {
		t.Fatal("the prompt has no fenced document")
	}
	if strings.Contains(strings.ToLower(preamble), "coverage") {
		t.Errorf("the prompt's preamble describes v0.2's coverage selector:\n%s", preamble)
	}
	for _, want := range []string{"in rounds", "rtdd runs no tests"} {
		if !strings.Contains(preamble, want) {
			t.Errorf("the prompt's preamble does not say %q:\n%s", want, preamble)
		}
	}
}

func TestSkillRejectsAnUnknownSubcommand(t *testing.T) {
	fakeHome(t)
	dir := newTestRepo(t)

	code, _, stderr := rtdd(t, dir, "skill", "frobnicate")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 for an unknown subcommand", code)
	}
	if !strings.Contains(stderr, "usage") {
		t.Errorf("stderr does not show usage:\n%s", stderr)
	}
}

// A successful `rtdd update` leaves a NEW binary behind, and that binary renders a
// different global skill than the one on disk. Without a refresh the machine keeps
// whichever skill the first install wrote, forever, while the binary moves on — so the
// two drift apart silently and the skill starts describing commands that have changed.
func TestRefreshGlobalSkillRewritesAnOlderReleasesSkill(t *testing.T) {
	home := fakeHome(t, ".claude")
	rel := filepath.FromSlash(".claude/skills/rtdd/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(home, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := "---\nname: rtdd\n---\n\n" + protocol.Generated + "\n\n# rtdd\n\nan older release\n"
	if err := os.WriteFile(filepath.Join(home, rel), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut strings.Builder
	refreshGlobalSkill(&out, &errOut)

	got := homeFile(t, home, ".claude/skills/rtdd/SKILL.md")
	if strings.Contains(got, "an older release") {
		t.Errorf("the stale skill survived the refresh:\n%s", got)
	}
	if !strings.Contains(got, protocol.GlobalSkillDescription) {
		t.Error("the refreshed skill is not this binary's rendering")
	}
}

// unplannedLegacy keeps every piece of footprint the plan leaves in place and drops the one
// it deletes.
func TestUnplannedLegacyDropsWhatThePlanDeletes(t *testing.T) {
	home := fakeHome(t, ".codex")
	writeFile(t, home, ".codex/skills/rtdd/SKILL.md", protocol.Generated+"\nv0.3.0's skill\n")
	writeFile(t, home, ".codex/AGENTS.md", "# Mine\n\n"+protocol.BeginMarker+"\nold\n"+protocol.EndMarker+"\n")
	var all []string
	for _, l := range globalLegacy() {
		all = append(all, l.Path)
	}
	if strings.Join(all, ",") != "~/.codex/AGENTS.md,~/.codex/skills/rtdd/" {
		t.Fatalf("globalLegacy = %v, want the AGENTS.md block and the retired directory", all)
	}
	got := unplannedLegacy([]install.Step{{Path: ".codex/skills/rtdd/", Action: install.Delete}})
	if len(got) != 1 || got[0].Path != "~/.codex/AGENTS.md" {
		t.Errorf("unplannedLegacy = %+v, want only ~/.codex/AGENTS.md", got)
	}
}

// The refresh runs from `rtdd update`, which has already replaced the binary by the time it
// is called. A home directory it cannot write must not turn a successful update into a
// failure — the update DID happen, and saying otherwise would send the user to re-run it.
func TestRefreshGlobalSkillIsSilentWhenNoAgentIsInstalled(t *testing.T) {
	fakeHome(t)
	var out, errOut strings.Builder
	refreshGlobalSkill(&out, &errOut)
	if errOut.Len() != 0 {
		t.Errorf("refresh complained on a machine with no agent installed: %s", errOut.String())
	}
}

// A plan a command would refuse to execute is a lie, and a plan is the one output a user
// reads as a promise — the same reasoning `rtdd init` applies to its own gate. So
// --dry-run has to print the machine-wide half of the plan too, not just the repo half.
func TestUninstallGlobalDryRunPrintsThePlanAndChangesNothing(t *testing.T) {
	// Codex, not Claude Code: the repo-scoped plan already lists
	// ".claude/skills/rtdd/SKILL.md" as a path relative to the REPOSITORY, so asserting on
	// that string cannot tell the machine-wide half of the plan from the repo half.
	// ".agents/skills/rtdd/" appears only in the machine-wide plan.
	home := fakeHome(t, ".codex/skills")
	dir := newTestRepo(t)
	if code, _, stderr := rtdd(t, dir, "skill", "install"); code != 0 {
		t.Fatalf("skill install: code = %d, stderr = %s", code, stderr)
	}

	code, stdout, stderr := rtdd(t, dir, "uninstall", "--global", "--dry-run")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, ".agents/skills/rtdd/") {
		t.Errorf("--dry-run --global did not print the machine-wide plan:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(".agents/skills/rtdd/SKILL.md"))); err != nil {
		t.Errorf("--dry-run removed the global front-end: %v", err)
	}
}

func TestUninstallGlobalRemovesTheMachineWideSkill(t *testing.T) {
	home := fakeHome(t, ".claude")
	dir := newTestRepo(t)
	if code, _, stderr := rtdd(t, dir, "skill", "install"); code != 0 {
		t.Fatalf("skill install: code = %d, stderr = %s", code, stderr)
	}

	if code, _, stderr := rtdd(t, dir, "uninstall", "--global"); code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(".claude/skills/rtdd/SKILL.md"))); !os.IsNotExist(err) {
		t.Error("uninstall --global left the machine-wide skill behind")
	}
}
