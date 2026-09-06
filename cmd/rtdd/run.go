package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/runner"
	"github.com/VocanicZ/rtdd/internal/selector"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// cmdRun selects, executes, refreshes the map, and reports.
//
// It exits nonzero when a test failed, and when an adapter never got as far as running —
// a subset or an enumeration that could not start is an environment failure, not a pass.
// A GENUINELY empty selection, one no failure caused, is exit 0 and is reported
// explicitly, so it can never read as "all passed" (spec §5).
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
	ads, err := detectAdapters(root, os.Stderr)
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

	merge, _ := gitctx.IsMergeCommit(root, "HEAD")

	// One selection per detected adapter (spec §4.4), through the same selectPerAdapter
	// `rtdd which` calls: the advisory command and the executing command disagreeing
	// about one tree is a defect this package has already shipped once.
	//
	// Enumerate is supplied here and nowhere else: enumerating costs a full collection —
	// and, for a junit-xml adapter, a full suite RUN, because that adapter's ids live in
	// report_path and only a run writes one — and T2 is the one tier whose test list IS
	// the whole suite, so paying it on every T0 run would put a collection on the critical
	// path of the loop this tool exists to make fast. What that run produced is carried on
	// the block and reused below rather than paid for twice.
	blocks, err := selectPerAdapter(root, ads, m, mt, selectionContext{
		Changes: changes,
		Cfg:     selector.DefaultConfig(),
		Cycles:  mt.Cycles,
		Merge:   merge,
		Distance: func(sha string) int {
			d, derr := gitctx.CommitDistance(root, sha)
			if derr != nil {
				return -1 // unknown, never "fresh"
			}
			return d
		},
		Enumerate: func(ad *adapter.Adapter) (suiteRun, error) {
			res, tests, err := runner.ListRun(ad, root)
			return suiteRun{Tests: tests, Result: res}, err
		},
	})
	if err != nil {
		return reportRunErr(err)
	}

	sel, pre, importFallback, suiteEnumerated := foldBlocks(blocks)

	// Under --json the document is the WHOLE of stdout: a consumer pipes it straight
	// into a parser, and a human-readable tier line ahead of it is a syntax error. The
	// same facts are in the document as `tier`, `selection` and `run`.
	if !*asJSON {
		fmt.Print(renderRunTiers(blocks))
	}

	// A failed scan DEGRADES selection; it never fails the command (internal/importscan:
	// Scanner.Err). It stays on stderr for a human — that keeps --json's stdout a single
	// document — and reaches the document itself as a `warnings` entry, because a --json
	// consumer normally discards stderr and would otherwise never learn the selection was
	// narrowed.
	var warnings []string
	for _, blk := range blocks {
		if blk.FallbackErr != nil {
			fmt.Fprintf(os.Stderr, "rtdd run: %s\n", importScanNote(blk.FallbackErr))
		}
		if blk.ScanErr != nil && blk.Ad != nil {
			fmt.Fprintf(os.Stderr, "rtdd run: %s\n", adapterImportScanNote(blk.Ad.Name, blk.ScanErr))
		}
		// Attributed in a polyglot repository, for the reason whichNotes attributes its
		// own: "an empty selection is not a pass" is a sentence about ONE adapter, and
		// unattributed it reads as a claim about the whole run — which just executed
		// another adapter's tests.
		for _, n := range runNotes(blk.Selection, blk.FallbackErr, adapterScanWarning(blk.Ad, blk.ScanErr)) {
			if len(blocks) > 1 {
				n = blk.Adapter + ": " + n
			}
			warnings = append(warnings, n)
		}
	}

	if selectionIsEmpty(sel) {
		// An adapter whose enumeration failed is reported HERE too, and folds its code.
		// The empty selection is a CONSEQUENCE of that failure — a T2 block whose
		// enumeration never returned a list has nothing to run — so returning 0 from this
		// branch, ahead of the loop below that exists to report EnumErr, turns a broken
		// toolchain into "nothing to do here" on both output paths (PRD #232 AC7).
		enumRuns := enumFailures(blocks)
		for _, r := range enumRuns {
			warnings = append(warnings, adapterFailureNote(r.Adapter, r.Err, true))
		}
		code := FoldExitCodes(enumRuns)
		if anyAdapterFailedToRun(enumRuns) {
			fmt.Fprint(os.Stderr, renderAdapterRuns(enumRuns))
		}
		if *asJSON {
			// Nothing executed, so there is no fresh coverage and therefore no honest
			// uncovered report: UncoveredOK stays false and `files` is omitted rather
			// than sent as an empty list a consumer would read as "nothing uncovered".
			// `pre` is exactly that Cov-less signal, already computed per adapter.
			out := BuildOutput(OutputInput{
				Command:         "run",
				Base:            *base,
				Adapter:         blocksAdapterName(blocks),
				Sel:             sel,
				Changes:         changes,
				Instrumentable:  pre.Instrumentable,
				UnmappedFiles:   pre.UnmappedFiles,
				ImportFallback:  importFallback,
				SuiteEnumerated: suiteEnumerated,
				Warnings:        warnings,
				Blocks:          blocks,
			})
			out.ExitCode = code
			if err := emitJSON(out); err != nil {
				return 2
			}
			return finishCycle(root, mt, code)
		}
		fmt.Println("EMPTY SELECTION - nothing ran. This is not a pass.")
		return finishCycle(root, mt, code)
	}

	sha, err := gitctx.HeadSHA(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
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

	// One subset invocation per adapter, each with ONLY its own selection's ids (spec
	// §4.4). Handing a pytest nodeid to `npx vitest run` selects nothing and reports
	// green, so the two lists never meet — not here, and not in the map rows below,
	// which carry the adapter that produced them.
	//
	// A per-adapter failure is RECOVERED, never returned: one broken toolchain does not
	// void another adapter's selection or its results (spec §4.4, PRD #232 AC7). Each
	// adapter runs to completion, its failure is reported in the block that names it, and
	// the codes are folded by FoldExitCodes at the end — worst wins.
	var (
		runs     []AdapterRun
		outcomes []report.Outcome
		reports  []uncovered.FileReport
		failed   []string
		ran      int
	)
	sig := SignalOutput{Instrumentable: map[string]bool{}}
	unmapped := map[string]bool{}
	for _, blk := range blocks {
		// An adapter whose enumeration failed is reported even with nothing to run: the
		// empty selection is a CONSEQUENCE of the failure, and skipping it silently would
		// turn a broken toolchain into "nothing to do here".
		if blk.EnumErr == nil && selectionIsEmpty(blk.Selection) {
			continue
		}
		res, err := runSelection(blk, root, *failFast)
		if err != nil {
			code, hints := runErrClass(err)
			runs = append(runs, AdapterRun{Adapter: blk.Adapter, Err: err, Code: code, Hints: hints})
			// The warning reaches the --json document too: a consumer discards stderr,
			// and a run whose Maven half never started must not read as a green one.
			warnings = append(warnings, adapterFailureNote(blk.Adapter, err, blk.EnumErr != nil))
			continue
		}
		for _, row := range rowsFrom(res, sha, blk.Adapter) {
			// UNION, NEVER REPLACE. See the doc comment on cmdRun. UnionFor rather than
			// Union because an untagged row IS this adapter's row when meta.json names
			// it, and writing a tagged copy beside it would leave the map holding both.
			m.UnionFor(row, mt.Adapter, older)
		}

		// Classify against the coverage THIS adapter just produced — never against
		// map.jsonl, which carries no line data at all and could therefore only ever
		// answer at whole-file granularity, on numbers that drifted the moment the file
		// was edited (spec §4, §6).
		blkSig := BuildSignal(SignalInput{
			Changes:          changes,
			Cov:              res.Coverage,
			IsInstrumentable: instrumentableOf(blk.Ad),
			Map:              m,
		})
		reports = append(reports, blkSig.Reports...)
		for p, ok := range blkSig.Instrumentable {
			sig.Instrumentable[p] = sig.Instrumentable[p] || ok
		}
		if unmappedNoticeApplies(blk.Ad) {
			for _, f := range blkSig.UnmappedFiles {
				unmapped[f] = true
			}
		}

		outcomes = append(outcomes, res.Outcomes...)
		failed = append(failed, res.Failed...)
		ran += len(res.Outcomes)

		// A non-empty uncovered report NEVER moves the exit code: RTDD reports, it does
		// not gate (spec §2 non-goals, §6, decision D3). res.ExitCode is still consulted
		// so a runner-level failure the report log did not name cannot be swallowed.
		blkCode := ExitCodeFor(res.Outcomes, blkSig.Reports)
		if blkCode == 0 && res.ExitCode != 0 {
			blkCode = res.ExitCode
		}
		runs = append(runs, AdapterRun{Adapter: blk.Adapter, Result: res, Code: blkCode})
	}
	// Worst wins, never last: an adapter that failed must not be reported as success
	// because the next one happened to pass.
	code := FoldExitCodes(runs)

	// The per-adapter block is printed whenever an adapter failed to run at all: a flat
	// "N ran, M failed" cannot say which toolchain never started, and under --json stdout
	// is one document, so it goes to stderr in both modes.
	if anyAdapterFailedToRun(runs) {
		fmt.Fprint(os.Stderr, renderAdapterRuns(runs))
	}
	for f := range unmapped {
		sig.UnmappedFiles = append(sig.UnmappedFiles, f)
	}
	sort.Strings(sig.UnmappedFiles)

	if err := os.MkdirAll(rtddDir(root), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	if err := m.Save(mapPath(root)); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}

	// The detected set, NOT the blocks: blocks are ordered by adapter name, and the
	// singular `adapter` names the coverage adapter that produced the map rather than
	// whichever name sorts first. See coverageAdapterName.
	mt = metaAfterRun(mt, ads)

	if *asJSON {
		out := BuildOutput(OutputInput{
			Command:         "run",
			Base:            *base,
			Adapter:         blocksAdapterName(blocks),
			Sel:             sel,
			Changes:         changes,
			Instrumentable:  sig.Instrumentable,
			Executed:        true,
			Outcomes:        outcomes,
			Reports:         reports,
			UncoveredOK:     true,
			UnmappedFiles:   sig.UnmappedFiles,
			ImportFallback:  importFallback,
			SuiteEnumerated: suiteEnumerated,
			Warnings:        warnings,
			Blocks:          blocks,
		})
		out.ExitCode = code
		if err := emitJSON(out); err != nil {
			return 2
		}
		return finishCycle(root, mt, code)
	}

	fmt.Printf("%d ran, %d failed, %d rows in the map\n", ran, len(failed), m.Len())
	for _, id := range failed {
		fmt.Printf("FAILED %s\n", id)
	}
	if s := RenderUncovered(reports); s != "" {
		fmt.Fprint(os.Stdout, "\n"+s)
	}
	return finishCycle(root, mt, code)
}

