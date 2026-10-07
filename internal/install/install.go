package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

type Step struct {
	Path    string
	Action  Action
	Content string
	Note    string
}

// graphCacheLine is the .gitignore line for the graph cache (spec §4.5, §12).
const graphCacheLine = ".rtdd/graph.json"

// Plan computes what `rtdd init` would do without touching the filesystem.
//
// files maps the generator's output paths to their content, i.e. exactly what
// protocol.RenderAll returns. Whole-file targets (the Claude Code skill, the Cursor rule)
// are created, or replaced when the file on disk is an earlier rtdd render — it carries
// protocol.Generated. A file there that rtdd did not write is a conflict: init never
// overwrites someone else's file, and there is no flag that makes it. Marker-delimited
// targets (AGENTS.md, CLAUDE.md) are merged, which is always safe.
func Plan(root string, files map[string]string) ([]Step, error) {
	steps := []Step{}

	// 1. Whole-file targets.
	whole := map[string]string{
		".claude/skills/rtdd/SKILL.md": files["dist/SKILL.md"],
		".cursor/rules/rtdd.mdc":       files["dist/cursor/rules/rtdd.mdc"],
	}
	for rel, content := range whole {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		existing, err := os.ReadFile(abs)
		switch {
		case os.IsNotExist(err):
			steps = append(steps, Step{Path: rel, Action: Create, Content: content})
		case err != nil:
			return nil, err
		case string(existing) == content:
			steps = append(steps, Step{Path: rel, Action: Skip, Note: "already current"})
		case strings.Contains(string(existing), protocol.Generated):
			steps = append(steps, Step{Path: rel, Action: Replace, Content: content, Note: "an earlier rtdd render"})
		default:
			steps = append(steps, Step{
				Path: rel, Action: Conflict,
				Note: "exists and was not written by rtdd; move it aside and re-run",
			})
		}
	}

	// 2. Marker-delimited targets. CLAUDE.md is only touched if the host already
	// has one — rtdd does not introduce a CLAUDE.md into a repo that has none,
	// because the skill file is the right Claude Code surface.
	block := files["dist/AGENTS.md"]
	markerTargets := []string{"AGENTS.md"}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); err == nil {
		markerTargets = append(markerTargets, "CLAUDE.md")
	}
	for _, rel := range markerTargets {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		existing, err := os.ReadFile(abs)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		merged, action, err := MergeBlock(string(existing), block)
		if err != nil {
			steps = append(steps, Step{Path: rel, Action: Conflict, Note: err.Error()})
			continue
		}
		steps = append(steps, Step{Path: rel, Action: action, Content: merged})
	}

	// 3. .rtdd/config.yaml — created, never overwritten.
	cfg := filepath.Join(root, ".rtdd", "config.yaml")
	if _, err := os.Stat(cfg); os.IsNotExist(err) {
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Create, Content: DefaultConfig()})
	} else {
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Skip, Note: "keeping your config"})
	}

	// 4. .gitignore — the graph cache is rebuilt in seconds and conflicts on every branch
	// when committed (spec §4.5), so init ignores it, once.
	step, err := planGitignore(root)
	if err != nil {
		return nil, err
	}
	return append(steps, step), nil
}

// planGitignore adds graphCacheLine to the host's .gitignore unless a line already names
// it, spelled root-anchored or not.
func planGitignore(root string) (Step, error) {
	b, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	switch {
	case os.IsNotExist(err):
		return Step{Path: ".gitignore", Action: Create, Content: graphCacheLine + "\n"}, nil
	case err != nil:
		return Step{}, err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l == graphCacheLine || l == "/"+graphCacheLine {
			return Step{Path: ".gitignore", Action: Skip, Note: "already ignores " + graphCacheLine}, nil
		}
	}
	body := string(b)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return Step{Path: ".gitignore", Action: AppendLine, Content: body + graphCacheLine + "\n"}, nil
}

func Apply(root string, steps []Step) error {
	for _, s := range steps {
		if s.Action == Skip {
			continue
		}
		if s.Action == Conflict {
			return fmt.Errorf("%s: %s", s.Path, s.Note)
		}
		abs := filepath.Join(root, filepath.FromSlash(s.Path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(abs, []byte(s.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Files returns the generated front-ends embedded in the binary, so `rtdd init`
// works in a host repo that has no copy of protocol/PROTOCOL.md.
func Files() (map[string]string, error) {
	doc, err := protocol.Parse(embeddedProtocol)
	if err != nil {
		return nil, err
	}
	return protocol.RenderAll(doc)
}
