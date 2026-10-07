package install

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// DefaultConfig renders .rtdd/config.yaml as `rtdd init` writes it: graph.DefaultConfig()
// with every key spelled out, so a user edits a value rather than learning a key name.
// It is generated from graph.DefaultConfig, never typed twice, so the file init writes and
// the defaults the graph applies cannot drift (TestDefaultConfigLoadsAsTheGraphDefaults).
func DefaultConfig() string {
	d := graph.DefaultConfig()
	var b strings.Builder
	b.WriteString("# .rtdd/config.yaml — written by `rtdd init`. Every list is globs over\n")
	b.WriteString("# repo-relative paths; a key you set replaces its default wholesale.\n")
	b.WriteString("# See docs/specs/2026-10-07-node-graph.md §4.1, §5 and §6.\n")
	list := func(comment, key string, globs []string) {
		fmt.Fprintf(&b, "\n# %s\n%s:\n", comment, key)
		for _, g := range globs {
			fmt.Fprintf(&b, "  - %q\n", g)
		}
	}
	list("Files the scanner never reads.", "scan_exclude", d.ScanExclude)
	list("A function, method or test in a matching file is a test.", "test_files", d.TestFiles)
	list("A matching file is never a test file, whatever test_files says.", "test_exclude", d.TestExclude)
	fmt.Fprintf(&b, "\n# graphify's graph, used when this file exists. rtdd never runs graphify.\ngraphify_path: %q\n", d.GraphifyPath)
	fmt.Fprintf(&b, "\n# graphify is ignored when more than this share of its code files is stale.\nmax_stale_ratio: %v\n", d.MaxStaleRatio)
	return b.String()
}
