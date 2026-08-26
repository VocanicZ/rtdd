// Package coverage reads coverage.py's .coverage SQLite store directly — it *is*
// the bipartite test<->file relation, at a fraction of the size of any exported
// format (spec §4, audit A6).
package coverage

// Numbits decodes coverage.py's numbits blob into sorted line numbers.
//
// numbits is a packed bitmap, not a line list: byte i, bit j set means line
// i*8+j is covered. Verified against coverage.numbits.numbits_to_nums on real
// data — e.g. []byte{0x12, 0x11} decodes to [1 4 8 12].
//
// Bit 0 of byte 0 can legitimately be set (an empty __init__.py records it), but
// line 0 is not a real source line, so it is dropped here rather than left for
// every caller to filter.
//
// The result is ascending and never nil: callers range over it and append.
func Numbits(b []byte) []int {
	out := make([]int, 0, len(b)*2)
	for i, by := range b {
		if by == 0 {
			continue
		}
		for j := 0; j < 8; j++ {
			if by&(1<<uint(j)) == 0 {
				continue
			}
			if line := i*8 + j; line > 0 {
				out = append(out, line)
			}
		}
	}
	return out
}
