package mapstore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestMapAccessors(t *testing.T) {
	m := New()
	if m.Len() != 0 {
		t.Fatalf("New().Len() = %d, want 0", m.Len())
	}
	if _, ok := m.Get("nope"); ok {
		t.Fatalf("Get on an empty map returned ok=true")
	}

	m.Replace(Row{T: "tests/test_b.py::test_x", F: []string{"src/b.py"}, C: "bbb1111", D: 5, S: "pass"})
	m.Replace(Row{T: "tests/test_a.py::test_y", F: []string{"src/a.py"}, C: "aaa2222", D: 9, S: "fail"})

	if m.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", m.Len())
	}
	got := m.Rows()
	wantOrder := []string{"tests/test_a.py::test_y", "tests/test_b.py::test_x"}
	for i, w := range wantOrder {
		if got[i].T != w {
			t.Errorf("Rows()[%d].T = %q, want %q (Rows must be sorted by T ascending)", i, got[i].T, w)
		}
	}

	r, ok := m.Get("tests/test_a.py::test_y")
	if !ok {
		t.Fatalf("Get missed a row that Replace inserted")
	}
	if r.S != "fail" || r.D != 9 || r.C != "aaa2222" {
		t.Errorf("Get returned %+v, want S=fail D=9 C=aaa2222", r)
	}

	m.Delete("tests/test_a.py::test_y")
	if m.Len() != 1 {
		t.Errorf("after Delete, Len() = %d, want 1", m.Len())
	}
}

func TestReplaceNormalizesF(t *testing.T) {
	tests := []struct {
		name  string
		in    []string
		wantF []string
	}{
		{"sorts", []string{"src/z.py", "src/a.py"}, []string{"src/a.py", "src/z.py"}},
		{"dedupes", []string{"src/a.py", "src/a.py", "src/b.py"}, []string{"src/a.py", "src/b.py"}},
		{"nil becomes empty, never null", nil, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			m.Replace(Row{T: "t", F: tc.in, C: "c", D: 1, S: "pass"})
			got, _ := m.Get("t")
			if !reflect.DeepEqual(got.F, tc.wantF) {
				t.Errorf("F = %#v, want %#v", got.F, tc.wantF)
			}
		})
	}
}

func TestReplaceDoesNotAliasCallerSlice(t *testing.T) {
	f := []string{"src/b.py", "src/a.py"}
	m := New()
	m.Replace(Row{T: "t", F: f, C: "c", D: 1, S: "pass"})
	f[0] = "MUTATED"
	got, _ := m.Get("t")
	for _, s := range got.F {
		if s == "MUTATED" {
			t.Fatalf("Replace stored the caller's slice by reference: %#v", got.F)
		}
	}
}

// olderLexical is a deterministic stand-in for gitctx.Older in unit tests:
// the lexically smaller sha is treated as the older one.
func olderLexical(a, b string) string {
	if a <= b {
		return a
	}
	return b
}

