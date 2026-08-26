package gitctx

import (
	"strings"
	"testing"
)

// RawDiff is the one text every changed-line range in the engine is derived from, so
// what it must prove is not "git ran" but "the header a hunk parser keys on is the
// header git actually emitted, for a real working tree".
func TestRawDiff(t *testing.T) {
	root := newRepo(t)
	write(t, root, "f.txt", "l1\nl2\nl3\nl4\nl5\n")
	commit(t, root, "init")
	write(t, root, "f.txt", "l1\nl2\nCHANGED\nl4\nl5\n")

	got, err := RawDiff(root, "HEAD")
	if err != nil {
		t.Fatalf("RawDiff() error = %v", err)
	}
	if !strings.Contains(got, "@@ -3 +3 @@") {
		t.Fatalf("RawDiff() missing single-line hunk header, got:\n%s", got)
	}
	if !strings.Contains(got, "+++ b/f.txt") {
		t.Fatalf("RawDiff() missing new-side path, got:\n%s", got)
	}
}

// The working tree, not the index, is the subject: a TDD cycle's newest edit is
// unstaged, and a diff that only reported staged content would miss it entirely.
func TestRawDiffCoversUnstagedWorkingTreeEdits(t *testing.T) {
	root := newRepo(t)
	write(t, root, "f.txt", "l1\nl2\n")
	commit(t, root, "init")
	write(t, root, "staged.txt", "s1\n")
	gitRun(t, root, "add", "staged.txt")
	write(t, root, "f.txt", "l1\nl2\nl3\n")

	got, err := RawDiff(root, "HEAD")
	if err != nil {
		t.Fatalf("RawDiff() error = %v", err)
	}
	for _, want := range []string{"+++ b/f.txt", "+++ b/staged.txt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("RawDiff() missing %q, got:\n%s", want, got)
		}
	}
}

// Renames are detected so the NEW-side path is what the hunks are keyed by. Without
// -M git reports a delete plus an add, and the new file's lines land under a path no
// Change carries.
func TestRawDiffDetectsRenamesOnTheNewPath(t *testing.T) {
	root := newRepo(t)
	write(t, root, "old.txt", "l1\nl2\nl3\nl4\nl5\nl6\n")
	commit(t, root, "init")
	gitRun(t, root, "mv", "old.txt", "new.txt")
	write(t, root, "new.txt", "l1\nl2\nCHANGED\nl4\nl5\nl6\n")

	got, err := RawDiff(root, "HEAD")
	if err != nil {
		t.Fatalf("RawDiff() error = %v", err)
	}
	if !strings.Contains(got, "rename to new.txt") {
		t.Fatalf("RawDiff() did not detect the rename, got:\n%s", got)
	}
	if !strings.Contains(got, "+++ b/new.txt") {
		t.Fatalf("RawDiff() missing new-side rename path, got:\n%s", got)
	}
}

// The `+++ b/<path>` header is the only key by which a hunk finds its file, so every
// git config that rewrites it is pinned off — quoting first among them, since a quoted
// path never matches the plain path a Change carries.
func TestRawDiffPathsAreNeverQuotedOrReprefixed(t *testing.T) {
	root := newRepo(t)
	write(t, root, "spät.txt", "l1\nl2\n")
	commit(t, root, "init")
	gitRun(t, root, "config", "core.quotePath", "true")
	gitRun(t, root, "config", "diff.noprefix", "true")
	gitRun(t, root, "config", "diff.mnemonicPrefix", "true")
	write(t, root, "spät.txt", "l1\nCHANGED\n")

	got, err := RawDiff(root, "HEAD")
	if err != nil {
		t.Fatalf("RawDiff() error = %v", err)
	}
	if !strings.Contains(got, "+++ b/spät.txt") {
		t.Fatalf("RawDiff() rewrote the new-side path, got:\n%s", got)
	}
}

// An empty base means HEAD, exactly as ChangedSet reads it: the two are called with the
// same base by the same caller, and a disagreement between them is a silently wrong
// line range, not an error anyone would see.
func TestRawDiffEmptyBaseMeansHEAD(t *testing.T) {
	root := newRepo(t)
	write(t, root, "f.txt", "l1\n")
	commit(t, root, "init")
	write(t, root, "f.txt", "CHANGED\n")

	got, err := RawDiff(root, "  ")
	if err != nil {
		t.Fatalf("RawDiff() error = %v", err)
	}
	if !strings.Contains(got, "+++ b/f.txt") {
		t.Fatalf("RawDiff() with an empty base did not diff against HEAD, got:\n%s", got)
	}
}

// A diff that cannot run is an error carrying git's own words. Swallowing it would
// report a repository with no changed lines, which reads as "nothing to test".
func TestRawDiffWrapsGitErrors(t *testing.T) {
	root := newRepo(t)
	write(t, root, "f.txt", "l1\n")
	commit(t, root, "init")

	_, err := RawDiff(root, "no-such-ref")
	if err == nil {
		t.Fatal("RawDiff() with an unknown base returned nil error")
	}
	if !strings.Contains(err.Error(), "no-such-ref") {
		t.Fatalf("RawDiff() error = %v, want it to name the bad base", err)
	}
}
