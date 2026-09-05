package adapter

// Marker files for toolchains no shipped adapter serves. This table exists for ONE
// message — the one `rtdd init` prints when nothing matched — and for nothing else. An
// entry here is not a claim of support, it is never consulted by detection, selection or
// classification, and adding one supports no language: only an adapter does that.
//
// Spec §5 requires the refusal to name what the repository plainly does contain, because
// "no adapter detected" alone reads as a broken install rather than as a missing adapter.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// UnsupportedMarkers maps a well-known toolchain marker to the language it announces.
// The key is either a literal file name or a `*.ext` pattern matched against the names
// in the repo root.
var UnsupportedMarkers = map[string]string{
	"package.json":     "JavaScript/TypeScript",
	"go.mod":           "Go",
	"Cargo.toml":       "Rust",
	"pom.xml":          "Java (Maven)",
	"build.gradle":     "Java/Kotlin (Gradle)",
	"build.gradle.kts": "Java/Kotlin (Gradle)",
	"Gemfile":          "Ruby",
	"composer.json":    "PHP",
	"mix.exs":          "Elixir",
	"*.csproj":         "C#",
}

// UnsupportedToolchains names the toolchain markers present in repoRoot, as
// `"package.json (JavaScript/TypeScript)"` strings sorted by marker.
//
// It reads the ROOT only, non-recursively: a marker deep in the tree is as likely to
// belong to a fixture or an example as to the repository itself, and naming one would
// make the refusal message wrong in exactly the repos it is trying to help. A directory
// named like a marker declares nothing, so only regular files count.
func UnsupportedToolchains(repoRoot string) []string {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var found []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		for marker, lang := range UnsupportedMarkers {
			hit := marker == e.Name()
			if !hit && strings.HasPrefix(marker, "*.") {
				hit = filepath.Ext(e.Name()) == marker[1:]
			}
			if !hit {
				continue
			}
			line := e.Name() + " (" + lang + ")"
			if !seen[line] {
				seen[line] = true
				found = append(found, line)
			}
		}
	}
	sort.Strings(found)
	return found
}
