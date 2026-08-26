package mapstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveEmitsSortedLinesWithTrailingNewline(t *testing.T) {
	m := New()
	m.Replace(Row{T: "tests/test_b.py::test_x", F: []string{"src/b.py"}, C: "bbb1111", D: 5, S: "pass"})
	m.Replace(Row{T: "tests/test_a.py::test_y", F: []string{"src/db.py", "src/a.py"}, C: "aaa2222", D: 9, S: "fail"})

	path := filepath.Join(t.TempDir(), "nested", "map.jsonl")
	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(b)

	want := `{"t":"tests/test_a.py::test_y","f":["src/a.py","src/db.py"],"c":"aaa2222","d":9,"s":"fail"}
{"t":"tests/test_b.py::test_x","f":["src/b.py"],"c":"bbb1111","d":5,"s":"pass"}
`
	if got != want {
		t.Errorf("Save wrote:\n%q\nwant:\n%q", got, want)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Save MUST end with a trailing newline: a missing one joins two rows on the next union merge")
	}
}

func TestSaveEmptyMapWritesEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "map.jsonl")
	if err := New().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(b) != 0 {
		t.Errorf("empty map wrote %q, want an empty file", string(b))
	}
}

func TestSaveEmitsEmptyArrayNotNullForF(t *testing.T) {
	m := New()
	m.Replace(Row{T: "t1", F: nil, C: "aaa", D: 1, S: "pass"})
	path := filepath.Join(t.TempDir(), "map.jsonl")
	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), `"f":null`) {
		t.Errorf("Save emitted %q; F must serialise as [] so the row stays machine-readable", string(b))
	}
}
