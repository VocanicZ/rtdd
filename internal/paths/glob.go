package paths

import (
	"fmt"
	"path"
	"strings"
)

// ValidateGlob reports whether pattern is a well-formed slash-separated glob.
//
// It exists so that a malformed pattern is a configuration error at load time rather
// than a silent non-match at classification time: adapter.Load runs it over every glob
// field, and a typo there would otherwise classify nothing as a test file while
// `rtdd which` reported "no test file changed".
func ValidateGlob(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty glob: a pattern that matches nothing declares nothing")
	}
	for _, seg := range strings.Split(pattern, "/") {
		// path.Match reports ErrBadPattern for a malformed pattern whatever the name is,
		// so an empty name is enough to validate the segment.
		if _, err := path.Match(seg, ""); err != nil {
			return fmt.Errorf("malformed glob %q: segment %q: %w", pattern, seg, err)
		}
	}
	return nil
}

// MatchGlob reports whether rel matches a slash-separated glob pattern.
// "*" and "?" match within one path segment; "**" matches zero or more whole segments.
//
// MatchGlob panics on a pattern that ValidateGlob rejects. Every pattern reaching it has
// already been validated at adapter.Load time, so a malformed one here is a programming
// error — and reporting it as a false would hide the bad pattern behind a plausible
// "this file is not a test file".
func MatchGlob(pattern, rel string) bool {
	if err := ValidateGlob(pattern); err != nil {
		panic("paths.MatchGlob: " + err.Error())
	}
	if rel == "" {
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}

func matchSegments(pat, seg []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if len(pat) == 1 {
				return true
			}
			for i := 0; i <= len(seg); i++ {
				if matchSegments(pat[1:], seg[i:]) {
					return true
				}
			}
			return false
		}
		if len(seg) == 0 {
			return false
		}
		// The pattern was validated above, so ErrBadPattern is unreachable here.
		if ok, err := path.Match(pat[0], seg[0]); err != nil || !ok {
			return false
		}
		pat, seg = pat[1:], seg[1:]
	}
	return len(seg) == 0
}
