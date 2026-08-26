package selector

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// Rank orders tests by: descending |F ∩ changed| / |F|, then S=="fail" first,
// then ascending len(F), then ascending D.
//
// The final tiebreak is the test id, so the printed list is stable across runs.
// A test with no map row keeps ratio 0, len(F) 0 and D 0; it is ordered, never dropped.
func Rank(m *mapstore.Map, tests, changedFiles []string) []string {
	changed := make(map[string]struct{}, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = struct{}{}
	}

	type sortKey struct {
		id     string
		ratio  float64
		failed bool
		nFiles int
		durMS  int
	}

	keys := make([]sortKey, 0, len(tests))
	seen := make(map[string]struct{}, len(tests))
	for _, id := range tests {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		k := sortKey{id: id}
		if m != nil {
			if r, ok := m.Get(id); ok {
				hits := 0
				for _, f := range r.F {
					if _, in := changed[f]; in {
						hits++
					}
				}
				if len(r.F) > 0 {
					k.ratio = float64(hits) / float64(len(r.F))
				}
				k.failed = r.S == "fail"
				k.nFiles = len(r.F)
				k.durMS = r.D
			}
		}
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.ratio != b.ratio {
			return a.ratio > b.ratio
		}
		if a.failed != b.failed {
			return a.failed
		}
		if a.nFiles != b.nFiles {
			return a.nFiles < b.nFiles
		}
		if a.durMS != b.durMS {
			return a.durMS < b.durMS
		}
		return a.id < b.id
	})

	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.id
	}
	return out
}
