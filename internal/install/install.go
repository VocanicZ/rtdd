package install

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

type Step struct {
	Path    string
	Action  Action
	Content string
	Note    string
}

// graphCacheLine is the line .rtdd/.gitignore holds for the graph cache (spec §4.5, §12).
// The ignore file is nested in .rtdd/ so the host's root .gitignore is never edited (#509).
const graphCacheLine = "graph.json"

// rootGraphCacheLine is the line an rtdd init before #509 appended to the host's root
// .gitignore. It is legacy now: legacy.go reports it and `--legacy-files` removes it.
const rootGraphCacheLine = ".rtdd/graph.json"

// OptInMarker sets apart what `rtdd init --project-skill` and `--append-to` write from
// what an older init wrote unasked: a file or block carrying it was asked for, and so is
// never reported or removed as legacy. Plan puts it on the line after protocol.Generated or
// protocol.BeginMarker.
const OptInMarker = "<!-- rtdd: opted in with `rtdd init --project-skill` or `--append-to`, so not legacy -->"

// ProjectSkillFiles maps the whole-file front-ends `rtdd init --project-skill` writes to the
// generator output each one is rendered from: every Agents() row's Project.
var ProjectSkillFiles = projectSkillFiles()

// PlanOptions are `rtdd init`'s opt-ins. Without them init writes inside .rtdd/ and
// nowhere else: a user's AGENTS.md, CLAUDE.md, .gitignore, .claude/ and .cursor/ stay
// theirs (#509).
type PlanOptions struct {
	// ProjectSkill writes the Claude Code skill and the Cursor rule into the repository, for
	// a team that wants rtdd's instructions committed for every contributor. Both are files
	// rtdd owns outright, never a block inside a shared file.
	ProjectSkill bool
	// AppendTo names shared files — AGENTS.md, CLAUDE.md, anything — to merge rtdd's
	// marker-delimited block into. Each one is named explicitly; none is implied.
	AppendTo []string
	// Home is the user's home directory. When the repository root is the home directory,
	// a ProjectSkillFiles path that is also a user-level install path is `rtdd skill
	// install`'s, not an older init's footprint: PlanLegacyFiles leaves it out (#554), and
	// --project-skill does not write it (#558).
	// Empty means the root is never treated as the home directory.
	Home string
}

