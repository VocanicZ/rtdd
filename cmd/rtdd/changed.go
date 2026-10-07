package main

import (
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// rtddDirName is the one directory rtdd writes into the host repository.
const rtddDirName = ".rtdd"

// changedSet is the ONLY changed set a command may act on: gitctx's, with rtdd's
// own bookkeeping removed.
//
// The exclusion lives here and not in gitctx on purpose. gitctx reports what git
// reports; ".rtdd/ is rtdd's own" is rtdd policy, and a git-context package that
// knows the name of its caller's state directory is the wrong shape.
func changedSet(repoRoot, base string) ([]gitctx.Change, error) {
	changes, err := gitctx.ChangedSet(repoRoot, base)
	if err != nil {
		return nil, err
	}
	return excludeRtddDir(changes), nil
}

// excludeRtddDir removes rtdd's own writes from a changed set.
//
// .rtdd/ holds rtdd's config and its graph cache (graph.json, rewritten whenever the
// graph is rebuilt). Left in, rtdd's own cache write would read as a change the
// developer made and select on nothing they touched.
//
// OldPath is cleared rather than kept: a rename out of .rtdd/ would otherwise smuggle a
// .rtdd/ path through a Path-only filter. The file itself survives — it is a real user
// file now.
func excludeRtddDir(in []gitctx.Change) []gitctx.Change {
	out := make([]gitctx.Change, 0, len(in))
	for _, c := range in {
		if underRtddDir(c.Path) {
			continue
		}
		if underRtddDir(c.OldPath) {
			c.OldPath = ""
		}
		out = append(out, c)
	}
	return out
}

// underRtddDir reports whether a repo-relative git path is rtdd's own state.
//
// git reports paths relative to the repository root, slash-separated and unclean-free,
// so the root-anchored prefix test is exact: a `src/.rtdd/` or `.rtddx/` belongs to
// the host repo, not to rtdd, and must keep selecting.
func underRtddDir(p string) bool {
	return p == rtddDirName || strings.HasPrefix(p, rtddDirName+"/")
}
