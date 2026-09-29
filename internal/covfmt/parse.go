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

func Parse(format string, r io.Reader) (Lines, error) {
	var (
		out Lines
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
		return nil, fmt.Errorf("covfmt: unknown format %q (only %v)", format, Formats)
	}
	if err != nil {
		return nil, fmt.Errorf("covfmt: %s: %w", format, err)
	}
	for p, ls := range out {
		out[p] = sortedUnique(ls)
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
