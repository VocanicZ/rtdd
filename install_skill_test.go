// install_skill_test.go covers the half of the installer that exists so a user who runs the
// one-line install ends up with an agent skill, not just a binary. Before this, the
// installer placed `rtdd` and stopped, and nothing on the machine told any coding agent that
// rtdd existed — `rtdd init` had to be discovered and run by hand, per repository.
package installtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// homeWithAgents builds a fake home directory holding the named agent config directories.
func homeWithAgents(t *testing.T, agents ...string) string {
	t.Helper()
	home := t.TempDir()
	for _, a := range agents {
		if err := os.MkdirAll(filepath.Join(home, a), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// installFixture serves one archive for this host and returns the base URL.
func installFixture(t *testing.T) string {
	t.Helper()
	osName, arch := fixtureOSArch(t)
	bin := buildFixtureBinary(t, t.TempDir())
	archive := buildArchive(t, bin)
	name := archiveName(osName, arch)
	return serveFixture(t, name, archive, sha256Hex(archive)+"  "+name+"\n")
}

// The point of the whole change: after the one-line install, Claude Code has a skill.
func TestInstallWritesTheAgentSkill(t *testing.T) {
	baseURL := installFixture(t)
	home := homeWithAgents(t, ".claude")

	code, output, _ := runInstaller(t, "RTDD_BASE_URL="+baseURL, "HOME="+home)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, output)
	}

	skill := filepath.Join(home, ".claude", "skills", "rtdd", "SKILL.md")
	body, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("the installer did not write the agent skill: %v\n%s", err, output)
	}
	if !strings.Contains(string(body), "rtdd init") {
		t.Error("the installed skill does not tell the agent how to set up a repository")
	}
	if !strings.Contains(output, "skill") {
		t.Errorf("the installer did not report that it installed a skill:\n%s", output)
	}
}

// Writing into $HOME is a side effect outside the install directory, so it has to be
// declinable — CI images and containers want the binary and nothing else.
func TestInstallHonoursRTDDNoSkill(t *testing.T) {
	baseURL := installFixture(t)
	home := homeWithAgents(t, ".claude")

	code, output, _ := runInstaller(t, "RTDD_BASE_URL="+baseURL, "HOME="+home, "RTDD_NO_SKILL=1")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, output)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "rtdd", "SKILL.md")); !os.IsNotExist(err) {
		t.Error("RTDD_NO_SKILL=1 did not suppress the skill install")
	}
}

// The binary install is the thing the user asked for. A home directory rtdd cannot write
// must not turn a successful binary install into a failure — the user would re-run an
// install that had already worked.
func TestInstallSucceedsWhenTheSkillInstallFails(t *testing.T) {
	baseURL := installFixture(t)
	// A regular file where a home directory should be: every path under it fails with
	// ENOTDIR rather than "does not exist", so the skill install errors instead of
	// skipping.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, output, installDir := runInstaller(t, "RTDD_BASE_URL="+baseURL, "HOME="+blocked)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0: a failed skill install must not fail the binary install\n%s", code, output)
	}
	if _, err := os.Stat(filepath.Join(installDir, "rtdd")); err != nil {
		t.Fatalf("binary was not installed: %v\n%s", err, output)
	}
}
