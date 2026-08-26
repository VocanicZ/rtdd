package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// cmdRun selects, executes, refreshes the map, and reports.
//
// It exits nonzero ONLY when a test failed. An empty selection is exit 0 and is
// reported explicitly, so it can never read as "all passed" (spec §5).
//
// `f` is UNIONED, never replaced (spec §4, decision D11, audit A4): a subset run
// legitimately records less coverage than the seed, because import-time and
// first-caller-wins lines migrate to whichever test ran first in that subset, and
// a failing test records a truncated prefix of its real path. Only rtdd seed may
// shrink a row, and TestOnlySeedCallsMapstoreReplace enforces that this file never
// names Replace.
func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	base := fs.String("base", "HEAD", "diff base ref for the changed set")
	failFast := fs.Bool("fail-fast", false, "stop at the first failure (opt-in only)")
	asJSON := fs.Bool("json", false, "machine-readable output (schema lands in M2)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "rtdd: run takes no positional arguments, got %v\n", fs.Args())
		return 2
	}
	_ = asJSON // M2 defines the schema; the flag is accepted now so front-ends can bind to it.

	root, err := findRepoRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}
	ad, err := detectAdapter(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}

	older := gitctx.Older(root)
	// LoadWith, not Load: duplicate `t` lines left by the union merge driver are
	// resolved with real commit ages rather than by line order.
	m, err := mapstore.LoadWith(mapPath(root), older)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}
	mt, err := readMeta(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}
	changes, err := gitctx.ChangedSet(root, *base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}

	merge, _ := gitctx.IsMergeCommit(root, "HEAD")
	choose := func(allTests []string) selector.Selection {
		return selector.Select(selector.Inputs{
			Map:      m,
			Changes:  changes,
			Adapter:  ad,
			Cfg:      selector.DefaultConfig(),
			AllTests: allTests,
			Distance: func(sha string) int {
				d, derr := gitctx.CommitDistance(root, sha)
				if derr != nil {
					return -1 // unknown, never "fresh"
				}
				return d
			},
			Cycles:     mt.Cycles,
			Merge:      merge,
			ImportOnly: func(string) []string { return nil }, // static import fallback lands in M2
		})
	}

	// Enumerate the suite ONLY for T2. runner.List costs a full pytest collection,
	// and T2 is the one tier whose test list is the whole suite — AllTests reaches
	// no other branch of Select. Paying it on every T0 run would put a collection
	// on the critical path of the loop this tool exists to make fast.
	sel := choose(nil)
	if sel.Tier == selector.TierT2 {
		all, listErr := runner.List(ad, root)
		if listErr != nil {
			return reportRunErr(listErr)
		}
		sel = choose(all)
	}

	fmt.Printf("tier %s: %d selected", sel.Tier, len(sel.Tests))
	if sel.Reason != "" {
		fmt.Printf(" (%s)", sel.Reason)
	}
	fmt.Println()

	if sel.Tier == selector.TierEmpty || len(sel.Tests) == 0 {
		fmt.Println("EMPTY SELECTION - nothing ran. This is not a pass.")
		return finishCycle(root, mt, 0)
	}

	res, err := runner.Run(ad, root, sel.Tests, *failFast)
	if err != nil {
		return reportRunErr(err)
	}

	sha, err := gitctx.HeadSHA(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	for _, row := range rowsFrom(res, sha) {
		// UNION, NEVER REPLACE. See the doc comment on cmdRun.
		m.Union(row, older)
	}
	if err := os.MkdirAll(rtddDir(root), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	if err := m.Save(mapPath(root)); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}

	if mt.Adapter == "" {
		mt.Adapter = ad.Name
	}
	if mt.V == 0 {
		mt.V = 1
	}

	fmt.Printf("%d ran, %d failed, %d rows in the map\n", len(res.Outcomes), len(res.Failed), m.Len())
	for _, id := range res.Failed {
		fmt.Printf("FAILED %s\n", id)
	}
	return finishCycle(root, mt, res.ExitCode)
}

// finishCycle increments `cycles` and persists meta.json, then returns code.
//
// It runs on EVERY completed run, pass or fail, and on an empty selection too: the
// counter is what buys the eventual DriftGuard full run, and a streak of red or
// empty runs is exactly when the map is most likely to have drifted. Incrementing
// only on green would postpone that full run for as long as the suite stays broken.
func finishCycle(root string, mt meta, code int) int {
	mt.Cycles++
	if err := writeMeta(root, mt); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	return code
}
