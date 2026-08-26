package mapstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Load reads map.jsonl. A missing file returns an empty Map and a nil error.
// Duplicate `t` lines are resolved with a nil comparator (first line's C wins).
func Load(path string) (*Map, error) { return LoadWith(path, nil) }

// LoadWith reads map.jsonl, resolving duplicate `t` lines exactly as Union does:
// F is set-unioned and C takes the older commit per the comparator. Duplicates are
// expected — the union merge driver leaves both lines whenever two agents touched the
// same test. A malformed line is a FATAL error, never a silent skip: dropping a row
// narrows selection, and selection may never silently narrow.
func LoadWith(path string, older func(a, b string) string) (*Map, error) {
	m := New()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, fmt.Errorf("mapstore: open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(line))
		var r Row
		if err := dec.Decode(&r); err != nil {
			return nil, fmt.Errorf("mapstore: %s line %d: malformed row: %w", path, lineNo, err)
		}
		if dec.More() {
			return nil, fmt.Errorf("mapstore: %s line %d: trailing content after the row "+
				"(two rows joined by a missing trailing newline?)", path, lineNo)
		}
		if r.T == "" {
			return nil, fmt.Errorf("mapstore: %s line %d: row has an empty \"t\"", path, lineNo)
		}
		m.Union(r, older)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("mapstore: read %s: %w", path, err)
	}
	return m, nil
}
