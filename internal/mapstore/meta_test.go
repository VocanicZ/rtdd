package mapstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMetaMissingFileIsZeroAndNotAnError(t *testing.T) {
	got, err := LoadMeta(filepath.Join(t.TempDir(), "meta.json"))
	if err != nil {
		t.Fatalf("LoadMeta of a missing file returned %v, want nil", err)
	}
	if got != (Meta{}) {
		t.Errorf("LoadMeta = %+v, want the zero Meta", got)
	}
}

func TestMetaRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "meta.json")
	want := Meta{V: 1, Adapter: "python", SeededAt: "a3f21e0", Cycles: 7}
	if err := SaveMeta(p, want); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != `{"v":1,"adapter":"python","seeded_at":"a3f21e0","cycles":7}`+"\n" {
		t.Errorf("SaveMeta wrote %q", string(b))
	}
	got, err := LoadMeta(p)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if got != want {
		t.Errorf("LoadMeta = %+v, want %+v", got, want)
	}
}

func TestLoadMetaIsFatalOnMalformedJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "meta.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMeta(p); err == nil {
		t.Fatal("LoadMeta returned nil error for malformed JSON")
	}
}
