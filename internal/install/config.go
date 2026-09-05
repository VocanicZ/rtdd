package install

import (
	"fmt"
	"strings"
)

// AdapterRecord is one DETECTED adapter as `.rtdd/config.yaml` records it. Spec §5's
// ≥1-adapter branch requires init to record what it detected and that adapter's
// `selection`; `fidelity` is recorded beside them because it is the value the installed
// front-ends state (§6), and a reader comparing the two should not have to derive one
// from the other.
//
// Fidelity is a string rather than adapter.Fidelity so internal/install keeps depending
// on nothing: this package renders text and never resolves an adapter.
type AdapterRecord struct {
	Name      string
	Selection string
	Fidelity  string
}

// ConfigWithAdapters renders .rtdd/config.yaml with a record of what init detected.
//
// The record is a HUMAN-READABLE trace of the last first-install, not an input: nothing
// reads it back, and `rtdd doctor` re-derives fidelity from the adapters live and remains
// the source of truth. That is deliberate — a config that pinned fidelity would go stale
// the moment someone edited .rtdd/adapters/, and would then be believed.
//
// No records renders the unchanged default: an empty `adapters:` key claims RTDD looked
// and found none, which is a different thing from an install that made no such promise.
func ConfigWithAdapters(recs []AdapterRecord) string {
	if len(recs) == 0 {
		return defaultConfig
	}
	var b strings.Builder
	b.WriteString(defaultConfig)
	b.WriteString("\n")
	b.WriteString("# What `rtdd init` detected in this repository (spec §5). A record, not an input:\n")
	b.WriteString("# `rtdd doctor` derives fidelity from .rtdd/adapters/ live and is the source of truth.\n")
	b.WriteString("adapters:\n")
	for _, r := range recs {
		fmt.Fprintf(&b, "  - name: %s\n", r.Name)
		fmt.Fprintf(&b, "    selection: %s\n", r.Selection)
		fmt.Fprintf(&b, "    fidelity: %s\n", r.Fidelity)
	}
	return b.String()
}
