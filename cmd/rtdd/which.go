package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/selector"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// cmdWhich answers "what should I run?" without running anything. It costs one map load,
// one git diff and one listing of the tracked files.
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
	asJSON := fs.Bool("json", false, "emit machine-readable JSON (schema v2)")
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
	// so no adapter can ever be handed another's test ids.
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
	sel, sig := foldBlocks(blocks)

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
		return emitWhichJSON(stdout, stderr, blocks, *base, changes, sel, sig, notes)
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
	return out
}

// emitWhichJSON writes the frozen v1 document — the same schema `rtdd run --json` emits,
// so a front-end binds once and reads both.
func emitWhichJSON(stdout, stderr io.Writer, blocks []AdapterSelection, base string,
	changes []gitctx.Change, sel selector.Selection, sig SignalOutput, warnings []string) int {
	out := BuildOutput(OutputInput{
		Command:        "which",
		Base:           base,
		Adapter:        blocksAdapterName(blocks),
		Sel:            sel,
		Changes:        changes,
		Instrumentable: sig.Instrumentable,
		Executed:       false,
		UncoveredOK:    false,
		Warnings:       warnings,
		UnmappedFiles:  sig.UnmappedFiles,
		Blocks:         blocks,
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
