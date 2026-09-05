package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
	"github.com/VocanicZ/rtdd/internal/selector"
	"github.com/VocanicZ/rtdd/internal/uncovered"
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
	asJSON := fs.Bool("json", false, "machine-readable output (schema v1)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "rtdd: run takes no positional arguments, got %v\n", fs.Args())
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
	// changedSet, not gitctx.ChangedSet: rtdd's own .rtdd/ writes must not select.
	changes, err := changedSet(root, *base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}

	// The static-import fallback, wired exactly as `rtdd which` wires it (newImportFallback
	// in which.go). Without it the advisory command and the executing command disagree:
	// an import-time-only file is covered by no map row, so T0 finds nothing and the
	// selection falls to empty — `which` says "run these tests", `run` runs none.
	//
	// The unmapped set is computed from the map alone, so Cov is nil here: this pre-run
	// signal answers only "which changed files does no row cover", which is precisely the
	// fallback's trigger (spec §6, D14). The post-run BuildSignal below, which classifies
	// against fresh coverage, is a separate call and stays that way.
	//
	// newImportFallback builds no scanner when that set is empty, so the ordinary T0 loop
	// pays no python subprocess at all.
	pre := BuildSignal(SignalInput{
		Changes:          changes,
		IsInstrumentable: ad.IsInstrumentable,
		Map:              m,
	})
	fb := newImportFallback(root, m, pre.UnmappedFiles)

	merge, _ := gitctx.IsMergeCommit(root, "HEAD")
	// The static tier's two resolvers, wired exactly as `which` wires them (staticResolvers
	// in static.go). A resolver `run` did not supply is a level skipped, so an unwired
	// `run` would execute the full suite in a repository `which` had narrowed to one test.
	exists, importDistance, staticScanErr := staticResolvers(root, ad)
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
			Cycles:         mt.Cycles,
			Merge:          merge,
			ImportOnly:     fb.testsImporting,
			Exists:         exists,
			ImportDistance: importDistance,
		})
	}

	// Enumerate the suite ONLY for T2. runner.List costs a full pytest collection,
	// and T2 is the one tier whose test list is the whole suite — AllTests reaches
	// no other branch of Select. Paying it on every T0 run would put a collection
	// on the critical path of the loop this tool exists to make fast.
	sel := choose(nil)
	// suiteEnumerated is what makes a T2 selection COMPLETE: `selection.tests` is then the
	// whole suite rather than the map rows rtdd happened to know. It reaches the document
	// as `complete`, so a consumer never has to guess whether the list is the run.
	suiteEnumerated := false
	if sel.Tier == selector.TierT2 {
		all, listErr := runner.List(ad, root)
		if listErr != nil {
			return reportRunErr(listErr)
		}
		sel = choose(all)
		suiteEnumerated = true
	}

	// Under --json the document is the WHOLE of stdout: a consumer pipes it straight
	// into a parser, and a human-readable tier line ahead of it is a syntax error. The
	// same facts are in the document as `tier`, `selection` and `run`.
	if !*asJSON {
		fmt.Printf("tier %s: %d selected", sel.Tier, len(sel.Tests))
		if sel.Reason != "" {
			fmt.Printf(" (%s)", sel.Reason)
		}
		fmt.Println()
	}

	// What the fallback actually produced, per file — `selection.import_fallback`. The map
	// is always non-nil, so the key is present even when nothing fired and a front-end can
	// bind to it unconditionally.
	importFallback := fb.fired

	// A failed scan DEGRADES selection; it never fails the command (internal/importscan:
	// Scanner.Err). It stays on stderr for a human — that keeps --json's stdout a single
	// document — and reaches the document itself as a `warnings` entry, because a --json
	// consumer normally discards stderr and would otherwise never learn the selection was
	// narrowed.
	scanErr := fb.err()
	if scanErr != nil {
		fmt.Fprintf(os.Stderr, "rtdd run: %s\n", importScanNote(scanErr))
	}

	// The declared scanner's failure, if any, is read AFTER selection: the scan runs inside
	// Select, through the resolver above.
	if err := staticScanErr(); err != nil {
		fmt.Fprintf(os.Stderr, "rtdd run: %s\n", adapterImportScanNote(ad.Name, err))
	}

	warnings := runNotes(sel, scanErr, adapterScanWarning(ad, staticScanErr()))

	if sel.Tier == selector.TierEmpty || len(sel.Tests) == 0 {
		if *asJSON {
			// Nothing executed, so there is no fresh coverage and therefore no honest
			// uncovered report: UncoveredOK stays false and `files` is omitted rather
			// than sent as an empty list a consumer would read as "nothing uncovered".
			// `pre` is exactly that Cov-less signal, already computed for the fallback.
			out := BuildOutput(OutputInput{
				Command:         "run",
				Base:            *base,
				Adapter:         ad.Name,
				Sel:             sel,
				Changes:         changes,
				Instrumentable:  pre.Instrumentable,
				UnmappedFiles:   pre.UnmappedFiles,
				ImportFallback:  importFallback,
				SuiteEnumerated: suiteEnumerated,
				Warnings:        warnings,
			})
			if err := emitJSON(out); err != nil {
				return 2
			}
			return finishCycle(root, mt, 0)
		}
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

	// Populate the changed line ranges from git diff --unified=0. WithLines is
	// authoritative and overwrites Lines, so hunk parsing is the single source of truth.
	rawDiff, err := gitctx.RawDiff(root, *base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	changes, err = uncovered.WithLines(root, changes, rawDiff)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}

	// Classify against the coverage THIS run just produced — never against map.jsonl,
	// which carries no line data at all and could therefore only ever answer at whole-file
	// granularity, on numbers that drifted the moment the file was edited (spec §4, §6).
	sig := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              res.Coverage,
		IsInstrumentable: ad.IsInstrumentable,
		Map:              m,
	})

	// A non-empty uncovered report NEVER moves the exit code: RTDD reports, it does not
	// gate (spec §2 non-goals, §6, decision D3). res.ExitCode is still consulted so a
	// runner-level failure the report log did not name cannot be swallowed.
	code := ExitCodeFor(res.Outcomes, sig.Reports)
	if code == 0 && res.ExitCode != 0 {
		code = res.ExitCode
	}

	if *asJSON {
		out := BuildOutput(OutputInput{
			Command:         "run",
			Base:            *base,
			Adapter:         ad.Name,
			Sel:             sel,
			Changes:         changes,
			Instrumentable:  sig.Instrumentable,
			Executed:        true,
			Outcomes:        res.Outcomes,
			Reports:         sig.Reports,
			UncoveredOK:     true,
			UnmappedFiles:   sig.UnmappedFiles,
			ImportFallback:  importFallback,
			SuiteEnumerated: suiteEnumerated,
			Warnings:        warnings,
		})
		out.ExitCode = code
		if err := emitJSON(out); err != nil {
			return 2
		}
		return finishCycle(root, mt, code)
	}

	fmt.Printf("%d ran, %d failed, %d rows in the map\n", len(res.Outcomes), len(res.Failed), m.Len())
	for _, id := range res.Failed {
		fmt.Printf("FAILED %s\n", id)
	}
	if s := RenderUncovered(sig.Reports); s != "" {
		fmt.Fprint(os.Stdout, "\n"+s)
	}
	return finishCycle(root, mt, code)
}

