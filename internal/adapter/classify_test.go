package adapter

import "testing"

func pyAdapter() *Adapter {
	return &Adapter{
		Name:         "python",
		TestGlobs:    []string{"tests/**/*.py", "**/test_*.py", "**/*_test.py"},
		SourceGlobs:  []string{"**/*.py"},
		Opaque:       []string{"**/*.yaml", "**/*.sql", "**/fixtures/**"},
		FullEscalate: []string{"requirements.txt", "pyproject.toml", "**/conftest.py"},
	}
}

func TestClassify(t *testing.T) {
	a := pyAdapter()
	cases := []struct {
		rel                                    string
		test, opaque, escalate, instrumentable bool
	}{
		{"src/logic.py", false, false, false, true},
		{"src/__init__.py", false, false, false, true},
		{"tests/test_a.py", true, false, false, false},
		{"tests/helpers.py", true, false, false, false},
		{"pkg/test_thing.py", true, false, false, false},
		{"pkg/thing_test.py", true, false, false, false},
		{"config/app.yaml", false, true, false, false},
		{"db/schema.sql", false, true, false, false},
		{"tests/fixtures/blob.json", false, true, false, false},
		{"requirements.txt", false, false, true, false},
		{"pyproject.toml", false, false, true, false},
		// FullEscalate wins over TestGlobs: conftest.py collects no tests, so naming it
		// as a selector is exit 5 (no-tests-collected). It escalates to a full run.
		// It stays instrumentable under this fixture's broad "**/*.py" source glob —
		// IsInstrumentable is SourceGlobs AND not-test AND not-opaque, and escalation is
		// none of the three. The shipped adapter scopes source to "src/**/*.py", so a
		// conftest under tests/ falls out there on the SourceGlobs term instead.
		{"tests/unit/conftest.py", false, false, true, true},
		{"README.md", false, false, false, false},
	}
	for _, tc := range cases {
		if got := a.IsTestFile(tc.rel); got != tc.test {
			t.Errorf("IsTestFile(%q) = %v, want %v", tc.rel, got, tc.test)
		}
		if got := a.IsOpaque(tc.rel); got != tc.opaque {
			t.Errorf("IsOpaque(%q) = %v, want %v", tc.rel, got, tc.opaque)
		}
		if got := a.IsFullEscalate(tc.rel); got != tc.escalate {
			t.Errorf("IsFullEscalate(%q) = %v, want %v", tc.rel, got, tc.escalate)
		}
		if got := a.IsInstrumentable(tc.rel); got != tc.instrumentable {
			t.Errorf("IsInstrumentable(%q) = %v, want %v", tc.rel, got, tc.instrumentable)
		}
	}
}

func TestClassifyNormalisesSeparators(t *testing.T) {
	a := pyAdapter()
	// Everything inside the engine is slash-separated and cleaned, but be tolerant
	// of a caller that has not been through internal/paths yet.
	if !a.IsTestFile("./tests/test_a.py") {
		t.Errorf(`IsTestFile("./tests/test_a.py") = false, want true`)
	}
	if !a.IsInstrumentable("src//logic.py") {
		t.Errorf(`IsInstrumentable("src//logic.py") = false, want true`)
	}
}

// The classifier reads globs, never the disk: classify.go must not import a filesystem
// or subprocess package, or a `rtdd which` in a clean checkout would answer differently
// from one in a dirty tree.
func TestClassifyIsPureStringWork(t *testing.T) {
	a := pyAdapter()
	if !a.IsInstrumentable("src/does/not/exist.py") {
		t.Error("IsInstrumentable must classify a path that does not exist on disk")
	}
	if a.IsTestFile("classify.go") {
		t.Error("IsTestFile must classify by glob, not by what happens to be on disk")
	}
}
