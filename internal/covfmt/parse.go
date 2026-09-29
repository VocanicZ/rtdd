// Package covfmt reads the coverage files stock tools write — lcov, cobertura, Go's
// coverprofile and JaCoCo XML — into "path -> lines hit", and resolves the paths those
// tools report to repo-relative ones. It knows nothing about tests: one file is one unit's
// coverage, because every unit runs in its own process (spec §4).
package covfmt

import (
	"fmt"
	"io"
	"sort"
)

// Lines maps a path, as the coverage tool spelled it, to the lines it hit (count > 0).
type Lines map[string][]int

var Formats = []string{"lcov", "cobertura", "gocover", "jacoco"}

// Report is one coverage file: Hit is the lines executed (count > 0), Exec every line the
// tool measured, hit or not — the executable lines. Hit is a subset of Exec.
type Report struct {
	Hit, Exec Lines
}

func newReport() Report { return Report{Hit: Lines{}, Exec: Lines{}} }

// add records one measured line, as hit when count > 0.
func (r Report) add(path string, line int, hit bool) {
	r.Exec.add(path, line)
	if hit {
		r.Hit.add(path, line)
	}
}

// Parse returns the hit lines of a coverage file.
func Parse(format string, r io.Reader) (Lines, error) {
	rep, err := ParseReport(format, r)
	return rep.Hit, err
}

// ParseReport returns a coverage file's hit and executable lines.
func ParseReport(format string, r io.Reader) (Report, error) {
	var (
		out Report
		err error
	)
	switch format {
	case "lcov":
		out, err = parseLcov(r)
	case "gocover":
		out, err = parseGocover(r)
	case "cobertura":
		out, err = parseCobertura(r)
	case "jacoco":
		out, err = parseJacoco(r)
	default:
		return Report{}, fmt.Errorf("covfmt: unknown format %q (only %v)", format, Formats)
	}
	if err != nil {
		return Report{}, fmt.Errorf("covfmt: %s: %w", format, err)
	}
	for _, l := range []Lines{out.Hit, out.Exec} {
		for p, ls := range l {
			l[p] = sortedUnique(ls)
		}
	}
	return out, nil
}

func (l Lines) add(path string, lines ...int) {
	l[path] = append(l[path], lines...)
}

func sortedUnique(ls []int) []int {
	sort.Ints(ls)
	out := ls[:0]
	for i, v := range ls {
		if i == 0 || v != ls[i-1] {
			out = append(out, v)
		}
	}
	return out
}
