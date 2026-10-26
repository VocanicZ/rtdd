package graph

import (
	"reflect"
	"testing"
)

func split(t *testing.T, nodes []Node, files []string, cfg Config) Graph {
	t.Helper()
	g := Graph{Nodes: nodes}
	Classify(g.Nodes, cfg)
	SplitTests(&g, files, cfg)
	return g
}

func role(n Node) string {
	switch {
	case n.IsTest:
		return "test"
	case n.Helper:
		return "helper"
	}
	return "code"
}

func roles(g Graph) map[string]string {
	out := map[string]string{}
	for _, n := range g.Nodes {
		out[n.ID] = role(n)
	}
	return out
}

// Issue #505: a shell test script is whole-file: its stubs are helpers, and one file node
// that calls each of them is its only test.
func TestWholeFileTestsGlobMakesEveryFuncAHelperUnderOneFileNode(t *testing.T) {
	const f = "test/test_drive.sh"
	g := split(t, []Node{
		{ID: f + "::git", File: f, Name: "git", Kind: KindFunc, Start: 3, End: 5},
		{ID: f + "::test_spawn", File: f, Name: "test_spawn", Kind: KindFunc, Start: 7, End: 9},
		{ID: "lib/drive.sh::spawn", File: "lib/drive.sh", Name: "spawn", Kind: KindFunc, Start: 1, End: 3},
	}, nil, DefaultConfig())

	want := map[string]string{f + "::git": "helper", f + "::test_spawn": "helper", f + "::<file>": "test", "lib/drive.sh::spawn": "code"}
	if got := roles(g); !reflect.DeepEqual(got, want) {
		t.Errorf("roles = %v, want %v", got, want)
	}
	wantEdges := []Edge{{f + "::<file>", f + "::git", RelCalls}, {f + "::<file>", f + "::test_spawn", RelCalls}}
	if !reflect.DeepEqual(g.Edges, wantEdges) {
		t.Errorf("edges = %v, want %v", g.Edges, wantEdges)
	}
}

// Issue #505: a test file that defines no test is whole-file too; one that does keeps its
// tests, and its other funcs are helpers.
func TestATestFileWithNoTestIsWholeFileAndOtherFilesKeepTheirTests(t *testing.T) {
	g := split(t, []Node{
		{ID: "tests/helpers.py::make", File: "tests/helpers.py", Name: "make", Kind: KindFunc, Start: 1, End: 2},
		{ID: "tests/test_a.py::TestA", File: "tests/test_a.py", Name: "TestA", Kind: KindClass, Start: 1, End: 9},
		{ID: "tests/test_a.py::TestA::setUp", File: "tests/test_a.py", Name: "setUp", Kind: KindMethod, Start: 2, End: 3},
		{ID: "tests/test_a.py::TestA::test_x", File: "tests/test_a.py", Name: "test_x", Kind: KindMethod, Start: 4, End: 5},
		{ID: "a_test.go::TestAdd", File: "a_test.go", Name: "TestAdd", Kind: KindFunc, Start: 1, End: 3},
		{ID: "a_test.go::BenchmarkAdd", File: "a_test.go", Name: "BenchmarkAdd", Kind: KindFunc, Start: 5, End: 7},
		{ID: "a_test.go::Testify", File: "a_test.go", Name: "Testify", Kind: KindFunc, Start: 9, End: 11},
		{ID: "src/a.test.js::adds", File: "src/a.test.js", Name: "adds", Kind: KindTest, Start: 3, End: 5},
		{ID: "src/a.test.js::build", File: "src/a.test.js", Name: "build", Kind: KindFunc, Start: 1, End: 2},
	}, nil, DefaultConfig())

	want := map[string]string{
		"tests/helpers.py::make": "helper", "tests/helpers.py::<file>": "test",
		"tests/test_a.py::TestA": "code", "tests/test_a.py::TestA::setUp": "helper", "tests/test_a.py::TestA::test_x": "test",
		"a_test.go::TestAdd": "test", "a_test.go::BenchmarkAdd": "test", "a_test.go::Testify": "helper",
		"src/a.test.js::adds": "test", "src/a.test.js::build": "helper",
	}
	if got := roles(g); !reflect.DeepEqual(got, want) {
		t.Errorf("roles = %v\nwant    %v", got, want)
	}
}

// Issue #505: a whole_file_tests file is a test file even outside test_files, and gets its
// file node when the scanner found nothing in it; test_exclude still wins.
func TestWholeFileTestsFileWithNoNodeGetsItsFileNode(t *testing.T) {
	cfg := DefaultConfig()
	if !IsTestFile("checks/smoke.bats", cfg) || IsTestFile("testdata/smoke.bats", cfg) {
		t.Errorf("IsTestFile(checks/smoke.bats, testdata/smoke.bats) = %v, %v; want true, false",
			IsTestFile("checks/smoke.bats", cfg), IsTestFile("testdata/smoke.bats", cfg))
	}
	g := split(t, nil, []string{"checks/smoke.bats", "testdata/smoke.bats", "src/a.sh"}, cfg)
	if want := []Node{WholeFileNode("checks/smoke.bats")}; !reflect.DeepEqual(g.Nodes, want) {
		t.Errorf("nodes = %v, want %v", g.Nodes, want)
	}
}

func TestWholeFileTestsOverrideReplacesTheDefaults(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, "whole_file_tests: [\"e2e/**\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.WholeFileTests, []string{"e2e/**"}) {
		t.Fatalf("WholeFileTests = %v", cfg.WholeFileTests)
	}
	g := split(t, []Node{
		{ID: "e2e/run.py::test_login", File: "e2e/run.py", Name: "test_login", Kind: KindFunc, Start: 1, End: 2},
		{ID: "test/test_a.sh::test_a", File: "test/test_a.sh", Name: "test_a", Kind: KindFunc, Start: 1, End: 2},
	}, nil, cfg)
	want := map[string]string{"e2e/run.py::test_login": "helper", "e2e/run.py::<file>": "test", "test/test_a.sh::test_a": "test"}
	if got := roles(g); !reflect.DeepEqual(got, want) {
		t.Errorf("roles = %v, want %v", got, want)
	}
}

// Issue #505: the runnable id is the file for a whole-file node, and drops only the
// "@line" suffix the scanner adds — never an "@" a label holds.
func TestRunnableID(t *testing.T) {
	for _, c := range []struct {
		n    Node
		want string
	}{
		{WholeFileNode("test/test_drive.sh"), "test/test_drive.sh"},
		{Node{ID: "a.test.js::works@12", Name: "works", Start: 12}, "a.test.js::works"},
		{Node{ID: "a.test.js::d::works", Name: "works", Start: 12}, "a.test.js::d::works"},
		{Node{ID: "a.test.js::pings @3", Name: "pings @3", Start: 3}, "a.test.js::pings @3"},
	} {
		if got := RunnableID(c.n); got != c.want {
			t.Errorf("RunnableID(%s) = %q, want %q", c.n.ID, got, c.want)
		}
	}
}