// Plan computes what `rtdd init` would do without touching the filesystem.
//
// files maps the generator's output paths to their content, i.e. exactly what
// protocol.RenderAll returns. By default the plan is .rtdd/config.yaml and .rtdd/.gitignore
// and nothing else, followed by a Legacy step for each piece of an older init's footprint it
// finds (legacy.go); those steps are a report, and Apply writes none of them.
//
// The opt-ins keep the old rules. A whole-file front-end is created, or replaced when the
// file on disk is an earlier rtdd render — it carries protocol.Generated; a file there that
// rtdd did not write is a conflict no flag overrides. A file named by AppendTo is merged
// between rtdd's markers, which never touches a byte outside them.
func Plan(root string, files map[string]string, opts PlanOptions) ([]Step, error) {
	// 0. What a v0.2 init left behind goes first, each removal its own printed step.
	steps, err := PlanMigration(root)
	if err != nil {
		return nil, err
	}

	// 1. .rtdd/config.yaml — created; replaced only when it is v0.2's (isV02Config).
	cfg, err := os.ReadFile(filepath.Join(root, ".rtdd", "config.yaml"))
	switch {
	case os.IsNotExist(err):
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Create, Content: DefaultConfig()})
	case err != nil:
		return nil, err
	case isV02Config(cfg):
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Replace, Content: DefaultConfig(), Note: "the v0.2 config; v0.3.0 reads none of its keys"})
	default:
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Skip, Note: "keeping your config"})
	}

	// 2. .rtdd/.gitignore — the graph cache is rebuilt in seconds and conflicts on every
	// branch when committed (spec §4.5), so init ignores it, once.
	step, err := planRtddGitignore(root)
	if err != nil {
		return nil, err
	}
	steps = append(steps, step)

	// 3. Opt-in: the whole-file front-ends.
	if opts.ProjectSkill {
		atHome := opts.Home != "" && samePath(root, opts.Home)
		for _, rel := range sortedKeys(ProjectSkillFiles) {
			content := markOptIn(files[ProjectSkillFiles[rel]], protocol.Generated)
			abs := filepath.Join(root, filepath.FromSlash(rel))
			existing, err := os.ReadFile(abs)
			switch {
			case atHome && userLevel(rel):
				// Run from the home directory, this path is a user-level install: writing the
				// project render there would change what agents in every repository read (#558).
				note := "kept; a user-level install — `rtdd skill install` manages it"
				if os.IsNotExist(err) {
					note = "a user-level install path — `rtdd skill install` writes it"
				} else if err != nil {
					return nil, err
				}
				steps = append(steps, Step{Path: rel, Action: Skip, Note: note})
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
	}

	// 4. Opt-in: rtdd's block, merged into each shared file named explicitly.
	block := markOptIn(files["dist/AGENTS.md"], protocol.BeginMarker)
	for _, name := range opts.AppendTo {
		rel, err := repoRel(root, name)
		if err != nil {
			steps = append(steps, Step{Path: name, Action: Conflict, Note: err.Error()})
			continue
		}
		existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
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

	// 5. What an older init wrote outside .rtdd/, reported and left alone.
	legacy, err := PlanLegacyFiles(root, opts)
	if err != nil {
		return nil, err
	}
	return append(steps, ReportLegacy(legacy, "")...), nil
}

// planRtddGitignore adds graphCacheLine to .rtdd/.gitignore unless a line already names it.
// The file is rtdd's own, so appending to one that holds other lines is safe.
func planRtddGitignore(root string) (Step, error) {
	const rel = ".rtdd/.gitignore"
	b, err := os.ReadFile(filepath.Join(root, ".rtdd", ".gitignore"))
	switch {
	case os.IsNotExist(err):
		return Step{Path: rel, Action: Create, Content: graphCacheLine + "\n"}, nil
	case err != nil:
		return Step{}, err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l == graphCacheLine || l == "/"+graphCacheLine {
			return Step{Path: rel, Action: Skip, Note: "already ignores " + graphCacheLine}, nil
		}
	}
	body := string(b)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return Step{Path: rel, Action: AppendLine, Content: body + graphCacheLine + "\n"}, nil
}

// markOptIn puts OptInMarker on the line after the first anchor in content. Every rendered
// front-end carries its anchor once (TestGolden), so this never misses.
func markOptIn(content, anchor string) string {
	return strings.Replace(content, anchor, anchor+"\n"+OptInMarker, 1)
}

// repoRel resolves an --append-to argument to a slash-separated path inside root. A path
// that leaves the repository, or lands in .rtdd/, is refused: the first is not this
// repository's file, and the second is rtdd's own.
func repoRel(root, name string) (string, error) {
	abs := name
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, name)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("--append-to %s: not a file inside this repository", name)
	}
	rel = filepath.ToSlash(rel)
	if rel == ".rtdd" || strings.HasPrefix(rel, ".rtdd/") {
		return "", fmt.Errorf("--append-to %s: .rtdd/ is rtdd's own directory", name)
	}
	return rel, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func Apply(root string, steps []Step) error {
	for _, s := range steps {
		if s.Action == Skip || s.Action == Legacy {
			continue
		}
		if s.Action == Conflict {
			return fmt.Errorf("%s: %s", s.Path, s.Note)
		}
		abs := filepath.Join(root, filepath.FromSlash(s.Path))
		if s.Action == Delete {
			if err := os.RemoveAll(abs); err != nil {
				return err
			}
			continue
		}
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
