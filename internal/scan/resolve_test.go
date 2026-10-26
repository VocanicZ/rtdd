package scan

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// assembleAll scans each file and assembles them into one graph.
func assembleAll(files ...[2]string) (graph.Graph, Stats) {
	var rs []FileResult
	for _, f := range files {
		rs = append(rs, ScanFile(f[0], []byte(f[1])))
	}
	return AssembleStats(rs)
}

func edgesOf(g graph.Graph, rel graph.Relation) []graph.Edge {
	var out []graph.Edge
	for _, e := range g.Edges {
		if e.Relation == rel {
			out = append(out, e)
		}
	}
	return out
}

// Issue #508 acceptance: two classes, each with its own get, and a caller in each — each
// caller links only to its own class's get, whether it calls `get(` or `this.get(`.
func TestACallInsideAClassLinksToItsOwnClassesMethod(t *testing.T) {
	g, _ := assembleAll(
		[2]string{"a/Store.java", "class Store {\n    int get(int k) {\n        return k;\n    }\n\n    int load(int k) {\n        return get(k);\n    }\n}\n"},
		[2]string{"b/Cache.java", "class Cache {\n    int get(int k) {\n        return k;\n    }\n\n    int fetch(int k) {\n        return this.get(k);\n    }\n}\n"},
	)
	want := []graph.Edge{
		{From: "a/Store.java::Store::load", To: "a/Store.java::Store::get", Relation: graph.RelCalls},
		{From: "b/Cache.java::Cache::fetch", To: "b/Cache.java::Cache::get", Relation: graph.RelCalls},
	}
	if got := edgesOf(g, graph.RelCalls); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v\nwant %v", got, want)
	}
}

// Issue #508 rule 4: `obj.get(` on a local declared `Type obj`, `obj: Type` or
// `obj = new Type(` links to Type's get only, though another class in the caller's own
// file defines one too.
func TestAReceiversDeclaredTypePicksItsClassesMethod(t *testing.T) {
	for _, c := range []struct{ name, file, src string }{
		{"java declaration", "app/Main.java", "class Main {\n    int get() {\n        return 0;\n    }\n\n    int run(Store s) {\n        return s.get();\n    }\n}\n"},
		{"java field", "app/Main.java", "class Main {\n    private final Store s;\n\n    int get() {\n        return 0;\n    }\n\n    int run() {\n        return s.get();\n    }\n}\n"},
		{"construction", "app/Main.java", "class Main {\n    int get() {\n        return 0;\n    }\n\n    int run() {\n        Object x = 1;\n        s = new Store();\n        return s.get();\n    }\n}\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, _ := assembleAll(
				[2]string{"lib/Store.java", "class Store {\n    int get() {\n        return 1;\n    }\n}\n"},
				[2]string{c.file, c.src},
			)
			want := []graph.Edge{{From: "app/Main.java::Main::run", To: "lib/Store.java::Store::get", Relation: graph.RelCalls}}
			if c.name == "construction" { // `new Store(` calls the class itself
				want = append([]graph.Edge{{From: "app/Main.java::Main::run", To: "lib/Store.java::Store", Relation: graph.RelCalls}}, want...)
			}
			if got := edgesOf(g, graph.RelCalls); !reflect.DeepEqual(got, want) {
				t.Errorf("calls = %v\nwant %v", got, want)
			}
		})
	}
	g, _ := assembleAll(
		[2]string{"lib/store.ts", "class Store {\n  get() {\n    return 1;\n  }\n}\n"},
		[2]string{"lib/cache.ts", "class Cache {\n  get() {\n    return 2;\n  }\n}\n"},
		[2]string{"app/main.ts", "function run(s: Store) {\n  return s.get();\n}\n"},
	)
	want := []graph.Edge{{From: "app/main.ts::run", To: "lib/store.ts::Store::get", Relation: graph.RelCalls}}
	if got := edgesOf(g, graph.RelCalls); !reflect.DeepEqual(got, want) {
		t.Errorf("annotation: calls = %v\nwant %v", got, want)
	}
}

