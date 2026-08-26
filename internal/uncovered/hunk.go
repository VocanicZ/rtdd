// Package uncovered computes the three-way classification of changed lines
// (Covered / Uncovered / ImportTime) against fresh post-run coverage.
package uncovered

import (
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// ParseHunks parses `git diff --unified=0` output and returns, per NEW-file path,
// the line ranges that exist in the new file. Hunks whose new-side count is 0
// (pure deletions) contribute nothing. Files whose new side is /dev/null are omitted.
func ParseHunks(diff string) map[string][]gitctx.LineRange {
	out := map[string][]gitctx.LineRange{}
	cur := ""
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ ") {
			cur = newSidePath(strings.TrimSpace(line[4:]))
			continue
		}
		if cur == "" || !strings.HasPrefix(line, "@@ ") {
			continue
		}
		r, ok := parseHunkHeader(line)
		if !ok {
			continue
		}
		out[cur] = append(out[cur], r)
	}
	return out
}

// newSidePath turns the `+++` operand into a repo-relative path, or "" for /dev/null.
func newSidePath(p string) string {
	if strings.HasPrefix(p, "\"") {
		if uq, err := strconv.Unquote(p); err == nil {
			p = uq
		}
	}
	if p == "/dev/null" {
		return ""
	}
	if strings.HasPrefix(p, "b/") {
		p = p[2:]
	}
	return p
}

// parseHunkHeader reads the new-side range from "@@ -a,b +c,d @@ optional context".
// The context suffix may itself contain "@@", so the range region is cut at the FIRST
// following " @@", never the last. A missing count means 1; a count of 0 means the hunk
// adds no new lines and ok is false.
func parseHunkHeader(line string) (gitctx.LineRange, bool) {
	body := line[3:] // strip "@@ "
	if i := strings.Index(body, " @@"); i >= 0 {
		body = body[:i]
	}
	for _, f := range strings.Fields(body) {
		if !strings.HasPrefix(f, "+") {
			continue
		}
		spec := f[1:]
		startStr, countStr := spec, "1"
		if i := strings.IndexByte(spec, ','); i >= 0 {
			startStr, countStr = spec[:i], spec[i+1:]
		}
		start, err := strconv.Atoi(startStr)
		if err != nil {
			return gitctx.LineRange{}, false
		}
		count, err := strconv.Atoi(countStr)
		if err != nil || count <= 0 {
			return gitctx.LineRange{}, false
		}
		return gitctx.LineRange{Start: start, End: start + count - 1}, true
	}
	return gitctx.LineRange{}, false
}
