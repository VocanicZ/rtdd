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
}

// Map is an in-memory map.jsonl keyed by test id.
type Map struct {
	rows map[string]Row
}

func New() *Map { return &Map{rows: make(map[string]Row)} }

func (m *Map) Get(t string) (Row, bool) {
	r, ok := m.rows[t]
	return r, ok
}

func (m *Map) Len() int { return len(m.rows) }

// Rows returns every row sorted by T ascending.
func (m *Map) Rows() []Row {
	out := make([]Row, 0, len(m.rows))
	for _, r := range m.rows {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

// Replace overwrites the row wholesale. ONLY rtdd seed may call this.
func (m *Map) Replace(r Row) { m.rows[r.T] = normalizeRow(r) }

func (m *Map) Delete(t string) { delete(m.rows, t) }

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
