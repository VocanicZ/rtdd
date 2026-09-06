// Package mapstore owns .rtdd/map.jsonl and .rtdd/meta.json: the committed,
// union-merged, file-level test-to-source relation.
package mapstore

import "sort"

// Row is one line of map.jsonl: one test, and the source files it executed.
type Row struct {
	T string   `json:"t"` // test id, exactly as the runner accepts it as a selector
	F []string `json:"f"` // repo-relative source files, sorted, deduped
	C string   `json:"c"` // short SHA of HEAD when recorded
	D int      `json:"d"` // last duration, ms
	S string   `json:"s"` // last outcome: "pass" | "fail" | "skip" | "error"
	// A is the adapter that produced the row (PRD #232 AC6). `omitempty` is not
	// cosmetic: it keeps every byte of an existing .rtdd/map.jsonl valid and
	// unrewritten, so upgrading rtdd does not produce a whole-file diff in every host
	// repository that has ever seeded. An EMPTY A is an untagged row — written before
	// this key existed — and belongs to the adapter .rtdd/meta.json names, and to
	// nothing else; see TestsCoveringFor.
	A string `json:"a,omitempty"`
}

// Map is an in-memory map.jsonl keyed by adapter AND test id. Keying on the id alone
// would make two ecosystems that name one test the same string — a Go package path and
// a Cargo target of the same name, say — overwrite each other, which is exactly the
// collision the adapter tag exists to survive.
type Map struct {
	rows map[string]Row
}

// rowKey is the internal identity of a row: its adapter and its test id, joined by a
// byte no test id and no adapter name may contain.
func rowKey(adapter, t string) string { return adapter + "\x00" + t }

func New() *Map { return &Map{rows: make(map[string]Row)} }

// Get returns the row with this test id, whichever adapter produced it. Callers that
// hold an id but no adapter — rtdd explain, the ranker — ask this. When two adapters
// share one id the lowest adapter name wins, so the answer is deterministic; callers
// that must not cross adapters use GetFor.
func (m *Map) Get(t string) (Row, bool) {
	var best Row
	found := false
	for _, r := range m.rows {
		if r.T != t {
			continue
		}
		if !found || r.A < best.A {
			best, found = r, true
		}
	}
	return best, found
}

func (m *Map) Len() int { return len(m.rows) }

// Rows returns every row sorted by T ascending, then by A ascending so two adapters
// sharing one test id have a stable order and Save stays byte-deterministic.
func (m *Map) Rows() []Row {
	out := make([]Row, 0, len(m.rows))
	for _, r := range m.rows {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].T != out[j].T {
			return out[i].T < out[j].T
		}
		return out[i].A < out[j].A
	})
	return out
}

// Replace overwrites the row wholesale. ONLY rtdd seed may call this.
func (m *Map) Replace(r Row) { m.rows[rowKey(r.A, r.T)] = normalizeRow(r) }

// Delete removes every row with this test id, whichever adapter produced it.
func (m *Map) Delete(t string) {
	for k, r := range m.rows {
		if r.T == t {
			delete(m.rows, k)
		}
	}
}

// normalizeRow copies F, sorts it, dedupes it, and guarantees a non-nil slice so the
// JSONL never emits `"f":null`.
func normalizeRow(r Row) Row {
	f := append([]string(nil), r.F...)
	sort.Strings(f)
	out := make([]string, 0, len(f))
	for i, s := range f {
		if i > 0 && s == f[i-1] {
			continue
		}
		out = append(out, s)
	}
	r.F = out
	return r
}

// Union merges r into the map: F becomes the set-union, D and S take r's values,
// and C takes the OLDER of the two commits (a row's F is only as trustworthy as its
// stalest component). Used by every path except seed.
func (m *Map) Union(r Row, older func(a, b string) string) {
	r = normalizeRow(r)
	key := rowKey(r.A, r.T)
	prev, ok := m.rows[key]
	if !ok {
		m.rows[key] = r
		return
	}
	m.rows[key] = Row{
		T: r.T,
		F: unionStrings(prev.F, r.F),
		C: pickOlder(prev.C, r.C, older),
		D: r.D,
		S: r.S,
		A: r.A,
	}
}

// pickOlder resolves C. With a nil comparator the existing value wins, which is
// deterministic but age-blind; callers that have git available pass gitctx.Older.
func pickOlder(existing, incoming string, older func(a, b string) string) string {
	switch {
	case existing == "":
		return incoming
	case incoming == "":
		return existing
	case existing == incoming:
		return existing
	case older == nil:
		return existing
	}
	return older(existing, incoming)
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, group := range [][]string{a, b} {
		for _, s := range group {
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// TestsCovering returns every test id whose F intersects any of files, across every
// adapter and including untagged rows. Unsorted, deduplicated by id. rtdd explain and
// the drift guard ask "which tests cover this file" with no adapter in hand, and
// answering that with the whole map is right; anything that will hand an id to a
// runner must use TestsCoveringFor instead.
func (m *Map) TestsCovering(files []string) []string {
	return m.testsCovering(files, func(Row) bool { return true })
}

// TestsCoveringFor is TestsCovering restricted to one adapter's rows (PRD #232 AC6): a
// row written by one adapter is never served to another. Serving a pytest row to the
// vitest adapter hands `npx vitest run` a pytest nodeid, which selects nothing and
// reports green — a false pass wearing a real id.
//
// legacyAdapter is .rtdd/meta.json's singular `adapter` field, which records the
// adapter a pre-PRD repository seeded with. An UNTAGGED row belongs to that adapter and
// to no other. When legacyAdapter is empty — a hand-made map, a corrupted meta — an
// untagged row is ignored for selection and preserved on write: serving it to an
// arbitrary adapter is the very violation AC6 forbids, and deleting it would throw away
// a map the user paid a full seed for.
func (m *Map) TestsCoveringFor(adapterName, legacyAdapter string, files []string) []string {
	return m.testsCovering(files, func(r Row) bool {
		if r.A != "" {
			return r.A == adapterName
		}
		return legacyAdapter != "" && legacyAdapter == adapterName
	})
}

func (m *Map) testsCovering(files []string, keep func(Row) bool) []string {
	if len(files) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(files))
	for _, f := range files {
		want[f] = struct{}{}
	}
	seen := make(map[string]struct{}, len(m.rows))
	out := make([]string, 0, len(m.rows))
	for _, r := range m.rows {
		if !keep(r) {
			continue
		}
		if _, dup := seen[r.T]; dup {
			continue
		}
		for _, f := range r.F {
			if _, hit := want[f]; hit {
				seen[r.T] = struct{}{}
				out = append(out, r.T)
				break
			}
		}
	}
	return out
}

// FanOut returns file -> number of tests whose F contains it.
func (m *Map) FanOut() map[string]int {
	out := make(map[string]int)
	for _, r := range m.rows {
		for _, f := range r.F {
			out[f]++
		}
	}
	return out
}
