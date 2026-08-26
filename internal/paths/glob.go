package paths

import (
	"path"
	"strings"
)

// MatchGlob reports whether rel matches a slash-separated glob pattern.
// "*" and "?" match within one path segment; "**" matches zero or more whole segments.
func MatchGlob(pattern, rel string) bool {
	if pattern == "" || rel == "" {
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
		ok, err := path.Match(pat[0], seg[0])
		if err != nil || !ok {
			return false
		}
		pat, seg = pat[1:], seg[1:]
	}
	return len(seg) == 0
}
