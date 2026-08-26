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

	e, code, err := loadEnv(*adapterPath)
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

	// Cov is nil on purpose: which ran nothing. sig.Reports is therefore NOT a report of
	// anything and is deliberately discarded — only Instrumentable and UnmappedFiles, both
	// computable from the map alone, survive into the output.
	sig := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              nil,
		IsInstrumentable: e.ad.IsInstrumentable,
		Map:              e.m,
	})

	// allTests is the enumerated suite. Enumerating it costs a collection run, which
	// `which` does not pay: a T2 selection is therefore a partial list, and the note
	// below says so.
	var allTests []string

	merge, _ := gitctx.IsMergeCommit(e.root, "HEAD")
	distance := func(sha string) int {
		d, derr := gitctx.CommitDistance(e.root, sha)
		if derr != nil {
			return -1 // unknown, never fresh
		}
		return d
	}

	fb := newImportFallback(e.root, e.m, sig.UnmappedFiles)

	sel := selector.Select(selector.Inputs{
		Map:        e.m,
		Changes:    changes,
		Adapter:    e.ad,
		Cfg:        selector.DefaultConfig(),
		AllTests:   allTests,
		Cycles:     e.meta.Cycles,
		Merge:      merge,
		Distance:   distance,
		ImportOnly: fb.testsImporting,
	})

	notes := whichNotes(e, sel, allTests, fb)

	if *asJSON {
		// The notes go BOTH ways. Structured, they are `complete` and `warnings` inside
		// the document, because under --json the document is the whole of stdout and a
		// consumer that discards stderr would otherwise lose the guarantee entirely.
		// As prose they stay on stderr, where a human sees them and where they cannot
		// corrupt the single JSON document a parser is reading from stdout.
		for _, n := range notes {
			fmt.Fprintf(stderr, "rtdd which: %s\n", n)
		}
		return emitWhichJSON(stdout, stderr, e, *base, changes, sel, sig, fb.fired, notes)
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
	fmt.Fprint(stdout, RenderWhich(sel, sig.UnmappedFiles))
	for _, n := range notes {
		fmt.Fprintf(stdout, "\nNOTE: %s\n", n)
	}
	return 0
}

// whichNotes are the conditions under which the selection is narrower, or less
// authoritative, than it looks. Each is a sentence a reader can act on.
//
// `status` already reports a missing adapter; `which` is the command agents call, and it
// used to run with file classification silently disabled.
func whichNotes(e *env, sel selector.Selection, allTests []string, fb *importFallbackScan) []string {
	var out []string
	if e.ad == nil {
		out = append(out, fmt.Sprintf("no adapter (%s not found) - file classification is disabled: "+
			"no changed file can be recognised as a test file, so the direct tier is empty", e.adPath))
	}
	if sel.Tier == selector.TierEmpty {
		out = append(out, "an empty selection is not a pass. Nothing was checked.")
	}
	// Gated on the suite being unenumerated, not on the selection being empty: a T2
	// selection that also carries a direct test is still a partial list of the full suite,
	// and reading it as "T2 satisfied by one test" is exactly the under-run this note
	// exists to prevent.
	if sel.Tier == selector.TierT2 && len(allTests) == 0 {
		out = append(out, fmt.Sprintf("T2 means the full suite. rtdd does not enumerate it here, "+
			"so the %d test id(s) listed above are not the whole run.", len(sel.Tests)))
	}
	if err := fb.err(); err != nil {
		out = append(out, importScanNote(err))
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
func emitWhichJSON(stdout, stderr io.Writer, e *env, base string, changes []gitctx.Change,
	sel selector.Selection, sig SignalOutput, importFallback map[string][]string,
	warnings []string) int {
	adapterName := ""
	if e.ad != nil {
		adapterName = e.ad.Name
	}
	out := BuildOutput(OutputInput{
		Command:        "which",
		Base:           base,
		Adapter:        adapterName,
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
