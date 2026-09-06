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
	// Decision 6: a mixed repository seeds its coverage half and NAMES its static half.
	// The exit-2 refusal is for a repository whose coverage half is empty — refusing here
	// would strand the map the Python half of a polyglot repository genuinely needs.
	//
	// Exiting 0 having built no map is the worse failure of the two: it leaves the caller
	// believing a map exists, and every later command is then read against a map that was
	// never written. So an empty coverage half is exit 2 — the adapters' own declarations
	// are what make the request impossible, which is a configuration error.
	plan, msg := seedPlan(detected)
	if len(plan) == 0 {
		fmt.Fprint(stderr, msg)
		return 2
	}
	// The static half's sentence goes to stdout beside the work, not to stderr: nothing
	// went wrong, and the reader needs it to know why the map holds no vitest row.
	if msg != "" {
		fmt.Fprint(stdout, msg)
	}

	sha, err := gitctx.HeadSHA(root)
	if err != nil {
		fmt.Fprintln(stderr, "rtdd:", err)
		return 3
	}

	// One instrumented run per coverage adapter, each row tagged with the adapter that
	// produced it so no adapter is ever served another's ids (PRD #232 AC6).
	//
	// A run that fails outright still returns here rather than folding a per-adapter code:
	// seed writes the map once, at the end, and a half-written map is worse than none.
	// Decision 5's fold is about `run`, which has a result per adapter to keep.
	m := mapstore.New()
	code := 0
	var failed []string
	for _, ad := range plan {
		fmt.Fprintf(stdout, "seeding with the %s adapter (one full instrumented run)\n", ad.Name)
		res, err := runner.Seed(ad, root)
		if err != nil {
			return reportRunErr(err)
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
	// `adapters` records the whole detected set, `adapter` the coverage adapter that
	// produced this map (decision 4). Both are written: the plural is what a polyglot
	// repository's commands read, and the singular is what says whose the untagged rows
	// of a map seeded by an older rtdd are. With several coverage adapters the singular
	// names the first — every row this seed wrote carries its own tag, so the singular is
	// only ever consulted for rows an older binary left behind.
	if err := writeMeta(root, meta{V: 1, Adapter: coverageAdapterName(detected), Adapters: detectedSet(detected),
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
	// One wording, rendered in one place: staticSeedRefusalFor answers for the whole
	// static set, and a second copy of the sentence here is a copy that drifts.
	return staticSeedRefusalFor([]*adapter.Adapter{ad})
}
