package mapstore

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestLoadMissingFileIsEmptyAndNotAnError(t *testing.T) {
	m, err := Load(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	if err != nil {
		t.Fatalf("Load of a missing file returned %v, want nil", err)
	}
	if m == nil || m.Len() != 0 {
		t.Fatalf("Load of a missing file must return an empty Map, got %+v", m)
	}
}

func TestLoadResolvesDuplicateTLinesLikeUnion(t *testing.T) {
	m, err := LoadWith("testdata/dup.jsonl", olderLexical)
	if err != nil {
		t.Fatalf("LoadWith: %v", err)
	}
	if m.Len() != 2 {
		t.Fatalf("Len() = %d, want 2 (the two lines for the same t must collapse)", m.Len())
	}
	got, ok := m.Get("tests/test_auth.py::test_login")
	if !ok {
		t.Fatalf("the duplicated test id is missing from the map")
	}
	wantF := []string{"src/auth.py", "src/db.py", "src/session.py"}
	if !reflect.DeepEqual(got.F, wantF) {
		t.Errorf("F = %#v, want %#v (union of both lines; a union merge may never narrow)", got.F, wantF)
	}
	if got.C != "aaa1111" {
		t.Errorf("C = %q, want %q (the OLDER commit wins)", got.C, "aaa1111")
	}
	if got.D != 88 || got.S != "fail" {
		t.Errorf("D/S = %d/%q, want 88/fail (the last line's values win)", got.D, got.S)
	}
}

func TestLoadIsFatalOnAMalformedLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantIn  string
	}{
		{"not json at all", "this is not json\n", "line 1"},
		{"row with no t", "{\"f\":[\"src/a.py\"],\"c\":\"aaa\",\"d\":1,\"s\":\"pass\"}\n", "empty"},
		{"truncated json", "{\"t\":\"t1\",\"f\":[\"src/a.py\"\n", "line 1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "map.jsonl")
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			m, err := Load(p)
			if err == nil {
				t.Fatalf("Load returned nil error and %d rows; a malformed line MUST be fatal, never a silent skip", m.Len())
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.wantIn)
			}
		})
	}
}

func TestLoadIsFatalOnTwoRowsJoinedByAMissingNewline(t *testing.T) {
	m, err := Load("testdata/malformed.jsonl")
	if err == nil {
		t.Fatalf("Load returned nil error and %d rows; two rows joined on one line MUST be fatal", m.Len())
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q does not point at line 2", err.Error())
	}
}

func TestLoadSkipsBlankLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "map.jsonl")
	content := "{\"t\":\"t1\",\"f\":[\"src/a.py\"],\"c\":\"aaa\",\"d\":1,\"s\":\"pass\"}\n\n   \n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Len() != 1 {
		t.Errorf("Len() = %d, want 1", m.Len())
	}
}

// Compaction (rtdd map compact) is LoadWith followed by Save: LoadWith resolves the
// duplicate lines a union merge leaves behind, Save writes exactly one line per t.
func TestCompactionRoundTrip(t *testing.T) {
	src, err := os.ReadFile("testdata/dup.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "map.jsonl")
	if err := os.WriteFile(p, src, 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadWith(p, olderLexical)
	if err != nil {
		t.Fatalf("LoadWith: %v", err)
	}
	if err := m.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"t":"tests/test_auth.py::test_login","f":["src/auth.py","src/db.py","src/session.py"],"c":"aaa1111","d":88,"s":"fail"}
{"t":"tests/test_db.py::test_query","f":["src/db.py"],"c":"bbb2222","d":15,"s":"pass"}
`
	if string(b) != want {
		t.Errorf("compaction produced:\n%s\nwant:\n%s", string(b), want)
	}

	// Compaction is idempotent: a second round trip is a no-op.
	m2, err := LoadWith(p, olderLexical)
	if err != nil {
		t.Fatalf("second LoadWith: %v", err)
	}
	if err := m2.Save(p); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	b2, _ := os.ReadFile(p)
	if string(b2) != string(b) {
		t.Errorf("compaction is not idempotent:\nfirst:\n%s\nsecond:\n%s", string(b), string(b2))
	}
}
