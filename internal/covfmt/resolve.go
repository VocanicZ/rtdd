package covfmt

import (
	"path"
	"path/filepath"
	"strings"
)

// Resolve maps each reported path onto a repo-relative path from repoFiles, merging lines
// that arrive under two spellings. In order: an absolute path under root; the path itself;
// the path with leading segments dropped (Go import paths); the unique repo file ending in
// "/"+path (JaCoCo package paths). A path that resolves to nothing, or to more than one
// file, is dropped — attributing a line to the wrong file is worse than losing it.
func Resolve(root string, raw Lines, repoFiles []string) map[string][]int {
	set := make(map[string]bool, len(repoFiles))
	byBase := map[string][]string{}
	for _, f := range repoFiles {
		set[f] = true
		byBase[path.Base(f)] = append(byBase[path.Base(f)], f)
	}
	out := map[string][]int{}
	for p, lines := range raw {
		rel, ok := resolveOne(root, p, set, byBase)
		if !ok {
			continue
		}
		out[rel] = sortedUnique(append(out[rel], lines...))
	}
	return out
}

func resolveOne(root, p string, set map[string]bool, byBase map[string][]string) (string, bool) {
	p = filepath.ToSlash(p)
	if path.IsAbs(p) || filepath.IsAbs(p) {
		r, err := filepath.Rel(root, filepath.FromSlash(p))
		if err != nil {
			return "", false
		}
		r = filepath.ToSlash(r)
		if r == ".." || strings.HasPrefix(r, "../") {
			return "", false
		}
		p = r
	}
	p = strings.TrimPrefix(path.Clean(p), "./")
	if set[p] {
		return p, true
	}
	for q := p; ; {
		i := strings.Index(q, "/")
		if i < 0 {
			break
		}
		q = q[i+1:]
		if set[q] {
			return q, true
		}
	}
	match := ""
	for _, f := range byBase[path.Base(p)] {
		if strings.HasSuffix(f, "/"+p) {
			if match != "" {
				return "", false
			}
			match = f
		}
	}
	return match, match != ""
}
