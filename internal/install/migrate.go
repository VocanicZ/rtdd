package install

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// v02Files are the state files a v0.2 repository holds and v0.3.0 reads none of (spec §8,
// "Removed"), with the note `rtdd init` prints as it deletes each one.
var v02Files = []struct{ path, note string }{
	{".rtdd/map.jsonl", "the v0.2 coverage map; v0.3.0 builds a node graph instead"},
	{".rtdd/meta.json", "the v0.2 map's metadata"},
	{".rtdd/adapters/", "v0.2 adapter definitions; v0.3.0 has no adapters"},
}

// PlanMigration computes what `rtdd init` removes from a repository a v0.2 init set up:
// the three state files above, and the merge-driver line in .gitattributes — the file
// itself only when that line was all it held, as uninstall does. A repository v0.2 never
// touched plans no step at all, so its init output carries no removal line.
func PlanMigration(root string) ([]Step, error) {
	steps := []Step{}
	for _, f := range v02Files {
		switch _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f.path))); {
		case os.IsNotExist(err):
		case err != nil:
			return nil, err
		default:
			steps = append(steps, Step{Path: f.path, Action: Delete, Note: f.note})
		}
	}
	b, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	remaining := removeLine(string(b), gitattributesLine)
	if remaining == string(b) {
		return steps, nil
	}
	note := "removed the v0.2 line " + gitattributesLine
	if strings.TrimSpace(remaining) != "" {
		return append(steps, Step{Path: ".gitattributes", Action: StripBlock, Content: remaining, Note: note}), nil
	}
	return append(steps, Step{Path: ".gitattributes", Action: Delete, Note: "held nothing but the v0.2 line " + gitattributesLine}), nil
}

// isV02Config reports whether a .rtdd/config.yaml is the one v0.2 wrote: it sets at least
// one key only v0.2 read and none v0.3.0 reads. Such a file is replaced by DefaultConfig;
// a file setting any v0.3.0 key is the user's and is kept. Unparseable YAML is kept too —
// graph.LoadConfig reports it, naming the file.
func isV02Config(b []byte) bool {
	var keys map[string]any
	if yaml.Unmarshal(b, &keys) != nil {
		return false
	}
	v02 := false
	for k := range keys {
		switch k {
		case "scan_exclude", "test_files", "test_exclude", "graphify_path", "max_stale_ratio":
			return false
		case "stale_commits", "drift_guard", "hub_threshold", "adapters":
			v02 = true
		}
	}
	return v02
}
