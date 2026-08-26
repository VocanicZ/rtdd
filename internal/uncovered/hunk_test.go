package uncovered

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
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
