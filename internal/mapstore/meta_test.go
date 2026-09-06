package mapstore

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadMetaMissingFileIsZeroAndNotAnError(t *testing.T) {
	got, err := LoadMeta(filepath.Join(t.TempDir(), "meta.json"))
	if err != nil {
		t.Fatalf("LoadMeta of a missing file returned %v, want nil", err)
	}
	if !reflect.DeepEqual(got, Meta{}) {
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
	if !reflect.DeepEqual(got, want) {
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

// Decision 4 of docs/plans/06-m6d-shipped-adapters.md: the plural key is added, the
// singular is kept, and a pre-PRD meta.json is still read correctly by the new binary.
// Turning `adapter` into a list would make every meta.json written before this PRD
// unreadable, and the singular is the answer to "whose is this untagged row?".
func TestMetaReadsThePluralKeyAndFallsBackToTheSingular(t *testing.T) {
	newer := Meta{V: 1, Adapter: "python", Adapters: []string{"python", "vitest"}}
	if got := newer.DetectedAdapters(); len(got) != 2 || got[0] != "python" || got[1] != "vitest" {
		t.Errorf("DetectedAdapters = %v, want [python vitest]", got)
	}
	prePRD := Meta{V: 1, Adapter: "python"}
	if got := prePRD.DetectedAdapters(); len(got) != 1 || got[0] != "python" {
		t.Errorf("DetectedAdapters on a pre-PRD meta = %v, want [python]", got)
	}
	empty := Meta{V: 1}
	if got := empty.DetectedAdapters(); len(got) != 0 {
		t.Errorf("DetectedAdapters on an empty meta = %v, want none", got)
	}
}

// `adapters` is omitempty for the same reason Row.A is: a repository that seeded before
// this PRD must not get a meta.json diff it did not ask for.
func TestSaveMetaOmitsTheAdaptersKeyWhenThereIsNoSet(t *testing.T) {
	p := filepath.Join(t.TempDir(), "meta.json")
	if err := SaveMeta(p, Meta{V: 1, Adapter: "python", SeededAt: "a3f21e0"}); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(b), "adapters") {
		t.Errorf("SaveMeta emitted an adapters key for a single-adapter meta: %s", b)
	}
}

// The detected set is what .rtdd/meta.json records (this issue's second acceptance
// criterion), so it has to survive the round trip.
func TestSaveMetaRoundTripsTheDetectedSet(t *testing.T) {
	p := filepath.Join(t.TempDir(), "meta.json")
	want := Meta{V: 1, Adapter: "python", Adapters: []string{"python", "vitest"}, SeededAt: "a3f21e0"}
	if err := SaveMeta(p, want); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}
	got, err := LoadMeta(p)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if len(got.Adapters) != 2 || got.Adapters[0] != "python" || got.Adapters[1] != "vitest" {
		t.Errorf("Adapters = %v, want [python vitest]", got.Adapters)
	}
}
