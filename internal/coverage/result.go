package coverage

import "sort"

// TestCoverage is one test's file->lines attribution, with the phase suffix
// already stripped from the id.
type TestCoverage struct {
	Test  string           // normalised id, phase suffix stripped
	Files map[string][]int // repo-relative path -> sorted covered line numbers
}

// Result is everything one .coverage store says.
//
// ImportTime holds the empty-context lines: executed during collection, before
// any dynamic context was set, and therefore attributed to no test. It is a
// distinct class and MUST NOT be reported as uncovered (spec §6, audit A1).
type Result struct {
	PerTest    []TestCoverage
	ImportTime map[string][]int
}

// Merge unions other into r, leaving PerTest sorted by Test.
//
// This exists because pytest ERASES .coverage at the start of every run unless
// --cov-append is passed (measured: chunk 2 left only chunk 2's contexts). RTDD
// reads .coverage after each argv chunk and merges here, rather than relying on
// --cov-append, which would also silently absorb a stale .coverage from an
// unrelated earlier run.
//
// Merge(nil) is a no-op, and merging into a zero-value Result yields other's
// content. Nothing in r ever aliases other's slices or maps.
func (r *Result) Merge(other *Result) {
	if other == nil {
		return
	}
	if r.ImportTime == nil {
		r.ImportTime = make(map[string][]int, len(other.ImportTime))
	}
	for f, lines := range other.ImportTime {
		r.ImportTime[f] = mergeLines(r.ImportTime[f], lines)
	}
	idx := make(map[string]int, len(r.PerTest))
	for i, tc := range r.PerTest {
		idx[tc.Test] = i
	}
	for _, tc := range other.PerTest {
		i, ok := idx[tc.Test]
		if !ok {
			cp := TestCoverage{Test: tc.Test, Files: make(map[string][]int, len(tc.Files))}
			for f, lines := range tc.Files {
				cp.Files[f] = mergeLines(nil, lines)
			}
			r.PerTest = append(r.PerTest, cp)
			idx[tc.Test] = len(r.PerTest) - 1
			continue
		}
		if r.PerTest[i].Files == nil {
			r.PerTest[i].Files = map[string][]int{}
		}
		for f, lines := range tc.Files {
			r.PerTest[i].Files[f] = mergeLines(r.PerTest[i].Files[f], lines)
		}
	}
	sort.Slice(r.PerTest, func(i, j int) bool { return r.PerTest[i].Test < r.PerTest[j].Test })
}

// mergeLines returns the sorted, deduped union of a and b. It never aliases
// either input, so a merged Result is safe to mutate. ReadSQLite reuses it when
// one (file, context) pair yields more than one numbits row.
func mergeLines(a, b []int) []int {
	seen := make(map[int]bool, len(a)+len(b))
	out := make([]int, 0, len(a)+len(b))
	for _, l := range a {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	for _, l := range b {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	sort.Ints(out)
	return out
}
