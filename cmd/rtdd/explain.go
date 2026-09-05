package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
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
//
// detected is the DETECTED adapter set, read only by the empty-map branch: a repository
// whose adapters all declare selection: static builds no map at all, so pointing at the
// map — or at `rtdd seed` — describes a setup step that does not exist (issue #279). A nil
// or coverage-only set renders the pre-existing text byte for byte.
func RenderExplain(m *mapstore.Map, path string, detected []*adapter.Adapter) string {
	ids := m.TestsCovering([]string{path})
	var b strings.Builder

	if len(ids) == 0 {
		fmt.Fprintf(&b, "%s is covered by 0 tests.\n", path)
		if m.Len() == 0 {
			b.WriteString(emptyMapExplanation(detected))
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

// emptyMapExplanation is what explain says when the map holds nothing, in the same three
// cases as doctor's fan-out line and init's next step.
//
// The static-only branch does more than drop the seed advice: `explain` was asked which
// tests touch a file, and for a static adapter that question HAS an answer — the
// correspondence the adapter declares, and the importers its own scan reports. Saying
// only "there is no map" leaves the caller with the impression that nothing can answer,
// which is the same lie in the other direction.
func emptyMapExplanation(detected []*adapter.Adapter) string {
	static, coverage := selectionSplit(detected)
	switch {
	case len(static) == 0:
		// Byte-identical to the pre-#279 line.
		return "  The map is empty. Run `rtdd seed` first.\n"
	case len(coverage) == 0:
		out := fmt.Sprintf("  The map is empty, and stays empty. %s.\n", staticNothingRecorded(static))
		// A skipped level is skipped (#275): an adapter that declares no importscan must
		// not be described as consulting imports.
		if ev := staticEvidence(static); ev != "" {
			return out + fmt.Sprintf("  What selects this file is %s — run `rtdd which` to see it.\n", ev)
		}
		return out + "  It declares neither test_for templates nor an importscan, so nothing\n" +
			"  narrower than the full suite can be selected for this file.\n"
	default:
		return fmt.Sprintf("  The map is empty. Run `rtdd seed` first to build it for %s. "+
			"%s,\n  so seeding does not apply to %s.\n",
			seedScope(coverage), staticClause(adapterNames(static)), pronoun(adapterNames(static)))
	}
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

	fmt.Fprint(stdout, RenderExplain(e.m, rel, detectedAdapters(e.root)))
	return 0
}
