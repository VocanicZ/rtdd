package gitctx

import "strings"

// RawDiff returns the full text of `git diff --unified=0 -M <base>` for the working
// tree, so a changed-line range is parsed from one text and there is exactly one source
// of truth for it. An empty base means HEAD, as in ChangedSet — a caller of both passes
// them the same base, and a disagreement between them is a wrong line range no one
// would see.
//
// Renames are detected so the new-side path is authoritative, and every git config that
// rewrites the `+++ b/<path>` header is pinned off: that header is the only key by which
// a hunk finds its file. Errors carry git's own stderr; a swallowed one would report a
// repository with no changed lines, which reads as "nothing to test".
func RawDiff(repoRoot, base string) (string, error) {
	if strings.TrimSpace(base) == "" {
		base = "HEAD"
	}
	return git(repoRoot,
		"-c", "core.quotePath=false",
		"-c", "diff.noprefix=false",
		"-c", "diff.mnemonicPrefix=false",
		"-c", "diff.srcPrefix=a/",
		"-c", "diff.dstPrefix=b/",
		"diff", "--unified=0", "--no-color", "-M", base)
}
