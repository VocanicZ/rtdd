// Package runner executes an adapter's commands as subprocesses: it forces the
// adapter's environment, chunks test ids across invocations, maps fatal exit
// codes, and merges the per-chunk coverage and report results.
package runner

// MaxArgvBytes is the per-invocation argv budget for test ids.
//
// Measured: 8,000 realistic pytest ids are 460 KB (spec §8 cites 613 KB for
// longer ids). Linux ARG_MAX measured at 2,097,152 — it fits. Windows CMD is
// 8,191 characters — 57-74x over. pytest has no argfile option, so the ids must
// go on the command line and the run must be split.
const MaxArgvBytes = 100_000

// Chunk splits tests into groups whose argv footprint stays under maxBytes,
// counting len(id)+1 per id for the separator. Order is preserved, no chunk is
// empty, and a single id longer than the budget gets its own chunk rather than
// being dropped.
func Chunk(tests []string, maxBytes int) [][]string {
	if len(tests) == 0 {
		return nil
	}
	if maxBytes <= 0 {
		maxBytes = MaxArgvBytes
	}
	var out [][]string
	var cur []string
	size := 0
	for _, t := range tests {
		n := len(t) + 1
		if len(cur) > 0 && size+n > maxBytes {
			out = append(out, cur)
			cur, size = nil, 0
		}
		cur = append(cur, t)
		size += n
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}
