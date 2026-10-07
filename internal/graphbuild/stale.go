package graphbuild

import (
	"path"
	"sort"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graphify"
)

// staleness decides how far graphify can be trusted: the stale set, or why graphify is
// ignored entirely (spec §5 step 4) — no built_at_commit, one git does not know, or more
// than maxRatio of graphify's code files stale. A graphify graph with no code files
// vouches for nothing and is too stale.
func staleness(root string, gf *graphify.Graph, files []string, changed map[string]bool, maxRatio float64) ([]string, string, error) {
	if gf.BuiltAtCommit == "" {
		return nil, IgnoredNoCommit, nil
	}
	if !gitctx.CommitKnown(root, gf.BuiltAtCommit) {
		return nil, IgnoredUnknownCommit, nil
	}
	stale, err := StaleSet(root, gf, files, changed)
	if err != nil {
		return nil, "", err
	}
	if len(gf.CodeFiles) == 0 || float64(len(stale)) > maxRatio*float64(len(gf.CodeFiles)) {
		return stale, IgnoredTooStale, nil
	}
	return stale, "", nil
}

// StaleSet is spec §5 step 1: files changed since graphify's built_at_commit (committed,
// staged or not), ∪ the working-tree changed set (untracked included), ∪ files absent from
// graphify's manifest.json — restricted to CODE files: one graphify holds code nodes for,
// or a listed file sharing an extension with one. A README edit does not make graphify stale.
func StaleSet(root string, gf *graphify.Graph, files []string, changed map[string]bool) ([]string, error) {
	codeFile := map[string]bool{}
	ext := map[string]bool{}
	for _, f := range gf.CodeFiles {
		codeFile[f] = true
		ext[path.Ext(f)] = true
	}
	isCode := func(f string) bool { return codeFile[f] || ext[path.Ext(f)] && path.Ext(f) != "" }
	listed := map[string]bool{}
	for _, f := range files {
		listed[f] = true
	}

	set := map[string]bool{}
	diff, err := gitctx.DiffNamesSince(root, gf.BuiltAtCommit)
	if err != nil {
		return nil, err
	}
	for _, f := range diff {
		set[f] = true
	}
	for f := range changed {
		set[f] = true
	}
	if gf.Manifest != nil {
		inManifest := map[string]bool{}
		for _, f := range gf.Manifest {
			inManifest[f] = true
		}
		for _, f := range files {
			if !inManifest[f] {
				set[f] = true
			}
		}
	}
	var out []string
	for f := range set {
		if isCode(f) && (listed[f] || codeFile[f]) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out, nil
}
