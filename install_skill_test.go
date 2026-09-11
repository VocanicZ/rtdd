// install_skill_test.go covers the half of `install.sh` that exists so a user who runs the
// one-line installer ends up with an agent skill, not just a binary. Before this, the
// script installed `rtdd` and stopped, and nothing on the machine told any coding agent
// that rtdd existed — `rtdd init` had to be discovered and run by hand, per repository.
package installtest

import (
	"os"
	"os/exec"
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

	code, output, _ := runInstall(t, "RTDD_BASE_URL="+baseURL, "HOME="+home)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, output)
	}

	skill := filepath.Join(home, ".claude", "skills", "rtdd", "SKILL.md")
	body, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("install.sh did not write the agent skill: %v\n%s", err, output)
	}
	if !strings.Contains(string(body), "rtdd init") {
		t.Error("the installed skill does not tell the agent how to set up a repository")
	}
	if !strings.Contains(output, "skill") {
		t.Errorf("install.sh did not report that it installed a skill:\n%s", output)
	}
}

// Writing into $HOME is a side effect outside the install directory, so it has to be
// declinable — CI images and containers want the binary and nothing else.
func TestInstallHonoursRTDDNoSkill(t *testing.T) {
	baseURL := installFixture(t)
	home := homeWithAgents(t, ".claude")

	code, output, _ := runInstall(t, "RTDD_BASE_URL="+baseURL, "HOME="+home, "RTDD_NO_SKILL=1")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, output)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "rtdd", "SKILL.md")); !os.IsNotExist(err) {
		t.Error("RTDD_NO_SKILL=1 did not suppress the skill install")
	}
}

// The binary install is the thing the user asked for. A home directory rtdd cannot write
// must not turn a successful binary install into a failure — the user would re-run a
// `curl | sh` that had already worked.
func TestInstallSucceedsWhenTheSkillInstallFails(t *testing.T) {
	baseURL := installFixture(t)
	// A regular file where a home directory should be: every path under it fails with
	// ENOTDIR rather than "does not exist", so the skill install errors instead of
	// skipping.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, output, installDir := runInstall(t, "RTDD_BASE_URL="+baseURL, "HOME="+blocked)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0: a failed skill install must not fail the binary install\n%s", code, output)
	}
	if _, err := os.Stat(filepath.Join(installDir, "rtdd")); err != nil {
		t.Fatalf("binary was not installed: %v\n%s", err, output)
	}
}

// Git Bash, MSYS2 and Cygwin all run this script on Windows and report a `uname -s` that
// the original detect_os rejected outright, so a Windows developer following the README's
// one-liner got "unsupported OS" even though a Windows binary ships.
func TestInstallTreatsGitBashAsWindows(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found on PATH")
	}
	for _, uname := range []string{"MINGW64_NT-10.0-22631", "MSYS_NT-10.0-19045", "CYGWIN_NT-10.0"} {
		t.Run(uname, func(t *testing.T) {
			bin := buildFixtureBinary(t, t.TempDir())
			archive := buildArchiveAs(t, bin, "rtdd.exe")
			name := archiveName("windows", "amd64")
			baseURL := serveFixture(t, name, archive, sha256Hex(archive)+"  "+name+"\n")

			path := isolatedPATH(t)
			fakeUname(t, path, uname, "x86_64")
			home := homeWithAgents(t)

			code, output, installDir := runInstall(t,
				"RTDD_BASE_URL="+baseURL, "HOME="+home, "PATH="+path)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0 under %s\n%s", code, uname, output)
			}
			if _, err := os.Stat(filepath.Join(installDir, "rtdd.exe")); err != nil {
				t.Errorf("did not install rtdd.exe under %s: %v\n%s", uname, err, output)
			}
		})
	}
}
