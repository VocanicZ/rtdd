package adapter

import (
	"os"
	"strings"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"**/test_*.py", "tests/test_a.py", true},
		{"**/test_*.py", "test_a.py", true},
		{"**/test_*.py", "a/b/c/test_a.py", true},
		{"**/test_*.py", "tests/helpers.py", false},
		{"tests/**/*.py", "tests/test_a.py", true},
		{"tests/**/*.py", "tests/unit/deep/test_a.py", true},
		{"tests/**/*.py", "src/test_a.py", false},
		{"**/fixtures/**", "tests/fixtures/data.json", true},
		{"**/fixtures/**", "fixtures/a/b/c.sql", true},
		{"**/fixtures/**", "tests/fixture/data.json", false},
		{"**/conftest.py", "conftest.py", true},
		{"**/conftest.py", "tests/unit/conftest.py", true},
		{"requirements.txt", "requirements.txt", true},
		{"requirements.txt", "sub/requirements.txt", false},
		{"**/*.py", "src/logic.py", true},
		{"**/*.py", "src/data.yaml", false},
		{"**/*.yaml", "config/app.yaml", true},
		{"**", "anything/at/all.txt", true},
	}
	for _, tc := range cases {
		if got := matchGlob(tc.pattern, tc.name); got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

// Everything inside the engine is repo-relative, slash-separated and cleaned, but a
// caller that has not been through internal/paths yet must not silently classify wrong.
func TestMatchGlobCleansTheName(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"tests/**/*.py", "./tests/test_a.py", true},
		{"**/*.py", "src//logic.py", true},
		{"**/*.py", "src/./logic.py", true},
		{"**/*.py", "src/pkg/../logic.py", true},
	}
	for _, tc := range cases {
		if got := matchGlob(tc.pattern, tc.name); got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

// An empty pattern or an empty name declares nothing; neither may match, and neither
// may reach paths.MatchGlob, which panics on a pattern ValidateGlob rejects.
func TestMatchGlobEmptyInputsMatchNothing(t *testing.T) {
	for _, tc := range []struct{ pattern, name string }{
		{"", "src/logic.py"},
		{"**", ""},
		{"**", "."},
		{"", ""},
	} {
		if matchGlob(tc.pattern, tc.name) {
			t.Errorf("matchGlob(%q, %q) = true, want false", tc.pattern, tc.name)
		}
	}
}

// The matcher is pure string work: a glob naming a path that does not exist on disk
// still matches, and one naming a real file it should not match still does not.
func TestMatchGlobNeverTouchesTheFilesystem(t *testing.T) {
	if !matchGlob("**/*.py", "no/such/directory/ghost.py") {
		t.Error("matchGlob must match a path that does not exist on disk")
	}
	if matchGlob("**/*.py", "glob.go") {
		t.Error("matchGlob must not match a real file that the pattern does not describe")
	}
}

// AC: "the matcher never touches the filesystem". A behavioural test can only sample
// paths; this one is exhaustive over the source, so a later refactor that reaches for
// os.Stat to disambiguate a glob fails here rather than in a host repo.
func TestGlobAndClassifySourceTouchNoFilesystem(t *testing.T) {
	for _, file := range []string{"glob.go", "classify.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"os.", "os/exec", "io/fs", "filepath", "syscall"} {
			if strings.Contains(string(src), banned) {
				t.Errorf("%s references %q; classification is pure string work over globs", file, banned)
			}
		}
	}
}