// Issue #508 rule 3: a call matching definitions in several files links to the one in
// the file the caller imports — Python `from`, a relative JS import, a Go module path.
func TestACallPrefersTheDefinitionInAnImportedFile(t *testing.T) {
	for _, c := range []struct {
		name  string
		files [][2]string
		want  []graph.Edge
	}{
		{"python from", [][2]string{
			{"pkg/a.py", "def load():\n    return 1\n"},
			{"pkg/b.py", "def load():\n    return 2\n"},
			{"app/main.py", "from pkg.b import load\n\ndef run():\n    return load()\n"},
		}, []graph.Edge{{From: "app/main.py::run", To: "pkg/b.py::load", Relation: graph.RelCalls}}},
		{"python relative", [][2]string{
			{"pkg/a.py", "def load():\n    return 1\n"},
			{"pkg/b.py", "def load():\n    return 2\n"},
			{"pkg/sub/main.py", "from ..a import load\n\ndef run():\n    return load()\n"},
		}, []graph.Edge{{From: "pkg/sub/main.py::run", To: "pkg/a.py::load", Relation: graph.RelCalls}}},
		{"js relative", [][2]string{
			{"src/a.js", "function load() {\n  return 1;\n}\n"},
			{"lib/b.js", "function load() {\n  return 2;\n}\n"},
			{"src/app/main.js", "import { load } from '../../lib/b';\n\nfunction run() {\n  return load();\n}\n"},
		}, []graph.Edge{{From: "src/app/main.js::run", To: "lib/b.js::load", Relation: graph.RelCalls}}},
		{"go module path", [][2]string{
			{"internal/a/a.go", "package a\n\nfunc Load() int {\n\treturn 1\n}\n"},
			{"internal/b/b.go", "package b\n\nfunc Load() int {\n\treturn 2\n}\n"},
			{"cmd/main.go", "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/m/internal/b\"\n)\n\nfunc run() int {\n\treturn b.Load()\n}\n"},
		}, []graph.Edge{{From: "cmd/main.go::run", To: "internal/b/b.go::Load", Relation: graph.RelCalls}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, st := assembleAll(c.files...)
			if got := edgesOf(g, graph.RelCalls); !reflect.DeepEqual(got, c.want) {
				t.Errorf("calls = %v\nwant %v", got, c.want)
			}
			if st.Imports != 1 {
				t.Errorf("imports = %d, want 1", st.Imports)
			}
		})
	}
}

// Issue #508 rule 5: a name that still matches more than maxCandidates definitions after
// every narrowing rule links to none, and is counted; at maxCandidates it links to all.
func TestACallMatchingTooManyDefinitionsLinksToNoneAndIsCounted(t *testing.T) {
	defs := func(n int) [][2]string {
		var fs [][2]string
		for i := range n {
			fs = append(fs, [2]string{"p" + string(rune('a'+i)) + "/x/m.py", "def load():\n    return 0\n"})
		}
		return append(fs, [2]string{"app/main.py", "def run():\n    return load()\n"})
	}
	g, st := assembleAll(defs(maxCandidates + 1)...)
	if got := edgesOf(g, graph.RelCalls); len(got) != 0 || st.AmbiguousCalls != 1 {
		t.Errorf("%d definitions: calls = %v, ambiguous = %d; want none and 1", maxCandidates+1, got, st.AmbiguousCalls)
	}
	g, st = assembleAll(defs(maxCandidates)...)
	if got := edgesOf(g, graph.RelCalls); len(got) != maxCandidates || st.AmbiguousCalls != 0 {
		t.Errorf("%d definitions: %d calls, ambiguous = %d; want %d and 0", maxCandidates, len(got), st.AmbiguousCalls, maxCandidates)
	}
}

