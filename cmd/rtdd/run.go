package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
	"github.com/VocanicZ/rtdd/internal/selector"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// cmdRun selects, executes, refreshes the map, and reports.
//
// It exits nonzero when a test failed, and when an adapter never got as far as running —
// a run that could not start is an environment failure, not a pass.
// A GENUINELY empty selection, one no failure caused, is exit 0 and is reported
// explicitly, so it can never read as "all passed" (spec §5).
//
// `f` is UNIONED, never replaced (spec §4, decision D11, audit A4): a failing unit
// records a truncated prefix of its real path, so a run may record less than the seed
// did. Only rtdd seed may shrink a row, and TestOnlySeedCallsMapstoreReplace enforces
// that this file never names Replace.
func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	base := fs.String("base", "HEAD", "diff base ref for the changed set")
	failFast := fs.Bool("fail-fast", false, "stop at the first failure (opt-in only)")
	asJSON := fs.Bool("json", false, "machine-readable output (schema v2)")
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
	m = currentMap(m, mt)
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
	escalateNow := escalateDigest(root, ads, changes)
	blocks, err := selectPerAdapter(root, ads, m, mt, selectionContext{
		Changes:                  changes,
		Cfg:                      selector.DefaultConfig(),
		Cycles:                   mt.Cycles,
		Merge:                    merge,
		EscalateDigest:           escalateNow,
		EscalateDigestAtLastFull: mt.EscalateDigest,
		Distance:                 memoDistance(root),
	})
	if err != nil {
		return reportRunErr(err)
	}

	sel, pre := foldBlocks(blocks)

	// Under --json the document is the WHOLE of stdout: a consumer pipes it straight
	// into a parser, and a human-readable tier line ahead of it is a syntax error. The
	// same facts are in the document as `tier`, `selection` and `run`.
	if !*asJSON {
		fmt.Print(renderRunTiers(blocks))
	}

	// Caveats reach the document as `warnings`, because a --json consumer normally
	// discards stderr.
	var warnings []string
	for _, blk := range blocks {
		// Attributed in a polyglot repository, for the reason whichNotes attributes its
		// own: "an empty selection is not a pass" is a sentence about ONE adapter, and
		// unattributed it reads as a claim about the whole run — which just executed
		// another adapter's tests.
		for _, n := range runNotes(blk.Selection) {
			if len(blocks) > 1 {
				n = blk.Adapter + ": " + n
			}
			warnings = append(warnings, n)
		}
	}

	if selectionIsEmpty(sel) {
		if *asJSON {
			// Nothing executed, so there is no fresh coverage and therefore no honest
			// uncovered report: UncoveredOK stays false and `files` is omitted rather
			// than sent as an empty list a consumer would read as "nothing uncovered".
			// `pre` is exactly that Cov-less signal, already computed per adapter.
			out := BuildOutput(OutputInput{
				Command:        "run",
				Base:           *base,
				Adapter:        blocksAdapterName(blocks),
				Sel:            sel,
				Changes:        changes,
				Instrumentable: pre.Instrumentable,
				UnmappedFiles:  pre.UnmappedFiles,
				Warnings:       warnings,
				Blocks:         blocks,
			})
			if err := emitJSON(out); err != nil {
				return 2
			}
			return finishCycle(root, mt, 0)
		}
		fmt.Println("EMPTY SELECTION - nothing ran. This is not a pass.")
		return finishCycle(root, mt, 0)
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

	// One run per adapter, each over ONLY its own selected units (spec §4.4): one
	// adapter's units never reach another adapter's runner — not here, and not in the
	// map rows below, which carry the adapter that produced them.
	//
	// A per-adapter failure is RECOVERED, never returned: one broken toolchain does not
	// void another adapter's selection or its results (spec §4.4, PRD #232 AC7). Each
	// adapter runs to completion, its failure is reported in the block that names it, and
	// the codes are folded by FoldExitCodes at the end — worst wins.
	var (
		runs     []AdapterRun
		outcomes []runner.Outcome
		reports  []uncovered.FileReport
		failed   []string
		ran      int
	)
	sig := SignalOutput{Instrumentable: map[string]bool{}}
	unmapped := map[string]bool{}
	// coverageRan is whether ANY adapter ran, and so whether fresh coverage backs an
	// uncovered report at all.
	var (
		coverageRan bool
		output      = map[string]string{}
	)
	for _, blk := range blocks {
		if selectionIsEmpty(blk.Selection) {
			continue
		}
		res, err := runSelection(blk, root, *failFast)
		if err != nil {
			code, hints := runErrClass(err)
			runs = append(runs, AdapterRun{Adapter: blk.Adapter, Err: err, Code: code, Hints: hints})
			// The warning reaches the --json document too: a consumer discards stderr,
			// and a run whose Maven half never started must not read as a green one.
			warnings = append(warnings, adapterFailureNote(blk.Adapter, err))
			continue
		}
		// Every executed unit's row is refreshed from the coverage its own run produced.
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
		coverageRan = true
		for p, ok := range blkSig.Instrumentable {
			sig.Instrumentable[p] = sig.Instrumentable[p] || ok
		}
		for _, f := range blkSig.UnmappedFiles {
			unmapped[f] = true
		}

		outcomes = append(outcomes, res.Outcomes...)
		failed = append(failed, res.Failed...)
		for u, o := range res.Output {
			output[u] = o
		}
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
	mt = metaAfterRun(mt, ads, sel.Tier, escalateNow)

	if *asJSON {
		out := BuildOutput(OutputInput{
			Command:        "run",
			Base:           *base,
			Adapter:        blocksAdapterName(blocks),
			Sel:            sel,
			Changes:        changes,
			Instrumentable: sig.Instrumentable,
			Executed:       true,
			Outcomes:       outcomes,
			Reports:        reports,
			// Available only when SOME adapter ran: an empty `files` list with nothing
			// behind it would read as "nothing uncovered".
			UncoveredOK:   coverageRan,
			UnmappedFiles: sig.UnmappedFiles,
			Warnings:      warnings,
			Blocks:        blocks,
		})
		out.ExitCode = code
		if err := emitJSON(out); err != nil {
			return 2
		}
		return finishCycle(root, mt, code)
	}

	fmt.Print(renderRunSummary(ran, len(failed), m.Len()))
	for _, id := range failed {
		fmt.Printf("FAILED %s\n", id)
	}
	for _, u := range failed {
		fmt.Printf("--- %s ---\n%s", u, output[u])
		if o := output[u]; o != "" && !strings.HasSuffix(o, "\n") {
			fmt.Println()
		}
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

// renderRunSummary is the one-line summary of what the run did.
func renderRunSummary(ran, failed, rows int) string {
	return fmt.Sprintf("%d ran, %d failed, %d rows in the map\n", ran, failed, rows)
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
func runNotes(sel selector.Selection) []string {
	if sel.Tier == selector.TierEmpty || len(sel.Tests) == 0 {
		return []string{"an empty selection is not a pass. Nothing was checked."}
	}
	return nil
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

// adapterFailureNote is the document warning for an adapter that produced no results.
func adapterFailureNote(name string, err error) string {
	return fmt.Sprintf("%s: the run failed: %v", name, err)
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
