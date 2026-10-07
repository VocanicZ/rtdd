package graphbuild

import (
	"path"
	"sort"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graphify"
)

// staleness decides how far graphify can be trusted: the stale set, how many of it are
// code files (the max_stale_ratio numerator), or why graphify is ignored entirely (spec
// §5 step 4) — no built_at_commit, one git does not know, or more than maxRatio of
// graphify's code files stale. A graphify graph with no code files vouches for nothing
// and is too stale.
func staleness(root string, gf *graphify.Graph, files []string, changed map[string]bool, maxRatio float64) ([]string, int, string, error) {
	if gf.BuiltAtCommit == "" {
		return nil, 0, IgnoredNoCommit, nil
	}
	if !gitctx.CommitKnown(root, gf.BuiltAtCommit) {
		return nil, 0, IgnoredUnknownCommit, nil
	}
	stale, err := StaleSet(root, gf, files, changed)
	if err != nil {
		return nil, 0, "", err
	}
	isCode := codeFiles(gf)
	code := 0
	for _, f := range stale {
		if isCode(f) {
			code++
		}
	}
	if len(gf.CodeFiles) == 0 || float64(code) > maxRatio*float64(len(gf.CodeFiles)) {
		return stale, code, IgnoredTooStale, nil
	}
	return stale, code, "", nil
}

// codeFiles reports whether a file is code by plan decision 10: one graphify holds code
// nodes for, or one sharing an extension with one. Only these count toward
// max_stale_ratio, so a README edit or a language graphify never saw does not make
// graphify too stale.
func codeFiles(gf *graphify.Graph) func(string) bool {
	codeFile := map[string]bool{}
	ext := map[string]bool{}
	for _, f := range gf.CodeFiles {
		codeFile[f] = true
		ext[path.Ext(f)] = true
	}
	return func(f string) bool { return codeFile[f] || ext[path.Ext(f)] && path.Ext(f) != "" }
}

// StaleSet is spec §5 step 1, the files the scanner overlays on graphify's graph: every
// listed file (files, already through the §4.1 filter) changed since graphify's
// built_at_commit (committed, staged or not) or in the working-tree changed set
// (untracked included), whatever its extension — "every changed file is always scanned"
// — ∪ code files absent from graphify's manifest.json, ∪ code files graphify holds that
// are no longer listed (deleted), whose nodes the overlay drops.
func StaleSet(root string, gf *graphify.Graph, files []string, changed map[string]bool) ([]string, error) {
	isCode := codeFiles(gf)
	graphifyFile := map[string]bool{}
	for _, f := range gf.CodeFiles {
		graphifyFile[f] = true
	}
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
			if !inManifest[f] && isCode(f) {
				set[f] = true
			}
		}
	}
	var out []string
	for f := range set {
		if listed[f] || graphifyFile[f] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out, nil
}
