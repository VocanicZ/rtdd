package gitctx

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListFiles is every file git would call part of the working tree: tracked, plus
// untracked files not ignored, minus tracked files deleted from disk. It is how the
// pipeline enumerates units — a test file written a moment ago is a unit before it is
// committed (spec §4.1).
func ListFiles(repoRoot string) ([]string, error) {
	out, err := git(repoRoot, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var files []string
	for _, p := range strings.Split(out, "\x00") {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(p))); err != nil {
			continue
		}
		files = append(files, p)
	}
	sort.Strings(files)
	return files, nil
}
