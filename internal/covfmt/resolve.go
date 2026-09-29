package covfmt

import (
	"path"
	"path/filepath"
	"strings"
)

// Resolve maps each reported path onto a repo-relative path from repoFiles, merging lines
// that arrive under two spellings. In order: an absolute path under root (exact match only);
// the path itself (exact match); consensus prefix (drop leading segments using prefix chosen
// by majority); the unique repo file ending in "/"+path (JaCoCo package paths). A path that
// resolves to nothing, or to more than one file, is dropped — attributing a line to the
// wrong file is worse than losing it.
func Resolve(root string, raw Lines, repoFiles []string) map[string][]int {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return map[string][]int{}
	}
	root = filepath.ToSlash(filepath.Clean(root))

	set := make(map[string]bool, len(repoFiles))
	byBase := map[string][]string{}
	for _, f := range repoFiles {
		set[f] = true
		byBase[path.Base(f)] = append(byBase[path.Base(f)], f)
	}

	// Compute consensus prefix: the most common prefix to strip across relative non-exact paths.
	consensusPrefix := computeConsensusPrefix(raw, set)

	out := map[string][]int{}
	for p, lines := range raw {
		rel, ok := resolveOne(root, p, set, byBase, consensusPrefix)
		if !ok {
			continue
		}
		out[rel] = sortedUnique(append(out[rel], lines...))
	}
	return out
}

// computeConsensusPrefix finds the prefix that, when stripped, best matches paths in set.
// A prefix "counts" for a path if stripping it from the path yields a repo file.
// Returns the prefix with the highest count; ties favor the shorter prefix.
func computeConsensusPrefix(raw Lines, set map[string]bool) string {
	prefixCount := make(map[string]int)

	for p := range raw {
		p = filepath.ToSlash(strings.ReplaceAll(p, "\\", "/"))

		// Skip absolute paths and exact matches.
		if path.IsAbs(p) || filepath.IsAbs(p) {
			continue
		}

		// Clean and normalize relative paths for exact match check.
		cleaned := strings.TrimPrefix(path.Clean(p), "./")
		if set[cleaned] {
			continue
		}

		// Enumerate candidate prefixes (all leading "/" runs).
		prefix := ""
		for q := cleaned; ; {
			i := strings.Index(q, "/")
			if i < 0 {
				break
			}
			segment := q[:i]
			prefix += segment + "/"
			candidate := q[i+1:]
			if candidate != "" && set[candidate] {
				prefixCount[prefix]++
			}
			q = candidate
		}
	}

	// Pick the prefix with the highest count; ties: shorter prefix.
	var best string
	bestCount := 0
	for prefix, count := range prefixCount {
		if count > bestCount || (count == bestCount && len(prefix) < len(best)) {
			best = prefix
			bestCount = count
		}
	}
	return best
}

func resolveOne(root, p string, set map[string]bool, byBase map[string][]string, consensusPrefix string) (string, bool) {
	p = filepath.ToSlash(strings.ReplaceAll(p, "\\", "/"))

	// Handle absolute paths: must be under root, exact match only.
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
		// For absolute paths, only exact match in set.
		if set[p] {
			return p, true
		}
		return "", false
	}

	// Relative path: clean and normalize.
	p = strings.TrimPrefix(path.Clean(p), "./")

	// Drop relative paths that escape the repo.
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}

	// Exact match.
	if set[p] {
		return p, true
	}

	// Consensus prefix: strip if it matches.
	if consensusPrefix != "" && strings.HasPrefix(p, consensusPrefix) {
		candidate := strings.TrimPrefix(p, consensusPrefix)
		if set[candidate] {
			return candidate, true
		}
	}

	// Suffix match: unique repo file ending in "/"+p.
	match := ""
	for _, f := range byBase[path.Base(p)] {
		if strings.HasSuffix(f, "/"+p) {
			if match != "" {
				return "", false // ambiguous
			}
			match = f
		}
	}
	return match, match != ""
}
