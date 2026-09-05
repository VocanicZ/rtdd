package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnsupportedToolchainsNamesTheMarkerAndTheLanguage(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", "{}\n")
	write(t, dir, "Cargo.toml", "[package]\n")

	got := UnsupportedToolchains(dir)
	joined := strings.Join(got, ", ")
	for _, want := range []string{"package.json (JavaScript/TypeScript)", "Cargo.toml (Rust)"} {
		if !strings.Contains(joined, want) {
			t.Errorf("UnsupportedToolchains = %q, want it to name %q", joined, want)
		}
	}
	// Sorted, so one repo always produces one message.
	if len(got) != 2 || got[0] != "Cargo.toml (Rust)" {
		t.Errorf("UnsupportedToolchains = %#v, want it sorted", got)
	}
}

// The `*.csproj` entry is a pattern, not a file name: the project file is named after the
// project, so a literal match would never fire.
func TestUnsupportedToolchainsMatchesTheCsprojPattern(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "Payments.csproj", "<Project/>\n")

	got := UnsupportedToolchains(dir)
	if len(got) != 1 || got[0] != "Payments.csproj (C#)" {
		t.Errorf("UnsupportedToolchains = %#v, want [Payments.csproj (C#)]", got)
	}
}

// A repo with nothing recognisable gets no line at all rather than an empty one, and a
// directory named like a marker declares nothing.
func TestUnsupportedToolchainsIgnoresDirectoriesAndUnknownRepos(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "package.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "README.md", "hi\n")

	if got := UnsupportedToolchains(dir); len(got) != 0 {
		t.Errorf("UnsupportedToolchains = %#v, want none", got)
	}
}

// The table is a message, never a claim of support: nothing in it may collide with an
// adapter that actually exists, or the refusal would name a language RTDD does serve.
func TestUnsupportedMarkersNameNoDetectableToolchain(t *testing.T) {
	built, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	for marker := range UnsupportedMarkers {
		for _, a := range built {
			if matchAny(a.Detect, marker) {
				t.Errorf("UnsupportedMarkers lists %q, which the built-in %q adapter detects", marker, a.Name)
			}
		}
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
