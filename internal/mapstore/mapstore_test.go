package mapstore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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

func fixtureMap() *Map {
	m := New()
	m.Replace(Row{T: "tests/test_auth.py::test_login", F: []string{"src/auth.py", "src/db.py"}, C: "aaa1111", D: 412, S: "pass"})
	m.Replace(Row{T: "tests/test_auth.py::test_logout", F: []string{"src/auth.py"}, C: "aaa1111", D: 90, S: "fail"})
	m.Replace(Row{T: "tests/test_db.py::test_query", F: []string{"src/db.py"}, C: "aaa1111", D: 15, S: "pass"})
	m.Replace(Row{T: "tests/test_render.py::test_page", F: []string{"src/render.py", "templates/page.html"}, C: "aaa1111", D: 230, S: "pass"})
	return m
}

func TestTestsCovering(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{"single file, two tests", []string{"src/auth.py"},
			[]string{"tests/test_auth.py::test_login", "tests/test_auth.py::test_logout"}},
		{"single file, one test", []string{"src/render.py"},
			[]string{"tests/test_render.py::test_page"}},
		{"two files union without duplicating a test", []string{"src/auth.py", "src/db.py"},
			[]string{"tests/test_auth.py::test_login", "tests/test_auth.py::test_logout", "tests/test_db.py::test_query"}},
		{"unknown file selects nothing", []string{"src/brand_new.py"}, []string{}},
		{"no files selects nothing", nil, []string{}},
		{"a deleted path still selects its tests", []string{"templates/page.html"},
			[]string{"tests/test_render.py::test_page"}},
	}
	m := fixtureMap()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.TestsCovering(tc.files)
			sort.Strings(got)
			if len(got) == 0 {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("TestsCovering(%v) = %#v, want %#v", tc.files, got, tc.want)
			}
		})
	}
}

func TestFanOut(t *testing.T) {
	got := fixtureMap().FanOut()
	want := map[string]int{
		"src/auth.py":         2,
		"src/db.py":           2,
		"src/render.py":       1,
		"templates/page.html": 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FanOut() = %#v, want %#v", got, want)
	}
}

// PRD #232 AC6: a row written by one adapter is never served to another. Serving a pytest
// row to the vitest adapter hands `npx vitest run` a pytest nodeid, which selects nothing
// and reports green — a false pass wearing a real id.
func TestTestsCoveringForNeverServesAnotherAdaptersRow(t *testing.T) {
	m := New()
	m.Replace(Row{T: "tests/test_a.py::test_one", F: []string{"src/calc.py"}, A: "python"})
	m.Replace(Row{T: "test/calc.test.js", F: []string{"src/calc.py"}, A: "vitest"})

	got := m.TestsCoveringFor("python", "", []string{"src/calc.py"})
	if len(got) != 1 || got[0] != "tests/test_a.py::test_one" {
		t.Errorf("TestsCoveringFor(python) = %v, want only the python row", got)
	}
	got = m.TestsCoveringFor("vitest", "", []string{"src/calc.py"})
	if len(got) != 1 || got[0] != "test/calc.test.js" {
		t.Errorf("TestsCoveringFor(vitest) = %v, want only the vitest row", got)
	}
}

// Decision 3: an untagged row is the whole map of every repo that seeded before this PRD.
// It belongs to the adapter meta.json names, and to nothing else.
func TestUntaggedRowsBelongToTheAdapterMetaNames(t *testing.T) {
	b, err := os.ReadFile("testdata/pre-prd-map.jsonl")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if strings.Contains(string(b), `"a"`) {
		t.Fatal("the pre-PRD fixture must contain no adapter tag; that is what makes it the fixture")
	}
	p := filepath.Join(t.TempDir(), "map.jsonl")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write map: %v", err)
	}
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := m.TestsCoveringFor("python", "python", []string{"src/calc.py"}); len(got) == 0 {
		t.Error("an untagged row was withheld from the adapter meta.json names; a seeded repo lost its map on upgrade")
	}
	if got := m.TestsCoveringFor("vitest", "python", []string{"src/calc.py"}); len(got) != 0 {
		t.Errorf("TestsCoveringFor(vitest) = %v, want none: an untagged row is python's, not everyone's", got)
	}
	// No meta.json to say whose it is: ignored for selection, never guessed at.
	if got := m.TestsCoveringFor("python", "", []string{"src/calc.py"}); len(got) != 0 {
		t.Errorf("TestsCoveringFor with no legacy adapter = %v, want none", got)
	}
}

// omitempty: an existing map.jsonl must round-trip byte-identically through read+write.
func TestWritingAnUntaggedRowEmitsNoAdapterKey(t *testing.T) {
	m := New()
	m.Replace(Row{T: "tests/test_a.py::test_one", F: []string{"src/calc.py"}, C: "abc1234", D: 12, S: "pass"})
	p := filepath.Join(t.TempDir(), "map.jsonl")
	if err := m.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(out), `"a"`) {
		t.Errorf("Save emitted an adapter key for an untagged row: %s", out)
	}
}

