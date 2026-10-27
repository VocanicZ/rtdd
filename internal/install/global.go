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
	// Present are paths relative to the home directory, any one of which existing is how rtdd
	// decides the agent is installed. A trailing slash is a directory.
	Present []string
	// Owned is rtdd's own directory, relative to the home directory and slash-separated
	// with a trailing slash. Everything rtdd writes for the agent is inside it. It is empty
	// for an agent with no file-based user level.
	Owned string
	// Files are what rtdd writes inside Owned. None means the agent has no file-based user
	// level, and Limit is the skip note that says so.
	Files []GlobalFile
	Limit string
	// Retired are directories an earlier rtdd wrote for the agent, relative to the home
	// directory with a trailing slash. Installing removes them, so the agent never loads two
	// copies of rtdd.
	Retired []string
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

// GlobalSurfaces is the machine-wide front-ends, one per Agents() row, in the order they
// are reported.
//
// An agent with no file-based user level still has a surface, with no files: rtdd has
// nothing to write for it, and says so rather than skipping it silently. `rtdd skill prompt`
// is the answer for it and for every agent not in the table.
func GlobalSurfaces() []GlobalSurface {
	var out []GlobalSurface
	for _, a := range Agents() {
		sf := GlobalSurface{
			Agent: a.Name, Present: a.Present,
			Owned: a.Owned, Files: a.User, Retired: a.Retired,
		}
		if len(a.User) == 0 && len(a.Limits) > 0 {
			sf.Limit = a.Limits[0] + "; `rtdd skill prompt` prints a copy to give it"
		}
		out = append(out, sf)
	}
	return out
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
			content := f.Content(files)
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

		for _, dir := range sf.Retired {
			retire, err := planRemoveDir(home, dir, generatedFile(home, dir))
			if err != nil {
				return nil, err
			}
			steps = append(steps, retire...)
		}
	}

	return steps, nil
}

// globalSkip is the step for a surface rtdd does not write on this machine, or nil when it
// does: the agent has no file-based user level, or is not installed.
func globalSkip(home string, sf GlobalSurface) (*Step, error) {
	if len(sf.Files) == 0 {
		return &Step{Path: sf.Present[0], Action: Skip, Note: sf.Limit}, nil
	}
	present, err := anyExists(home, sf.Present)
	if err != nil {
		return nil, err
	}
	if !present {
		return &Step{Path: sf.Owned, Action: Skip, Note: sf.Agent + " is not installed on this machine"}, nil
	}
	return nil, nil
}

// PlanGlobalUninstall is PlanGlobal's inverse: it removes rtdd's own directory for each
// agent, and any directory an earlier rtdd wrote for it.
//
// It never removes an agent's config directory, only rtdd's own inside it. A file the user
// put in rtdd's directory is not rtdd's to delete, so then only rtdd's files go and the
// directory stays.
func PlanGlobalUninstall(home string) ([]Step, error) {
	steps := []Step{}

	for _, sf := range GlobalSurfaces() {
		// No Owned directory means nothing of rtdd's to remove — and filepath.Join(home, "")
		// is the home directory itself, which no plan may ever name.
		if sf.Owned == "" {
			continue
		}
		ours := map[string]bool{}
		for _, f := range sf.Files {
			ours[filepath.Base(f.Rel)] = true
		}
		owned, err := planRemoveDir(home, sf.Owned, func(e os.DirEntry) (bool, error) { return ours[e.Name()], nil })
		if err != nil {
			return nil, err
		}
		if owned == nil {
			owned = []Step{{Path: sf.Owned, Action: Skip, Note: "not present"}}
		}
		steps = append(steps, owned...)

		for _, dir := range sf.Retired {
			retire, err := planRemoveDir(home, dir, generatedFile(home, dir))
			if err != nil {
				return nil, err
			}
			steps = append(steps, retire...)
		}
	}

	return steps, nil
}

// planRemoveDir is the steps that remove dir, a home-relative directory rtdd wrote, or nil
// when it does not exist. ours says whether an entry in it is rtdd's. The directory goes
// whole when every entry is; otherwise only rtdd's entries go, and a Skip says why the
// directory stays.
func planRemoveDir(home, dir string, ours func(os.DirEntry) (bool, error)) ([]Step, error) {
	entries, err := os.ReadDir(filepath.Join(home, filepath.FromSlash(dir)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var mine []Step
	for _, e := range entries {
		ok, err := ours(e)
		if err != nil {
			return nil, err
		}
		if ok {
			mine = append(mine, Step{Path: dir + e.Name(), Action: Delete})
		}
	}
	if len(mine) == len(entries) {
		return []Step{{Path: dir, Action: Delete}}, nil
	}
	return append(mine, Step{Path: dir, Action: Skip, Note: "kept; it holds files rtdd did not write"}), nil
}

// generatedFile is planRemoveDir's rule for a Retired directory: an entry is rtdd's when it
// is a file carrying protocol.Generated, which every whole-file render rtdd wrote carries.
func generatedFile(home, dir string) func(os.DirEntry) (bool, error) {
	return func(e os.DirEntry) (bool, error) {
		if !e.Type().IsRegular() {
			return false, nil
		}
		b, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(dir), e.Name()))
		if err != nil {
			return false, err
		}
		return strings.Contains(string(b), protocol.Generated), nil
	}
}
