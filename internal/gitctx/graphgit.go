package gitctx

import (
	"sort"
	"strings"
)

// DiffNamesSince lists every path that differs between commit and the working tree —
// committed since, staged, or unstaged; deletions included; untracked files NOT included
// (ChangedSet has those). Sorted. graphify's staleness reads it (spec §5 step 1).
func DiffNamesSince(repoRoot, commit string) ([]string, error) {
	out, err := git(repoRoot, "diff", "--name-only", "-z", "--no-renames", commit, "--")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			names = append(names, p)
		}
	}
	sort.Strings(names)
	return names, nil
}
