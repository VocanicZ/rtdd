package graph

import "github.com/VocanicZ/rtdd/internal/paths"

// IsTestFile reports whether rel is a test file: it matches a TestFiles glob and no
// TestExclude glob (spec §6 — testdata/ and fixtures/ are never test files).
func IsTestFile(rel string, cfg Config) bool {
	return matchAny(cfg.TestFiles, rel) && !matchAny(cfg.TestExclude, rel)
}

// Classify sets IsTest on every node, whichever source produced it: a func, method or
// test node in a test file is a test; a class never is (spec §6).
func Classify(nodes []Node, cfg Config) {
	memo := map[string]bool{}
	for i := range nodes {
		n := &nodes[i]
		is, ok := memo[n.File]
		if !ok {
			is = IsTestFile(n.File, cfg)
			memo[n.File] = is
		}
		n.IsTest = is && n.Kind != KindClass
	}
}

func matchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if paths.MatchGlob(g, rel) {
			return true
		}
	}
	return false
}
