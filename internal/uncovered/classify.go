package uncovered

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// Class is the two-way classification of a changed line (spec §8).
type Class int

const (
	// Covered — an executing test touched this line.
	Covered Class = iota
	// Uncovered — no test executed it. The real signal.
	Uncovered
)

// String returns "covered" | "uncovered".
func (c Class) String() string {
	switch c {
	case Covered:
		return "covered"
	case Uncovered:
		return "uncovered"
	default:
		return "unknown"
	}
}

// ClassifiedRange is a maximal run of consecutive changed lines sharing one Class.
type ClassifiedRange struct {
	Range gitctx.LineRange
	Class Class
}

// FileReport is the classification of one changed file's changed lines.
type FileReport struct {
	Path   string
	Ranges []ClassifiedRange
}

// UncoveredLines counts the changed lines classified Uncovered.
func (r FileReport) UncoveredLines() int {
	n := 0
	for _, cr := range r.Ranges {
		if cr.Class == Uncovered {
			n += cr.Range.End - cr.Range.Start + 1
		}
	}
	return n
}

// Summary aggregates a set of FileReports for the --json output and the text report.
type Summary struct {
	Files          int
	CoveredLines   int
	UncoveredLines int
}

// Summarize totals the reports.
func Summarize(reports []FileReport) Summary {
	s := Summary{Files: len(reports)}
	for _, r := range reports {
		for _, cr := range r.Ranges {
			n := cr.Range.End - cr.Range.Start + 1
			switch cr.Class {
			case Covered:
				s.CoveredLines += n
			case Uncovered:
				s.UncoveredLines += n
			}
		}
	}
	return s
}

// Classify intersects each Change's line ranges with FRESH post-run coverage. A line
// some unit executed is Covered; everything else is Uncovered. In an isolated run a line
// executed while importing is executed by that unit, so there is no third class (spec §8).
//
// cov must be the coverage produced by the run that just finished. Line data is never
// persisted in map.jsonl, so there is no line-drift problem: both sides are current.
//
// A file some unit MEASURED (its tool reported executable lines for it) reports only its
// executable changed lines: a changed comment, blank or closing brace is not code and is
// neither class. A file no unit measured keeps the whole-changed-set rule.
//
// Callers must pass only instrumentable changes; a file absent from cov entirely
// classifies wholly Uncovered, which is correct for a new source file and wrong for a
// test file or an opaque asset.
func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport {
	covered := map[string]map[int]bool{}
	executable := map[string]map[int]bool{}
	if cov != nil {
		for _, tc := range cov.PerTest {
			addLines(covered, tc.Files)
			addLines(executable, tc.Exec)
		}
	}

	var out []FileReport
	for _, ch := range changes {
		if ch.Status == gitctx.Deleted || len(ch.Lines) == 0 {
			continue
		}
		lines := expandLines(ch.Lines)
		if len(lines) == 0 {
			continue
		}
		cSet, eSet := covered[ch.Path], executable[ch.Path]
		if eSet != nil {
			code := lines[:0:0]
			for _, ln := range lines {
				if cSet[ln] || eSet[ln] {
					code = append(code, ln)
				}
			}
			if lines = code; len(lines) == 0 {
				continue
			}
		}
		ranges := coalesce(lines, func(ln int) Class {
			if cSet[ln] {
				return Covered
			}
			return Uncovered
		})
		out = append(out, FileReport{Path: ch.Path, Ranges: ranges})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// addLines merges path -> lines into sets.
func addLines(sets map[string]map[int]bool, files map[string][]int) {
	for path, lines := range files {
		set := sets[path]
		if set == nil {
			set = map[int]bool{}
			sets[path] = set
		}
		for _, ln := range lines {
			set[ln] = true
		}
	}
}

// expandLines flattens ranges into a sorted, deduplicated line list.
func expandLines(rs []gitctx.LineRange) []int {
	seen := map[int]bool{}
	var out []int
	for _, r := range rs {
		for ln := r.Start; ln <= r.End; ln++ {
			if ln < 1 || seen[ln] {
				continue
			}
			seen[ln] = true
			out = append(out, ln)
		}
	}
	sort.Ints(out)
	return out
}

// coalesce merges consecutive lines of the same Class into maximal ranges.
func coalesce(lines []int, classOf func(int) Class) []ClassifiedRange {
	var out []ClassifiedRange
	for _, ln := range lines {
		c := classOf(ln)
		if n := len(out); n > 0 && out[n-1].Class == c && out[n-1].Range.End == ln-1 {
			out[n-1].Range.End = ln
			continue
		}
		out = append(out, ClassifiedRange{Range: gitctx.LineRange{Start: ln, End: ln}, Class: c})
	}
	return out
}
