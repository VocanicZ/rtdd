// Package coverage holds what a run recorded: each unit's file -> hit lines.
package coverage

// TestCoverage is one unit's file->lines attribution.
type TestCoverage struct {
	Test  string           // the unit (test file), repo-relative
	Files map[string][]int // repo-relative path -> sorted covered line numbers
}

// Result is everything one run recorded, one entry per unit.
type Result struct {
	PerTest []TestCoverage
}
