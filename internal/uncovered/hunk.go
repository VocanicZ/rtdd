// Package uncovered computes the three-way classification of changed lines
// (Covered / Uncovered / ImportTime) against fresh post-run coverage.
package uncovered

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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

// WithLines returns changes with Lines populated from rawDiff. It is authoritative:
// it OVERWRITES any Lines already present, so there is exactly one source of truth.
// Deleted changes get nil Lines. A change absent from rawDiff (an untracked file git
// diff never lists) gets the whole file as one range, counted from disk; a missing or
// empty file gets nil Lines.
//
// Hunks are keyed by the NEW-side path, which is the path a renamed Change carries in
// Path — matching on OldPath would find nothing and silently degrade the file to a
// whole-file range. The input slice is never mutated; changes are copied by value.
func WithLines(repoRoot string, changes []gitctx.Change, rawDiff string) ([]gitctx.Change, error) {
	hunks := ParseHunks(rawDiff)
	out := make([]gitctx.Change, 0, len(changes))
	for _, c := range changes {
		c.Lines = nil
		if c.Status == gitctx.Deleted {
			out = append(out, c)
			continue
		}
		if rs, ok := hunks[c.Path]; ok {
			c.Lines = rs
			out = append(out, c)
			continue
		}
		n, err := countLines(repoRoot, c.Path)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			c.Lines = []gitctx.LineRange{{Start: 1, End: n}}
		}
		out = append(out, c)
	}
	return out, nil
}

// countLines returns the number of lines in repoRoot/rel. A missing file is 0, not an
// error: a path can be listed as changed and then removed before the report runs. A
// final line with no trailing newline still counts.
func countLines(repoRoot, rel string) (int, error) {
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("uncovered: counting lines of %s: %w", rel, err)
	}
	if len(b) == 0 {
		return 0, nil
	}
	n := bytes.Count(b, []byte{'\n'})
	if b[len(b)-1] != '\n' {
		n++
	}
	return n, nil
}
