package install

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

// GlobalSurface is one machine-wide agent front-end: a directory in the user's home
// directory that a coding agent reads in every repository, rather than one `rtdd init`
// writes into a single repository.
//
// The two scopes answer different questions. The repo-scoped front-ends are written by
// `rtdd init --project-skill` into a repository that is already set up. These are read in
// repositories rtdd has never touched, so they carry the `setup` section and their whole
// job is to say "run `rtdd init` here first".
//
// Every surface is a directory rtdd owns outright (#509). rtdd never merges into a file an
// agent shares with the user — ~/.codex/AGENTS.md and ~/.gemini/GEMINI.md are the user's —
// so installing, upgrading and uninstalling never edit the user's text.
type GlobalSurface struct {
	// Agent is the tool's name, for the line the command prints.
	Agent string
	// Dir is the agent's config directory relative to the home directory. Its presence is
	// how rtdd decides the agent is installed.
	Dir string
	// Needs is a directory, relative to the home directory, that must also exist before rtdd
	// writes this surface, and NeedsNote is the skip note when it does not. Codex reads
	// skills only in a release that keeps ~/.codex/skills/.
	Needs, NeedsNote string
	// Owned is rtdd's own directory, relative to the home directory and slash-separated
	// with a trailing slash. Everything rtdd writes for the agent is inside it.
	Owned string
	// Files are what rtdd writes inside Owned.
	Files []GlobalFile
}

// GlobalFile is one file a GlobalSurface writes.
type GlobalFile struct {
	// Rel is the file's path relative to the home directory, slash-separated.
	Rel string
	// Source keys into the map install.Files() returns; Literal is used when it is empty.
	Source, Literal string
	// Marker is text every rtdd render of this file carries. A file holding it is rtdd's own
	// output from an earlier release and is refreshed without --force.
	Marker string
}

// geminiManifest is the Gemini CLI extension manifest. Its version is the extension's
// format, not rtdd's release: the extension only names a context file rtdd regenerates on
// every install, and `rtdd --version` is where the release lives.
const geminiManifest = `{
  "name": "rtdd",
  "version": "1.0.0",
  "description": "Find the tests a code change needs, in rounds; rtdd runs no tests.",
  "contextFileName": "GEMINI.md"
}
`

