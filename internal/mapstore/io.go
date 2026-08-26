package mapstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Save writes the map as JSONL sorted by T ascending. Every line, including the last,
// is terminated with "\n": map.jsonl is merged with the union driver, and a missing
// final newline joins the last row of one side to the first row of the other, producing
// a line no parser can read.
func (m *Map) Save(path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mapstore: mkdir %s: %w", dir, err)
		}
	}
	var buf bytes.Buffer
	for _, r := range m.Rows() {
		b, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("mapstore: marshal %q: %w", r.T, err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("mapstore: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("mapstore: rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
