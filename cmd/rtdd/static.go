package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/importscan"
)

// The impure halves of the static tier (spec §4.1). internal/selector is a pure function —
// no filesystem, no subprocess, no git — so the two resolvers it reads through
// selector.Inputs are built here, in cmd/, and injected by both `which` and `run`.
//
// Both commands must supply them. A resolver left nil is a LEVEL SKIPPED, not a level
// that failed, so a CLI that never supplied one produced a `TS` tier that could not be
// reached at all: correspondence was never consulted, and the T2 reason then said it had
// been. That is what issue #275 closes.

// staticSkipDirs are never descended into when enumerating the repository's test files.
// A vendored dependency tree carries its own tests, and selecting one would run somebody
// else's suite. It mirrors internal/adapter's detection skip set for the same reason.
var staticSkipDirs = map[string]bool{
	".git": true, ".venv": true, "venv": true, "node_modules": true,
	"__pycache__": true, ".tox": true, ".mypy_cache": true, ".pytest_cache": true,
	".rtdd": true,
}

// repoExists reports whether the repository has a given repo-relative path.
//
// It resolves test_for templates (spec §4.2) without the selector touching a filesystem.
// It answers for FILES only: a template names a test file, and a directory that happens to
// share its name is not one. A path that escapes the root is always false — the engine's
// paths never leave the repository, so one that tries is a bug, not a candidate.
func repoExists(root string) func(rel string) bool {
	abs, err := filepath.Abs(root)
	if err != nil {
		// An unresolvable root cannot answer anything; level 1 is skipped rather than
		// answered wrongly.
		return func(string) bool { return false }
	}
	return func(rel string) bool {
		if rel == "" {
			return false
		}
		clean := filepath.Clean(filepath.FromSlash(rel))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return false
		}
		info, err := os.Lstat(filepath.Join(abs, clean))
		return err == nil && info.Mode().IsRegular()
	}
}

// adapterImportDistance is level 2: the tests that transitively import a changed file,
// valued by hop count, from the adapter's DECLARED importscan. It returns the resolver and
// a reader for the first failure the scan recorded.
//
// The resolver is nil — never an empty function — when the adapter declares no scanner,
// because nil is how selector.Inputs spells "level 2 is skipped, and the reason must not
// claim it was consulted" (PRD #230 AC7). One scanner is built per command invocation and
// its result is memoised, so a changed file asked about twice costs one child process.
//
// The error reader is the second return rather than a discarded field: a declared scanner
// that fails narrows the selection, and the reason string then says no import reaches the
// changed set — a sentence about a level that could not run, which is the same defect
// class as the one issue #275 filed. The callers turn it into a warning instead.
func adapterImportDistance(root string, ad *adapter.Adapter, tests []string) (func(string) map[string]int, func() error) {
	if ad == nil || ad.Importscan == nil {
		return nil, func() error { return nil }
	}
	s := importscan.NewAdapterScanner(root, ad, tests)
	return s.Distances, s.Err
}

// staticTestCandidates enumerates the repository's own test files, for the declared
// scanner to rank against.
//
// The map cannot supply them: a `selection: static` adapter declares `coverage: none`, so
// it never builds a map, and `which` does not enumerate the suite — that costs a
// collection run it deliberately does not pay. A filesystem walk filtered through the
// adapter's own test globs answers the same question for free.
//
// It walks only when an adapter is present to classify with, and it is called only when
// that adapter declares an importscan, so the ordinary coverage path pays nothing.
func staticTestCandidates(root string, ad *adapter.Adapter) []string {
	if ad == nil {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable directory narrows the candidate list; it never fails the
			// command, exactly as a failed scan does not.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != filepath.Clean(root) && staticSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if ad.IsTestFile(rel) {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// staticResolvers builds the pair of resolvers selector.Inputs needs for the static tier.
//
// It exists so `which` and `run` cannot wire them differently: the advisory command and
// the executing command disagreeing about one tree is a defect this package has already
// shipped once (see the import fallback in run.go), and a second copy of this wiring is
// how it would ship again.
//
// The candidate test files are enumerated only when the adapter declares an importscan —
// nothing else reads them, so a coverage repository pays no walk.
func staticResolvers(root string, ad *adapter.Adapter) (func(string) bool, func(string) map[string]int, func() error) {
	var scanCandidates []string
	if ad != nil && ad.Importscan != nil {
		scanCandidates = staticTestCandidates(root, ad)
	}
	importDistance, scanErr := adapterImportDistance(root, ad, scanCandidates)
	return repoExists(root), importDistance, scanErr
}

// adapterImportScanNote is the one wording for a failed DECLARED import scan, shared by
// `run` and `which` so the two commands never describe the same degradation differently.
// It is the sibling of importScanNote, which reports RTDD's own embedded Python scan.
func adapterImportScanNote(name string, err error) string {
	return fmt.Sprintf("the %s adapter's importscan failed, so the static selection was "+
		"narrowed to declared correspondence only: %v", name, err)
}