// Issue #508: a class line's supertypes are inherits/implements edges, and a bare call
// to a method the class inherits links to the supertype's method.
func TestClassLinesEmitInheritsAndImplementsEdges(t *testing.T) {
	g, _ := assembleAll(
		[2]string{"Base.java", "abstract class Base {\n    int size() {\n        return 0;\n    }\n}\n"},
		[2]string{"Op.java", "interface Op {\n    int apply(int a);\n}\n"},
		[2]string{"Other.java", "class Other {\n    int size() {\n        return 9;\n    }\n}\n"},
		[2]string{"Calc.java", "public class Calc extends Base implements Op, Comparable<Calc> {\n    public int apply(int a) {\n        return size() + a;\n    }\n}\n"},
		[2]string{"shapes.py", "class Shape(Base, metaclass=ABCMeta):\n    pass\n\nclass Base:\n    pass\n"},
	)
	want := []graph.Edge{
		{From: "Calc.java::Calc", To: "Base.java::Base", Relation: graph.RelInherits},
		{From: "Calc.java::Calc", To: "Op.java::Op", Relation: graph.RelImplements},
		{From: "shapes.py::Shape", To: "shapes.py::Base", Relation: graph.RelInherits},
	}
	var got []graph.Edge
	for _, e := range g.Edges {
		if e.Relation == graph.RelInherits || e.Relation == graph.RelImplements {
			got = append(got, e)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("supertype edges = %v\nwant %v", got, want)
	}
	// size() is Base's, not Other's; Op's abstract `apply(` line is a call, as before #508.
	wantCalls := []graph.Edge{
		{From: "Calc.java::Calc::apply", To: "Base.java::Base::size", Relation: graph.RelCalls},
		{From: "Op.java::Op", To: "Calc.java::Calc::apply", Relation: graph.RelCalls},
	}
	if calls := edgesOf(g, graph.RelCalls); !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("calls = %v\nwant %v", calls, wantCalls)
	}
}

func TestSupertypesReadsEachLanguagesClassLine(t *testing.T) {
	for line, want := range map[string][]Super{
		"class A extends B implements C, D {":          {{"B", graph.RelInherits}, {"C", graph.RelImplements}, {"D", graph.RelImplements}},
		"export class A extends React.Component<P> {":  {{"Component", graph.RelInherits}},
		"interface A extends B, C {":                   {{"B", graph.RelInherits}, {"C", graph.RelInherits}},
		"class A(B, mod.C):":                           {{"B", graph.RelInherits}, {"C", graph.RelInherits}},
		"class A:":                                     nil,
		"class Derived : public Base, private Mixin {": {{"Base", graph.RelInherits}, {"Mixin", graph.RelInherits}},
		"class A(val x: Int) : B(x), I {":              {{"B", graph.RelInherits}, {"I", graph.RelInherits}},
		"class A < B":                                  {{"B", graph.RelInherits}},
		"impl fmt::Display for Calc {":                 {{"Display", graph.RelImplements}},
	} {
		if got := supertypes(line); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v, want %v", line, got, want)
		}
	}
}

func TestImportsReadsEachLanguagesImportLines(t *testing.T) {
	src := []string{
		"import java.util.List;",
		"import static a.b.C.m;",
		"import a.b.*;",
		"from pkg.mod import f, g",
		"from . import sibling",
		"import os.path",
		"import { x } from './y';",
		"} from '../z'",
		"const q = require('lib/q');",
		"local m = require \"a.b\"",
		"require_relative 'helper'",
		"use crate::scan::{Link, Refs};",
		"mod tests;",
		"#include \"calc.h\"",
		"using System.Text;",
		"source \"$DIR/lib.sh\"",
		"import (",
		"\t\"fmt\"",
		"\tg \"example.com/m/internal/graph\"",
		")",
	}
	want := []string{"java/util/List", "a/b/C/m", "a/b", "pkg/mod", "pkg/mod/f", "pkg/mod/g", "./sibling", "os/path",
		"./y", "../z", "lib/q", "a/b", "./helper", "scan", "scan/Link", "scan/Refs", "./tests", "./calc.h", "calc.h",
		"System/Text", "lib.sh", "fmt", "example.com/m/internal/graph"}
	if got := imports(src); !reflect.DeepEqual(got, want) {
		t.Errorf("imports =\n %q\nwant\n %q", got, want)
	}
}

