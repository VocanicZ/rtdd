package adapter

// Detection is the one part of this package that reads the filesystem. Everything it
// finds goes through internal/paths before it is matched, and it is matched with the
// same **-aware matcher classification uses — a marker that "exists" for detection but
// classifies differently would make `rtdd which` and `rtdd seed` disagree about the
// same repo.

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/paths"
)

// skipDirs are never descended into when looking for detection markers. A vendored
// dependency tree carries its own markers; walking into one detects the dependency's
// toolchain instead of the host repo's.
var skipDirs = map[string]bool{
	".git": true, ".venv": true, "venv": true, "node_modules": true,
	"__pycache__": true, ".tox": true, ".mypy_cache": true, ".pytest_cache": true,
	".rtdd": true,
}

// Detect returns EVERY adapter whose Detect globs match a file in repoRoot, in the order
// the adapters were given (adapter.Available sorts them by name), so a second call on an
// unchanged repo reproduces the same slice.
//
// Zero matches is an error: RTDD has no toolchain to run, which is a configuration error
// the CLI reports as exit 2.
//
// Two or more is NOT an error. Spec §4.4 makes a TypeScript service with a Python tooling
// directory ordinary, so the v1 arity rule is gone: it returned a message instead of a
// selection, which left such a repo on the null baseline of both toolchains rather than a
// narrowed suite from each. Each adapter's rows, selections and invocations carry its
// name instead.
//
// Markers are files. A directory named pyproject.toml declares nothing.
func Detect(repoRoot string, adapters []*Adapter) ([]*Adapter, error) {
	matched, err := DetectAll(repoRoot, adapters)
	if err != nil {
		return nil, err
	}
	if len(matched) == 0 {
		return nil, fmt.Errorf("adapter: no adapter detected in %s", repoRoot)
	}
	return matched, nil
}

// DetectAll walks repoRoot ONCE and reports every adapter with a matching marker, in the
// order the adapters were given (adapter.Available sorts them by name). One walk rather
// than one per pattern keeps detection linear in the size of the repo however many
// adapters are installed, and it stops early once nothing is left to decide.
//
// Detect is this walk plus the zero-match policy, and that policy is the only thing that
// separates them. Both survive because the two questions are genuinely different: `rtdd
// run` and `rtdd seed` cannot proceed without a toolchain and want the refusal, while
// `rtdd init --force`, `rtdd doctor` and the seed advice have something to say about a
// repo nothing matched and want the bare walk — and they must be able to tell an empty
// result apart from a walk that failed.
func DetectAll(repoRoot string, adapters []*Adapter) ([]*Adapter, error) {
	hit := make([]bool, len(adapters))
	undecided := 0
	for _, a := range adapters {
		if a != nil && len(a.Detect) > 0 {
			undecided++
		}
	}
	if undecided == 0 {
		return nil, nil
	}

	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != filepath.Clean(repoRoot) && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, ok := paths.Normalize(repoRoot, p)
		if !ok {
			return nil
		}
		for i, a := range adapters {
			if hit[i] || a == nil {
				continue
			}
			if matchAny(a.Detect, rel) {
				hit[i] = true
				undecided--
			}
		}
		if undecided == 0 {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("adapter: walking %s: %w", repoRoot, err)
	}

	var matched []*Adapter
	for i, a := range adapters {
		if hit[i] {
			matched = append(matched, a)
		}
	}
	return matched, nil
}
