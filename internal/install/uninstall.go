package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

// RemoveBlock is MergeBlock's inverse: it deletes the marker-delimited rtdd block and
// leaves every other byte of the host's file where it was.
//
// It refuses the same files MergeBlock refuses. A half-written block or two blocks cannot
// be read unambiguously, and this function edits documents that belong to the project
// rather than to rtdd, so a wrong guess here destroys someone else's work.
func RemoveBlock(existing string) (string, Action, error) {
	begins := strings.Count(existing, protocol.BeginMarker)
	ends := strings.Count(existing, protocol.EndMarker)
	if begins == 0 && ends == 0 {
		return existing, Skip, nil
	}
	if begins != 1 || ends != 1 {
		return "", Conflict, fmt.Errorf(
			"found %d BEGIN and %d END rtdd markers — expected exactly one of each; fix the file by hand",
			begins, ends)
	}
	start := strings.Index(existing, protocol.BeginMarker)
	endAt := strings.Index(existing, protocol.EndMarker)
	if endAt < start {
		return "", Conflict, fmt.Errorf("rtdd END marker appears before BEGIN — fix the file by hand")
	}
	endAt += len(protocol.EndMarker)
	if endAt < len(existing) && existing[endAt] == '\n' {
		endAt++
	}

	// The seam is closed the way MergeBlock opened it: the blank line that separated the
	// block from the prose above goes with the block.
	head := strings.TrimRight(existing[:start], "\n")
	tail := strings.TrimLeft(existing[endAt:], "\n")
	switch {
	case head == "" && tail == "":
		return "", StripBlock, nil
	case head == "":
		return tail, StripBlock, nil
	case tail == "":
		return head + "\n", StripBlock, nil
	default:
		return head + "\n\n" + tail, StripBlock, nil
	}
}

// UninstallOptions selects how much of rtdd's footprint to take out.
type UninstallOptions struct {
	// State also removes .rtdd/, which holds the config and the recorded coverage map.
	// Off by default: the map is the expensive thing to rebuild, and removing the agent
	// front-ends is not a request to throw away a seed run.
	State bool
}

// PlanUninstall computes what `rtdd uninstall` would do, without touching the filesystem.
//
// It is Plan's inverse and covers exactly what Plan writes: the two whole-file front-ends,
// the marker block in AGENTS.md and CLAUDE.md, and the .gitattributes line. The binary is
// not in this list, because a repository is not where the binary lives.
func PlanUninstall(root string, opts UninstallOptions) ([]Step, error) {
	steps := []Step{}

	for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc"} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		switch _, err := os.Stat(abs); {
		case os.IsNotExist(err):
			steps = append(steps, Step{Path: rel, Action: Skip, Note: "not present"})
		case err != nil:
			return nil, err
		default:
			steps = append(steps, Step{Path: rel, Action: Delete})
		}
	}

	for _, rel := range []string{"AGENTS.md", "CLAUDE.md"} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		existing, err := os.ReadFile(abs)
		if os.IsNotExist(err) {
			steps = append(steps, Step{Path: rel, Action: Skip, Note: "not present"})
			continue
		}
		if err != nil {
			return nil, err
		}
		stripped, action, err := RemoveBlock(string(existing))
		switch {
		case err != nil:
			steps = append(steps, Step{Path: rel, Action: Conflict, Note: err.Error()})
		case action == Skip:
			steps = append(steps, Step{Path: rel, Action: Skip, Note: "carries no rtdd block"})
		case strings.TrimSpace(stripped) == "":
			// A file holding nothing but rtdd's block goes with the block. Leaving an
			// empty AGENTS.md behind is litter, not restraint.
			steps = append(steps, Step{Path: rel, Action: Delete, Note: "held nothing but the rtdd block"})
		default:
			steps = append(steps, Step{Path: rel, Action: StripBlock, Content: stripped})
		}
	}

	existing, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	switch remaining := removeLine(string(existing), gitattributesLine); {
	case !strings.Contains(string(existing), gitattributesLine):
		steps = append(steps, Step{Path: ".gitattributes", Action: Skip, Note: "no merge=union line"})
	case strings.TrimSpace(remaining) == "":
		steps = append(steps, Step{Path: ".gitattributes", Action: Delete, Note: "held nothing but the rtdd line"})
	default:
		steps = append(steps, Step{Path: ".gitattributes", Action: StripBlock, Content: remaining})
	}

	rtddDir := filepath.Join(root, ".rtdd")
	switch _, err := os.Stat(rtddDir); {
	case !opts.State:
		steps = append(steps, Step{Path: ".rtdd/", Action: Skip, Note: "kept; --state removes the config and the recorded map"})
	case os.IsNotExist(err):
		steps = append(steps, Step{Path: ".rtdd/", Action: Skip, Note: "not present"})
	case err != nil:
		return nil, err
	default:
		steps = append(steps, Step{Path: ".rtdd/", Action: Delete, Note: "config and recorded map"})
	}

	return steps, nil
}

// removeLine drops every line equal to want and preserves the rest exactly.
func removeLine(body, want string) string {
	lines := strings.Split(body, "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == want {
			continue
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n")
}

// ApplyUninstall performs the steps PlanUninstall computed. A conflict stops it: a file
// rtdd cannot read unambiguously is left exactly as it was found.
func ApplyUninstall(root string, steps []Step) error {
	for _, s := range steps {
		abs := filepath.Join(root, filepath.FromSlash(s.Path))
		switch s.Action {
		case Skip:
			continue
		case Conflict:
			return fmt.Errorf("%s: %s", s.Path, s.Note)
		case Delete:
			if err := os.RemoveAll(abs); err != nil {
				return err
			}
		case StripBlock:
			if err := os.WriteFile(abs, []byte(s.Content), 0o644); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: %v is not an uninstall action", s.Path, s.Action)
		}
	}
	return nil
}