// receiverOf reads every member-call form; a call with no receiver is not a member call.
// Issue #508 rule 1: a call on a receiver of a declared type imported from outside the
// repository (`Supplier<T> src; src.get()`) is a library call and links to nothing; one whose class
// declares no body for it (an interface) links to every implementation.
func TestADeclaredReceiverTypeLinksOnlyWithinItsClassHierarchy(t *testing.T) {
	g, _ := assembleAll(
		[2]string{"app/Cap.java", "import java.util.function.Supplier;\n\nclass Cap {\n    private final Supplier<Server> src;\n\n    int run(Op op) {\n        src.get();\n        return op.apply(1);\n    }\n}\n"},
		[2]string{"app/Set.java", "class Set {\n    int get() {\n        return 0;\n    }\n}\n"},
		[2]string{"app/Op.java", "interface Op {\n    int apply(int a);\n}\n"},
		[2]string{"app/Add.java", "class Add implements Op {\n    public int apply(int a) {\n        return a + 1;\n    }\n}\n"},
		[2]string{"app/Neg.java", "class Neg implements Op {\n    public int apply(int a) {\n        return -a;\n    }\n}\n"},
		[2]string{"app/Other.java", "class Other {\n    int apply(int a) {\n        return a;\n    }\n}\n"},
	)
	want := []graph.Edge{
		{From: "app/Cap.java::Cap::run", To: "app/Add.java::Add::apply", Relation: graph.RelCalls},
		{From: "app/Cap.java::Cap::run", To: "app/Neg.java::Neg::apply", Relation: graph.RelCalls},
	}
	var got []graph.Edge
	for _, e := range edgesOf(g, graph.RelCalls) {
		if e.From == "app/Cap.java::Cap::run" {
			got = append(got, e)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v\nwant %v", got, want)
	}
}

// A receiver typed by an import of a file links only to that file's definitions — here
// the Java record's label, never the same-directory Worker's.
func TestADeclaredReceiverTypeImportedFromAFileLinksOnlyIntoIt(t *testing.T) {
	g, _ := assembleAll(
		[2]string{"src/a/mat/Material.java", "package a.mat;\n\npublic record Material(String id) {\n    String label() {\n        return id;\n    }\n}\n"},
		[2]string{"src/a/sched/Worker.java", "class Worker {\n    String label() {\n        return \"w\";\n    }\n}\n"},
		[2]string{"src/a/sched/Cap.java", "import a.mat.Material;\n\nclass Cap {\n    String run(Material m) {\n        return m.label();\n    }\n}\n"},
	)
	var got []graph.Edge
	for _, e := range edgesOf(g, graph.RelCalls) {
		if e.From == "src/a/sched/Cap.java::Cap::run" {
			got = append(got, e)
		}
	}
	want := []graph.Edge{{From: "src/a/sched/Cap.java::Cap::run", To: "src/a/mat/Material.java::Material::label", Relation: graph.RelCalls}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v\nwant %v", got, want)
	}
}

func TestReceiverOfReadsEachMemberCallForm(t *testing.T) {
	for _, c := range []struct {
		line, call, recv string
		member           bool
	}{
		{"x = f(1)", "f(", "", false},
		{"store.get(k)", "get(", "store", true},
		{"user?.name()", "name(", "user", true},
		{"$repo->find($id)", "find(", "repo", true},
		{"Calc::new()", "new(", "Calc", true},
		{"obj:push(n)", "push(", "obj", true},
		{"make().run()", "run(", "", true},
		{"super().setUp()", "setUp(", "super", true},
	} {
		at := strings.Index(c.line, c.call)
		if recv, member := receiverOf(c.line, at); recv != c.recv || member != c.member {
			t.Errorf("%s at %q: (%q, %v), want (%q, %v)", c.line, c.call, recv, member, c.recv, c.member)
		}
	}
}

// `super.X(` and `super().X(` link to the supertype's X, never the class's own override;
// a cycle of supertypes ends.
func TestASuperCallLinksToTheSupertypesMethod(t *testing.T) {
	g, _ := assembleAll(
		[2]string{"base.py", "class Base:\n    def setUp(self):\n        return 0\n"},
		[2]string{"case.py", "from base import Base\n\nclass Case(Base):\n    def setUp(self):\n        return super().setUp()\n"},
		[2]string{"Loop.java", "class A extends B {\n    int m() {\n        return z();\n    }\n}\n\nclass B extends A {\n    int n() {\n        return 1;\n    }\n}\n"},
	)
	want := graph.Edge{From: "case.py::Case::setUp", To: "base.py::Base::setUp", Relation: graph.RelCalls}
	if got := edgesOf(g, graph.RelCalls); !reflect.DeepEqual(got, []graph.Edge{want}) {
		t.Errorf("calls = %v\nwant [%v]", got, want)
	}
}

// A same-package test in another source root (`src/test/java/a/b` beside
// `src/main/java/a/b`) imports nothing, yet its call narrows to its package's definition.
func TestACallNarrowsToItsPackageAcrossSourceRoots(t *testing.T) {
	var files [][2]string
	for _, p := range []string{"src/main/java/net/x/sched", "src/main/java/net/x/cmd", "lib/a", "lib/b", "lib/c", "lib/d"} {
		files = append(files, [2]string{p + "/Set.java", "class Set {\n    int size() {\n        return 0;\n    }\n}\n"})
	}
	files = append(files, [2]string{"src/test/java/net/x/sched/SetTest.java", "class SetTest {\n    void testSize() {\n        assertEquals(0, make().size());\n    }\n}\n"})
	g, st := assembleAll(files...)
	want := []graph.Edge{{From: "src/test/java/net/x/sched/SetTest.java::SetTest::testSize", To: "src/main/java/net/x/sched/Set.java::Set::size", Relation: graph.RelCalls}}
	if got := edgesOf(g, graph.RelCalls); !reflect.DeepEqual(got, want) || st.AmbiguousCalls != 0 {
		t.Errorf("calls = %v (ambiguous %d)\nwant %v", got, st.AmbiguousCalls, want)
	}
}

// A spec naming a directory — a wildcard import, a Rust `self::`/`super::` path — reaches
// the files in it; a relative import above the repository root reaches nothing.
func TestImportSpecsResolveToDirectoriesAndStayInsideTheRepository(t *testing.T) {
	ix := newFileIndex(map[string]string{"src/net/x/a/One.java": "", "src/net/x/a/Two.java": "", "src/lib.rs": "", "src/scan/mod.rs": "", "README.sh": ""})
	for spec, want := range map[string][]string{
		"net/x/a":       {"src/net/x/a/One.java", "src/net/x/a/Two.java"},
		"/src/scan":     {"src/scan/mod.rs"},
		"/src/lib":      {"src/lib.rs"},
		"net/x/a/One/m": {"src/net/x/a/One.java"},
		"nothing/here":  nil,
	} {
		got := ix.resolve(spec)
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("resolve(%q) = %v, want %v", spec, got, want)
		}
	}
	r := ScanFile("src/scan/link.rs", []byte("use self::util;\nuse super::graph::Node;\nsource /etc/profile\n"))
	if want := []string{"/src/scan/util", "/src/graph/Node", "etc/profile"}; !reflect.DeepEqual(r.Imports, want) {
		t.Errorf("imports = %q, want %q", r.Imports, want)
	}
	if r := ScanFile("a.py", []byte("from ...up import x\n")); len(r.Imports) != 0 {
		t.Errorf("an import above the root = %q, want none", r.Imports)
	}
}

