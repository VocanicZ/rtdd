package gitctx

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

// tenLines returns a ten-line file whose line n reads "line n".
func tenLines() string {
	var b strings.Builder
	for i := 1; i <= 10; i++ {
		b.WriteString("line ")
		b.WriteString(string(rune('0' + i%10)))
		b.WriteString("\n")
	}
	return b.String()
}

// A source line whose own text begins with "++ " renders in a unified diff body as
// "+++ ...", which is indistinguishable from a file header by prefix alone. Mistaking
// it for a header repoints the parser at a bogus path and every later hunk in the same
// file is lost — reported as unchanged, which downstream reads as clean.
func TestChangedSetContentLineLookingLikeAFileHeader(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{"plus plus space", "++ diff marker line"},
		{"triple plus", "+++ b/nowhere.py"},
		{"minus minus space", "-- diff marker line"},
		{"triple minus", "--- a/nowhere.py"},
		{"at at", "@@ -1 +999 @@ bogus"},
		{"diff git", "diff --git a/nowhere.py b/nowhere.py"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := newRepo(t)
			write(t, dir, "src/a.py", tenLines())
			commit(t, dir, "init")

			lines := strings.Split(tenLines(), "\n")
			lines[1] = tc.content // line 2
			lines[7] = "line eight changed"
			write(t, dir, "src/a.py", strings.Join(lines, "\n"))

			cs, err := ChangedSet(dir, "HEAD")
			if err != nil {
				t.Fatalf("ChangedSet: %v", err)
			}
			c, ok := byPath(cs)["src/a.py"]
			if !ok {
				t.Fatalf("src/a.py missing; got %v", paths(cs))
			}
			want := []LineRange{{Start: 2, End: 2}, {Start: 8, End: 8}}
			if !reflect.DeepEqual(c.Lines, want) {
				t.Errorf("Lines = %#v, want %#v (a later hunk was dropped)", c.Lines, want)
			}
		})
	}
}

// git C-quotes paths in a diff header when they are non-ASCII (core.quotePath, on by
// default) or contain a quote or backslash. `--name-status -z` is never quoted, so a
// quoted header path matches no known file and the ranges vanish while the file itself
// is still reported — a change with no changed lines, the dangerous half.
func TestChangedSetQuotedPathsKeepTheirRanges(t *testing.T) {
	for _, rel := range []string{
		"src/café.py",
		"src/with space.py",
		`src/with"quote.py`,
		`src/with\backslash.py`,
	} {
		t.Run(rel, func(t *testing.T) {
			dir := newRepo(t)
			gitRun(t, dir, "config", "core.quotePath", "true")
			write(t, dir, rel, tenLines())
			write(t, dir, "src/plain.py", tenLines())
			commit(t, dir, "init")

			lines := strings.Split(tenLines(), "\n")
			lines[1] = "line two changed"
			write(t, dir, rel, strings.Join(lines, "\n"))
			write(t, dir, "src/plain.py", strings.Join(lines, "\n"))

			cs, err := ChangedSet(dir, "HEAD")
			if err != nil {
				t.Fatalf("ChangedSet: %v", err)
			}
			got := byPath(cs)
			want := []LineRange{{Start: 2, End: 2}}
			c, ok := got[rel]
			if !ok {
				t.Fatalf("%q missing; got %v", rel, paths(cs))
			}
			if !reflect.DeepEqual(c.Lines, want) {
				t.Errorf("Lines for %q = %#v, want %#v (same as the ASCII-named file)", rel, c.Lines, want)
			}
			if p := got["src/plain.py"]; !reflect.DeepEqual(p.Lines, want) {
				t.Errorf("Lines for src/plain.py = %#v, want %#v", p.Lines, want)
			}
		})
	}
}

// A user's git config must not be able to narrow the result: diff.noprefix and
// diff.mnemonicPrefix both rewrite the "b/" in a diff header.
func TestChangedSetIgnoresDiffPrefixConfig(t *testing.T) {
	for _, cfg := range [][2]string{
		{"diff.noprefix", "true"},
		{"diff.mnemonicPrefix", "true"},
	} {
		t.Run(cfg[0], func(t *testing.T) {
			dir := newRepo(t)
			gitRun(t, dir, "config", cfg[0], cfg[1])
			write(t, dir, "src/a.py", tenLines())
			commit(t, dir, "init")

			lines := strings.Split(tenLines(), "\n")
			lines[1] = "line two changed"
			write(t, dir, "src/a.py", strings.Join(lines, "\n"))

			cs, err := ChangedSet(dir, "HEAD")
			if err != nil {
				t.Fatalf("ChangedSet: %v", err)
			}
			c := byPath(cs)["src/a.py"]
			want := []LineRange{{Start: 2, End: 2}}
			if !reflect.DeepEqual(c.Lines, want) {
				t.Errorf("Lines = %#v, want %#v", c.Lines, want)
			}
		})
	}
}

// errReader fails partway through, the way a read from a truncated pipe does.
type errReader struct {
	head string
	err  error
	done bool
}

func (r *errReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		n := copy(p, r.head)
		return n, nil
	}
	return 0, r.err
}

// A scanner error means the remainder of the diff was never seen. Returning the hunks
// parsed so far reports the unseen files as unchanged; the error must reach the caller.
func TestParseHunksPropagatesScannerError(t *testing.T) {
	boom := errors.New("boom")
	r := &errReader{head: "+++ b/src/a.py\n@@ -1 +1 @@\n", err: boom}

	_, err := parseHunks(r)
	if !errors.Is(err, boom) {
		t.Fatalf("parseHunks error = %v, want it to wrap %v", err, boom)
	}
}

func TestParseHunksOnAWellFormedDiff(t *testing.T) {
	diff := "diff --git a/src/a.py b/src/a.py\n" +
		"--- a/src/a.py\n+++ b/src/a.py\n" +
		"@@ -2 +2 @@\n-old\n+new\n@@ -8 +8,2 @@\n-old8\n+new8\n+new9\n"
	got, err := parseHunks(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("parseHunks: %v", err)
	}
	want := map[string][]LineRange{"src/a.py": {{Start: 2, End: 2}, {Start: 8, End: 9}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseHunks = %#v, want %#v", got, want)
	}
}

var _ io.Reader = (*errReader)(nil)
