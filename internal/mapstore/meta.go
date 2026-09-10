package mapstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Meta is .rtdd/meta.json. It is kept out of map.jsonl because the JSONL is union-merged
// and these fields must not be duplicated by a merge.
type Meta struct {
	V int `json:"v"`
	// Adapter is the COVERAGE adapter that produced the map, and it is kept rather than
	// widened into a list: turning it into one would make every meta.json written before
	// the detected set existed unreadable by the new binary and every meta.json written
	// after it unreadable by the old one. It is also the answer to "whose is this untagged
	// row?" that Map.TestsCoveringFor asks, and deleting the field deletes that answer.
	Adapter string `json:"adapter"`
	// Adapters is the full detected set at seed time, sorted. `omitempty` for the same
	// reason Row.A carries it: a repository that seeded before the set existed must not
	// get a meta.json diff it never asked for.
	Adapters []string `json:"adapters,omitempty"`
	SeededAt string   `json:"seeded_at"`
	Cycles   int      `json:"cycles"`
	// EscalateDigest names the state of the adapter's `full_escalate` files that the
	// last COMPLETED full run covered. The selector compares it with the working
	// tree's current state to tell "the config changed" from "the config changed and
	// nothing has run everything since"; only the second is a reason to run
	// everything. `omitempty` for the reason Adapters carries it: a repository that
	// seeded before the field existed must not get a meta.json diff it never asked
	// for, and an absent record escalates exactly as it did before.
	EscalateDigest string `json:"escalate_digest,omitempty"`
}

// DetectedAdapters is the set this Meta names, newest key first: `adapters` when present,
// otherwise the singular `adapter`, otherwise nothing. A pre-PRD meta.json has only the
// singular, and reading it as a one-element set is what keeps a repository that seeded
// under an older rtdd selecting exactly as it did before.
func (m Meta) DetectedAdapters() []string {
	if len(m.Adapters) > 0 {
		return append([]string(nil), m.Adapters...)
	}
	if m.Adapter != "" {
		return []string{m.Adapter}
	}
	return nil
}

// LoadMeta reads .rtdd/meta.json. A missing file returns the zero Meta and a nil error.
func LoadMeta(path string) (Meta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Meta{}, nil
		}
		return Meta{}, fmt.Errorf("mapstore: read %s: %w", path, err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, fmt.Errorf("mapstore: %s: malformed meta: %w", path, err)
	}
	return m, nil
}

// SaveMeta writes m to path as one line of JSON with a trailing newline, creating the
// containing directory if it does not exist.
func SaveMeta(path string, m Meta) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mapstore: mkdir %s: %w", dir, err)
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("mapstore: marshal meta: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("mapstore: write %s: %w", path, err)
	}
	return nil
}
