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
	// Needs and NeedsNote: a further home-relative directory the user surface requires, and
	// the skip note when it is absent. Codex reads skills only in a release that keeps
	// ~/.codex/skills/.
	Needs, NeedsNote string
	// Owned: rtdd's user-level directory, home-relative with a trailing slash; "" when User
	// is empty.
	Owned   string
	User    []GlobalFile // `rtdd skill install`; empty: no file-based user level
	Project []GlobalFile // `rtdd init --project-skill`; Rel is repo-relative
	Limits  []string     // one sentence each: the skip note, doctor's line, the README cell
	// Verified: "verified on <tool> <version>, <YYYY-MM-DD>"; "" until the agent is checked
	// against a release of its tool.
	Verified string
}

// Agents is every agent rtdd installs for, in the order they are reported.
func Agents() []Agent {
	return []Agent{
		{
			ID: "claude", Name: "Claude Code",
			Present: []string{".claude/"},
			Owned:   ".claude/skills/rtdd/",
			User:    []GlobalFile{{Rel: ".claude/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
			Project: []GlobalFile{{Rel: ".claude/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
		},
		{
			ID: "codex", Name: "Codex",
			Present:   []string{".codex/"},
			Needs:     ".codex/skills",
			NeedsNote: "this Codex keeps no ~/.codex/skills/, so it reads no skills; rtdd never writes ~/.codex/AGENTS.md — `rtdd skill prompt` prints a copy to give it",
			Owned:     ".codex/skills/rtdd/",
			User:      []GlobalFile{{Rel: ".codex/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		},
		{
			ID: "gemini", Name: "Gemini CLI",
			Present: []string{".gemini/"},
			Owned:   ".gemini/extensions/rtdd/",
			User: []GlobalFile{
				{Rel: ".gemini/extensions/rtdd/gemini-extension.json", Literal: geminiManifest, Marker: `"name": "rtdd"`},
				{Rel: ".gemini/extensions/rtdd/GEMINI.md", Source: protocol.GlobalAgentsPath, Marker: protocol.BeginMarker},
			},
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