// renderRunTiers is the tier line, one per adapter. The heading appears only when more
// than one adapter answered: there is nothing to disambiguate in a single-toolchain
// repository, and the line it printed before per-adapter selection existed is the line it
// still prints.
func renderRunTiers(blocks []AdapterSelection) string {
	var b strings.Builder
	for _, blk := range blocks {
		if len(blocks) > 1 {
			fmt.Fprintf(&b, "adapter: %s\n", blk.Adapter)
		}
		fmt.Fprintf(&b, "tier %s: %d selected", blk.Selection.Tier, len(blk.Selection.Tests))
		if blk.Selection.Reason != "" {
			fmt.Fprintf(&b, " (%s)", blk.Selection.Reason)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// selectionIsEmpty is the one reading of "nothing to run": an explicit empty tier, or a
// tier that named no test. Both are reported, and neither is a pass.
func selectionIsEmpty(sel selector.Selection) bool {
	return sel.Tier == selector.TierEmpty || len(sel.Tests) == 0
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

// enumFailures is the per-adapter report for every block whose enumeration failed, with
// the same exit code and operator hints the subset path derives from a runner error: for
// a `report: junit-xml` adapter enumerating IS running, so the two are one class of
// failure and must not be classified two ways.
func enumFailures(blocks []AdapterSelection) []AdapterRun {
	var runs []AdapterRun
	for _, blk := range blocks {
		if blk.EnumErr == nil {
			continue
		}
		code, hints := runErrClass(blk.EnumErr)
		runs = append(runs, AdapterRun{Adapter: blk.Adapter, Err: blk.EnumErr, Code: code, Hints: hints})
	}
	return runs
}

// adapterFailureNote is the document warning for an adapter that produced no results,
// worded by WHICH invocation failed. An enumeration that could not start never got as far
// as a subset, and calling it "the subset invocation" would send an operator to the wrong
// command — the enumeration is the one to reproduce.
func adapterFailureNote(name string, err error, enum bool) string {
	if enum {
		return fmt.Sprintf("%s: enumerating the suite failed: %v", name, err)
	}
	return fmt.Sprintf("%s: the subset invocation failed: %v", name, err)
}

// anyAdapterFailedToRun reports whether some adapter's subset invocation never produced a
// result. A failing TEST is not this: that is an outcome, and it is already reported by id.
func anyAdapterFailedToRun(runs []AdapterRun) bool {
	for _, r := range runs {
		if r.Err != nil {
			return true
		}
	}
	return false
}
