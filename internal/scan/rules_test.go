package scan

import (
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

func TestInnermostNodeOwnsALineAndAClassHasMethodEdges(t *testing.T) {
	r := ScanFile("pkg/shapes.py", []byte("class Shape:\n    def area(self):\n        return measure(self)\n\n    def name(self):\n        return label()\n"))
	if got, want := r.Calls["pkg/shapes.py::Shape::area"], []string{"measure"}; !reflect.DeepEqual(got, want) {
		t.Errorf("area calls %v, want %v", got, want)
	}
	if got := r.Calls["pkg/shapes.py::Shape"]; got != nil {
		t.Errorf("the class owns no line its methods own, but calls %v", got)
	}
	want := []graph.Edge{
		{From: "pkg/shapes.py::Shape", To: "pkg/shapes.py::Shape::area", Relation: graph.RelMethod},
		{From: "pkg/shapes.py::Shape", To: "pkg/shapes.py::Shape::name", Relation: graph.RelMethod},
	}
	if !reflect.DeepEqual(r.Edges, want) {
		t.Errorf("edges = %v\nwant %v", r.Edges, want)
	}
}

func TestEverySameNamedDefinitionIsCalledButNotFromTheDefinitionLine(t *testing.T) {
	var nodes []graph.Node
	calls := map[string][]string{}
	for _, f := range []struct{ path, src string }{
		{"a.py", "def load():\n    return 1\n"},
		{"b.py", "def load():\n    return 2\n"},
		{"c.py", "def start():\n    return load()\n\n\ndef run(cb=load()):\n    return 0\n"},
	} {
		r := ScanFile(f.path, []byte(f.src))
		nodes = append(nodes, r.Nodes...)
		for k, v := range r.Calls {
			calls[k] = v
		}
	}
	want := []graph.Edge{
		{From: "c.py::start", To: "a.py::load", Relation: graph.RelCalls},
		{From: "c.py::start", To: "b.py::load", Relation: graph.RelCalls},
	}
	if got, _ := Link(nodes, Refs{Calls: calls}); !reflect.DeepEqual(got, want) {
		t.Errorf("Link = %v\nwant %v", got, want)
	}
}

// linkAll scans each file and links the calls across all of them.
func linkAll(files ...[2]string) []graph.Edge {
	var nodes []graph.Node
	calls := map[string][]string{}
	for _, f := range files {
		r := ScanFile(f[0], []byte(f[1]))
		nodes = append(nodes, r.Nodes...)
		for k, v := range r.Calls {
			calls[k] = v
		}
	}
	got, _ := Link(nodes, Refs{Calls: calls})
	return got
}

// Issue #467: a Go call can never reach a Python definition, so when a same-named
// definition exists in the caller's own file extension only those are linked.
func TestACallLinksToSameNamedDefinitionsOfItsOwnFileTypeFirst(t *testing.T) {
	got := linkAll(
		[2]string{"util.go", "package util\n\nfunc Parse(s string) int {\n\treturn 0\n}\n"},
		[2]string{"util.py", "def Parse(s):\n    return 0\n"},
		[2]string{"main.go", "package util\n\nfunc Run() int {\n\treturn Parse(\"x\")\n}\n"},
	)
	want := []graph.Edge{{From: "main.go::Run", To: "util.go::Parse", Relation: graph.RelCalls}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Link = %v\nwant %v", got, want)
	}
}

// Issue #467: with no same-named definition in the caller's extension, the call falls
// back to other extensions, so a .ts -> .js call is kept.
func TestACallFallsBackToOtherFileTypesWhenItsOwnHasNoDefinition(t *testing.T) {
	got := linkAll(
		[2]string{"lib.js", "function helper(x) {\n  return x;\n}\n"},
		[2]string{"app.ts", "function main() {\n  return helper(1);\n}\n"},
	)
	want := []graph.Edge{{From: "app.ts::main", To: "lib.js::helper", Relation: graph.RelCalls}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Link = %v\nwant %v", got, want)
	}
}

// Issue #467: the fallback reaches only extensions that share a directory with the
// caller's somewhere in the graph. A Python test's `len(` names a builtin; a stray `len`
// defined in a Markdown plan or a Go file, which never sit beside a .py file, is no
// target. `src/app.ts` -> `lib/helper.js` is kept: .ts and .js meet in src/.
func TestTheFallbackReachesOnlyExtensionsThatShareADirectoryWithTheCaller(t *testing.T) {
	got := linkAll(
		[2]string{"docs/plan.md", "func (b BuildOutput) len() int {\n\treturn 0\n}\n"},
		[2]string{"internal/x.go", "package x\n\nfunc min(a, b int) int {\n\treturn a\n}\n"},
		[2]string{"bench/test_a.py", "def test_a():\n    assert len(xs) == min(1, 2)\n"},
		[2]string{"src/app.ts", "function main() {\n  return helper(1);\n}\n"},
		[2]string{"src/legacy.js", "function old() {\n  return 0;\n}\n"},
		[2]string{"lib/helper.js", "function helper(x) {\n  return x;\n}\n"},
	)
	want := []graph.Edge{{From: "src/app.ts::main", To: "lib/helper.js::helper", Relation: graph.RelCalls}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Link = %v\nwant %v", got, want)
	}
}

// A keyword is a call statement's opener only when it stands alone: after `.` or `::` it is
// a member or path segment, so `Calc::new()` and `p.delete()` are calls that must not be
// missed (spec §4.3), while a bare `new(` or `return(` is still no call. Each call token
// carries the receiver it was made on (issue #508).
func TestAQualifiedNameIsACallEvenWhenItIsAKeyword(t *testing.T) {
	r := ScanFile("lib.rs", []byte("fn make() {\n    let c = Calc::new();\n    store.delete(c);\n    return(c);\n}\n"))
	if got, want := r.Calls["lib.rs::make"], []string{"Calc.new", "store.delete"}; !reflect.DeepEqual(got, want) {
		t.Errorf("make calls %v, want %v", got, want)
	}
}

func TestAnUnbalancedQuoteOpensNoStringButLiteralBracesStaySkipped(t *testing.T) {
	for _, c := range []struct {
		line         string
		paren, brace int
	}{
		{`fn greet(name: &'static str) -> String {`, 0, 1},
		{`if c == '{' {`, 0, 1},
		{`let s = "}"; let t = '\'' ; {`, 0, 1},
		{`print("it's {")`, 0, 0},
	} {
		var tk tokenizer
		tk.line(c.line)
		if tk.paren != c.paren || tk.brace != c.brace {
			t.Errorf("%s: paren %d brace %d, want %d %d", c.line, tk.paren, tk.brace, c.paren, c.brace)
		}
	}
}

// Issue #507: a parameter list that never closes within 50 lines is no signature, so the
// line defines nothing and the class keeps its span.
func TestAWrappedHeaderThatNeverClosesDefinesNothing(t *testing.T) {
	src := "class A {\n    static void broken(int a,\n" + strings.Repeat("        int b,\n", 60) + "}\n"
	r := ScanFile("A.java", []byte(src))
	want := []graph.Node{{ID: "A.java::A", File: "A.java", Name: "A", Kind: graph.KindClass, Start: 1, End: 63}}
	if !reflect.DeepEqual(r.Nodes, want) {
		t.Errorf("nodes = %+v\nwant %+v", r.Nodes, want)
	}
}

// Issue #507: comments and strings that span lines are blanked, each line keeping its
// length; a string that opens and closes on one line is kept, as a test's label is read
// from it.
func TestCodeBlanksCommentsAndStringsThatSpanLines(t *testing.T) {
	lines := []string{
		`x = 1 # def a(`,
		`s = """def b(`,
		`def c(): """ + 'it("d")'`,
		"q = `e(",
		"f(`",
		`/* g( */ h() // i(`,
	}
	want := []string{
		`x = 1         `,
		`s =          `,
		`             + 'it("d")'`,
		`q =    `,
		`   `,
		`         h()      `,
	}
	if got := code(lines); !reflect.DeepEqual(got, want) {
		t.Errorf("code =\n%q\nwant\n%q", got, want)
	}
}

// Issue #507: a comment or string the file never closes opens none — here a shell glob
// and a backtick in a Lua comment — so only its opener is blanked and the definitions
// after it are still read. A shell `== /*` opens none at all.
func TestAnOpenerTheFileNeverClosesBlanksOnlyItself(t *testing.T) {
	lines := []string{
		`[[ "$B" == /* ]] || ls -d /*`,
		"-- a `tick in a comment",
		`run() { echo; }`,
	}
	want := []string{
		`[[ "$B" == /* ]] || ls -d   `,
		"                       ",
		`run() { echo; }`,
	}
	if got := code(lines); !reflect.DeepEqual(got, want) {
		t.Errorf("code =\n%q\nwant\n%q", got, want)
	}
	if got, want := code([]string{`s = '''x(`, `def f(): pass`}), []string{`s =    x(`, `def f(): pass`}; !reflect.DeepEqual(got, want) {
		t.Errorf("code = %q, want %q", got, want)
	}
	r := ScanFile("t.sh", []byte("ls -d /*\nx=`date\nrun() {\n  echo\n}\n"))
	if len(r.Nodes) != 1 || r.Nodes[0].ID != "t.sh::run" || r.Nodes[0].End != 5 {
		t.Errorf("nodes = %+v, want t.sh::run at 3-5", r.Nodes)
	}
}
