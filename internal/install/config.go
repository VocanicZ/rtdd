package install

import (
	"fmt"
	"strings"
)

// AdapterRecord is one DETECTED adapter as `.rtdd/config.yaml` records it (spec §5).
type AdapterRecord struct {
	Name string
}

// ConfigWithAdapters renders .rtdd/config.yaml with a record of what init detected.
//
// The record is a HUMAN-READABLE trace of the last first-install, not an input: nothing
// reads it back, and `rtdd doctor` reports the adapters live.
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
	b.WriteString("# `rtdd doctor` reports the adapters live and is the source of truth.\n")
	b.WriteString("adapters:\n")
	for _, r := range recs {
		fmt.Fprintf(&b, "  - name: %s\n", r.Name)
	}
	return b.String()
}
