package main

import (
	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/doctor"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// staticSelectionNote is the static-tier caveat for ONE selection (PRD #233 AC7, spec
// §6), or "" when that selection was execution-derived and the caveat would be false.
//
// It is gated on the selection's own fidelity rather than on the adapter's declaration,
// for the reason selectionFidelity exists: the declaration answers "what is the best this
// adapter could ever produce", and a coverage adapter whose map could not answer escalates
// to the full suite — an answer derived from nothing at all, which is `none` rather than
// `static` and is not what this sentence describes.
//
// The test count is deliberately not consulted. A static selection that named nothing is
// the weakest evidence on either surface, so the caveat belongs there too — beside the
// note that says an empty selection is not a pass, never instead of it.
func staticSelectionNote(sel selector.Selection) string {
	if selectionFidelity(sel.Tier) != adapter.FidelityStatic {
		return ""
	}
	return doctor.StaticSelectionCaveat
}
