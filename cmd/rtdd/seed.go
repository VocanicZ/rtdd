package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
)

// cmdSeed runs the whole suite once, instrumented, and writes a fresh map.
//
// This is the ONLY operation permitted to shrink a row, and therefore the only
// caller of mapstore.Replace in the tree (spec §4, decision D11, audit A4). A
// subset run legitimately records LESS coverage than a seed — import-time and
// first-caller-wins lines migrate to whichever test ran first, and a failing test
// records a truncated prefix of its real path — so every other command unions.
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
	// TODO(#320): seeding a polyglot repository seeds every coverage adapter in the
	// detected set; until Task 11 lands it seeds the first, exactly as it did when
	// detection could only ever return one. The set is already recorded in meta.json
	// below, so the map it writes says which adapter produced every row.
	ad := detected[0]
	// Seeding is advice that only applies to an adapter that records coverage. A
	// selection: static adapter declares coverage: none, so there is no map to build and
	// nothing this command could do. Exiting 0 here would be the worse failure: it leaves
	// the caller believing a map exists, and every later command would be read against a
	// map that was never written. Exit 2 — the adapter's declaration is what makes the
	// request impossible, which is a configuration error.
	if msg := staticSeedRefusal(ad); msg != "" {
		fmt.Fprint(stderr, msg)
		return 2
	}

	sha, err := gitctx.HeadSHA(root)
	if err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 3
	}

	fmt.Fprintf(stdout, "seeding with the %s adapter (one full instrumented run)\n", ad.Name)
	res, err := runner.Seed(ad, root)
	if err != nil {
		return reportRunErr(err)
	}

	m := mapstore.New()
	for _, row := range rowsFrom(res, sha, ad.Name) {
		m.Replace(row) // seed only; see the doc comment above
	}
	if err := os.MkdirAll(rtddDir(root), 0o755); err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 3
	}
	if err := m.Save(mapPath(root)); err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 3
	}
	// `adapters` records the whole detected set, `adapter` the coverage adapter that
	// produced this map (decision 4). Both are written: the plural is what a polyglot
	// repository's commands read, and the singular is what says whose the untagged rows
	// of a map seeded by an older rtdd are.
	if err := writeMeta(root, meta{V: 1, Adapter: ad.Name, Adapters: detectedSet(detected),
		SeededAt: sha, Cycles: 0}); err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 3
	}

	fmt.Fprintf(stdout, "seeded %d tests at %s\n", m.Len(), sha)
	if len(res.Failed) > 0 {
		fmt.Fprintf(stdout, "%d failed during seeding: %v\n", len(res.Failed), res.Failed)
	}
	return res.ExitCode
}

// staticSeedRefusal is why `rtdd seed` cannot run against ad, or "" when ad records
// coverage and seeding is a real operation.
//
// It names the adapter: a repository may resolve one of several, and "seeding is
// unsupported here" is unusable to someone who does not know which declaration is being
// talked about. It also names the command that DOES answer for a static adapter, because
// a refusal with no alternative reads as "this toolchain is unsupported, full stop".
func staticSeedRefusal(ad *adapter.Adapter) string {
	if ad == nil || ad.Selection != adapter.SelectionStatic {
		return ""
	}
	return fmt.Sprintf("rtdd seed: the %s adapter declares selection: static, so it records nothing "+
		"and there is no map to build.\n"+
		"  Nothing was written; seeding applies only to an adapter that records coverage.\n"+
		"  Run `rtdd which` instead: it selects from declared test_for correspondence and imports.\n",
		ad.Name)
}
