package paths

import "testing"

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
