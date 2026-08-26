package adapter

import (
	"path"

	"github.com/VocanicZ/rtdd/internal/paths"
)

// matchGlob reports whether the slash-separated path name matches pattern.
//
// It understands `**` as "zero or more path segments"; the stdlib's path.Match does not,
// and every adapter glob field in adapters/*.yaml uses it. The segment walk itself lives
// in internal/paths so that the engine has exactly one glob semantics; matchGlob adds the
// one thing classification needs on top of it — tolerance for a name that has not been
// through paths.Normalize yet.
//
// It is pure string work: it never touches the filesystem, so a path that does not exist
// classifies exactly like one that does.
func matchGlob(pattern, name string) bool {
	if pattern == "" {
		return false
	}
	name = path.Clean(name)
	if name == "" || name == "." {
		return false
	}
	return paths.MatchGlob(pattern, name)
}

// matchAny reports whether rel matches any of patterns. An empty pattern list matches
// nothing, which is what an adapter that declares no globs for a field should mean.
func matchAny(patterns []string, rel string) bool {
	for _, p := range patterns {
		if matchGlob(p, rel) {
			return true
		}
	}
	return false
}
