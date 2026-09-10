package main

import "github.com/VocanicZ/rtdd/internal/gitctx"

// memoDistance is `gitctx.CommitDistance` with an answer cache, per command invocation.
//
// The staleness scan (spec §4.1, T1) asks the age of the commit recorded on EVERY row of
// the T0 selection, and each miss costs three git subprocesses — `rev-parse --git-dir`,
// a reachability check, and `rev-list --count`. On a freshly seeded map every row carries
// the same sha, so a 437-row selection asked one question 437 times and paid roughly
// thirteen hundred process spawns for it.
//
// Measured on a real flask clone before this cache: `rtdd which` took 9774 ms, against
// 2932 ms to run the entire test suite and 751 ms to run the 437 tests it selected. The
// tool spent three times the cost of running everything deciding what to run. Afterwards
// the same command answers from one lookup.
//
// The cache is per invocation and never persisted: HEAD cannot move underneath a single
// `rtdd which`, and a cache that outlived the process would answer with a distance
// measured against a HEAD that has since changed.
func memoDistance(root string) func(string) int {
	return memoize(func(sha string) int {
		d, err := gitctx.CommitDistance(root, sha)
		if err != nil {
			return -1 // unknown, never "fresh"
		}
		return d
	})
}

// memoize caches one answer per distinct argument. Split out from memoDistance so the
// caching itself is testable without a git repository: the property that matters is that
// N lookups of one sha cost one call, and that is what the selector's row scan needs.
func memoize(f func(string) int) func(string) int {
	seen := make(map[string]int)
	return func(k string) int {
		if v, ok := seen[k]; ok {
			return v
		}
		v := f(k)
		seen[k] = v
		return v
	}
}
