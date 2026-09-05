package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

// DetectAll answers the question `rtdd init` asks — "is there at least one?" — which is
// weaker than the one Detect asks. A repo two adapters match is still a repo RTDD can be
// installed into, so DetectAll reports both where Detect refuses.
func TestDetectAllReportsEveryMatchWhereDetectRefuses(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"pyproject.toml", "package.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	adapters := []*Adapter{
		{Name: "python", Detect: []string{"pyproject.toml"}},
		{Name: "vitest", Detect: []string{"package.json"}},
	}

	got, err := DetectAll(dir, adapters)
	if err != nil {
		t.Fatalf("DetectAll: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("DetectAll matched %d adapters, want 2", len(got))
	}
	if _, err := Detect(dir, adapters); err == nil {
		t.Error("Detect accepted a polyglot repo; its arity check must still refuse one")
	}
}

func TestDetectAllReportsNoneWhenNothingMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DetectAll(dir, []*Adapter{{Name: "python", Detect: []string{"pyproject.toml"}}})
	if err != nil {
		t.Fatalf("DetectAll: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("DetectAll matched %d adapters in a repo with no marker, want 0", len(got))
	}
}
