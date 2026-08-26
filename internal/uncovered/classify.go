package uncovered

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// Class is the three-way classification of a changed line.
type Class int

const (
	// Covered — an executing test touched this line.
	Covered Class = iota
	// Uncovered — no test executed it. The real signal.
	Uncovered
	// ImportTime — executed during collection/import, attributed to no test.
	// NEVER counted as Uncovered. See spec §6 and audit finding A1.
	ImportTime
)

// String returns "covered" | "uncovered" | "import-time".
func (c Class) String() string {
	switch c {
	case Covered:
		return "covered"
	case Uncovered:
		return "uncovered"
	case ImportTime:
		return "import-time"
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
	Files           int
	CoveredLines    int
	UncoveredLines  int
	ImportTimeLines int
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
			case ImportTime:
				s.ImportTimeLines += n
			}
		}
	}
	return s
}

// Classify intersects each Change's line ranges with FRESH post-run coverage.
//
// A line covered by any test is Covered. A line present only in Result.ImportTime is
// ImportTime and MUST NOT be reported as Uncovered. Everything else is Uncovered.
//
// One exception, and it is the reason audit finding A1 exists: a file that coverage
// measured but that NO test context touches is an import-time-only file (spec §6, D14).
// Its whole module body runs during collection, and coverage stores only executed lines
// — it cannot tell a blank line, a comment or a decorator continuation apart from a dead
// statement. Fixture F1's src/constants.py is exactly this: a constants/Enum/@dataclass
// module asserted on by two passing tests whose import-time lines are 1,2,4,7,8,9,12-15,
// the gaps being blank lines. Classifying those gaps Uncovered would scream UNCOVERED at
// a correctly tested dataclass module, which is the false positive RTDD exists to avoid,
// so every changed line of an import-time-only file is ImportTime. The cost is a real
// dead function added to such a module reading clean; selection for these files does not
// go through the coverage relation at all but through the static import scan.
//
// cov must be the coverage produced by the run that just finished. Line data is never
// persisted in map.jsonl, so there is no line-drift problem: both sides are current.
//
// Callers must pass only instrumentable changes; a file absent from cov entirely
// classifies wholly Uncovered, which is correct for a new source file and wrong for a
// test file or an opaque asset.
func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport {
	covered := map[string]map[int]bool{}
	importTime := map[string]map[int]bool{}

	if cov != nil {
		for _, tc := range cov.PerTest {
			for path, lines := range tc.Files {
				if len(lines) == 0 {
					continue
				}
				set := covered[path]
				if set == nil {
					set = map[int]bool{}
					covered[path] = set
				}
				for _, ln := range lines {
					set[ln] = true
				}
			}
		}
		for path, lines := range cov.ImportTime {
			set := importTime[path]
			if set == nil {
				set = map[int]bool{}
				importTime[path] = set
			}
			for _, ln := range lines {
				set[ln] = true
			}
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
		cSet, iSet := covered[ch.Path], importTime[ch.Path]
		importOnly := len(cSet) == 0 && len(iSet) > 0
		ranges := coalesce(lines, func(ln int) Class {
			switch {
			case cSet[ln]:
				return Covered
			case iSet[ln] || importOnly:
				return ImportTime
			default:
				return Uncovered
			}
		})
		out = append(out, FileReport{Path: ch.Path, Ranges: ranges})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
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
