package install

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

// GlobalSurface is one machine-wide agent front-end: a file in the user's home directory
// that a coding agent reads in every repository, rather than one `rtdd init` writes into a
// single repository.
//
// The two scopes answer different questions. The repo-scoped front-ends are written by
// `rtdd init` into a repository that is already set up, so they can assume a map exists.
// These are read in repositories rtdd has never touched, so they carry the `setup` section
// and their whole job is to say "run `rtdd init` here first".
type GlobalSurface struct {
	// Agent is the tool's name, for the line the command prints.
	Agent string
	// Dir is the agent's config directory relative to the home directory. Its presence is
	// how rtdd decides the agent is installed.
	Dir string
	// Rel is the front-end's path relative to the home directory, slash-separated.
	Rel string
	// Source keys into the map install.Files() returns.
	Source string
	// Whole is true for a file rtdd owns outright, false for one it merges a
	// marker-delimited block into because the file belongs to the user.
	Whole bool
}

// GlobalSurfaces is the fixed set of machine-wide front-ends, in the order they are
// reported.
//
// Cursor is absent on purpose: it has no file-based user-level rules — its User Rules live
// inside the application's settings — so there is nothing for rtdd to write. `rtdd skill
// prompt` is the answer for Cursor and for every other agent not listed here.
func GlobalSurfaces() []GlobalSurface {
	return []GlobalSurface{
		{
			Agent:  "Claude Code",
			Dir:    ".claude",
			Rel:    ".claude/skills/rtdd/SKILL.md",
			Source: protocol.GlobalSkillPath,
			Whole:  true,
		},
		{
			Agent:  "Codex",
			Dir:    ".codex",
			Rel:    ".codex/AGENTS.md",
			Source: protocol.GlobalAgentsPath,
		},
		{
			Agent:  "Gemini CLI",
			Dir:    ".gemini",
			Rel:    ".gemini/GEMINI.md",
			Source: protocol.GlobalAgentsPath,
		},
	}
}

// PlanGlobal computes what `rtdd skill install` would do, without touching the filesystem.
//
// It is Plan's machine-wide counterpart and keeps Plan's two rules: a whole-file front-end
// that exists and differs is a conflict rather than an overwrite, because the user may have
// edited it, and a marker-delimited file is merged, which is always safe.
//
// It adds one rule Plan has no need for. A surface whose agent directory does not exist is
// skipped, never created: rtdd is a test selector, and conjuring a ~/.codex on a machine
// with no Codex would litter someone's home directory and make `ls ~` misreport which
// tools are installed. Every surface still produces a step, so the output says what was
// considered as well as what was written.
func PlanGlobal(home string, files map[string]string, force bool) ([]Step, error) {
	steps := []Step{}

	for _, sf := range GlobalSurfaces() {
		if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(sf.Dir))); os.IsNotExist(err) {
			steps = append(steps, Step{
				Path: sf.Rel, Action: Skip,
				Note: sf.Agent + " is not installed on this machine",
			})
			continue
		} else if err != nil {
			return nil, err
		}

		content := files[sf.Source]
		abs := filepath.Join(home, filepath.FromSlash(sf.Rel))
		existing, err := os.ReadFile(abs)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		if !sf.Whole {
			merged, action, err := MergeBlock(string(existing), content)
			if err != nil {
				steps = append(steps, Step{Path: sf.Rel, Action: Conflict, Note: err.Error()})
				continue
			}
			steps = append(steps, Step{Path: sf.Rel, Action: action, Content: merged})
			continue
		}

		switch {
		case os.IsNotExist(err):
			steps = append(steps, Step{Path: sf.Rel, Action: Create, Content: content})
		case string(existing) == content:
			steps = append(steps, Step{Path: sf.Rel, Action: Skip, Note: "already current"})
		case strings.Contains(string(existing), protocol.Generated):
			// rtdd's own output from an earlier release. This is the ordinary upgrade
			// path — `rtdd update` re-runs this install, and the new binary renders a
			// different skill than the old one did — so it must not need --force. Treating
			// it as a conflict would fail every upgrade and pin everyone to whichever
			// skill their first install happened to write. The repo-scoped Plan is
			// deliberately stricter: nothing re-runs `rtdd init` behind the user's back.
			steps = append(steps, Step{Path: sf.Rel, Action: Create, Content: content, Note: "refreshed for this release"})
		case force:
			steps = append(steps, Step{Path: sf.Rel, Action: Create, Content: content, Note: "overwritten by --force"})
		default:
			steps = append(steps, Step{
				Path: sf.Rel, Action: Conflict,
				Note: "exists and differs; re-run with --force to overwrite",
			})
		}
	}

	return steps, nil
}

// PlanGlobalUninstall is PlanGlobal's inverse: it removes the whole-file front-ends rtdd
// owns and strips its marker-delimited block out of the files it does not.
//
// It never removes an agent's config directory, only rtdd's own files inside it, and a file
// left holding nothing but rtdd's block goes with the block rather than surviving as an
// empty husk — the same rule the repo-scoped uninstall applies to AGENTS.md.
func PlanGlobalUninstall(home string) ([]Step, error) {
	steps := []Step{}

	for _, sf := range GlobalSurfaces() {
		abs := filepath.Join(home, filepath.FromSlash(sf.Rel))

		if sf.Whole {
			switch _, err := os.Stat(abs); {
			case os.IsNotExist(err):
				steps = append(steps, Step{Path: sf.Rel, Action: Skip, Note: "not present"})
			case err != nil:
				return nil, err
			default:
				steps = append(steps, Step{Path: sf.Rel, Action: Delete})
			}
			continue
		}

		existing, err := os.ReadFile(abs)
		if os.IsNotExist(err) {
			steps = append(steps, Step{Path: sf.Rel, Action: Skip, Note: "not present"})
			continue
		}
		if err != nil {
			return nil, err
		}
		stripped, action, err := RemoveBlock(string(existing))
		switch {
		case err != nil:
			steps = append(steps, Step{Path: sf.Rel, Action: Conflict, Note: err.Error()})
		case action == Skip:
			steps = append(steps, Step{Path: sf.Rel, Action: Skip, Note: "carries no rtdd block"})
		case strings.TrimSpace(stripped) == "":
			steps = append(steps, Step{Path: sf.Rel, Action: Delete, Note: "held nothing but the rtdd block"})
		default:
			steps = append(steps, Step{Path: sf.Rel, Action: StripBlock, Content: stripped})
		}
	}

	return steps, nil
}
