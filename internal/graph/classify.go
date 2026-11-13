package graph

import (
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/paths"
)

// WholeFileName names the synthetic test node of a whole-file test file; its ID is
// "<file>::<file>" and its runnable id is the file path (#505).
const WholeFileName = "<file>"

// testName is the test-function naming convention shared by the languages whose tests
// are plain functions or methods: test_x / testX (Python, Ruby minitest, JUnit 3, PHPUnit),
// TestX, BenchmarkX, FuzzX, ExampleX (Go). A test node (it/test/describe) needs no name.
var testName = regexp.MustCompile(`^(?:[Tt]est|Benchmark|Fuzz|Example)(?:_|[A-Z0-9]|$)`)

// IsTestFile reports whether rel is a test file: it matches a TestFiles or WholeFileTests
// glob and no TestExclude glob (spec §6 — testdata/ and fixtures/ are never test files).
func IsTestFile(rel string, cfg Config) bool {
	if isDataFile(rel) {
		return false
	}
	return (matchAny(cfg.TestFiles, rel) || matchAny(cfg.WholeFileTests, rel)) && !matchAny(cfg.TestExclude, rel)
}

// dataExtensions are the extensions of files that hold output or data, never tests: a log
// or transcript a smoke run wrote, a table a test reads (#573). .json is left out: a
// repository may keep test code in it. Built in rather than a
// test_exclude default, which only a repository with no config — or a new init — would get.
var dataExtensions = []string{".log", ".jsonl", ".txt", ".out", ".csv", ".tsv"}

func isDataFile(rel string) bool {
	return slices.Contains(dataExtensions, strings.ToLower(path.Ext(rel)))
}

// Classify sets IsTest on every node, whichever source produced it: a func, method or
// test node in a test file is a test; a class or a file never is (spec §6).
func Classify(nodes []Node, cfg Config) {
	memo := map[string]bool{}
	for i := range nodes {
		n := &nodes[i]
		is, ok := memo[n.File]
		if !ok {
			is = IsTestFile(n.File, cfg)
			memo[n.File] = is
		}
		n.IsTest = is && n.Kind != KindClass && n.Kind != KindFile
	}
}

// SplitTests runs after Classify and separates the runnable tests of each test file from
// its helpers (spec §6, #505). A test file is whole-file when it matches WholeFileTests or
// defines no test — no test node and no func or method named by the test convention:
// every func in it becomes a helper, and one synthetic WholeFileNode, which calls each of
// them, is its only test. In any other test file the tests are its test nodes and its
// convention-named funcs; every other func is a helper. files is the scanned file list: a
// WholeFileTests file in it gets its file node even when it defines nothing, and so does a
// test file whose file node calls (#570).
func SplitTests(g *Graph, files []string, cfg Config) {
	tested := map[string]bool{} // test file -> it defines a test
	for _, n := range g.Nodes {
		if n.IsTest {
			tested[n.File] = tested[n.File] || n.Kind == KindTest || testName.MatchString(n.Name)
		}
	}
	whole := map[string]bool{}
	for f, t := range tested {
		if !t || matchAny(cfg.WholeFileTests, f) {
			whole[f] = true
		}
	}
	for _, f := range files {
		if matchAny(cfg.WholeFileTests, f) && !matchAny(cfg.TestExclude, f) {
			whole[f] = true
		}
	}
	// A test file that defines nothing yet calls code — a spec whose cases are anonymous
	// literals in the table it returns (#570) — is whole-file too: its file node calls.
	for _, e := range g.Edges {
		if _, defines := tested[e.From]; e.Relation == RelCalls && !defines && !whole[e.From] &&
			!strings.Contains(e.From, "::") && IsTestFile(e.From, cfg) {
			whole[e.From] = true
		}
	}
	if len(whole) == 0 && len(tested) == 0 {
		return
	}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if !n.IsTest {
			continue
		}
		if whole[n.File] || (n.Kind != KindTest && !testName.MatchString(n.Name)) {
			n.IsTest, n.Helper = false, true
			if whole[n.File] {
				g.Edges = append(g.Edges, Edge{From: WholeFileNode(n.File).ID, To: n.ID, Relation: RelCalls})
			}
		}
	}
	for f := range whole {
		g.Nodes = append(g.Nodes, WholeFileNode(f))
	}
	Sort(g)
}

// WholeFileNode is the synthetic test node standing for all of file, run as a whole. It
// owns no line: a changed top-level line of its file maps to the file's <module> node,
// which selects it.
func WholeFileNode(file string) Node {
	return Node{ID: file + "::" + WholeFileName, File: file, Name: WholeFileName, Kind: KindTest, IsTest: true}
}

// RunnableID is the id an agent runs a test by: a whole-file node's file path, or the
// node's ID without the "@line" suffix the scanner adds to tell same-named definitions
// apart (#505).
func RunnableID(n Node) string {
	if n.Name == WholeFileName {
		return n.File
	}
	if s := "@" + strconv.Itoa(n.Start); strings.HasSuffix(n.ID, n.Name+s) {
		return strings.TrimSuffix(n.ID, s)
	}
	return n.ID
}

func matchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if paths.MatchGlob(g, rel) {
			return true
		}
	}
	return false
}
