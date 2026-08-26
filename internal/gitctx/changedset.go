package gitctx

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ChangedSet returns the union of `git diff --name-only --unified=0 <base>` and
// `git status --porcelain -uall`. Untracked files are included (Status=Added with the
// whole file as one LineRange). Deletions are retained.
//
// The union is not an optimisation. `git diff` does not list a file that was written but
// never added, and a just-written file is the most common input in a TDD cycle; omitting
// it is what made v1's flagship case invisible.
func ChangedSet(repoRoot, base string) ([]Change, error) {
	if strings.TrimSpace(base) == "" {
		base = "HEAD"
	}

	hasBase := true
	if _, err := git(repoRoot, "rev-parse", "--verify", "-q", base+"^{commit}"); err != nil {
		if base != "HEAD" {
			return nil, fmt.Errorf("gitctx: unknown base %q", base)
		}
		hasBase = false // a repository with no commits yet
	}

	changes := map[string]*Change{}

	if hasBase {
		nameStatus, err := git(repoRoot, "diff", "--name-status", "-M", "-z", base)
		if err != nil {
			return nil, err
		}
		fields := splitZ(nameStatus)
		for i := 0; i < len(fields); {
			code := fields[i]
			i++
			if code == "" {
				continue
			}
			switch code[0] {
			case 'R', 'C':
				if i+1 >= len(fields) {
					return nil, fmt.Errorf("gitctx: truncated rename record from git diff --name-status")
				}
				oldPath, newPath := fields[i], fields[i+1]
				i += 2
				changes[newPath] = &Change{Path: newPath, OldPath: oldPath, Status: Renamed}
			default:
				if i >= len(fields) {
					return nil, fmt.Errorf("gitctx: truncated record from git diff --name-status")
				}
				p := fields[i]
				i++
				changes[p] = &Change{Path: p, Status: statusFromDiffCode(code[0])}
			}
		}

		diff, err := git(repoRoot, "diff", "--unified=0", "--no-color", "-M", base)
		if err != nil {
			return nil, err
		}
		for p, ranges := range parseHunks(diff) {
			if c, ok := changes[p]; ok {
				c.Lines = append(c.Lines, ranges...)
			}
		}
	}

	porcelain, err := git(repoRoot, "status", "--porcelain", "-uall", "-z")
	if err != nil {
		return nil, err
	}
	// Paths already recorded as the source of a rename must not be re-added as plain
	// modifications: git status reports both halves of a rename, and the diff pass has
	// already attached the old path to its Change via OldPath.
	renameSources := map[string]struct{}{}
	for _, c := range changes {
		if c.OldPath != "" {
			renameSources[c.OldPath] = struct{}{}
		}
	}

	pf := splitZ(porcelain)
	for i := 0; i < len(pf); {
		rec := pf[i]
		i++
		if len(rec) < 4 {
			continue
		}
		x, y := rec[0], rec[1]
		p := rec[3:]
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			if i < len(pf) {
				i++ // consume the origin-path field that accompanies a rename record
			}
		}
		if _, seen := changes[p]; seen {
			continue
		}
		if _, isSource := renameSources[p]; isSource {
			continue
		}
		if x == '?' && y == '?' {
			changes[p] = &Change{Path: p, Status: Added, Lines: wholeFile(repoRoot, p)}
			continue
		}
		changes[p] = &Change{Path: p, Status: statusFromPorcelain(x, y)}
	}

	out := make([]Change, 0, len(changes))
	for _, c := range changes {
		if c.Status == Deleted {
			c.Lines = nil
		} else {
			c.Lines = mergeRanges(c.Lines)
		}
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func statusFromDiffCode(c byte) Status {
	switch c {
	case 'A':
		return Added
	case 'D':
		return Deleted
	case 'R', 'C':
		return Renamed
	default:
		return Modified
	}
}

func statusFromPorcelain(x, y byte) Status {
	for _, c := range []byte{x, y} {
		switch c {
		case 'A':
			return Added
		case 'D':
			return Deleted
		case 'R', 'C':
			return Renamed
		case 'M', 'T', 'U':
			return Modified
		}
	}
	return Modified
}

// splitZ splits NUL-separated git output, dropping empty trailing fields.
func splitZ(s string) []string {
	parts := strings.Split(s, "\x00")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// wholeFile returns the whole of an untracked file as one LineRange.
func wholeFile(repoRoot, rel string) []LineRange {
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil || len(b) == 0 {
		return nil
	}
	n := bytes.Count(b, []byte("\n"))
	if b[len(b)-1] != '\n' {
		n++
	}
	if n == 0 {
		return nil
	}
	return []LineRange{{Start: 1, End: n}}
}

// parseHunks extracts NEW-file line ranges from a `git diff --unified=0` body,
// keyed by the repo-relative path in the `+++ b/<path>` header.
func parseHunks(diff string) map[string][]LineRange {
	out := map[string][]LineRange{}
	cur := ""
	sc := bufio.NewScanner(strings.NewReader(diff))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "+++ "):
			p := strings.TrimPrefix(line, "+++ ")
			if i := strings.IndexByte(p, '\t'); i >= 0 {
				p = p[:i]
			}
			if p == "/dev/null" {
				cur = ""
				continue
			}
			cur = strings.TrimPrefix(p, "b/")
		case strings.HasPrefix(line, "@@") && cur != "":
			if r, ok := parseHunkHeader(line); ok {
				out[cur] = append(out[cur], r)
			}
		}
	}
	return out
}

// parseHunkHeader parses "@@ -a,b +c,d @@ ..." and returns the NEW-side range.
func parseHunkHeader(line string) (LineRange, bool) {
	i := strings.IndexByte(line, '+')
	if i < 0 {
		return LineRange{}, false
	}
	rest := line[i+1:]
	j := strings.IndexAny(rest, " @")
	if j < 0 {
		return LineRange{}, false
	}
	spec := rest[:j]
	startStr, countStr := spec, "1"
	if k := strings.IndexByte(spec, ','); k >= 0 {
		startStr, countStr = spec[:k], spec[k+1:]
	}
	start, err1 := strconv.Atoi(startStr)
	count, err2 := strconv.Atoi(countStr)
	if err1 != nil || err2 != nil {
		return LineRange{}, false
	}
	if count == 0 {
		// A pure deletion: attribute it to the surviving line it was removed after.
		if start == 0 {
			start = 1
		}
		return LineRange{Start: start, End: start}, true
	}
	return LineRange{Start: start, End: start + count - 1}, true
}

// mergeRanges sorts and coalesces adjacent or overlapping ranges.
func mergeRanges(rs []LineRange) []LineRange {
	if len(rs) == 0 {
		return nil
	}
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].Start != rs[j].Start {
			return rs[i].Start < rs[j].Start
		}
		return rs[i].End < rs[j].End
	})
	out := []LineRange{rs[0]}
	for _, r := range rs[1:] {
		last := &out[len(out)-1]
		if r.Start <= last.End+1 {
			if r.End > last.End {
				last.End = r.End
			}
			continue
		}
		out = append(out, r)
	}
	return out
}