// runNotes are the caveats that say this run is narrower, or less authoritative, than it
// looks — the `run` counterpart of whichNotes. They reach the document as `warnings`.
//
// Under --json the "EMPTY SELECTION - nothing ran" line is never printed: the text branch
// is skipped entirely. Without this warning the document an agent front-end reads is
// indistinguishable from a green run of a real subset, which is precisely the silent
// narrowing the contract forbids.
func runNotes(sel selector.Selection, scanErr error, extra ...string) []string {
	var out []string
	if sel.Tier == selector.TierEmpty || len(sel.Tests) == 0 {
		out = append(out, "an empty selection is not a pass. Nothing was checked.")
	}
	if scanErr != nil {
		out = append(out, importScanNote(scanErr))
	}
	for _, e := range extra {
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

// adapterScanWarning is the declared scanner's failure as a document warning, or "" when
// there was none. Under --json a consumer discards stderr, so a degradation that lives
// only there is one the agent front-end never learns about.
func adapterScanWarning(ad *adapter.Adapter, err error) string {
	if err == nil || ad == nil {
		return ""
	}
	return adapterImportScanNote(ad.Name, err)
}

// importScanNote is the one wording for a failed static import scan, shared by `run` and
// `which` so the two commands never describe the same degradation differently.
func importScanNote(err error) string {
	return fmt.Sprintf("the static import scan failed, so an import-time-only file "+
		"may be under-selected: %v", err)
}

// emitJSON writes the schema document to stdout, indented, as the whole of stdout.
func emitJSON(out Output) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return err
	}
	return nil
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
