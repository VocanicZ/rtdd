package paths

import (
	"fmt"
	"strings"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		rel     string
		want    bool
	}{
		{"exact literal", "pyproject.toml", "pyproject.toml", true},
		{"literal does not match nested", "pyproject.toml", "sub/pyproject.toml", false},
		{"star within a segment", "src/*.py", "src/auth.py", true},
		{"star does not cross a separator", "src/*.py", "src/pkg/auth.py", false},
		{"question mark within a segment", "src/a?.py", "src/ab.py", true},
		{"doublestar spans many segments", "tests/**/*.py", "tests/unit/api/test_a.py", true},
		{"doublestar spans zero segments", "tests/**/*.py", "tests/test_a.py", true},
		{"leading doublestar", "**/test_*.py", "a/b/test_x.py", true},
		{"leading doublestar at root", "**/test_*.py", "test_x.py", true},
		{"leading doublestar wrong basename", "**/test_*.py", "a/b/helper.py", false},
		{"doublestar suffix matches subtree", "**/fixtures/**", "tests/fixtures/data/a.json", true},
		{"doublestar suffix wrong directory", "**/fixtures/**", "tests/unit/a.json", false},
		{"extension glob at root", "**/*.yaml", "config.yaml", true},
		{"extension glob nested", "**/*.yaml", "deploy/k8s/config.yaml", true},
		{"extension glob wrong extension", "**/*.yaml", "deploy/k8s/config.json", false},
		{"conftest anywhere", "**/conftest.py", "tests/unit/conftest.py", true},
		{"pattern longer than path", "a/b/c", "a/b", false},
		{"path longer than pattern", "a/b", "a/b/c", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchGlob(tc.pattern, tc.rel); got != tc.want {
				t.Errorf("MatchGlob(%q, %q) = %v, want %v", tc.pattern, tc.rel, got, tc.want)
			}
		})
	}
}

// A malformed glob must be a loud configuration error, never a silent non-match.
// Swallowing path.ErrBadPattern is how a typo'd test_globs classifies nothing at all
// and the direct tier — the one tier that must never depend on the map — goes empty.
func TestValidateGlob(t *testing.T) {
	valid := []string{
		"pyproject.toml",
		"tests/**/*.py",
		"**/test_*.py",
		"src/a?.py",
		"**",
		"**/fixtures/**",
		"tests/[a-z]*.py",
	}
	for _, p := range valid {
		if err := ValidateGlob(p); err != nil {
			t.Errorf("ValidateGlob(%q) = %v, want nil", p, err)
		}
	}

	bad := []string{
		"",              // an empty glob declares nothing
		"tests/[a-*.py", // unterminated character class
		"[a-",           // unterminated, whole pattern
		"src/**/[!.py",  // unterminated negated class
		"a[",            // trailing open bracket
	}
	for _, p := range bad {
		err := ValidateGlob(p)
		if err == nil {
			t.Errorf("ValidateGlob(%q) = nil, want an error", p)
			continue
		}
		if p != "" && !strings.Contains(err.Error(), p) {
			t.Errorf("ValidateGlob(%q) error = %q, want it to name the offending pattern", p, err)
		}
	}
}

// MatchGlob must not be able to hide a bad pattern behind a false. Every pattern that
// reaches it has already passed ValidateGlob at adapter.Load time, so a malformed one
// is a programming error and is reported as one.
func TestMatchGlobDoesNotSwallowABadPattern(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("MatchGlob returned for a malformed pattern; a bad glob must never look like a non-match")
		}
		if !strings.Contains(fmt.Sprint(r), "tests/[a-*.py") {
			t.Errorf("panic = %v, want it to name the offending pattern", r)
		}
	}()
	MatchGlob("tests/[a-*.py", "tests/test_new.py")
}
