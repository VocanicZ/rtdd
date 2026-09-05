package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/paths"
)

// explainIDColumn is the minimum width of the test-id column. A longer id widens the
// column for the whole listing rather than pushing its own duration out of line.
const explainIDColumn = 25

// RenderExplain lists the tests covering path, ascending by duration then id.
//
// A file with zero covering tests is the import-time-only case as often as it is the
// untested case, and the output says so rather than implying the file is untested.
func RenderExplain(m *mapstore.Map, path string) string {
	ids := m.TestsCovering([]string{path})
	var b strings.Builder

	if len(ids) == 0 {
		fmt.Fprintf(&b, "%s is covered by 0 tests.\n", path)
		if m.Len() == 0 {
			b.WriteString("  The map is empty. Run `rtdd seed` first.\n")
			return b.String()
		}
		b.WriteString("  No map row lists this file. Either nothing exercises it, or it only ever\n")
		b.WriteString("  executes at import time, where coverage attributes it to no test at all\n")
		b.WriteString("  (spec §6). Selection falls back to a static import scan for this file.\n")
		return b.String()
	}

	rows := make([]mapstore.Row, 0, len(ids))
	width := explainIDColumn
	for _, id := range ids {
		r, ok := m.Get(id)
		if !ok {
			continue
		}
		rows = append(rows, r)
		if len(r.T) > width {
			width = len(r.T)
		}
	}
	// Cheapest first: the listing doubles as a run order, and a fast covering test is
	// the one an agent should reach for.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].D != rows[j].D {
			return rows[i].D < rows[j].D
		}
		return rows[i].T < rows[j].T
	})

	fmt.Fprintf(&b, "%s is covered by %d %s:\n", path, len(rows), plural(len(rows), "test", "tests"))
	for _, r := range rows {
		fmt.Fprintf(&b, "    %-*s%4dms  %s\n", width, r.T, r.D, r.S)
	}
	return b.String()
}

// plural picks the noun form for n.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// cmdExplain implements `rtdd explain <file>`: which tests cover this path, and what it
// means when none do. It runs no tests, so its only non-zero exits are usage and
// environment errors.
func cmdExplain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: rtdd explain <file>")
		return 2
	}
	arg := fs.Arg(0)

	e, code, err := loadEnv("", stderr)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd explain: %v\n", err)
		return code
	}

	// The argument is relative to the caller's working directory, not to the repo root;
	// internal/paths is the only place either becomes a map key.
	abs := arg
	if !filepath.IsAbs(abs) {
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			fmt.Fprintf(stderr, "rtdd explain: %v\n", wdErr)
			return 3
		}
		abs = filepath.Join(wd, abs)
	}
	rel, ok := paths.Normalize(e.root, abs)
	if !ok {
		fmt.Fprintf(stderr, "rtdd explain: %q is outside the repository\n", arg)
		return 2
	}

	fmt.Fprint(stdout, RenderExplain(e.m, rel))
	return 0
}
