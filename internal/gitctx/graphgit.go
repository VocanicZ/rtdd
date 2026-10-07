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

// BlobIDs maps every file in HEAD's tree to its git blob id. A file whose id is unchanged
// since the graph cache recorded it need not be re-scanned (spec §4.5). An unborn HEAD —
// no commits yet — is an empty map, not an error: every file is then in the changed set.
func BlobIDs(repoRoot string) (map[string]string, error) {
	out := map[string]string{}
	if _, err := git(repoRoot, "rev-parse", "--verify", "-q", "HEAD^{commit}"); err != nil {
		return out, nil
	}
	ls, err := git(repoRoot, "ls-tree", "-r", "-z", "--full-tree", "HEAD")
	if err != nil {
		return nil, err
	}
	for _, rec := range strings.Split(ls, "\x00") {
		// <mode> SP <type> SP <object> TAB <path>
		meta, p, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) == 3 && f[1] == "blob" {
			out[p] = f[2]
		}
	}
	return out, nil
}
