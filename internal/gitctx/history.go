package gitctx

import (
	"os/exec"
	"strconv"
	"strings"
)

// CommitDistance returns the number of commits from sha to HEAD.
// Returns (-1, nil) when sha is unreachable — after a rebase, squash, or shallow clone.
// Callers MUST treat -1 as "unknown", never as "fresh": reporting an unreachable seed
// commit as distance 0 would present an arbitrarily stale map as current and staleness
// escalation would never fire.
// An error is returned only when git itself is unusable, which is exit code 3 territory.
func CommitDistance(repoRoot, sha string) (int, error) {
	if strings.TrimSpace(sha) == "" {
		return -1, nil
	}
	if _, err := git(repoRoot, "rev-parse", "--git-dir"); err != nil {
		return -1, err
	}
	if !reachable(repoRoot, sha) {
		return -1, nil
	}
	out, err := git(repoRoot, "rev-list", "--count", sha+"..HEAD")
	if err != nil {
		return -1, nil
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(out))
	if convErr != nil {
		return -1, nil
	}
	return n, nil
}

// IsMergeCommit reports whether sha names a commit with more than one parent. An empty
// sha means HEAD.
func IsMergeCommit(repoRoot, sha string) (bool, error) {
	if strings.TrimSpace(sha) == "" {
		sha = "HEAD"
	}
	out, err := git(repoRoot, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return false, err
	}
	// "<sha> <parent1> [<parent2> ...]"
	return len(strings.Fields(strings.TrimSpace(out))) > 2, nil
}

// Older returns whichever of a or b is the earlier ancestor. If either is unreachable,
// it returns that one (unknown age is treated as older, i.e. less trustworthy).
// The returned function is the commit-age comparator mapstore.LoadWith takes.
func Older(repoRoot string) func(a, b string) string {
	return func(a, b string) string {
		switch {
		case a == "":
			return b
		case b == "":
			return a
		case a == b:
			return a
		}
		aOK, bOK := reachable(repoRoot, a), reachable(repoRoot, b)
		switch {
		case !aOK:
			return a
		case !bOK:
			return b
		case isAncestor(repoRoot, a, b):
			return a
		case isAncestor(repoRoot, b, a):
			return b
		}
		// Unrelated histories: the one further from HEAD is the older one.
		da, _ := CommitDistance(repoRoot, a)
		db, _ := CommitDistance(repoRoot, b)
		if db > da {
			return b
		}
		return a
	}
}

// reachable reports whether sha resolves to a commit object in repoRoot.
func reachable(repoRoot, sha string) bool {
	_, err := git(repoRoot, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

// isAncestor reports whether a is an ancestor of b. merge-base exits 1 for "no", which
// is not an error condition, so it is run directly rather than through git().
func isAncestor(repoRoot, a, b string) bool {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", a, b)
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}
