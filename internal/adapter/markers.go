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
//
// Every entry names a marker NO shipped adapter detects, and that is checked:
// TestUnsupportedMarkersNameNoDetectableToolchain fails on a collision, because a marker
// an adapter matches can never reach this message — detection would have succeeded — and
// naming a language RTDD does serve in a refusal is worse than saying nothing. The
// shipped set therefore removed `go.mod`, `pom.xml`, `build.gradle`, `build.gradle.kts`
// and `*.csproj` from this table when the go, maven, gradle and dotnet adapters shipped.
//
// The four that remain are the ones whose ecosystem RTDD serves only through a NARROWER
// marker — the runner's own config file rather than the ecosystem's manifest (plan
// 06-m6d decision 1). `package.json` is not a vitest or jest marker, `Cargo.toml` is not
// a cargo-nextest one, `Gemfile` is not an rspec one and `composer.json` is not a phpunit
// one, so a repo of that ecosystem with no runner config detects nothing — and this is
// the message that tells it so, and points at the six-line .rtdd/adapters/ fix.
var UnsupportedMarkers = map[string]string{
	"package.json":  "JavaScript/TypeScript",
	"Cargo.toml":    "Rust",
	"Gemfile":       "Ruby",
	"composer.json": "PHP",
	"mix.exs":       "Elixir",
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
