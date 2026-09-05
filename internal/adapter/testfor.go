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
// outside {dir} and {name} at load time, so an unknown one cannot reach this function.
func (a *Adapter) TestForCandidate(rel string, exists func(string) bool) (string, bool) {
	if a == nil || exists == nil || len(a.TestFor) == 0 {
		return "", false
	}
	dir := path.Dir(rel)
	name := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	for _, tmpl := range a.TestFor {
		// Clean collapses the "./" a root-level {dir} would otherwise produce; engine
		// paths are always cleaned and slash-separated.
		cand := path.Clean(expandTestFor(tmpl, dir, name))
		if cand == "" || cand == "." {
			continue
		}
		if exists(cand) {
			return cand, true
		}
	}
	return "", false
}

// expandTestFor substitutes {dir} and {name} in one left-to-right pass, so a substituted
// value is never rescanned for a placeholder it happens to contain.
//
// It is written by hand rather than with strings.NewReplacer for a structural reason:
// TestOnlySeedCallsMapstoreReplace greps the whole tree textually to keep mapstore's
// row-shrinking method confined to cmd/rtdd/seed.go, and a replacer's method call here
// would read as that offence to a guard that cannot tell the two apart.
func expandTestFor(tmpl, dir, name string) string {
	var b strings.Builder
	b.Grow(len(tmpl) + len(dir) + len(name))
	for i := 0; i < len(tmpl); {
		switch {
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
