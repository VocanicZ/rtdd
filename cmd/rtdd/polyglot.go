package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// Per-adapter selection (spec §4.4). adapter.Detect returns a SET since #309, so a
// TypeScript service with a Python tooling directory resolves two toolchains and the CLI
// must answer once per toolchain rather than once per repository.
//
// The rule the whole file exists to keep: two adapters' ids never meet. A pytest nodeid
// handed to `npx vitest run` selects nothing and reports green — a false pass wearing a
// real id — which is what PRD #232 AC6 forbids. The enforcement is structural rather
// than a filter applied afterwards: each adapter selects over a map that holds only its
// own rows, so there is no point at which the other adapter's ids are in scope.

// AdapterSelection is one adapter's answer, kept whole. cmd/rtdd renders one block per
// element and never merges two adapters' ids into one list.
type AdapterSelection struct {
	Adapter   string // the adapter's name, "" when the repository resolved none
	Ad        *adapter.Adapter
	Selection selector.Selection

	// Signal is the map-only signal for THIS adapter: which of the changed files no row
	// of its own covers. It is computed per adapter because "unmapped" is a question
	// about one adapter's map, and answering it from the union would report a Python
	// file as covered because a Vitest row happened to name it.
	Signal SignalOutput

	// ImportFallback is what the static-import fallback produced for this adapter, per
	// file — `selection.import_fallback` in the JSON document.
	ImportFallback map[string][]string

	// FallbackErr and ScanErr are degradations, never failures: a scan that could not run
	// narrows the selection, and the notes say so. They are carried here so the command
	// that renders the block can name the adapter the degradation belongs to.
	FallbackErr error
	ScanErr     error

	// SuiteEnumerated is true when a T2 selection listed the whole suite rather than the
	// map rows rtdd happened to know. `which` never enumerates; `run` does, for T2 only.
	SuiteEnumerated bool
}

// selectionContext is everything a selection needs that does NOT vary by adapter: the
// changed set and the git-derived facts. Splitting it out is what lets one repository's
// git work be paid once and read by every adapter.
type selectionContext struct {
	Changes  []gitctx.Change
	Cfg      selector.Config
	Cycles   int
	Merge    bool
	Distance func(sha string) int

	// Enumerate lists an adapter's whole suite, for a T2 escalation only. A nil Enumerate
	// means the suite is not enumerated — `rtdd which` deliberately does not pay a
	// collection run — and a T2 selection is then a partial list, which whichNotes says
	// in as many words.
	Enumerate func(ad *adapter.Adapter) ([]string, error)
}

// selectPerAdapter runs the existing pure selector once per detected adapter, each over
// the rows that adapter is allowed to see.
//
// A repository that resolved NO adapter still gets exactly one block, with a nil adapter:
// file classification is disabled and `rtdd which` has always said so, and losing that
// answer to an empty loop would turn a stated degradation into silence.
func selectPerAdapter(root string, ads []*adapter.Adapter, m *mapstore.Map, mt mapstore.Meta,
	ctx selectionContext) ([]AdapterSelection, error) {
	if len(ads) == 0 {
		ads = []*adapter.Adapter{nil}
	}
	out := make([]AdapterSelection, 0, len(ads))
	for _, ad := range ads {
		blk, err := selectFor(root, ad, ads, m, mt, ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, blk)
	}
	return out, nil
}

