package main

import (
	"flag"
	"fmt"
	"os"

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
func cmdSeed(args []string) int {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "rtdd: seed takes no arguments, got %v\n", fs.Args())
		return 2
	}

	root, err := findRepoRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}
	ad, err := detectAdapter(root, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}
	sha, err := gitctx.HeadSHA(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}

	fmt.Printf("seeding with the %s adapter (one full instrumented run)\n", ad.Name)
	res, err := runner.Seed(ad, root)
	if err != nil {
		return reportRunErr(err)
	}

	m := mapstore.New()
	for _, row := range rowsFrom(res, sha) {
		m.Replace(row) // seed only; see the doc comment above
	}
	if err := os.MkdirAll(rtddDir(root), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	if err := m.Save(mapPath(root)); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	if err := writeMeta(root, meta{V: 1, Adapter: ad.Name, SeededAt: sha, Cycles: 0}); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}

	fmt.Printf("seeded %d tests at %s\n", m.Len(), sha)
	if len(res.Failed) > 0 {
		fmt.Printf("%d failed during seeding: %v\n", len(res.Failed), res.Failed)
	}
	return res.ExitCode
}
