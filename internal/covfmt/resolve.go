package covfmt

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Resolver maps reported paths onto repo files. Build it once per run (NewResolver) and
// call Resolve per unit: the repo-file index is the expensive part.
type Resolver struct {
	root   string
	roots  []string // root, and root with symlinks evaluated when that differs
	set    map[string]bool
	byBase map[string][]string
}

// NewResolver indexes repoFiles under root. A root that cannot be made absolute resolves
// nothing.
func NewResolver(root string, repoFiles []string) *Resolver {
	r := &Resolver{set: make(map[string]bool, len(repoFiles)), byBase: map[string][]string{}}
	abs, err := filepath.Abs(root)
	if err != nil {
		return r
	}
	r.root = filepath.ToSlash(filepath.Clean(abs))
	r.roots = []string{r.root}
	// Tools report the real path of a repo reached through a symlink.
	if real, err := filepath.EvalSymlinks(abs); err == nil && filepath.ToSlash(real) != r.root {
		r.roots = append(r.roots, filepath.ToSlash(real))
	}
	for _, f := range repoFiles {
		r.set[f] = true
		r.byBase[path.Base(f)] = append(r.byBase[path.Base(f)], f)
	}
	return r
}

// Resolve is NewResolver(root, repoFiles).Resolve(raw).
func Resolve(root string, raw Lines, repoFiles []string) map[string][]int {
	return NewResolver(root, repoFiles).Resolve(raw)
}

// ResolveReport resolves a report's hit and executable lines with ONE path mapping, taken
// over every reported path, hit or not: a unit that hit one file often reports other
// measured files at count 0 (the -coverpkg packages linked into a Go test binary), and
// those give the consensus prefix the support it needs to resolve the hit file.
func (r *Resolver) ResolveReport(rep Report) (hit, exec map[string][]int) {
	hit, exec = map[string][]int{}, map[string][]int{}
	if r.root == "" {
		return hit, exec
	}
	all := Lines{}
	for p := range rep.Exec {
		all[p] = nil
	}
	for p := range rep.Hit {
		all[p] = nil
	}
	consensusPrefix := computeConsensusPrefix(all, r.set)
	for _, pair := range []struct {
		in  Lines
		out map[string][]int
	}{{rep.Hit, hit}, {rep.Exec, exec}} {
		for p, lines := range pair.in {
			rel, ok := resolveOne(r.roots, p, r.set, r.byBase, consensusPrefix)
			if ok {
				pair.out[rel] = sortedUnique(append(pair.out[rel], lines...))
			}
		}
	}
	return hit, exec
}

// Resolve maps each reported path onto a repo-relative path, merging lines that arrive
// under two spellings. For relative paths: exact match, then unique suffix match, then
// consensus prefix (if support >= 2 distinct reported paths). Absolute paths: rel-to-root
// exact match only. Paths that escape the repo (..) or resolve to nothing/ambiguously are
// dropped.
func (r *Resolver) Resolve(raw Lines) map[string][]int {
	out := map[string][]int{}
	if r.root == "" {
		return out
	}
	consensusPrefix := computeConsensusPrefix(raw, r.set)
	for p, lines := range raw {
		rel, ok := resolveOne(r.roots, p, r.set, r.byBase, consensusPrefix)
		if !ok {
			continue
		}
		out[rel] = sortedUnique(append(out[rel], lines...))
	}
	return out
}

// computeConsensusPrefix finds the prefix that, when stripped, best resolves distinct reported paths.
// A prefix "counts" for each distinct reported path that can be resolved by it (starts with prefix
// and stripping yields a repo file). Only returns the prefix with highest count if count >= 2.
// Skips absolute paths, exact matches, and ".." paths (which escape the repo).
func computeConsensusPrefix(raw Lines, set map[string]bool) string {
	prefixPaths := make(map[string]map[string]bool) // map[prefix]map[path]bool to track distinct paths

	for p := range raw {
		p = filepath.ToSlash(strings.ReplaceAll(p, "\\", "/"))

		// Skip absolute paths and exact matches.
		if path.IsAbs(p) || filepath.IsAbs(p) {
			continue
		}

		// Clean and normalize relative paths for exact match check.
		cleaned := strings.TrimPrefix(path.Clean(p), "./")

		// Skip paths that escape the repo (.. or ../)
		if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
			continue
		}

		// Skip exact matches
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
				// This prefix resolves this path
				if prefixPaths[prefix] == nil {
					prefixPaths[prefix] = make(map[string]bool)
				}
				prefixPaths[prefix][cleaned] = true
			}
			q = candidate
		}
	}

	// Pick the prefix with highest count of distinct paths (>= 2); ties: lexically smallest.
	// Sort prefixes to make tie-breaking deterministic.
	var prefixes []string
	for prefix := range prefixPaths {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	var best string
	bestCount := 0
	for _, prefix := range prefixes {
		count := len(prefixPaths[prefix])
		if count >= 2 && (count > bestCount || (count == bestCount && (best == "" || prefix < best))) {
			best = prefix
			bestCount = count
		}
	}
	return best
}

func resolveOne(roots []string, p string, set map[string]bool, byBase map[string][]string, consensusPrefix string) (string, bool) {
	p = filepath.ToSlash(strings.ReplaceAll(p, "\\", "/"))

	// Handle absolute paths: must be under a root, exact match only.
	if path.IsAbs(p) || filepath.IsAbs(p) {
		for _, root := range roots {
			r, err := filepath.Rel(root, filepath.FromSlash(p))
			if err != nil {
				continue
			}
			r = filepath.ToSlash(r)
			if r == ".." || strings.HasPrefix(r, "../") {
				continue
			}
			if set[r] {
				return r, true
			}
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
	if match != "" {
		return match, true
	}

	// Consensus prefix: strip if it matches and resolves to set.
	if consensusPrefix != "" && strings.HasPrefix(p, consensusPrefix) {
		candidate := strings.TrimPrefix(p, consensusPrefix)
		if set[candidate] {
			return candidate, true
		}
	}

	return "", false
}