// Two ecosystems naming one test the same string is a collision the tag exists to
// survive: Union keys on the adapter AND the id, so neither row overwrites the other.
func TestUnionKeepsTwoAdaptersRowsWithTheSameTestID(t *testing.T) {
	m := New()
	keep := func(a, b string) string { return a }
	m.Union(Row{T: "src/calc_test", F: []string{"src/calc.go"}, A: "go", C: "aaa", D: 4, S: "pass"}, keep)
	m.Union(Row{T: "src/calc_test", F: []string{"src/calc.rs"}, A: "cargo", C: "aaa", D: 9, S: "pass"}, keep)

	if m.Len() != 2 {
		t.Fatalf("Len() = %d, want 2: one id under two adapters is two rows", m.Len())
	}
	if got := m.TestsCoveringFor("go", "", []string{"src/calc.go"}); len(got) != 1 {
		t.Errorf("TestsCoveringFor(go) = %v, want the go row", got)
	}
	if got := m.TestsCoveringFor("cargo", "", []string{"src/calc.go"}); len(got) != 0 {
		t.Errorf("TestsCoveringFor(cargo) = %v, want none: the go row is not cargo's", got)
	}
}

// M1a's union resolution still holds inside one adapter: same id, same tag, one row,
// F set-unioned and the tag carried through unchanged.
func TestUnionMergesWithinOneAdapterAndCarriesTheTag(t *testing.T) {
	m := New()
	keep := func(a, b string) string { return a }
	m.Union(Row{T: "t1", F: []string{"src/b.py"}, A: "python", C: "aaa", D: 1, S: "pass"}, keep)
	m.Union(Row{T: "t1", F: []string{"src/a.py"}, A: "python", C: "bbb", D: 7, S: "fail"}, keep)

	if m.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", m.Len())
	}
	got := m.Rows()[0]
	want := Row{T: "t1", F: []string{"src/a.py", "src/b.py"}, A: "python", C: "aaa", D: 7, S: "fail"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Union() = %#v, want %#v", got, want)
	}
}

// Compaction is LoadWith followed by Save. With tags present it must keep both rows of a
// cross-adapter id collision, and it must stay idempotent.
func TestCompactionPreservesTagsAndCollidingIDs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "map.jsonl")
	in := `{"t":"t1","f":["src/b.go"],"c":"aaa","d":1,"s":"pass","a":"go"}
{"t":"t1","f":["src/a.rs"],"c":"aaa","d":2,"s":"pass","a":"cargo"}
{"t":"t1","f":["src/a.go"],"c":"aaa","d":3,"s":"pass","a":"go"}
`
	if err := os.WriteFile(p, []byte(in), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := m.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := `{"t":"t1","f":["src/a.rs"],"c":"aaa","d":2,"s":"pass","a":"cargo"}
{"t":"t1","f":["src/a.go","src/b.go"],"c":"aaa","d":3,"s":"pass","a":"go"}
`
	if string(b) != want {
		t.Errorf("compaction produced:\n%s\nwant:\n%s", b, want)
	}
	m2, err := Load(p)
	if err != nil {
		t.Fatalf("Load again: %v", err)
	}
	if err := m2.Save(p); err != nil {
		t.Fatalf("Save again: %v", err)
	}
	b2, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read back again: %v", err)
	}
	if string(b2) != string(b) {
		t.Errorf("compaction is not idempotent:\nfirst:\n%s\nsecond:\n%s", b, b2)
	}
}

// TestsCovering answers "which tests cover this file" with the whole map, adapter or not —
// rtdd explain and the drift guard ask it without an adapter in hand.
func TestTestsCoveringStillSpansEveryAdapter(t *testing.T) {
	m := New()
	m.Replace(Row{T: "tests/test_a.py::test_one", F: []string{"src/calc.py"}, A: "python"})
	m.Replace(Row{T: "test/calc.test.js", F: []string{"src/calc.py"}, A: "vitest"})
	m.Replace(Row{T: "tests/test_legacy.py::test_old", F: []string{"src/calc.py"}})

	got := m.TestsCovering([]string{"src/calc.py"})
	sort.Strings(got)
	want := []string{"test/calc.test.js", "tests/test_a.py::test_one", "tests/test_legacy.py::test_old"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TestsCovering() = %v, want %v", got, want)
	}
}

// Get holds an id but no adapter. When two adapters share one id its answer must still
// be deterministic, or rtdd explain prints a different row on every run of the same map.
func TestGetIsDeterministicWhenTwoAdaptersShareOneID(t *testing.T) {
	m := New()
	m.Replace(Row{T: "t1", F: []string{"src/a.rs"}, A: "cargo"})
	m.Replace(Row{T: "t1", F: []string{"src/a.go"}, A: "go"})

	for i := 0; i < 20; i++ {
		got, ok := m.Get("t1")
		if !ok {
			t.Fatal("Get(t1) found nothing")
		}
		if got.A != "cargo" {
			t.Fatalf("Get(t1).A = %q, want the lowest adapter name %q", got.A, "cargo")
		}
	}
}