// Refs naming a node the graph does not hold resolve to nothing, and do not fail.
func TestRefsOfAnAbsentNodeLinkNothing(t *testing.T) {
	r := ScanFile("a.py", []byte("def f():\n    return 1\n"))
	edges, _ := Link(r.Nodes, Refs{Calls: map[string][]string{"gone.py::g": {"f"}}, Supers: map[string][]Super{"gone.py::C": {{"f", graph.RelInherits}}}})
	if len(edges) != 0 {
		t.Errorf("edges = %v, want none", edges)
	}
}

// Review of #508: each of these once resolved wrong.
func TestResolutionEdgeCases(t *testing.T) {
	for _, c := range []struct {
		name  string
		files [][2]string
		want  []graph.Edge
	}{
		// A binding keyword is no type: `const s = new Store()` types s as Store.
		{"const binding", [][2]string{
			{"lib/store.ts", "class Store {\n  get() {\n    return 1;\n  }\n}\n"},
			{"lib/cache.ts", "class Cache {\n  get() {\n    return 2;\n  }\n}\n"},
			{"app/main.ts", "function run() {\n  const s = new Store();\n  return s.get();\n}\n"},
		}, []graph.Edge{
			{From: "app/main.ts::run", To: "lib/store.ts::Store", Relation: graph.RelCalls},
			{From: "app/main.ts::run", To: "lib/store.ts::Store::get", Relation: graph.RelCalls},
		}},
		// A method delegating to the module function of its own name reaches that function.
		{"delegation", [][2]string{
			{"repo.py", "def load():\n    return 1\n\n\nclass Repo:\n    def load(self):\n        return load()\n"},
		}, []graph.Edge{{From: "repo.py::Repo::load", To: "repo.py::load", Relation: graph.RelCalls}}},
		// `this.sock.close()` inside Conn.close is the imported socket's close.
		{"member of another object", [][2]string{
			{"net/sock.js", "class Socket {\n  close() {\n    return 0;\n  }\n}\n"},
			{"app/conn.js", "import { Socket } from '../net/sock';\n\nclass Conn {\n  close() {\n    return this.sock.close();\n  }\n}\n"},
		}, []graph.Edge{{From: "app/conn.js::Conn::close", To: "net/sock.js::Socket::close", Relation: graph.RelCalls}}},
		// A Go bare call keeps its own package's definition beside an imported one.
		{"go same directory", [][2]string{
			{"a/x.go", "package a\n\nimport \"example.com/m/b\"\n\nfunc run() int {\n\treturn helper()\n}\n"},
			{"a/y.go", "package a\n\nfunc helper() int {\n\treturn 1\n}\n"},
			{"b/z.go", "package b\n\nfunc helper() int {\n\treturn 2\n}\n"},
			{"c/w.go", "package c\n\nfunc helper() int {\n\treturn 3\n}\n"},
		}, []graph.Edge{
			{From: "a/x.go::run", To: "a/y.go::helper", Relation: graph.RelCalls},
			{From: "a/x.go::run", To: "b/z.go::helper", Relation: graph.RelCalls},
		}},
		// `import pkg.b as b` and an ESM `.js` spec for a `.ts` source both resolve.
		{"python import as", [][2]string{
			{"pkg/a.py", "def load():\n    return 1\n"},
			{"pkg/b.py", "def load():\n    return 2\n"},
			{"main.py", "import os, pkg.b as b\n\ndef run():\n    return b.load()\n"},
		}, []graph.Edge{{From: "main.py::run", To: "pkg/b.py::load", Relation: graph.RelCalls}}},
		{"esm js spec", [][2]string{
			{"lib/a.ts", "export function load() {\n  return 1;\n}\n"},
			{"lib/b.ts", "export function load() {\n  return 2;\n}\n"},
			{"src/app/main.ts", "import { load } from '../../lib/b.js';\n\nfunction run() {\n  return load();\n}\n"},
		}, []graph.Edge{{From: "src/app/main.ts::run", To: "lib/b.ts::load", Relation: graph.RelCalls}}},
		// `super().__init__()` with a supertype outside the graph links to nothing.
		{"external super", [][2]string{
			{"m.py", "class A(Ext):\n    def __init__(self):\n        super().__init__()\n\n\nclass B:\n    def __init__(self):\n        pass\n"},
		}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, _ := assembleAll(c.files...)
			if got := edgesOf(g, graph.RelCalls); !reflect.DeepEqual(got, c.want) {
				t.Errorf("calls = %v\nwant %v", got, c.want)
			}
		})
	}
	// One segment names a top-level module, not any file so named; an assignment to a
	// variable called source is no shell import.
	ix := newFileIndex(map[string]string{"utils.py": "", "internal/c/errors.go": "", "src/errors/e.go": ""})
	if got := ix.resolve("errors"); !reflect.DeepEqual(got, []string{"src/errors/e.go"}) {
		t.Errorf(`resolve("errors") = %v, want [src/errors/e.go]`, got)
	}
	if got := ix.resolve("utils"); !reflect.DeepEqual(got, []string{"utils.py"}) {
		t.Errorf(`resolve("utils") = %v, want [utils.py]`, got)
	}
	if got := imports([]string{"    source = f()", "use a::b::*;", "use x::Y as Z;"}); !reflect.DeepEqual(got, []string{"a/b", "x/Y"}) {
		t.Errorf("imports = %q, want [a/b x/Y]", got)
	}
}
