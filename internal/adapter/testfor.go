package adapter

import (
	"path"
	"strings"
)

// TestForCandidate resolves this adapter's test_for templates against one changed source
// file and returns the FIRST template naming a file the repository actually has.
//
// Templates are tried in declaration order because the order is the adapter author's
// confidence ranking: "{dir}/{name}.test.ts" before "tests/{name}.test.ts" says
// co-located tests are the convention here and the central directory is the fallback.
// Resolving to an existing file, rather than to a plausible path, is what makes level 1
// of spec §4.1 evidence rather than a guess — a template always expands, so expansion
// alone would nominate a test for every changed file in the repository.
//
// exists is injected rather than called through os.Stat here because every caller up to
// selector.Select is pure; see that package's contract.
//
// No placeholder validation happens here: validateTemplates already rejects any name
// outside {dir}, {subdir} and {name} at load time, so an unknown one cannot reach this
// function.
func (a *Adapter) TestForCandidate(rel string, exists func(string) bool) (string, bool) {
	if a == nil || exists == nil || len(a.TestFor) == 0 {
		return "", false
	}
	dir := path.Dir(rel)
	name := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	subs := trailingDirs(dir)
	for _, tmpl := range a.TestFor {
		// A template naming no {subdir} has exactly one expansion, and trying it once per
		// trailing directory would ask the filesystem the same question len(subs) times.
		tries := subs
		if !strings.Contains(tmpl, "{subdir}") {
			tries = subs[:1]
		}
		for _, sub := range tries {
			// Clean collapses the "./" a root-level {dir} would otherwise produce; engine
			// paths are always cleaned and slash-separated.
			cand := path.Clean(expandTestFor(tmpl, dir, sub, name))
			if cand == "" || cand == "." {
				continue
			}
			if exists(cand) {
				return cand, true
			}
		}
	}
	return "", false
}

// trailingDirs is the value sequence {subdir} takes, longest first: the changed file's
// directory, then that directory with one leading segment dropped, and so on down to its
// last segment.
//
// It exists because a test tree very often mirrors a SUFFIX of the source tree rather
// than the whole of it. `src/main/java/calc/Calc.java`'s test is
// `src/test/java/calc/CalcTest.java`: the mirrored part is `calc`, and `src/main/java` is
// the source root the layout replaces with `src/test/java`. No fixed placeholder can name
// that suffix, because how many leading segments the source root occupies is a property
// of the repository, not of the adapter — a Gradle project with several source sets, or a
// module under `services/api/`, moves it. Trying every suffix, longest first, lets one
// declared template mirror any of them.
//
// The widening is safe for the same reason level 1 is evidence at all: a candidate counts
// only when the file EXISTS. A shorter suffix that names nothing is skipped, and the
// longest suffix that names something wins, so the most specific correspondence the
// repository actually has is the one selected.
func trailingDirs(dir string) []string {
	if dir == "" || dir == "." || dir == "/" {
		return []string{dir}
	}
	segs := strings.Split(dir, "/")
	out := make([]string, 0, len(segs))
	for i := range segs {
		out = append(out, strings.Join(segs[i:], "/"))
	}
	return out
}

// expandTestFor substitutes {dir}, {subdir} and {name} in one left-to-right pass, so a
// substituted value is never rescanned for a placeholder it happens to contain.
//
// It is written by hand rather than with strings.NewReplacer for a structural reason:
// TestOnlySeedCallsMapstoreReplace greps the whole tree textually to keep mapstore's
// row-shrinking method confined to cmd/rtdd/seed.go, and a replacer's method call here
// would read as that offence to a guard that cannot tell the two apart.
func expandTestFor(tmpl, dir, subdir, name string) string {
	var b strings.Builder
	b.Grow(len(tmpl) + len(dir) + len(name))
	for i := 0; i < len(tmpl); {
		switch {
		case strings.HasPrefix(tmpl[i:], "{subdir}"):
			b.WriteString(subdir)
			i += len("{subdir}")
		case strings.HasPrefix(tmpl[i:], "{dir}"):
			b.WriteString(dir)
			i += len("{dir}")
		case strings.HasPrefix(tmpl[i:], "{name}"):
			b.WriteString(name)
			i += len("{name}")
		default:
			b.WriteByte(tmpl[i])
			i++
		}
	}
	return b.String()
}