func TestUnion(t *testing.T) {
	tests := []struct {
		name  string
		seed  []Row
		apply Row
		wantF []string
		wantC string
		wantD int
		wantS string
	}{
		{
			name:  "insert into an empty map",
			seed:  nil,
			apply: Row{T: "t1", F: []string{"src/b.py", "src/a.py"}, C: "ccc", D: 10, S: "pass"},
			wantF: []string{"src/a.py", "src/b.py"},
			wantC: "ccc", wantD: 10, wantS: "pass",
		},
		{
			name:  "F is the set union, never a replacement",
			seed:  []Row{{T: "t1", F: []string{"src/a.py", "src/db.py"}, C: "bbb", D: 10, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/a.py"}, C: "ccc", D: 3, S: "pass"},
			wantF: []string{"src/a.py", "src/db.py"},
			wantC: "bbb", wantD: 3, wantS: "pass",
		},
		{
			name:  "D and S take the new row's values",
			seed:  []Row{{T: "t1", F: []string{"src/a.py"}, C: "bbb", D: 999, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/a.py"}, C: "ccc", D: 7, S: "fail"},
			wantF: []string{"src/a.py"},
			wantC: "bbb", wantD: 7, wantS: "fail",
		},
		{
			name:  "C takes the older commit even when the new row is older",
			seed:  []Row{{T: "t1", F: []string{"src/a.py"}, C: "zzz", D: 1, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/b.py"}, C: "aaa", D: 2, S: "pass"},
			wantF: []string{"src/a.py", "src/b.py"},
			wantC: "aaa", wantD: 2, wantS: "pass",
		},
		{
			name:  "an empty incoming C keeps the existing one",
			seed:  []Row{{T: "t1", F: []string{"src/a.py"}, C: "bbb", D: 1, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/a.py"}, C: "", D: 2, S: "pass"},
			wantF: []string{"src/a.py"},
			wantC: "bbb", wantD: 2, wantS: "pass",
		},
		{
			name:  "duplicate files across both sides collapse",
			seed:  []Row{{T: "t1", F: []string{"src/a.py", "src/b.py"}, C: "bbb", D: 1, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/b.py", "src/c.py"}, C: "bbb", D: 2, S: "pass"},
			wantF: []string{"src/a.py", "src/b.py", "src/c.py"},
			wantC: "bbb", wantD: 2, wantS: "pass",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			for _, r := range tc.seed {
				m.Replace(r)
			}
			m.Union(tc.apply, olderLexical)
			got, ok := m.Get("t1")
			if !ok {
				t.Fatalf("Union did not insert the row")
			}
			if !reflect.DeepEqual(got.F, tc.wantF) {
				t.Errorf("F = %#v, want %#v", got.F, tc.wantF)
			}
			if got.C != tc.wantC {
				t.Errorf("C = %q, want %q", got.C, tc.wantC)
			}
			if got.D != tc.wantD {
				t.Errorf("D = %d, want %d", got.D, tc.wantD)
			}
			if got.S != tc.wantS {
				t.Errorf("S = %q, want %q", got.S, tc.wantS)
			}
		})
	}
}

func TestUnionNilComparatorKeepsExistingC(t *testing.T) {
	m := New()
	m.Replace(Row{T: "t1", F: []string{"src/a.py"}, C: "first", D: 1, S: "pass"})
	m.Union(Row{T: "t1", F: []string{"src/b.py"}, C: "second", D: 2, S: "pass"}, nil)
	got, _ := m.Get("t1")
	if got.C != "first" {
		t.Errorf("C = %q, want %q (a nil comparator must be deterministic: first wins)", got.C, "first")
	}
	if !reflect.DeepEqual(got.F, []string{"src/a.py", "src/b.py"}) {
		t.Errorf("F = %#v, want the union", got.F)
	}
}

// printOffences reports every expression in src that would write to stdout or stderr.
// The engine's rule is that cmd/ is the only place that prints; a library package that
// prints corrupts the JSON and JSONL streams its callers emit.
func printOffences(src string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", src, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.SelectorExpr:
			pkg, ok := e.X.(*ast.Ident)
			if !ok {
				return true
			}
			name := pkg.Name + "." + e.Sel.Name
			switch {
			case pkg.Name == "fmt" && (strings.HasPrefix(e.Sel.Name, "Print") || strings.HasPrefix(e.Sel.Name, "Fprint")),
				pkg.Name == "log",
				name == "os.Stdout", name == "os.Stderr":
				out = append(out, name)
			}
		case *ast.CallExpr:
			if id, ok := e.Fun.(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
				out = append(out, id.Name)
			}
		}
		return true
	})
	return out, nil
}

func TestPrintOffencesDetectsWriters(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"clean package", "package p\n\nfunc f() int { return 1 }\n", nil},
		{"fmt.Println", "package p\n\nimport \"fmt\"\n\nfunc f() { fmt.Println(\"x\") }\n", []string{"fmt.Println"}},
		{"fmt.Fprintf to stderr", "package p\n\nimport (\"fmt\"; \"os\")\n\nfunc f() { fmt.Fprintf(os.Stderr, \"x\") }\n", []string{"fmt.Fprintf", "os.Stderr"}},
		{"log.Printf", "package p\n\nimport \"log\"\n\nfunc f() { log.Printf(\"x\") }\n", []string{"log.Printf"}},
		{"builtin println", "package p\n\nfunc f() { println(\"x\") }\n", []string{"println"}},
		{"fmt.Sprintf is not printing", "package p\n\nimport \"fmt\"\n\nfunc f() string { return fmt.Sprintf(\"x\") }\n", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := printOffences(tc.src)
			if err != nil {
				t.Fatalf("printOffences: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("printOffences = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestPackagePrintsNothingOutsideTests(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		offences, err := printOffences(string(b))
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if len(offences) > 0 {
			t.Errorf("%s writes to stdout/stderr via %v; only cmd/ may print", name, offences)
		}
		scanned++
	}
	if scanned == 0 {
		t.Fatal("scanned no non-test source files; the guard would pass vacuously")
	}
}