// selectFor is one adapter's block.
func selectFor(root string, ad *adapter.Adapter, ads []*adapter.Adapter, m *mapstore.Map,
	mt mapstore.Meta, ctx selectionContext) (AdapterSelection, error) {
	blk := AdapterSelection{Ad: ad}
	if ad != nil {
		blk.Adapter = ad.Name
	}

	sub := rowsVisibleTo(ad, ads, m, mt)

	// Cov is nil: this is the map-only signal, which answers "which changed files does no
	// row of this adapter's cover" — precisely the import fallback's trigger (spec §6,
	// D14). A run's post-execution signal, classified against fresh coverage, is a
	// separate call and stays that way.
	blk.Signal = BuildSignal(SignalInput{
		Changes:          ctx.Changes,
		IsInstrumentable: instrumentableOf(ad),
		Map:              sub,
	})

	fb := newImportFallback(root, sub, blk.Signal.UnmappedFiles)
	exists, importDistance, scanErr := staticResolvers(root, ad)

	choose := func(allTests []string) selector.Selection {
		return selector.Select(selector.Inputs{
			Map:            sub,
			Changes:        ctx.Changes,
			Adapter:        ad,
			Cfg:            ctx.Cfg,
			AllTests:       allTests,
			Cycles:         ctx.Cycles,
			Merge:          ctx.Merge,
			Distance:       ctx.Distance,
			ImportOnly:     fb.testsImporting,
			Exists:         exists,
			ImportDistance: importDistance,
		})
	}
	blk.Selection = choose(nil)

	// Enumerating the suite costs a full collection run, and T2 is the one tier whose
	// test list IS the whole suite — AllTests reaches no other branch of Select. Paying
	// it on every T0 run would put a collection on the critical path of the loop this
	// tool exists to make fast.
	if ctx.Enumerate != nil && blk.Selection.Tier == selector.TierT2 && ad != nil {
		all, err := ctx.Enumerate(ad)
		if err != nil {
			return AdapterSelection{}, err
		}
		blk.Selection = choose(all)
		blk.SuiteEnumerated = true
	}

	// Read AFTER selection: the declared scan runs inside Select, through the resolver.
	blk.ImportFallback = fb.fired
	blk.FallbackErr = fb.err()
	blk.ScanErr = scanErr()
	return blk, nil
}

// rowsVisibleTo is the map this adapter may select from.
//
// A single-adapter repository sees the WHOLE map, untagged rows and all: the tag exists
// to stop one adapter's ids reaching another's runner, and there is no other adapter.
// That is also what keeps spec §4.1's promise literal — a seeded Python repository's
// selection is byte-identical to what it was before rows carried a tag, whatever
// .rtdd/meta.json happens to say the adapter is called.
//
// A polyglot repository filters, with meta.json's singular `adapter` deciding whose the
// untagged rows are (decision 3).
func rowsVisibleTo(ad *adapter.Adapter, ads []*adapter.Adapter, m *mapstore.Map, mt mapstore.Meta) *mapstore.Map {
	if ad == nil || len(ads) < 2 {
		return m
	}
	return m.ForAdapter(ad.Name, mt.Adapter)
}

// RenderSelections is the human form of the per-adapter blocks.
//
// The heading appears only when more than one adapter was detected. Spec §4.1's
// byte-identical promise is about the selection, but a gratuitous output change in every
// Python repository is the kind of churn that makes a golden test worthless — and there
// is nothing to disambiguate when one toolchain answered.
func RenderSelections(blocks []AdapterSelection) string {
	var b strings.Builder
	for _, blk := range blocks {
		if len(blocks) > 1 {
			fmt.Fprintf(&b, "adapter: %s\n", blk.Adapter)
		}
		b.WriteString(RenderWhich(blk.Selection, blk.Signal.UnmappedFiles, blk.Ad))
	}
	return b.String()
}

// instrumentableOf is an adapter's file classifier, or nil when the repository resolved
// no adapter at all — nil is how BuildSignal spells "classification is disabled", and
// synthesising a classifier here would silently answer for an adapter that does not exist.
func instrumentableOf(ad *adapter.Adapter) func(string) bool {
	if ad == nil {
		return nil
	}
	return ad.IsInstrumentable
}

// tierBreadth ranks the tiers by how much of the suite they name. It exists for exactly
// one job — folding several adapters' tiers into the one the flat --json document
// reports — and the widest wins, because the flat `selection.tests` it labels is the
// union of every block and is therefore as wide as the widest of them.
func tierBreadth(t selector.Tier) int {
	switch t {
	case selector.TierT2:
		return 4
	case selector.TierTS:
		return 3
	case selector.TierT1:
		return 2
	case selector.TierT0:
		return 1
	default:
		return 0
	}
}

