package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
)

// cmdSeed runs the whole suite once, instrumented, and writes a fresh map.
//
// This is the ONLY operation permitted to shrink a row, and therefore the only
// caller of mapstore.Replace in the tree (spec §4, decision D11, audit A4). A failing
// unit records a truncated prefix of its real path, so every other command unions.
//
// The map it writes is built with mapstore.New, never loaded from disk: loading
// would keep rows for tests the suite no longer collects, and a re-seed that
// cannot forget is not a re-seed.
func cmdSeed(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "rtdd: seed takes no arguments, got %v\n", fs.Args())
		return 2
	}

	root, err := findRepoRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 2
	}
	detected, err := detectAdapters(root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 2
	}
	sha, err := gitctx.HeadSHA(root)
	if err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 3
	}

	// One run per detected adapter, each row tagged with the adapter that
	// produced it so no adapter is ever served another's ids (PRD #232 AC6).
	//
	// A run that fails outright still returns here rather than folding a per-adapter code:
	// seed writes the map once, at the end, and a half-written map is worse than none.
	// Decision 5's fold is about `run`, which has a result per adapter to keep.
	m := mapstore.New()
	code := 0
	var failed []string
	for _, ad := range detected {
		fmt.Fprintf(stdout, "seeding with the %s adapter (every unit, instrumented)\n", ad.Name)
		res, err := runner.Seed(ad, root)
		if err != nil {
			return reportRunErr(err)
		}
		for _, u := range res.Failed {
			o := res.Output[u]
			if o != "" && !strings.HasSuffix(o, "\n") {
				o += "\n"
			}
			fmt.Fprintf(stdout, "--- %s ---\n%s", u, o)
		}
		// Every unit errored: nothing was recorded, and a map of error rows would read as
		// seeded. Write nothing (the previous map, if any, stays) and say the environment broke.
		if allErrored(res.Outcomes) {
			fmt.Fprintf(stderr, "rtdd: every %s unit errored; nothing was recorded and the map was not written\n", ad.Name)
			return 3
		}
		for _, row := range rowsFrom(res, sha, ad.Name) {
			m.Replace(row) // seed only; see the doc comment above
		}
		failed = append(failed, res.Failed...)
		// Worst wins, never last: an adapter whose suite failed must not be reported as
		// success because the next one happened to pass.
		if res.ExitCode > code {
			code = res.ExitCode
		}
	}
	if err := os.MkdirAll(rtddDir(root), 0o755); err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 3
	}
	if err := m.Save(mapPath(root)); err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 3
	}
	// `adapters` records the whole detected set, `adapter` the first of them. Every row
	// this seed wrote carries its own tag, so the singular is only ever consulted for
	// untagged rows.
	if err := writeMeta(root, meta{V: mapstore.MapVersion, Adapter: coverageAdapterName(detected), Adapters: detectedSet(detected),
		SeededAt: sha, Cycles: 0}); err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 3
	}

	fmt.Fprintf(stdout, "seeded %d tests at %s\n", m.Len(), sha)
	if len(failed) > 0 {
		fmt.Fprintf(stdout, "%d failed during seeding: %v\n", len(failed), failed)
	}
	return code
}

// allErrored reports whether an adapter's units all errored — none passed, failed or
// skipped. Zero units is not that.
func allErrored(outcomes []runner.Outcome) bool {
	for _, o := range outcomes {
		if o.Status != "error" {
			return false
		}
	}
	return len(outcomes) > 0
}
