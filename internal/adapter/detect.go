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
	"strings"

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

// Detect returns the adapter whose Detect globs match a file in repoRoot.
//
// Exactly one match is required. Zero is an error because RTDD has no toolchain to run;
// two or more is an error because a polyglot repo is out of scope in v1 (spec §11) and
// silently picking one would seed a map from the wrong test suite. Both are configuration
// errors — the CLI reports them as exit 2.
//
// Markers are files. A directory named pyproject.toml declares nothing.
func Detect(repoRoot string, adapters []*Adapter) (*Adapter, error) {
	matched, err := DetectAll(repoRoot, adapters)
	if err != nil {
		return nil, err
	}
	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return nil, fmt.Errorf("adapter: no adapter detected in %s", repoRoot)
	default:
		names := make([]string, 0, len(matched))
		for _, a := range matched {
			names = append(names, a.Name)
		}
		return nil, fmt.Errorf("adapter: %d adapters detected in %s (%s); polyglot repos are out of scope in v1",
			len(matched), repoRoot, strings.Join(names, ", "))
	}
}

// DetectAll walks repoRoot ONCE and reports every adapter with a matching marker, in the
// order the adapters were given (adapter.Available sorts them by name). One walk rather
// than one per pattern keeps detection linear in the size of the repo however many
// adapters are installed, and it stops early once nothing is left to decide.
//
// Detect stays the one-adapter arity check over it. `rtdd init` gates on "at least one"
// (spec §5), which is a weaker question than selection asks: a repo two adapters match
// is a repo RTDD can be installed into, even though v1 will not select in it.
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
