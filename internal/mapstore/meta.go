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
	V        int    `json:"v"`
	Adapter  string `json:"adapter"`
	SeededAt string `json:"seeded_at"`
	Cycles   int    `json:"cycles"`
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
