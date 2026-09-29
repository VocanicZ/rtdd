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
