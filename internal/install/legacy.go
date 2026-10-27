package install

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

// Before #509, `rtdd init` wrote outside .rtdd/ unasked — a Claude Code skill, a Cursor
// rule, a block in AGENTS.md and CLAUDE.md, a line in the root .gitignore — and `rtdd skill
// install` merged a block into the shared ~/.codex/AGENTS.md and ~/.gemini/GEMINI.md. This
// file finds that footprint. `rtdd init`, `rtdd doctor` and `rtdd update` report it
// (ReportLegacy); `rtdd uninstall --legacy-files` removes exactly it, by markers, leaving
// every byte of the user's own text where it was.
//
// What an opt-in wrote is not legacy. A file or block carrying OptInMarker was asked for,
// and a whole file without protocol.Generated, or a shared file without rtdd's markers, was
// never rtdd's.

// legacyHint closes every report line: the one command that removes what it names.
const legacyHint = "`rtdd uninstall --legacy-files` removes it"

// legacySharedFiles are the shared files an older init merged its block into.
var legacySharedFiles = []string{"AGENTS.md", "CLAUDE.md"}

// legacyGlobalFiles are the shared machine-wide files an older `rtdd skill install` merged
// its block into, relative to the home directory.
var legacyGlobalFiles = []string{".codex/AGENTS.md", ".gemini/GEMINI.md"}

// PlanLegacyFiles computes what `rtdd uninstall --legacy-files` removes from a repository.
// opts excludes what this init run is about to own: with ProjectSkill the two whole-file
// front-ends are init's to refresh, and a file named by AppendTo is init's to merge. When
// the root is opts.Home, the user-level installs at ProjectSkillFiles paths are excluded too.
//
// Removing the root .gitignore line would leave the graph cache unignored, so when
// .rtdd/.gitignore does not already ignore it the plan writes that first.
func PlanLegacyFiles(root string, opts PlanOptions) ([]Step, error) {
	steps := []Step{}

	if !opts.ProjectSkill {
		notes := map[string]string{
			".claude/skills/rtdd/SKILL.md": "the Claude Code skill an older `rtdd init` wrote",
			".cursor/rules/rtdd.mdc":       "the Cursor rule an older `rtdd init` wrote",
		}
		atHome := opts.Home != "" && samePath(root, opts.Home)
		for _, rel := range sortedKeys(ProjectSkillFiles) {
			// Run from the home directory, these paths are `rtdd skill install`'s, and they
			// carry the same marker an older init's front-end does (#554).
			if atHome && userLevel(rel) {
				continue
			}
			step, err := legacyWholeFile(root, rel, notes[rel])
			if err != nil {
				return nil, err
			}
			if step != nil {
				steps = append(steps, *step)
			}
		}
	}

	appended := map[string]bool{}
	for _, name := range opts.AppendTo {
		if rel, err := repoRel(root, name); err == nil {
			appended[rel] = true
		}
	}
	for _, rel := range legacySharedFiles {
		if appended[rel] {
			continue
		}
		step, err := legacyBlock(root, rel, "`rtdd init`")
		if err != nil {
			return nil, err
		}
		if step != nil {
			steps = append(steps, *step)
		}
	}

	b, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	switch {
	case os.IsNotExist(err):
		return steps, nil
	case err != nil:
		return nil, err
	}
	remaining := removeLine(removeLine(string(b), rootGraphCacheLine), "/"+rootGraphCacheLine)
	if remaining == string(b) {
		return steps, nil
	}
	ignore, err := planRtddGitignore(root)
	if err != nil {
		return nil, err
	}
	if ignore.Action != Skip {
		ignore.Note = "ignores the graph cache from inside .rtdd/ instead"
		steps = append(steps, ignore)
	}
	if strings.TrimSpace(remaining) == "" {
		return append(steps, Step{Path: ".gitignore", Action: Delete, Note: "held nothing but the " + rootGraphCacheLine + " line an older `rtdd init` wrote"}), nil
	}
	return append(steps, Step{Path: ".gitignore", Action: StripBlock, Content: remaining, Note: "the " + rootGraphCacheLine + " line an older `rtdd init` appended"}), nil
}

// PlanGlobalLegacyFiles is PlanLegacyFiles for the home directory: the block an older
// `rtdd skill install` merged into ~/.codex/AGENTS.md and ~/.gemini/GEMINI.md, and rtdd's
// files in a directory an agent's row has Retired.
func PlanGlobalLegacyFiles(home string) ([]Step, error) {
	steps := []Step{}
	for _, rel := range legacyGlobalFiles {
		step, err := legacyBlock(home, rel, "`rtdd skill install`")
		if err != nil {
			return nil, err
		}
		if step != nil {
			steps = append(steps, *step)
		}
	}
	for _, a := range Agents() {
		for _, dir := range a.Retired {
			retire, err := planRemoveDir(home, dir, generatedFile(home, dir))
			if err != nil {
				return nil, err
			}
			// What stays is the user's, and footprint is only what rtdd wrote.
			for _, s := range retire {
				if s.Action == Delete {
					s.Note = "the " + a.Name + " skill an older `rtdd skill install` wrote"
					steps = append(steps, s)
				}
			}
		}
	}
	return steps, nil
}

// legacyWholeFile plans the removal of a whole-file front-end an older init rendered: one
// carrying protocol.Generated and not OptInMarker. When it is the only file in its directory
// and that directory is rtdd's own (.claude/skills/rtdd/), the directory goes with it.
func legacyWholeFile(root, rel, note string) (*Step, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, err
	}
	if !strings.Contains(string(b), protocol.Generated) || strings.Contains(string(b), OptInMarker) {
		return nil, nil
	}
	dir := filepath.Dir(rel)
	if filepath.Base(dir) == "rtdd" {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			return nil, err
		}
		if len(entries) == 1 {
			return &Step{Path: filepath.ToSlash(dir) + "/", Action: Delete, Note: note}, nil
		}
	}
	return &Step{Path: rel, Action: Delete, Note: note}, nil
}

// legacyBlock plans the removal of rtdd's marker block from a shared file under base, when
// the block was not opted into. who names the command that wrote it, for the note.
func legacyBlock(base, rel, who string) (*Step, error) {
	b, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel)))
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, err
	}
	existing := string(b)
	if strings.Contains(existing, OptInMarker) {
		return nil, nil
	}
	stripped, action, err := RemoveBlock(existing)
	switch {
	case err != nil:
		return &Step{Path: rel, Action: Conflict, Note: "an rtdd block " + who + " wrote, but " + err.Error()}, nil
	case action == Skip:
		return nil, nil
	case strings.TrimSpace(stripped) == "":
		return &Step{Path: rel, Action: Delete, Note: "held nothing but the rtdd block an older " + who + " wrote"}, nil
	}
	return &Step{Path: rel, Action: StripBlock, Content: stripped, Note: "the rtdd block an older " + who + " merged in; every other byte stays"}, nil
}

// ReportLegacy turns a legacy removal plan into the report `rtdd init`, `rtdd doctor` and
// `rtdd update` print: one Legacy step per piece of footprint, which nothing applies. prefix
// is prepended to each path — "~/" for the home directory's, "" for the repository's. The
// .rtdd/.gitignore step that only supports a removal is not footprint and is left out.
func ReportLegacy(steps []Step, prefix string) []Step {
	out := make([]Step, 0, len(steps))
	for _, s := range steps {
		if s.Action == Create || s.Action == AppendLine {
			continue
		}
		out = append(out, Step{Path: prefix + s.Path, Action: Legacy, Note: s.Note + "; " + legacyHint})
	}
	return out
}
