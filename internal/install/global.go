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
	// Link installs the file as a symbolic link to CanonicalSkill rather than a copy (#566).
	// It is set only for an agent verified to load a SKILL.md through a link
	// (docs/agent-capabilities.md), and only for a file rendered from
	// protocol.GlobalSkillPath, which is what the canonical copy holds.
	Link bool
}

// CanonicalSkill is the one copy of the machine-wide skill rtdd owns outright,
// home-relative. Every agent whose GlobalFile is a Link reads it through a symbolic link, so
// a refresh rewrites one file and no agent's copy goes stale on its own (#566).
const CanonicalSkill = ".local/share/rtdd/skill/SKILL.md"

// canonicalDir is CanonicalSkill's directory and canonicalRoot rtdd's directory around it;
// uninstall removes the second when nothing but the first is in it.
const (
	canonicalDir  = ".local/share/rtdd/skill/"
	canonicalRoot = ".local/share/rtdd/"
)

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
//
// An agent whose file is a Link gets a symbolic link to CanonicalSkill, which is planned
// first, rather than a copy of its own (#566); the rest keep a generated file.
func PlanGlobal(home string, files map[string]string, force bool) ([]Step, error) {
	canonical := GlobalFile{Rel: CanonicalSkill, Source: protocol.GlobalSkillPath, Marker: protocol.Generated}
	canonStep, err := planGlobalFile(home, canonical, files, force)
	if err != nil {
		return nil, err
	}
	// A canonical copy rtdd did not write is not a target to link to: the agents that would
	// link get copies instead, and the file is left alone and said so.
	canonOK := canonStep.Action != Conflict
	if !canonOK {
		canonStep = Step{Path: CanonicalSkill, Action: Skip, Note: "kept; not written by rtdd, so every agent gets its own copy; re-run with --force to overwrite"}
	}
	target := filepath.Join(home, filepath.FromSlash(CanonicalSkill))

	steps := []Step{}
	linked := false
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
			var step Step
			if f.Link && canonOK {
				step, err = planGlobalLink(home, f, files, target, force)
				linked = true
			} else {
				step, err = planGlobalFile(home, f, files, force)
			}
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
		}

		for _, dir := range sf.Retired {
			retire, err := planRemoveDir(home, dir, generatedFile(home, dir))
			if err != nil {
				return nil, err
			}
			steps = append(steps, retire...)
		}
	}

	// The canonical copy goes first, so every link Apply makes points at a file that exists.
	// With no agent linking to it there is nothing for it to serve, and it is not written.
	if linked || !canonOK {
		steps = append([]Step{canonStep}, steps...)
	}
	return steps, nil
}

// planGlobalFile is the step that writes f as a file of its own.
//
// It keeps Plan's rule for a whole-file front-end: one that exists and differs is a
// conflict rather than an overwrite, because the user may have edited it — unless it is an
// earlier rtdd render, which is refreshed, or --force is given.
func planGlobalFile(home string, f GlobalFile, files map[string]string, force bool) (Step, error) {
	content := f.Content(files)
	existing, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(f.Rel)))
	if err != nil && !os.IsNotExist(err) {
		return Step{}, err
	}
	switch {
	case os.IsNotExist(err):
		return Step{Path: f.Rel, Action: Create, Content: content}, nil
	case string(existing) == content:
		return Step{Path: f.Rel, Action: Skip, Note: "already current"}, nil
	case strings.Contains(string(existing), f.Marker):
		// rtdd's own output from an earlier release. This is the ordinary upgrade
		// path — `rtdd update` re-runs this install, and the new binary renders a
		// different skill than the old one did — so it must not need --force. Treating
		// it as a conflict would fail every upgrade and pin everyone to whichever
		// skill their first install happened to write. The repo-scoped Plan is
		// deliberately stricter: nothing re-runs `rtdd init` behind the user's back.
		return Step{Path: f.Rel, Action: Create, Content: content, Note: "refreshed for this release"}, nil
	case force:
		return Step{Path: f.Rel, Action: Create, Content: content, Note: "overwritten by --force"}, nil
	default:
		return Step{Path: f.Rel, Action: Conflict, Note: "exists and differs; re-run with --force to overwrite"}, nil
	}
}

