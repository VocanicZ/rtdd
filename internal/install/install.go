package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

type Step struct {
	Path    string
	Action  Action
	Content string
	Note    string
}

const defaultConfig = `# .rtdd/config.yaml — rtdd defaults, see docs/specs for what each one does.
stale_commits: 50
drift_guard: 100
hub_threshold: 0.40
`

// Plan computes what `rtdd init` would do without touching the filesystem.
//
// files maps the generator's output paths to their content, i.e. exactly what
// protocol.RenderAll returns. Whole-file targets (the Claude Code skill, the
// Cursor rule) are created but never overwritten without --force, because the
// host may have edited them. Marker-delimited targets (AGENTS.md, CLAUDE.md) are
// merged, which is always safe.
//
// detected is the adapter set `rtdd init` matched against this repository; it is recorded
// into a NEWLY CREATED .rtdd/config.yaml (spec §5) and ignored when one already exists.
// An empty slice is the --force install into a repo nothing matched, and writes the
// unchanged defaults.
func Plan(root string, files map[string]string, force bool, detected []AdapterRecord) ([]Step, error) {
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
		case force:
			steps = append(steps, Step{Path: rel, Action: Create, Content: content, Note: "overwritten by --force"})
		default:
			steps = append(steps, Step{
				Path: rel, Action: Conflict,
				Note: "exists and differs; re-run with --force to overwrite",
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
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Create, Content: ConfigWithAdapters(detected)})
	} else {
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Skip, Note: "keeping your config"})
	}

	return steps, nil
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
