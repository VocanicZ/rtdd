// Package initrepo installs RTDD into a host repository: the union merge driver, the
// config, and the agent front-ends. Nothing is ever clobbered.
package initrepo

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/agents-block.md
var agentsBlock string

// Markers delimiting the RTDD-managed region of a host repo's agent front-end file.
const (
	BeginMarker = "<!-- BEGIN RTDD -->"
	EndMarker   = "<!-- END RTDD -->"
)

const gitAttributesLine = ".rtdd/map.jsonl merge=union"

const defaultConfig = "# rtdd configuration — see docs/specs for meaning\n" +
	"stale_commits: 50\n" +
	"drift_guard: 100\n" +
	"hub_threshold: 0.40\n"

// Action records what init did to one path.
type Action struct {
	Path string // repo-relative
	Kind string // "created" | "updated" | "unchanged"
}

// Block returns the managed agent front-end text, marker lines included.
func Block() string { return strings.TrimRight(agentsBlock, "\n") }

// MergeManagedBlock returns existing with block installed between the markers.
//
// Never clobbers. If the markers are present, only the text between them changes and
// everything outside is preserved byte for byte. If they are absent, block is appended
// after a blank line and the original content comes first.
func MergeManagedBlock(existing, block string) string {
	block = strings.TrimRight(block, "\n")
	if existing == "" {
		return block + "\n"
	}
	b := strings.Index(existing, BeginMarker)
	e := strings.Index(existing, EndMarker)
	if b >= 0 && e > b {
		return existing[:b] + block + existing[e+len(EndMarker):]
	}
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return existing + "\n" + block + "\n"
}

// EnsureGitAttributes adds the union merge driver for .rtdd/map.jsonl. Every other line
// the host repo wrote is left exactly where it was.
func EnsureGitAttributes(repoRoot string) (Action, error) {
	rel := ".gitattributes"
	cur, existed, err := readIfExists(repoRoot, rel)
	if err != nil {
		return Action{}, err
	}
	for _, line := range strings.Split(cur, "\n") {
		if strings.TrimSpace(line) == gitAttributesLine {
			return Action{Path: rel, Kind: "unchanged"}, nil
		}
	}
	next := cur
	if next != "" && !strings.HasSuffix(next, "\n") {
		next += "\n"
	}
	next += gitAttributesLine + "\n"
	if err := writeFile(repoRoot, rel, next); err != nil {
		return Action{}, err
	}
	if existed {
		return Action{Path: rel, Kind: "updated"}, nil
	}
	return Action{Path: rel, Kind: "created"}, nil
}

// EnsureConfig writes .rtdd/config.yaml only when it does not already exist. A host
// repo's tuned config is never overwritten.
func EnsureConfig(repoRoot string) (Action, error) {
	rel := ".rtdd/config.yaml"
	_, existed, err := readIfExists(repoRoot, rel)
	if err != nil {
		return Action{}, err
	}
	if existed {
		return Action{Path: rel, Kind: "unchanged"}, nil
	}
	if err := writeFile(repoRoot, rel, defaultConfig); err != nil {
		return Action{}, err
	}
	return Action{Path: rel, Kind: "created"}, nil
}

// EnsureFrontEnd installs block into repoRoot/rel using MergeManagedBlock. Nothing is
// written when the merge is already a fixed point.
func EnsureFrontEnd(repoRoot, rel, block string) (Action, error) {
	cur, existed, err := readIfExists(repoRoot, rel)
	if err != nil {
		return Action{}, err
	}
	next := MergeManagedBlock(cur, block)
	if next == cur {
		return Action{Path: rel, Kind: "unchanged"}, nil
	}
	if err := writeFile(repoRoot, rel, next); err != nil {
		return Action{}, err
	}
	if existed {
		return Action{Path: rel, Kind: "updated"}, nil
	}
	return Action{Path: rel, Kind: "created"}, nil
}

// Run installs .gitattributes, .rtdd/config.yaml, AGENTS.md, CLAUDE.md and
// .cursor/rules/rtdd.mdc. Actions are returned in installation order.
func Run(repoRoot string) ([]Action, error) {
	var acts []Action

	a, err := EnsureGitAttributes(repoRoot)
	if err != nil {
		return nil, err
	}
	acts = append(acts, a)

	a, err = EnsureConfig(repoRoot)
	if err != nil {
		return nil, err
	}
	acts = append(acts, a)

	for _, rel := range []string{"AGENTS.md", "CLAUDE.md", ".cursor/rules/rtdd.mdc"} {
		a, err = EnsureFrontEnd(repoRoot, rel, Block())
		if err != nil {
			return nil, err
		}
		acts = append(acts, a)
	}
	return acts, nil
}

func readIfExists(repoRoot, rel string) (string, bool, error) {
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(b), true, nil
}

func writeFile(repoRoot, rel, content string) error {
	p := filepath.Join(repoRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}
