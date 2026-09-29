package covfmt

import (
	"reflect"
	"testing"
)

func TestResolve(t *testing.T) {
	files := []string{
		"calc/calc.go", "util.go",
		"src/main/java/com/foo/Bar.java",
		"a/com/dup/X.java", "b/com/dup/X.java",
		"src/a.py",
	}
	raw := Lines{
		"/r/src/a.py":                {1, 3},
		"./util.go":                  {2},
		"example.com/m/calc/calc.go": {3, 4},
		"com/foo/Bar.java":           {5},
		"com/dup/X.java":             {6},    // ambiguous: dropped
		"/elsewhere/lib.py":          {1},    // outside the root: dropped
		"src/a.py":                   {3, 9}, // merges with the absolute spelling
	}
	got := Resolve("/r", raw, files)
	want := map[string][]int{
		"src/a.py":                       {1, 3, 9},
		"util.go":                        {2},
		"calc/calc.go":                   {3, 4},
		"src/main/java/com/foo/Bar.java": {5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v\nwant %v", got, want)
	}
}

func TestResolveEscapingPath(t *testing.T) {
	// Relative paths with .. escape the repo: dropped.
	files := []string{"a.py", "lib/x.py"}
	raw := Lines{
		"../lib/x.py": {1},
		"./a.py":      {2},
	}
	got := Resolve("/r", raw, files)
	want := map[string][]int{
		"a.py": {2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v, want %v", got, want)
	}
}

func TestResolveAmbiguousSuffix(t *testing.T) {
	// com/foo/Bar.java with root-level Bar.java and no src match → dropped.
	files := []string{"Bar.java"} // no src/main/java/com/foo/Bar.java
	raw := Lines{
		"com/foo/Bar.java": {5},
	}
	got := Resolve("/r", raw, files)
	want := map[string][]int{
		"Bar.java": {5}, // consensus prefix com/foo/ strips to Bar.java
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v, want %v", got, want)
	}
}

func TestResolveAmbiguousSuffixDropped(t *testing.T) {
	// Suffix match is ambiguous when multiple files match.
	files := []string{"Bar.java", "a/Bar.java", "b/Bar.java"}
	raw := Lines{
		"Bar.java": {1},
	}
	got := Resolve("/r", raw, files)
	want := map[string][]int{
		"Bar.java": {1}, // exact match takes precedence
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v, want %v", got, want)
	}
}

func TestResolveConsensusPrefix(t *testing.T) {
	// Consensus prefix: example.com/m/calc/calc.go and example.com/m/util.go
	// consensus prefix is "example.com/m/", but x/util.go doesn't exist so it's dropped.
	files := []string{
		"calc/calc.go", "util.go", // not x/util.go
	}
	raw := Lines{
		"example.com/m/calc/calc.go": {1, 2},
		"example.com/m/util.go":      {3, 4},
		"example.com/m/x/util.go":    {5}, // no match in repo, dropped
	}
	got := Resolve("/r", raw, files)
	want := map[string][]int{
		"calc/calc.go": {1, 2},
		"util.go":      {3, 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v, want %v", got, want)
	}
}

func TestResolveAbsoluteWithAmbiguousSuffix(t *testing.T) {
	// Absolute /r/vendor/a.py with root a.py present but vendor/a.py absent → dropped
	// Absolute paths only match exactly in the set, no suffix fallback.
	files := []string{"a.py"} // vendor/a.py not in repo
	raw := Lines{
		"/r/vendor/a.py": {1},
	}
	got := Resolve("/r", raw, files)
	want := map[string][]int{} // dropped: vendor/a.py not in set (only a.py is)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v, want %v (dropped: vendor/a.py not in repo)", got, want)
	}
}

func TestResolveJacocoDefaultPackage(t *testing.T) {
	// JaCoCo with empty package name produces "Foo.java", not "/Foo.java".
	files := []string{"src/main/java/Foo.java"}
	raw := Lines{
		"Foo.java": {1}, // from path.Join("", "Foo.java")
	}
	got := Resolve("/r", raw, files)
	want := map[string][]int{
		"src/main/java/Foo.java": {1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v, want %v", got, want)
	}
}

func TestResolveCRLFInput(t *testing.T) {
	// Backslashes in paths are normalized to forward slashes (Windows paths).
	files := []string{"calc.go"}
	raw := Lines{
		"example\\com\\calc.go": {1}, // Windows-style path
	}
	got := Resolve("/r", raw, files)
	want := map[string][]int{
		"calc.go": {1}, // "example\com\calc.go" → "example/com/calc.go" → drop "example/com/" → "calc.go"
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v, want %v", got, want)
	}
}

func TestResolveAbsoluteRootNormalization(t *testing.T) {
	// Root is normalized to absolute path. Absolute paths in raw are made relative to root.
	files := []string{"a.py"}
	raw := Lines{
		"/home/user/project/a.py": {1},
	}
	// Use /home/user/project as root (will be normalized to absolute)
	got := Resolve("/home/user/project", raw, files)
	want := map[string][]int{
		"a.py": {1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v, want %v", got, want)
	}
}
