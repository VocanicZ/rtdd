// Package gitctx is the only package in the engine that shells out to git.
// Git is never mocked: every test in this package runs against a real repository.
package gitctx

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Status int

const (
	Added Status = iota
	Modified
	Deleted
	Renamed
	Untracked
)

// String returns "added" | "modified" | "deleted" | "renamed" | "untracked".
func (s Status) String() string {
	switch s {
	case Added:
		return "added"
	case Modified:
		return "modified"
	case Deleted:
		return "deleted"
	case Renamed:
		return "renamed"
	case Untracked:
		return "untracked"
	}
	return "unknown"
}

// LineRange is 1-indexed and inclusive, in the NEW file.
type LineRange struct{ Start, End int }

type Change struct {
	Path    string
	OldPath string // set only when Status == Renamed
	Status  Status
	Lines   []LineRange // empty for Deleted
}

// git runs a git subcommand in repoRoot and returns its stdout.
func git(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return string(out), fmt.Errorf("gitctx: git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// RepoRoot returns the absolute, cleaned top level of the git work tree containing start.
func RepoRoot(start string) (string, error) {
	out, err := git(start, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return "", fmt.Errorf("gitctx: git rev-parse --show-toplevel returned nothing for %q", start)
	}
	return filepath.Clean(root), nil
}

// HeadSHA returns the short SHA of HEAD. An unborn HEAD — a repository with no commits
// yet — is an error, never an empty string: callers record this value as a row's commit.
func HeadSHA(repoRoot string) (string, error) {
	out, err := git(repoRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(out)
	if sha == "" {
		return "", fmt.Errorf("gitctx: git rev-parse --short HEAD returned nothing in %q", repoRoot)
	}
	return sha, nil
}
