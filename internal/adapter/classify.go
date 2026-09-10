package adapter

// The four file-classification predicates. All of them are glob work over the adapter's
// declared fields — no filesystem access — and all of them answer false for a nil
// *Adapter rather than panicking, so a caller that has not detected an adapter yet
// degrades to "classifies nothing" instead of crashing.

// IsTestFile reports whether rel is a file the runner may name as a test selector.
//
// FullEscalate wins over TestGlobs: a fixture module such as tests/conftest.py matches
// a broad test glob like tests/**/*.py, but collects no tests. Naming it as a selector
// makes the runner exit 5 (no-tests-collected), which is fatal. A change to it escalates
// to a full run through IsFullEscalate instead.
func (a *Adapter) IsTestFile(rel string) bool {
	return a != nil && matchAny(a.TestGlobs, rel) && !matchAny(a.FullEscalate, rel)
}

// IsOpaque reports whether rel is a file coverage cannot see into (templates, fixtures,
// SQL, config data). Changing one escalates to T1.
func (a *Adapter) IsOpaque(rel string) bool {
	return a != nil && matchAny(a.Opaque, rel)
}

// CanRunPlain reports whether this adapter can execute a selection without recording
// coverage. An adapter that cannot is never asked to: it records every cycle, as before.
func (a *Adapter) CanRunPlain() bool { return a != nil && a.SubsetPlain != "" }

// IsFullEscalate reports whether changing rel forces a full-suite run (T2).
func (a *Adapter) IsFullEscalate(rel string) bool {
	return a != nil && matchAny(a.FullEscalate, rel)
}

// IsInstrumentable reports whether rel is source code coverage can attribute: it matches
// SourceGlobs AND is not a test file AND is not opaque. The AND is what keeps a YAML
// fixture living under src/ out of the coverage scope.
func (a *Adapter) IsInstrumentable(rel string) bool {
	if a == nil {
		return false
	}
	return matchAny(a.SourceGlobs, rel) && !a.IsTestFile(rel) && !a.IsOpaque(rel)
}
