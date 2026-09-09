package gitctx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindRepoRoot walks up from start and returns the first ancestor that is a git
// repository working tree.
//
// The test is for a `.git` marker that actually identifies a repository, not merely for
// an entry with that name. A stray `.git` — an empty directory left by an unrelated
// process, or a `.git` file whose `gitdir:` target is gone — is skipped and the walk
// CONTINUES to the parent, exactly as `git rev-parse --show-toplevel` behaves. Accepting
// one silently made the wrong tree "the repository root" for every command that resolves
// a root, and the user-visible symptom did not point at the cause (issue #369).
//
// Both marker shapes are accepted, because both occur:
//
//   - a `.git` DIRECTORY containing HEAD — an ordinary repository;
//   - a `.git` FILE holding `gitdir: <path>` whose target contains HEAD — a linked
//     worktree or a submodule. A relative target resolves against the directory holding
//     the `.git` file.
//
// HEAD is the discriminator. Both stray shapes observed in the wild are directories: one
// empty, one holding only `info/`. Any "is a directory and non-empty" rule is fooled by
// the second. git's own is_git_directory() is stricter still — it wants `objects/` and
// `refs/` too — but a `.git` directory carrying HEAD and nothing else is not a shape that
// occurs in practice.
func FindRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", start, err)
	}
	for {
		if IsRepoRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a git repository (searched upward from %s)", start)
		}
		dir = parent
	}
}

// IsRepoRoot reports whether dir holds a `.git` marker that identifies a repository.
func IsRepoRoot(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return hasHEAD(gitPath)
	}
	target, ok := readGitdirPointer(gitPath, dir)
	if !ok {
		return false
	}
	return hasHEAD(target)
}

// readGitdirPointer reads a `.git` FILE and returns the directory its `gitdir:` line
// names, resolved against base when the recorded path is relative.
func readGitdirPointer(gitFile, base string) (string, bool) {
	raw, err := os.ReadFile(gitFile)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), "gitdir:")
		if !found {
			continue
		}
		target := strings.TrimSpace(rest)
		if target == "" {
			return "", false
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(base, target)
		}
		return target, true
	}
	return "", false
}

// hasHEAD reports whether gitDir is a directory containing a HEAD entry.
func hasHEAD(gitDir string) bool {
	info, err := os.Stat(gitDir)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(gitDir, "HEAD"))
	return err == nil
}
