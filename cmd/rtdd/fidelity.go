package main

import (
	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// selectionFidelity is what THIS selection was derived from (spec §6).
//
// It is deliberately not `Adapter.Fidelity()`, which answers a different question — "what
// is the best this adapter could ever produce" — and the two genuinely differ: a coverage
// adapter with an unseeded map escalates to T2, an answer derived from nothing at all,
// while its declaration still says execution-derived. An agent calibrating on the
// declaration would trust a full-suite escalation as recorded evidence.
//
// TierEmpty is the entry that repays stating. It is reachable ONLY from a usable map, so
// the map did answer and its answer was "nothing"; labelling it `none` would say "no
// evidence" about the one outcome `warnings` already has to defend as a real result.
//
// T2 is the opposite case: the full suite is what you run when no relation could answer,
// so nothing was derived. An unrecognised tier takes the same value through the default
// arm — a tier this table does not know is not evidence of anything, and `none` is the
// only value that cannot overstate it.
func selectionFidelity(t selector.Tier) adapter.Fidelity {
	switch t {
	case selector.TierDirect, selector.TierT0, selector.TierT1, selector.TierEmpty:
		return adapter.FidelityExecution
	case selector.TierTS:
		return adapter.FidelityStatic
	default:
		return adapter.FidelityNone
	}
}
