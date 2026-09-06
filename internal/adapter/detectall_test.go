package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

// DetectAll is the walk; Detect is the walk plus the zero-match policy. Since spec §4.4
// the two agree on a polyglot repo — this assertion used to say "Detect accepted a
// polyglot repo; its arity check must still refuse one" and now says the opposite,
// because the arity check is gone. They still differ on an EMPTY result, which is why
// both functions survive: `rtdd init --force` and `rtdd doctor` want the bare walk,
// `rtdd run` and `rtdd seed` want the refusal.
func TestDetectAllAndDetectAgreeOnAPolyglotRepo(t *testing.T) {
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
	fromDetect, err := Detect(dir, adapters)
	if err != nil {
		t.Fatalf("Detect: %v, want no error — spec §4.4 makes a polyglot repo ordinary", err)
	}
	if len(fromDetect) != len(got) {
		t.Fatalf("Detect matched %d adapters, DetectAll matched %d; on a repo with a match they must agree",
			len(fromDetect), len(got))
	}
	for i := range got {
		if fromDetect[i] != got[i] {
			t.Errorf("Detect[%d] = %q, DetectAll[%d] = %q", i, fromDetect[i].Name, i, got[i].Name)
		}
	}
}

// The one question the two functions answer differently. DetectAll reports an empty set;
// Detect refuses, because a repo with no toolchain is a repo RTDD has nothing to run in.
func TestDetectRefusesTheEmptyResultDetectAllReports(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapters := []*Adapter{{Name: "python", Detect: []string{"pyproject.toml"}}}

	got, err := DetectAll(dir, adapters)
	if err != nil {
		t.Fatalf("DetectAll: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("DetectAll matched %d adapters, want 0", len(got))
	}
	if _, err := Detect(dir, adapters); err == nil {
		t.Error("Detect accepted a repo with no adapter; the zero-match refusal is what init gates on")
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
