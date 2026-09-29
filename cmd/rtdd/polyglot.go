package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
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

	// EscalateDigest names the state of the full-escalate files the diff touches;
	// EscalateDigestAtLastFull is meta.json's record of the state the last completed
	// full run covered. Equal means that run already paid for this config, and the
	// tier rule stops re-escalating on an edit that is still in an uncommitted diff.
	EscalateDigest           string
	EscalateDigestAtLastFull string
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
	// row of this adapter's cover". A run's post-execution signal, classified against
	// fresh coverage, is a separate call and stays that way.
	blk.Signal = BuildSignal(SignalInput{
		Changes:          ctx.Changes,
		IsInstrumentable: instrumentableOf(ad),
		Map:              sub,
	})

	units, err := runner.Units(ad, root)
	if err != nil {
		return blk, err
	}
	blk.Selection = selector.Select(selector.Inputs{
		Map:                      sub,
		Changes:                  ctx.Changes,
		Adapter:                  ad,
		Cfg:                      ctx.Cfg,
		AllTests:                 units,
		Cycles:                   ctx.Cycles,
		Merge:                    ctx.Merge,
		EscalateDigest:           ctx.EscalateDigest,
		EscalateDigestAtLastFull: ctx.EscalateDigestAtLastFull,
		Distance:                 ctx.Distance,
	})
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
		b.WriteString(RenderWhich(blk.Selection, blk.Signal.UnmappedFiles))
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
func foldBlocks(blocks []AdapterSelection) (selector.Selection, SignalOutput) {
	if len(blocks) == 0 {
		return selector.Selection{}, SignalOutput{}
	}
	if len(blocks) == 1 {
		return blocks[0].Selection, blocks[0].Signal
	}
	var (
		sel       selector.Selection
		sig       SignalOutput
		unmapped  = map[string]bool{}
		seenTest  = map[string]bool{}
		seenDirct = map[string]bool{}
	)
	sig.Instrumentable = map[string]bool{}
	for _, blk := range blocks {
		if tierBreadth(blk.Selection.Tier) > tierBreadth(sel.Tier) {
			sel.Tier = blk.Selection.Tier
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
		for _, f := range blk.Signal.UnmappedFiles {
			unmapped[f] = true
		}
	}
	for f := range unmapped {
		sig.UnmappedFiles = append(sig.UnmappedFiles, f)
	}
	sort.Strings(sig.UnmappedFiles)
	sel.Reason = fmt.Sprintf("%d adapters answered; each adapter's own list is in selections", len(blocks))
	return sel, sig
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

// AdapterRun is one adapter's run, kept whole. A failure is data on this struct, never a
// short circuit: the next adapter's selection is still worth running and still worth
// reporting (spec §4.4, PRD #232 AC7). A broken Maven install says nothing at all about
// whether the Vitest half of the repository passes.
type AdapterRun struct {
	Adapter string
	Result  *runner.RunResult
	Err     error
	Code    int

	// Hints are the operator lines that explain Err — the same ones the single-adapter
	// path prints — carried here so the block that names the adapter is the block that
	// says them.
	Hints []string
}

// FoldExitCodes returns the worst code any adapter produced, by the plan's precedence:
// 3 > 2 > 1 > 0. "Worst" and not "last": a fatal environment error in the first adapter
// must not be reported as success because the second one happened to pass.
//
// 3 and 2 say RTDD's answer is untrustworthy, 1 says the answer is trustworthy and is bad
// news, and collapsing either of the first two into 1 would tell an agent a test failed
// when in fact nothing ran.
func FoldExitCodes(runs []AdapterRun) int {
	worst := 0
	for _, r := range runs {
		if c := foldableCode(r.Code); exitSeverity(c) > exitSeverity(worst) {
			worst = c
		}
	}
	return worst
}

// foldableCode maps a per-adapter code onto the frozen exit-code table. `rtdd run` may
// only ever exit 0, 1, 2 or 3 (00-interfaces.md), so a code from outside it — a signal
// death a runner surfaced as 137, say — is reported as 3: it is an environment that broke,
// and passing it through would invent a fifth code while treating it as 0 would report a
// broken toolchain as a pass.
func foldableCode(code int) int {
	switch code {
	case 0, 1, 2, 3:
		return code
	default:
		return 3
	}
}

// exitSeverity ranks the four codes by how badly they contradict "this run answered".
func exitSeverity(code int) int {
	switch code {
	case 3:
		return 3
	case 2:
		return 2
	case 1:
		return 1
	default:
		return 0
	}
}

// renderAdapterRuns is the per-adapter execution report AC7 requires: one block per
// adapter, naming the adapter, and then either what broke or what ran. It is rendered
// whenever any adapter failed, because that is when a flat count of failures stops saying
// which toolchain produced them.
func renderAdapterRuns(runs []AdapterRun) string {
	var b strings.Builder
	for _, r := range runs {
		fmt.Fprintf(&b, "adapter: %s\n", r.Adapter)
		if r.Err != nil {
			fmt.Fprintf(&b, "  FAILED TO RUN (exit %d): %v\n", r.Code, r.Err)
			for _, h := range r.Hints {
				fmt.Fprintf(&b, "  %s\n", h)
			}
			continue
		}
		if r.Result == nil {
			fmt.Fprintf(&b, "  nothing ran\n")
			continue
		}
		fmt.Fprintf(&b, "  %d ran, %d failed (exit %d)\n", len(r.Result.Outcomes), len(r.Result.Failed), r.Code)
		for _, id := range r.Result.Failed {
			fmt.Fprintf(&b, "  FAILED %s\n", id)
		}
	}
	return b.String()
}

// runSubset is the subset invocation, as a variable so a test can count how often the
// suite is actually invoked. It is runner.Run and nothing else.
var runSubset = runner.Run

// runSelection executes one adapter's selection.
func runSelection(blk AdapterSelection, root string, failFast bool) (*runner.RunResult, error) {
	return runSubset(blk.Ad, root, blk.Selection.Tests, failFast)
}
