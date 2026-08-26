package uncovered

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestParseHunks(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want map[string][]gitctx.LineRange
	}{
		{
			name: "single line modification, both counts omitted",
			diff: "diff --git a/f.txt b/f.txt\n" +
				"index 8afd661..858e521 100644\n" +
				"--- a/f.txt\n" +
				"+++ b/f.txt\n" +
				"@@ -3 +3 @@ l2\n" +
				"-l3\n" +
				"+L3CHANGED\n",
			want: map[string][]gitctx.LineRange{"f.txt": {{Start: 3, End: 3}}},
		},
		{
			name: "multi line insertion",
			diff: "--- a/f.txt\n+++ b/f.txt\n@@ -8,0 +9,2 @@ l8\n+NEW8a\n+NEW8b\n",
			want: map[string][]gitctx.LineRange{"f.txt": {{Start: 9, End: 10}}},
		},
		{
			name: "single line insertion, new count omitted",
			diff: "--- a/f.py\n+++ b/f.py\n@@ -3,0 +4 @@ l3\n+INSERTED\n",
			want: map[string][]gitctx.LineRange{"f.py": {{Start: 4, End: 4}}},
		},
		{
			name: "pure deletion contributes no new lines",
			diff: "--- a/f.txt\n+++ b/f.txt\n@@ -11,2 +12,0 @@ l10\n-l11\n-l12\n",
			want: map[string][]gitctx.LineRange{},
		},
		{
			name: "whole new file",
			diff: "diff --git a/added.txt b/added.txt\n" +
				"new file mode 100644\n" +
				"index 0000000..de98044\n" +
				"--- /dev/null\n" +
				"+++ b/added.txt\n" +
				"@@ -0,0 +1,3 @@\n+a\n+b\n+c\n",
			want: map[string][]gitctx.LineRange{"added.txt": {{Start: 1, End: 3}}},
		},
		{
			name: "empty new file has no hunk header",
			diff: "diff --git a/empty.txt b/empty.txt\n" +
				"new file mode 100644\n" +
				"index 0000000..e69de29\n",
			want: map[string][]gitctx.LineRange{},
		},
		{
			name: "context suffix may itself contain @@",
			diff: "--- a/h.py\n+++ b/h.py\n" +
				"@@ -5 +5 @@ def foo():  # @@ marker @@\n" +
				"-    z = 3\n+    z = 99\n",
			want: map[string][]gitctx.LineRange{"h.py": {{Start: 5, End: 5}}},
		},
		{
			name: "rename uses the new path",
			diff: "diff --git a/f.txt b/g.txt\n" +
				"similarity index 94%\nrename from f.txt\nrename to g.txt\n" +
				"index 8afd661..cfeb442 100644\n" +
				"--- a/f.txt\n+++ b/g.txt\n@@ -2 +2 @@ l1\n-l2\n+B\n",
			want: map[string][]gitctx.LineRange{"g.txt": {{Start: 2, End: 2}}},
		},
		{
			name: "deleted file has /dev/null on the new side",
			diff: "--- a/gone.txt\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n",
			want: map[string][]gitctx.LineRange{},
		},
		{
			name: "three files in one diff",
			diff: "--- a/f.txt\n+++ b/f.txt\n@@ -3 +3 @@ l2\n-l3\n+L3CHANGED\n" +
				"@@ -8,0 +9,2 @@ l8\n+NEW8a\n+NEW8b\n" +
				"--- a/g.py\n+++ b/g.py\n@@ -1,0 +2 @@ x\n+y\n" +
				"--- a/h.py\n+++ /dev/null\n@@ -1 +0,0 @@\n-z\n",
			want: map[string][]gitctx.LineRange{
				"f.txt": {{Start: 3, End: 3}, {Start: 9, End: 10}},
				"g.py":  {{Start: 2, End: 2}},
			},
		},
		{
			name: "empty diff",
			diff: "",
			want: map[string][]gitctx.LineRange{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseHunks(tc.diff)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseHunks()\n got: %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}

// TestParseHunksQuotedPaths covers the paths git writes when core.quotePath is on:
// the whole operand is wrapped in double quotes and non-ASCII bytes are octal-escaped.
// The quoting must be decoded BEFORE the "b/" prefix is stripped.
func TestParseHunksQuotedPaths(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want map[string][]gitctx.LineRange
	}{
		{
			name: "octal-escaped non-ascii path is decoded",
			diff: "--- \"a/src/caf\\303\\251.py\"\n+++ \"b/src/caf\\303\\251.py\"\n" +
				"@@ -2 +2 @@ ctx\n-old\n+new\n",
			want: map[string][]gitctx.LineRange{"src/café.py": {{Start: 2, End: 2}}},
		},
		{
			name: "escaped space and quote in path, trailing tab as git emits it",
			diff: "--- \"a/src/we\\\"ird name.py\"\t\n+++ \"b/src/we\\\"ird name.py\"\t\n" +
				"@@ -1,0 +2,2 @@ ctx\n+x\n+y\n",
			want: map[string][]gitctx.LineRange{"src/we\"ird name.py": {{Start: 2, End: 3}}},
		},
		{
			name: "quoted /dev/null new side is still omitted",
			diff: "--- \"a/src/caf\\303\\251.py\"\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n",
			want: map[string][]gitctx.LineRange{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseHunks(tc.diff)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseHunks()\n got: %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}

// WithLines is the one place a Change's Lines are decided, and every case below is a
// path by which a second source of truth used to creep in: ranges already on the
// Change, a file git diff never lists, a file that is gone by the time we look.
func TestWithLines(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "untracked.py", "a = 1\nb = 2\nc = 3\n")
	writeFile(t, root, "noeol.py", "x = 1\ny = 2")
	writeFile(t, root, "empty.py", "")
	writeFile(t, root, "mod.py", "l1\nl2\nl3\nl4\n")

	diff := "--- a/mod.py\n+++ b/mod.py\n@@ -3 +3 @@ l2\n-old\n+new\n" +
		"--- a/stale.py\n+++ b/stale.py\n@@ -0,0 +1,2 @@\n+p\n+q\n"

	in := []gitctx.Change{
		{Path: "mod.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 99, End: 99}}},
		{Path: "stale.py", Status: gitctx.Added},
		{Path: "untracked.py", Status: gitctx.Untracked},
		{Path: "noeol.py", Status: gitctx.Untracked},
		{Path: "empty.py", Status: gitctx.Untracked},
		{Path: "gone.py", Status: gitctx.Deleted, Lines: []gitctx.LineRange{{Start: 1, End: 5}}},
		{Path: "vanished.py", Status: gitctx.Untracked},
	}

	got, err := WithLines(root, in, diff)
	if err != nil {
		t.Fatalf("WithLines() error = %v", err)
	}

	want := []gitctx.Change{
		{Path: "mod.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 3, End: 3}}},
		{Path: "stale.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 2}}},
		{Path: "untracked.py", Status: gitctx.Untracked, Lines: []gitctx.LineRange{{Start: 1, End: 3}}},
		{Path: "noeol.py", Status: gitctx.Untracked, Lines: []gitctx.LineRange{{Start: 1, End: 2}}},
		{Path: "empty.py", Status: gitctx.Untracked, Lines: nil},
		{Path: "gone.py", Status: gitctx.Deleted, Lines: nil},
		{Path: "vanished.py", Status: gitctx.Untracked, Lines: nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithLines()\n got: %#v\nwant: %#v", got, want)
	}
}

// The input slice is the caller's; WithLines returns a new one and must not reach back
// into it. A caller that still holds the pre-call Change would otherwise see it mutate.
func TestWithLinesDoesNotMutateItsInput(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "mod.py", "l1\nl2\nl3\n")
	in := []gitctx.Change{
		{Path: "mod.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 99, End: 99}}},
	}

	if _, err := WithLines(root, in, "--- a/mod.py\n+++ b/mod.py\n@@ -1 +1 @@\n-l1\n+L1\n"); err != nil {
		t.Fatalf("WithLines() error = %v", err)
	}
	want := []gitctx.LineRange{{Start: 99, End: 99}}
	if !reflect.DeepEqual(in[0].Lines, want) {
		t.Fatalf("WithLines() mutated its input: in[0].Lines = %#v, want %#v", in[0].Lines, want)
	}
}

// A renamed Change carries both paths; the hunks are keyed by the new one. Matching on
// OldPath finds nothing and silently degrades the file to a whole-file range.
func TestWithLinesMatchesRenamesOnTheNewPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "new.py", "l1\nl2\nl3\nl4\nl5\n")
	in := []gitctx.Change{
		{Path: "new.py", OldPath: "old.py", Status: gitctx.Renamed},
	}

	got, err := WithLines(root, in, "--- a/old.py\n+++ b/new.py\n@@ -2 +2 @@\n-l2\n+L2\n")
	if err != nil {
		t.Fatalf("WithLines() error = %v", err)
	}
	want := []gitctx.Change{
		{Path: "new.py", OldPath: "old.py", Status: gitctx.Renamed, Lines: []gitctx.LineRange{{Start: 2, End: 2}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithLines()\n got: %#v\nwant: %#v", got, want)
	}
}

// Nil in, nil out — a caller with nothing changed gets an empty result, not a panic.
func TestWithLinesEmptyInput(t *testing.T) {
	got, err := WithLines(t.TempDir(), nil, "")
	if err != nil {
		t.Fatalf("WithLines() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("WithLines(nil) = %#v, want empty", got)
	}
}

// The end-to-end statement, against a real repository and real git output: whatever
// ChangedSet reports and whatever RawDiff prints, WithLines reconciles them into one
// set of ranges. Hand-written diff strings can only prove the parser; this proves the
// contract with git.
func TestWithLinesOverRealGitDiff(t *testing.T) {
	root := gittest.Init(t)
	gittest.Write(t, root, "mod.py", "l1\nl2\nl3\nl4\n")
	gittest.Write(t, root, "old.py", "r1\nr2\nr3\nr4\nr5\nr6\n")
	gittest.Write(t, root, "gone.py", "d1\nd2\n")
	gittest.Commit(t, root, "init")

	gittest.Write(t, root, "mod.py", "l1\nl2\nCHANGED\nl4\n")
	gittest.Run(t, root, "mv", "old.py", "new.py")
	gittest.Write(t, root, "new.py", "r1\nR2\nr3\nr4\nr5\nr6\n")
	gittest.Run(t, root, "rm", "-q", "gone.py")
	gittest.Write(t, root, "untracked.py", "u1\nu2\nu3\n")

	changes, err := gitctx.ChangedSet(root, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet() error = %v", err)
	}
	raw, err := gitctx.RawDiff(root, "HEAD")
	if err != nil {
		t.Fatalf("RawDiff() error = %v", err)
	}

	got, err := WithLines(root, changes, raw)
	if err != nil {
		t.Fatalf("WithLines() error = %v", err)
	}

	want := map[string][]gitctx.LineRange{
		"mod.py":       {{Start: 3, End: 3}},
		"new.py":       {{Start: 2, End: 2}},
		"gone.py":      nil,
		"untracked.py": {{Start: 1, End: 3}},
	}
	if len(got) != len(want) {
		t.Fatalf("WithLines() returned %d changes, want %d: %#v", len(got), len(want), got)
	}
	for _, c := range got {
		w, ok := want[c.Path]
		if !ok {
			t.Fatalf("WithLines() returned an unexpected path %q", c.Path)
		}
		if !reflect.DeepEqual(c.Lines, w) {
			t.Errorf("WithLines()[%q].Lines = %#v, want %#v", c.Path, c.Lines, w)
		}
	}
}

// WithLines is authoritative over ChangedSet too: ChangedSet already fills Lines, and
// running both must leave exactly one answer, not an accumulation of two.
func TestWithLinesIsIdempotentOverChangedSet(t *testing.T) {
	root := gittest.Init(t)
	gittest.Write(t, root, "mod.py", "l1\nl2\nl3\nl4\n")
	gittest.Commit(t, root, "init")
	gittest.Write(t, root, "mod.py", "l1\nl2\nCHANGED\nl4\n")

	changes, err := gitctx.ChangedSet(root, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet() error = %v", err)
	}
	raw, err := gitctx.RawDiff(root, "HEAD")
	if err != nil {
		t.Fatalf("RawDiff() error = %v", err)
	}

	once, err := WithLines(root, changes, raw)
	if err != nil {
		t.Fatalf("WithLines() error = %v", err)
	}
	twice, err := WithLines(root, once, raw)
	if err != nil {
		t.Fatalf("WithLines() error = %v", err)
	}
	if !reflect.DeepEqual(once, twice) {
		t.Fatalf("WithLines() is not idempotent\n once: %#v\ntwice: %#v", once, twice)
	}
	if len(once) != 1 || !reflect.DeepEqual(once[0].Lines, []gitctx.LineRange{{Start: 3, End: 3}}) {
		t.Fatalf("WithLines() = %#v, want mod.py lines 3-3", once)
	}
}

// writeFile creates root/rel, and any missing parent directories, with the given content.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
