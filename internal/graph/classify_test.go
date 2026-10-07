package graph

import (
	"testing"
)

// PRD #409 AC7: every default test_files glob, alone, makes a func node in a matching
// file a test.
func TestEachDefaultTestFilesGlobMarksAFuncNodeATest(t *testing.T) {
	cases := map[string]string{
		"**/test_*":       "pkg/test_calc.py",
		"**/*_test.*":     "calc_test.go",
		"**/*.test.*":     "src/calc.test.ts",
		"**/*.spec.*":     "src/calc.spec.js",
		"**/*Test.*":      "app/CalcTest.java",
		"**/*Tests.*":     "app/CalcTests.cs",
		"**/tests/**":     "tests/helpers.py",
		"**/test/**":      "test/helpers.rb",
		"**/spec/**":      "spec/calc_helper.rb",
		"**/__tests__/**": "src/__tests__/calc.js",
	}
	defaults := DefaultConfig()
	if len(cases) != len(defaults.TestFiles) {
		t.Fatalf("%d cases for %d default globs", len(cases), len(defaults.TestFiles))
	}
	for _, glob := range defaults.TestFiles {
		file, ok := cases[glob]
		if !ok {
			t.Errorf("default glob %q has no case", glob)
			continue
		}
		cfg := defaults
		cfg.TestFiles = []string{glob}
		nodes := []Node{{ID: file + "::f", File: file, Name: "f", Kind: KindFunc}}
		Classify(nodes, cfg)
		if !nodes[0].IsTest {
			t.Errorf("%s alone does not make a func in %s a test", glob, file)
		}
	}
}

func TestClassesAndNonTestFilesAreNeverTests(t *testing.T) {
	nodes := []Node{
		{ID: "tests/test_a.py::TestA", File: "tests/test_a.py", Name: "TestA", Kind: KindClass},
		{ID: "tests/test_a.py::TestA::test_x", File: "tests/test_a.py", Name: "test_x", Kind: KindMethod},
		{ID: "src/a.test.js::adds", File: "src/a.test.js", Name: "adds", Kind: KindTest},
		{ID: "src/a.py::f", File: "src/a.py", Name: "f", Kind: KindFunc},
		{ID: "tests/testdata/b_test.py::f", File: "tests/testdata/b_test.py", Name: "f", Kind: KindFunc},
		{ID: "tests/fixtures/c.py::f", File: "tests/fixtures/c.py", Name: "f", Kind: KindFunc},
	}
	Classify(nodes, DefaultConfig())
	want := []bool{false, true, true, false, false, false}
	for i, n := range nodes {
		if n.IsTest != want[i] {
			t.Errorf("%s: IsTest = %v, want %v", n.ID, n.IsTest, want[i])
		}
	}
}

func TestTestFilesOverrideReplacesTheDefaults(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, "test_files: [\"checks/**\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	nodes := []Node{
		{ID: "checks/a.py::f", File: "checks/a.py", Name: "f", Kind: KindFunc},
		{ID: "tests/test_a.py::f", File: "tests/test_a.py", Name: "f", Kind: KindFunc},
	}
	Classify(nodes, cfg)
	if !nodes[0].IsTest || nodes[1].IsTest {
		t.Errorf("IsTest = %v, %v; want the override alone to decide (true, false)", nodes[0].IsTest, nodes[1].IsTest)
	}
}
