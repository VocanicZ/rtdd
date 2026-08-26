package gitctx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoRoot(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/auth.py", "x = 1\n")
	commit(t, dir, "init")

	tests := []struct {
		name  string
		start string
	}{
		{"from the root", dir},
		{"from a subdirectory", filepath.Join(dir, "src")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RepoRoot(tc.start)
			if err != nil {
				t.Fatalf("RepoRoot(%q): %v", tc.start, err)
			}
			wantBase := filepath.Base(dir)
			if filepath.Base(got) != wantBase {
				t.Errorf("RepoRoot = %q, want a path ending in %q", got, wantBase)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("RepoRoot = %q, want an absolute path", got)
			}
			if got != filepath.Clean(got) {
				t.Errorf("RepoRoot = %q, want a cleaned path", got)
			}
		})
	}
}

func TestRepoRootOutsideARepoIsAnError(t *testing.T) {
	if _, err := RepoRoot(t.TempDir()); err == nil {
		t.Fatal("RepoRoot outside a work tree returned nil error")
	}
}

func TestHeadSHA(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/auth.py", "x = 1\n")
	want := commit(t, dir, "init")

	got, err := HeadSHA(dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if got != want {
		t.Errorf("HeadSHA = %q, want %q", got, want)
	}
}

// An unborn HEAD is a repo that has been init'd but never committed to. HeadSHA must
// report that as an error rather than returning an empty string a caller would record
// into the map as a commit.
func TestHeadSHAOnUnbornHeadIsAnError(t *testing.T) {
	dir := newRepo(t)

	got, err := HeadSHA(dir)
	if err == nil {
		t.Fatalf("HeadSHA on an unborn HEAD returned nil error, got %q", got)
	}
	if got != "" {
		t.Errorf("HeadSHA on an unborn HEAD = %q, want an empty string", got)
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		s    Status
		want string
	}{
		{Added, "added"},
		{Modified, "modified"},
		{Deleted, "deleted"},
		{Renamed, "renamed"},
		{Untracked, "untracked"},
	}
	for _, tc := range tests {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Status(%d).String() = %q, want %q", int(tc.s), got, tc.want)
		}
	}
}

// The engine prints only from cmd/. A stray print inside gitctx would corrupt the
// --json surface every agent front-end depends on.
func TestPackagePrintsNothing(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"fmt.Print", "fmt.Fprint", "println(", "print(", "os.Stdout", "os.Stderr", "log."}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, bad := range banned {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q: only cmd/ may write to stdout/stderr", name, bad)
			}
		}
	}
}