// planGlobalLink is the step that makes f a symbolic link to target, the canonical copy.
//
// The conflict rule is planGlobalFile's: a link already pointing at the canonical copy is
// current, a file an earlier rtdd wrote is replaced by the link, and anything else — a file
// the user wrote, a link the user made to somewhere else — is left alone unless --force.
func planGlobalLink(home string, f GlobalFile, files map[string]string, target string, force bool) (Step, error) {
	abs := filepath.Join(home, filepath.FromSlash(f.Rel))
	link := Step{Path: f.Rel, Action: Link, Target: target, Content: f.Content(files), Note: "links to ~/" + CanonicalSkill}
	fi, err := os.Lstat(abs)
	switch {
	case os.IsNotExist(err):
		return link, nil
	case err != nil:
		return Step{}, err
	case fi.Mode()&os.ModeSymlink != 0:
		if linksTo(abs, target) {
			return Step{Path: f.Rel, Action: Skip, Note: "already current; links to ~/" + CanonicalSkill}, nil
		}
		if force {
			link.Note = "overwritten by --force"
			return link, nil
		}
		return Step{Path: f.Rel, Action: Conflict, Note: "a link rtdd did not make; re-run with --force to replace it"}, nil
	case fi.IsDir():
		return Step{Path: f.Rel, Action: Conflict, Note: "is a directory; move it aside and re-run"}, nil
	}
	existing, err := os.ReadFile(abs)
	if err != nil {
		return Step{}, err
	}
	switch {
	case strings.Contains(string(existing), f.Marker):
		link.Note = "replaces an earlier release's copy; links to ~/" + CanonicalSkill
		return link, nil
	case force:
		link.Note = "overwritten by --force"
		return link, nil
	default:
		return Step{Path: f.Rel, Action: Conflict, Note: "exists and differs; re-run with --force to overwrite"}, nil
	}
}

// linksTo reports whether abs is a symbolic link that resolves to target: by its literal
// destination, or — for a home directory reached through another link — by the file both
// name.
func linksTo(abs, target string) bool {
	dest, err := os.Readlink(abs)
	if err != nil {
		return false
	}
	if dest == target {
		return true
	}
	a, aerr := os.Stat(abs)
	b, berr := os.Stat(target)
	return aerr == nil && berr == nil && os.SameFile(a, b)
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
		owned, err := planRemoveDir(home, sf.Owned, ownedFile(home, sf))
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

	// The canonical copy goes last, after every link to it.
	canon, err := planRemoveCanonical(home)
	if err != nil {
		return nil, err
	}
	return append(steps, canon...), nil
}

// planRemoveCanonical is the steps that remove CanonicalSkill's directory, and rtdd's
// directory around it when that holds nothing else.
func planRemoveCanonical(home string) ([]Step, error) {
	steps, err := planRemoveDir(home, canonicalDir, generatedFile(home, canonicalDir))
	if err != nil || len(steps) != 1 || steps[0].Action != Delete {
		return steps, err
	}
	entries, err := os.ReadDir(filepath.Join(home, filepath.FromSlash(canonicalRoot)))
	if err != nil {
		return nil, err
	}
	if len(entries) == 1 {
		return []Step{{Path: canonicalRoot, Action: Delete}}, nil
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

// ownedFile is planRemoveDir's rule for a surface's Owned directory: an entry is rtdd's when
// it is a file named for one of sf.Files and carrying that file's Marker. The name alone is
// not enough (#550) — PlanGlobal calls an unmarked file at rtdd's path a conflict and will
// not overwrite it, so uninstalling must not delete it either.
func ownedFile(home string, sf GlobalSurface) func(os.DirEntry) (bool, error) {
	markers := map[string]string{}
	for _, f := range sf.Files {
		markers[filepath.Base(f.Rel)] = f.Marker
	}
	target := filepath.Join(home, filepath.FromSlash(CanonicalSkill))
	return func(e os.DirEntry) (bool, error) {
		marker, named := markers[e.Name()]
		abs := filepath.Join(home, filepath.FromSlash(sf.Owned), e.Name())
		// A link is rtdd's only when it is one rtdd makes: to the canonical copy (#566).
		if named && e.Type()&os.ModeSymlink != 0 {
			return linksTo(abs, target), nil
		}
		if !named || !e.Type().IsRegular() {
			return false, nil
		}
		b, err := os.ReadFile(abs)
		if err != nil {
			return false, err
		}
		return strings.Contains(string(b), marker), nil
	}
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