// GlobalSurfaces is the fixed set of machine-wide front-ends, in the order they are
// reported.
//
// Cursor is absent on purpose: it has no file-based user-level rules — its User Rules live
// inside the application's settings — so there is nothing for rtdd to write. `rtdd skill
// prompt` is the answer for Cursor and for every other agent not listed here.
func GlobalSurfaces() []GlobalSurface {
	return []GlobalSurface{
		{
			Agent: "Claude Code",
			Dir:   ".claude",
			Owned: ".claude/skills/rtdd/",
			Files: []GlobalFile{{Rel: ".claude/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		},
		{
			Agent:     "Codex",
			Dir:       ".codex",
			Needs:     ".codex/skills",
			NeedsNote: "this Codex keeps no ~/.codex/skills/, so it reads no skills; rtdd never writes ~/.codex/AGENTS.md — `rtdd skill prompt` prints a copy to give it",
			Owned:     ".codex/skills/rtdd/",
			Files:     []GlobalFile{{Rel: ".codex/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		},
		{
			Agent: "Gemini CLI",
			Dir:   ".gemini",
			Owned: ".gemini/extensions/rtdd/",
			Files: []GlobalFile{
				{Rel: ".gemini/extensions/rtdd/gemini-extension.json", Literal: geminiManifest, Marker: `"name": "rtdd"`},
				{Rel: ".gemini/extensions/rtdd/GEMINI.md", Source: protocol.GlobalAgentsPath, Marker: protocol.BeginMarker},
			},
		},
	}
}

// PlanGlobal computes what `rtdd skill install` would do, without touching the filesystem.
//
// It keeps Plan's rule for a whole-file front-end: one that exists and differs is a
// conflict rather than an overwrite, because the user may have edited it — unless it is an
// earlier rtdd render, which is refreshed, or --force is given.
//
// It adds one rule Plan has no need for. A surface whose agent directory does not exist is
// skipped, never created: rtdd is a test selector, and conjuring a ~/.codex on a machine
// with no Codex would litter someone's home directory and make `ls ~` misreport which
// tools are installed. Every surface still produces a step, so the output says what was
// considered as well as what was written.
func PlanGlobal(home string, files map[string]string, force bool) ([]Step, error) {
	steps := []Step{}

	for _, sf := range GlobalSurfaces() {
		skip, err := globalSkip(home, sf)
		if err != nil {
			return nil, err
		}
		if skip != nil {
			steps = append(steps, *skip)
			continue
		}

		for _, f := range sf.Files {
			content := f.Literal
			if f.Source != "" {
				content = files[f.Source]
			}
			existing, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(f.Rel)))
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			switch {
			case os.IsNotExist(err):
				steps = append(steps, Step{Path: f.Rel, Action: Create, Content: content})
			case string(existing) == content:
				steps = append(steps, Step{Path: f.Rel, Action: Skip, Note: "already current"})
			case strings.Contains(string(existing), f.Marker):
				// rtdd's own output from an earlier release. This is the ordinary upgrade
				// path — `rtdd update` re-runs this install, and the new binary renders a
				// different skill than the old one did — so it must not need --force. Treating
				// it as a conflict would fail every upgrade and pin everyone to whichever
				// skill their first install happened to write. The repo-scoped Plan is
				// deliberately stricter: nothing re-runs `rtdd init` behind the user's back.
				steps = append(steps, Step{Path: f.Rel, Action: Create, Content: content, Note: "refreshed for this release"})
			case force:
				steps = append(steps, Step{Path: f.Rel, Action: Create, Content: content, Note: "overwritten by --force"})
			default:
				steps = append(steps, Step{
					Path: f.Rel, Action: Conflict,
					Note: "exists and differs; re-run with --force to overwrite",
				})
			}
		}
	}

	return steps, nil
}

// globalSkip is the step for a surface rtdd does not write on this machine, or nil when it
// does: the agent is not installed, or is installed without what the surface Needs.
func globalSkip(home string, sf GlobalSurface) (*Step, error) {
	for _, gate := range []struct{ dir, note string }{
		{sf.Dir, sf.Agent + " is not installed on this machine"},
		{sf.Needs, sf.NeedsNote},
	} {
		if gate.dir == "" {
			continue
		}
		switch _, err := os.Stat(filepath.Join(home, filepath.FromSlash(gate.dir))); {
		case os.IsNotExist(err):
			return &Step{Path: sf.Owned, Action: Skip, Note: gate.note}, nil
		case err != nil:
			return nil, err
		}
	}
	return nil, nil
}

// PlanGlobalUninstall is PlanGlobal's inverse: it removes rtdd's own directory for each
// agent.
//
// It never removes an agent's config directory, only rtdd's own inside it. A file the user
// put in rtdd's directory is not rtdd's to delete, so then only rtdd's files go and the
// directory stays.
func PlanGlobalUninstall(home string) ([]Step, error) {
	steps := []Step{}

	for _, sf := range GlobalSurfaces() {
		entries, err := os.ReadDir(filepath.Join(home, filepath.FromSlash(sf.Owned)))
		if os.IsNotExist(err) {
			steps = append(steps, Step{Path: sf.Owned, Action: Skip, Note: "not present"})
			continue
		}
		if err != nil {
			return nil, err
		}
		ours := map[string]bool{}
		for _, f := range sf.Files {
			ours[filepath.Base(f.Rel)] = true
		}
		foreign := false
		for _, e := range entries {
			if !ours[e.Name()] {
				foreign = true
			}
		}
		if !foreign {
			steps = append(steps, Step{Path: sf.Owned, Action: Delete})
			continue
		}
		for _, f := range sf.Files {
			if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(f.Rel))); err == nil {
				steps = append(steps, Step{Path: f.Rel, Action: Delete})
			}
		}
		steps = append(steps, Step{Path: sf.Owned, Action: Skip, Note: "kept; it holds files rtdd did not write"})
	}

	return steps, nil
}
