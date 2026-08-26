package paths

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		root    string
		in      string
		wantRel string
		wantOK  bool
	}{
		{"absolute inside repo", "/repo", "/repo/src/auth.py", "src/auth.py", true},
		{"already relative", "/repo", "src/auth.py", "src/auth.py", true},
		{"needs cleaning", "/repo", "./src/../src/auth.py", "src/auth.py", true},
		{"nested", "/repo", "/repo/tests/unit/test_a.py", "tests/unit/test_a.py", true},
		{"trailing slash on root", "/repo/", "/repo/src/auth.py", "src/auth.py", true},
		{"repo root itself is not a file", "/repo", "/repo", "", false},
		{"escapes via absolute path", "/repo", "/etc/passwd", "", false},
		{"escapes via dotdot", "/repo", "../outside.py", "", false},
		{"sibling directory prefix is not inside", "/repo", "/repo-other/a.py", "", false},
		{"empty path", "/repo", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotRel, gotOK := Normalize(tc.root, tc.in)
			if gotRel != tc.wantRel || gotOK != tc.wantOK {
				t.Errorf("Normalize(%q, %q) = (%q, %v), want (%q, %v)",
					tc.root, tc.in, gotRel, gotOK, tc.wantRel, tc.wantOK)
			}
		})
	}
}

func TestStripModulePrefix(t *testing.T) {
	tests := []struct {
		name string
		mod  string
		in   string
		want string
	}{
		{"strips module prefix", "github.com/VocanicZ/rtdd", "github.com/VocanicZ/rtdd/internal/paths/paths.go", "internal/paths/paths.go"},
		{"module with trailing slash", "github.com/VocanicZ/rtdd/", "github.com/VocanicZ/rtdd/main.go", "main.go"},
		{"leaves unrelated path alone", "github.com/VocanicZ/rtdd", "vendor/x/y.go", "vendor/x/y.go"},
		{"empty module is identity", "", "internal/paths/paths.go", "internal/paths/paths.go"},
		{"prefix match must be on a segment boundary", "github.com/VocanicZ/rtdd", "github.com/VocanicZ/rtdd-other/a.go", "github.com/VocanicZ/rtdd-other/a.go"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripModulePrefix(tc.mod, tc.in); got != tc.want {
				t.Errorf("StripModulePrefix(%q, %q) = %q, want %q", tc.mod, tc.in, got, tc.want)
			}
		})
	}
}
