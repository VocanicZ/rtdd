package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/importscan"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/selector"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// cmdWhich answers "what should I run?" without running anything. It costs one map load,
// one git diff and — only for a changed file no map row covers — one static import scan.
// It is the primary agent integration point. Every tier exits 0: an empty selection is a
// signal, and the human output says so in as many words.
//
// which executes NO test, so it has no fresh coverage and therefore no line-level
// uncovered report. Emitting a stale one would reintroduce exactly the line-drift problem
// spec §4 removes, so `uncovered.available` is always false and `uncovered.files` is
// omitted. The file-level `unmapped_files` is the signal which can honestly compute.
func cmdWhich(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("which", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "HEAD", "base ref for the changed set")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON (schema v1)")
	adapterPath := fs.String("adapter", "", "path to the adapter YAML (default .rtdd/adapter.yaml)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	e, code, err := loadEnv(*adapterPath, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return code
	}

	// changedSet, not gitctx.ChangedSet: rtdd's own .rtdd/ writes must not select.
	changes, err := changedSet(e.root, *base)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 3
	}

	// Populate the changed line ranges from git diff --unified=0. WithLines is
	// authoritative and overwrites Lines, so hunk parsing is the single source of truth.
	rawDiff, err := gitctx.RawDiff(e.root, *base)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 3
	}
	changes, err = uncovered.WithLines(e.root, changes, rawDiff)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 3
	}

	merge, _ := gitctx.IsMergeCommit(e.root, "HEAD")
	distance := memoDistance(e.root)

	// One block per detected adapter (spec §4.4): each adapter selects over its own rows,
	// so no adapter can ever be handed another's test ids. Enumerate stays nil — `which`
	// does not pay a collection run, so a T2 selection is a partial list and the note
	// below says so in as many words.
	blocks, err := selectPerAdapter(e.root, e.ads, e.m, e.meta, selectionContext{
		Changes:                  changes,
		Cfg:                      selector.DefaultConfig(),
		Cycles:                   e.meta.Cycles,
		Merge:                    merge,
		Distance:                 distance,
		EscalateDigest:           escalateDigest(e.root, e.ads, changes),
		EscalateDigestAtLastFull: e.meta.EscalateDigest,
	})
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 2
	}

	// The flat half of the output is the fold of the blocks, and with one adapter it IS
	// that block — which is what keeps a single-adapter repository's output byte-identical.
	sel, sig, importFallback, _ := foldBlocks(blocks)

	var notes []string
	for _, blk := range blocks {
		notes = append(notes, whichNotes(e, blk, len(blocks) > 1)...)
	}

	if *asJSON {
		// The notes go BOTH ways. Structured, they are `complete` and `warnings` inside
		// the document, because under --json the document is the whole of stdout and a
		// consumer that discards stderr would otherwise lose the guarantee entirely.
		// As prose they stay on stderr, where a human sees them and where they cannot
		// corrupt the single JSON document a parser is reading from stdout.
		for _, n := range notes {
			fmt.Fprintf(stderr, "rtdd which: %s\n", n)
		}
		return emitWhichJSON(stdout, stderr, blocks, *base, changes, sel, sig, importFallback, notes)
	}

	fmt.Fprintf(stdout, "base:     %s\n", *base)
	fmt.Fprintf(stdout, "changed:  %d files\n", len(changes))
	for _, c := range changes {
		if c.OldPath != "" {
			fmt.Fprintf(stdout, "  %-9s %s (from %s)\n", c.Status.String(), c.Path, c.OldPath)
		} else {
			fmt.Fprintf(stdout, "  %-9s %s\n", c.Status.String(), c.Path)
		}
	}
	fmt.Fprint(stdout, RenderSelections(blocks))
	for _, n := range notes {
		fmt.Fprintf(stdout, "\nNOTE: %s\n", n)
	}
	return 0
}