// foldBlocks collapses the per-adapter blocks into the one selection the flat half of the
// --json document describes.
//
// The flat list spans toolchains and is NOT a runner invocation: `selections` carries the
// split, and a consumer that means to invoke a runner reads one block from there. Folding
// exists so a document written before polyglot repositories were possible still parses
// and still means something — not so that two adapters' ids can be handed to one runner.
func foldBlocks(blocks []AdapterSelection) (selector.Selection, SignalOutput, map[string][]string, bool) {
	if len(blocks) == 0 {
		return selector.Selection{}, SignalOutput{}, map[string][]string{}, true
	}
	if len(blocks) == 1 {
		return blocks[0].Selection, blocks[0].Signal, blocks[0].ImportFallback, blocks[0].SuiteEnumerated
	}
	var (
		sel       selector.Selection
		sig       SignalOutput
		fallback  = map[string][]string{}
		complete  = true
		unmapped  = map[string]bool{}
		seenTest  = map[string]bool{}
		seenDirct = map[string]bool{}
	)
	sig.Instrumentable = map[string]bool{}
	for _, blk := range blocks {
		if tierBreadth(blk.Selection.Tier) > tierBreadth(sel.Tier) {
			sel.Tier = blk.Selection.Tier
		}
		if blk.Selection.Tier == selector.TierT2 && !blk.SuiteEnumerated {
			complete = false
		}
		for _, id := range blk.Selection.Tests {
			if !seenTest[id] {
				seenTest[id] = true
				sel.Tests = append(sel.Tests, id)
			}
		}
		for _, id := range blk.Selection.Direct {
			if !seenDirct[id] {
				seenDirct[id] = true
				sel.Direct = append(sel.Direct, id)
			}
		}
		for p, ok := range blk.Signal.Instrumentable {
			sig.Instrumentable[p] = sig.Instrumentable[p] || ok
		}
		// Only from an adapter the notice is true of: a `selection: static` adapter
		// records nothing, so EVERY changed file is trivially unmapped for it, and
		// unioning that in would report a Python file as uncovered because the Vitest
		// half of the repository has no map — see unmappedNoticeApplies.
		if unmappedNoticeApplies(blk.Ad) {
			for _, f := range blk.Signal.UnmappedFiles {
				unmapped[f] = true
			}
		}
		for f, ids := range blk.ImportFallback {
			fallback[f] = ids
		}
	}
	for f := range unmapped {
		sig.UnmappedFiles = append(sig.UnmappedFiles, f)
	}
	sort.Strings(sig.UnmappedFiles)
	sel.Reason = fmt.Sprintf("%d adapters answered; each adapter's own list is in selections", len(blocks))
	return sel, sig, fallback, complete
}

// blocksAdapterName is the document's flat `adapter` field. One adapter names itself; a
// polyglot repository names them all, comma-separated, because the flat half of the
// document it labels is the union of them all. A consumer that needs one adapter's answer
// reads `selections`, where each block names exactly one.
func blocksAdapterName(blocks []AdapterSelection) string {
	names := make([]string, 0, len(blocks))
	for _, blk := range blocks {
		if blk.Adapter != "" {
			names = append(names, blk.Adapter)
		}
	}
	return strings.Join(names, ",")
}

// detectedSet is the detected adapters as .rtdd/meta.json records them: sorted,
// deduplicated, and carrying only adapters that actually resolved.
func detectedSet(ads []*adapter.Adapter) []string {
	seen := make(map[string]bool, len(ads))
	out := make([]string, 0, len(ads))
	for _, ad := range ads {
		if ad == nil || seen[ad.Name] {
			continue
		}
		seen[ad.Name] = true
		out = append(out, ad.Name)
	}
	sort.Strings(out)
	return out
}
