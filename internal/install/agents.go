package install

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

// The agent table is one list on purpose. Before it, three hand-kept lists — GlobalSurfaces,
// ProjectSkillFiles and the README — said where rtdd writes for each agent, and they drifted
// the day #509 moved two of them. Every list that names an agent's paths is now derived from
// Agents(), and the feature × agent matrix (cmd/rtdd/agent_matrix_test.go) restates it by
// hand, so the table and the test are two statements that have to agree.

// Agent is one agent CLI rtdd installs for, and every path rtdd writes for it.
type Agent struct {
	ID, Name string
	// Present: home-relative paths, any one of which existing means the agent is installed.
	// A trailing slash is a directory.
	Present []string
	// Owned: rtdd's user-level directory, home-relative with a trailing slash; "" when User
	// is empty.
	Owned   string
	User    []GlobalFile // `rtdd skill install`; empty: no file-based user level
	Project []GlobalFile // `rtdd init --project-skill`; Rel is repo-relative
	Limits  []string     // one sentence each: the skip note, doctor's line, the README cell
	// Retired: home-relative directories, trailing slash, an earlier rtdd wrote for this
	// agent and this one removes.
	Retired []string
	// Verified: "verified on <tool> <version>, <YYYY-MM-DD>"; "" until the agent is checked
	// against a release of its tool.
	Verified string
}

// Agents is every agent rtdd installs for, in the order they are reported.
func Agents() []Agent {
	return []Agent{
		{
			ID: "claude", Name: "Claude Code",
			Present:  []string{".claude/"},
			Owned:    ".claude/skills/rtdd/",
			User:     []GlobalFile{{Rel: ".claude/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
			Project:  []GlobalFile{{Rel: ".claude/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
			Verified: "verified on Claude Code 2.1.311, 2026-10-27",
		},
		{
			ID: "antigravity", Name: "Antigravity",
			Present: []string{".gemini/config/"},
			Owned:   ".gemini/config/skills/rtdd/",
			User:    []GlobalFile{{Rel: ".gemini/config/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
			Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
			Limits: []string{
				"also reads `GEMINI.md` and `AGENTS.md` as rules; rtdd writes neither",
			},
			Verified: "verified on agy 1.1.27, 2026-10-27",
		},
		{
			ID: "codex", Name: "Codex",
			Present: []string{".codex/"},
			Owned:   ".agents/skills/rtdd/",
			User:    []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
			Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
			// v0.3.0 wrote ~/.codex/skills/rtdd/, which Codex's documentation no longer names.
			Retired: []string{".codex/skills/rtdd/"},
			Limits: []string{
				"`~/.agents/skills/` is shared with other agents; `rtdd skill uninstall` removes rtdd's copy for all of them",
			},
			Verified: "verified on codex-cli 0.154.0, 2026-10-27",
		},
		{
			ID: "gemini", Name: "Gemini CLI",
			// Not .gemini/: Antigravity keeps its configuration in ~/.gemini/config/, and a
			// machine with only Antigravity must not get a Gemini CLI extension.
			Present: []string{".gemini/settings.json", ".gemini/oauth_creds.json"},
			Owned:   ".gemini/extensions/rtdd/",
			User: []GlobalFile{
				{Rel: ".gemini/extensions/rtdd/gemini-extension.json", Literal: geminiManifest, Marker: `"name": "rtdd"`},
				{Rel: ".gemini/extensions/rtdd/GEMINI.md", Source: protocol.GlobalAgentsPath, Marker: protocol.BeginMarker},
			},
			Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
			Limits: []string{
				"the project skill loads only in a folder Gemini CLI trusts",
			},
			Verified: "verified on Gemini CLI 0.65.0, 2026-10-27",
		},
		{
			ID: "cursor", Name: "Cursor",
			Present: []string{".cursor/"},
			Project: []GlobalFile{{Rel: ".cursor/rules/rtdd.mdc", Source: "dist/cursor/rules/rtdd.mdc", Marker: protocol.Generated}},
			Limits:  []string{"Cursor's user rules live in its settings, not in a file"},
		},
	}
}

// Content is what rtdd writes for f: its Literal, or the render files holds for its Source.
func (f GlobalFile) Content(files map[string]string) string {
	if f.Source != "" {
		return files[f.Source]
	}
	return f.Literal
}

// AgentState is what `rtdd doctor` reports for one agent.
type AgentState struct {
	Agent Agent
	// State is "installed" (every User file exists), "present" (the agent is installed and
	// rtdd's user surface is not), "absent" (the agent is not installed) or "limited" (the
	// agent has no file-based user level, so its line states the limitation).
	State string
}

// AgentStatus reads home for every Agents() row.
func AgentStatus(home string) ([]AgentState, error) {
	var out []AgentState
	for _, a := range Agents() {
		state, err := agentState(home, a)
		if err != nil {
			return nil, err
		}
		out = append(out, AgentState{Agent: a, State: state})
	}
	return out, nil
}

func agentState(home string, a Agent) (string, error) {
	if len(a.User) == 0 {
		return "limited", nil
	}
	present, err := anyExists(home, a.Present)
	if err != nil || !present {
		return "absent", err
	}
	for _, f := range a.User {
		switch _, err := os.Stat(filepath.Join(home, filepath.FromSlash(f.Rel))); {
		case os.IsNotExist(err):
			return "present", nil
		case err != nil:
			return "", err
		}
	}
	return "installed", nil
}

// anyExists reports whether any of the home-relative paths exists; one with a trailing
// slash counts only as a directory.
func anyExists(home string, paths []string) (bool, error) {
	for _, p := range paths {
		switch fi, err := os.Stat(filepath.Join(home, filepath.FromSlash(strings.TrimSuffix(p, "/")))); {
		case err == nil:
			if !strings.HasSuffix(p, "/") || fi.IsDir() {
				return true, nil
			}
		case !os.IsNotExist(err):
			return false, err
		}
	}
	return false, nil
}

// projectSkillFiles is every row's Project, Rel to Source. A path several agents share is
// one entry.
func projectSkillFiles() map[string]string {
	out := map[string]string{}
	for _, a := range Agents() {
		for _, f := range a.Project {
			out[f.Rel] = f.Source
		}
	}
	return out
}
