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
		// none of the three. The shipped adapter scopes source the same way, so it
		// classifies a conftest under tests/ identically.
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

// TestShippedAdapterClassifiesConftest pins the side effect of IsTestFile's FullEscalate
// exclusion against the adapter that actually ships. The exclusion keeps conftest.py out
// of the selector set — naming it is a fatal exit 5 — but IsInstrumentable is SourceGlobs
// AND not-test AND not-opaque, and escalation is none of those three. So a conftest.py
// that sits inside SourceGlobs is instrumentable, where the unqualified predicate would
// have made it a test file and therefore not instrumentable. Documented in
// docs/plans/00-interfaces.md; pinned here so the narrowing cannot drift silently.
func TestShippedAdapterClassifiesConftest(t *testing.T) {
	a := builtinPython(t)
	cases := []struct {
		rel                                    string
		test, opaque, escalate, instrumentable bool
	}{
		// source_globs is **/*.py, so every conftest.py is inside it: each escalates, is
		// not a selector, and stays instrumentable.
		{"src/conftest.py", false, false, true, true},
		{"tests/conftest.py", false, false, true, true},
		{"conftest.py", false, false, true, true},
		// The exclusion is exactly conftest-shaped; a real test module is unaffected.
		{"tests/test_a.py", true, false, false, false},
		{"src/logic.py", false, false, false, true},
	}
	checkClassification(t, a, cases)
}

// The hand-built table above proves the classifier and nothing about what the binary
// actually ships. This case classifies through Builtin() — the embedded
// adapters/python.yaml — so a glob dropped from the shipped file fails here. The direct
// tier in internal/selector is built from IsTestFile alone, so a missing test glob is a
// silent narrowing: editing a test selects nothing at all.
func TestBuiltinPythonClassifiesTheShippedGlobs(t *testing.T) {
	a := builtinPython(t)
	cases := []struct {
		rel                                    string
		test, opaque, escalate, instrumentable bool
	}{
		// pytest's default python_files is "test_*.py *_test.py" — both spellings are
		// tests, or a *_test.py repo gets an empty direct tier.
		{"pkg/foo_test.py", true, false, false, false},
		{"pkg/test_foo.py", true, false, false, false},
		{"tests/helpers.py", true, false, false, false},
		// Flat layout: the package sits at the repo root, not under src/. Scoping
		// source_globs to src/** would classify this as nothing at all.
		{"pkg/mod.py", false, false, false, true},
		{"src/logic.py", false, false, false, true},
		// setup.cfg and pytest.ini are detection markers: editing what configures the
		// test runner must escalate to a full run.
		{"setup.cfg", false, false, true, false},
		{"pytest.ini", false, false, true, false},
		{"tox.ini", false, false, true, false},
		{"poetry.lock", false, false, true, false},
		{"uv.lock", false, false, true, false},
		{"pyproject.toml", false, false, true, false},
		{"requirements.txt", false, false, true, false},
		// A JSON data file outside fixtures/ still has to produce a signal.
		{"data/x.json", false, true, false, false},
		{"config/app.yml", false, true, false, false},
		{"tpl/page.j2", false, true, false, false},
		{"README.md", false, false, false, false},
	}
	checkClassification(t, a, cases)
}

// builtinPython returns the shipped python adapter, read through the embedded FS.
func builtinPython(t *testing.T) *Adapter {
	t.Helper()
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	for _, cand := range all {
		if cand.Name == "python" {
			return cand
		}
	}
	t.Fatal("Builtin() has no adapter named python")
	return nil
}

// checkClassification runs all four predicates over a table of expectations.
func checkClassification(t *testing.T, a *Adapter, cases []struct {
	rel                                    string
	test, opaque, escalate, instrumentable bool
}) {
	t.Helper()
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