// whichNotes are the conditions under which THIS adapter's selection is narrower, or
// less authoritative, than it looks. Each is a sentence a reader can act on.
//
// `status` already reports a missing adapter; `which` is the command agents call, and it
// used to run with file classification silently disabled.
//
// In a polyglot repository every note names its adapter: two adapters produce two sets of
// caveats, and an unattributed one sends the reader to the wrong half of the repository.
// A single-adapter repository prints them exactly as it always did — there is nothing to
// disambiguate, and the prefix would be churn in every existing Python repo.
func whichNotes(e *env, blk AdapterSelection, multi bool) []string {
	var out []string
	note := func(format string, args ...any) {
		s := fmt.Sprintf(format, args...)
		if multi {
			s = blk.Adapter + ": " + s
		}
		out = append(out, s)
	}
	if blk.Ad == nil {
		note("no adapter (%s) - file classification is disabled: "+
			"no changed file can be recognised as a test file, so the direct tier is empty",
			e.noAdapterReason())
	}
	if blk.Selection.Tier == selector.TierEmpty {
		note("an empty selection is not a pass. Nothing was checked.")
	}
	// Gated on the suite being unenumerated, not on the selection being empty: a T2
	// selection that also carries a direct test is still a partial list of the full suite,
	// and reading it as "T2 satisfied by one test" is exactly the under-run this note
	// exists to prevent.
	if blk.Selection.Tier == selector.TierT2 && !blk.SuiteEnumerated {
		note("T2 means the full suite. rtdd does not enumerate it here, "+
			"so the %d test id(s) listed above are not the whole run.", len(blk.Selection.Tests))
	}
	if blk.FallbackErr != nil {
		note("%s", importScanNote(blk.FallbackErr))
	}
	// A DECLARED scanner that failed is a level that could not run. Without this note the
	// selection is narrower than the adapter promises and nothing says so — and the tier's
	// own reason, which only knows the resolver was supplied, reads as though the imports
	// were checked and found nothing.
	if blk.ScanErr != nil && blk.Ad != nil {
		note("%s", adapterImportScanNote(blk.Ad.Name, blk.ScanErr))
	}
	// Last, because it qualifies the whole of this adapter's answer rather than naming
	// one thing that went wrong with it: a static selection is a working selection, and
	// the note says what believing it is worth.
	if s := staticSelectionNote(blk.Selection); s != "" {
		note("%s", s)
	}
	return out
}

// importFallbackScan wires the static-import fallback into selection and records, per
// file, which tests it produced — that record is `selection.import_fallback`.
//
// The trigger condition is spec §6, D14: a changed instrumentable file that NO map row
// covers. Import-time lines are attributed to no test, so they never enter any row's f;
// "zero tests cover this file in the map" is precisely the import-time-only case (or a
// genuinely untested file, where selecting importers is still the best available guess).
// Firing it on a mapped file would pay for an AST scan the coverage relation already
// answered.
type importFallbackScan struct {
	scanner *importscan.Scanner
	trigger map[string]bool
	fired   map[string][]string
}

// newImportFallback builds the fallback over unmapped. A scanner is constructed only when
// there is something to scan, so the common case costs no subprocess at all.
func newImportFallback(root string, m *mapstore.Map, unmapped []string) *importFallbackScan {
	fb := &importFallbackScan{
		trigger: make(map[string]bool, len(unmapped)),
		fired:   map[string][]string{},
	}
	for _, p := range unmapped {
		fb.trigger[p] = true
	}
	if len(unmapped) > 0 {
		fb.scanner = importscan.NewScanner(root, candidateTestFiles(m))
	}
	return fb
}

// testsImporting satisfies selector.Inputs.ImportOnly. A failed scan degrades selection —
// it never fails the command — so a nil return is a legitimate answer here.
func (fb *importFallbackScan) testsImporting(rel string) []string {
	if fb.scanner == nil || !fb.trigger[rel] {
		return nil
	}
	ids := fb.scanner.TestsImporting(rel)
	if len(ids) > 0 {
		fb.fired[rel] = ids
	}
	return ids
}

func (fb *importFallbackScan) err() error {
	if fb.scanner == nil {
		return nil
	}
	return fb.scanner.Err()
}

// candidateTestFiles is the set of test FILES the scan may return: the file part of every
// map row's test id. The scan answers "which of these modules transitively imports the
// target", so a module absent here can never be selected by the fallback — which is
// harmless for a just-written test module, because the direct tier already runs it
// without consulting the map at all.
func candidateTestFiles(m *mapstore.Map) []string {
	seen := map[string]bool{}
	for _, r := range m.Rows() {
		seen[testFileOf(r.T)] = true
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		if f != "" {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// testFileOf strips the pytest node-id suffix: "tests/test_it.py::test_logic" is the
// module "tests/test_it.py", which is what an import scan can reason about.
func testFileOf(id string) string {
	if i := strings.Index(id, "::"); i >= 0 {
		return id[:i]
	}
	return id
}

// emitWhichJSON writes the frozen v1 document — the same schema `rtdd run --json` emits,
// so a front-end binds once and reads both.
func emitWhichJSON(stdout, stderr io.Writer, blocks []AdapterSelection, base string,
	changes []gitctx.Change, sel selector.Selection, sig SignalOutput,
	importFallback map[string][]string, warnings []string) int {
	out := BuildOutput(OutputInput{
		Command:        "which",
		Base:           base,
		Adapter:        blocksAdapterName(blocks),
		Sel:            sel,
		Changes:        changes,
		Instrumentable: sig.Instrumentable,
		Executed:       false,
		UncoveredOK:    false,
		// which never enumerates the suite — that costs a collection run it does not
		// pay — so a T2 selection here is always reported as incomplete.
		SuiteEnumerated: false,
		Warnings:        warnings,
		UnmappedFiles:   sig.UnmappedFiles,
		ImportFallback:  importFallback,
		Blocks:          blocks,
	})

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 2
	}
	return 0
}

// nonNilStrings guarantees a JSON array rather than null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
