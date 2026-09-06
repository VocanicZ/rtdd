package adapter

// The selection side of the two vocabularies a static adapter speaks.
//
// `id_template` renders a <testcase> the runner ALREADY ran back into an id — the
// reporting vocabulary, and the only one a JUnit report can supply. `test_selector`
// renders a test FILE the tier chose into the selector that will make the runner run it —
// the selection vocabulary, and the only one available before anything has run.
//
// Plan 06-m6d decision 7 claimed one template served both. It does not: the TS tier's
// output is a test file path, six of the nine shipped runners select by test name, and
// splicing the first into the second matches nothing and exits 0 over zero executed tests
// (#334). Decision 13 splits them, and this file is the split.

import (
	"fmt"
	"path"
	"strings"
)

// Selectors renders each selected test file through `test_selector` and returns the tokens
// `subset` should splice, in the order the tier ranked them and with duplicates collapsed.
//
// De-duplication is not tidiness: a go selector is a PACKAGE, so two test files in one
// package render the same `./calc`, and passing it twice would run that package twice and
// fold every outcome in it against itself. The first occurrence keeps its rank, because
// the order is the tier's confidence order and a chunk boundary must not reorder it.
//
// An adapter declaring no test_selector gets its ids back untouched — the identity — which
// is what every coverage-tier adapter has always done and what the three file-granular
// shipped adapters mean when they write `test_selector: "{file}"` out loud.
func (a *Adapter) Selectors(tests []string) ([]string, error) {
	if a == nil || a.TestSelector == "" {
		return tests, nil
	}
	out := make([]string, 0, len(tests))
	seen := make(map[string]bool, len(tests))
	for _, t := range tests {
		sel, err := a.SelectorFor(t)
		if err != nil {
			return nil, err
		}
		if seen[sel] {
			continue
		}
		seen[sel] = true
		out = append(out, sel)
	}
	return out, nil
}

// SelectorFor renders ONE test file path through `test_selector`.
//
// The path is treated as a repo-relative slash path, which is what the selector and the
// adapter's globs both produce. The result is spliced verbatim: it is the runner's own
// syntax, so nothing here quotes, escapes or re-splits it — expand.go's rule that an id is
// data holds for a rendered selector exactly as it holds for a measured one.
func (a *Adapter) SelectorFor(testPath string) (string, error) {
	if a == nil || a.TestSelector == "" {
		return testPath, nil
	}
	clean := strings.TrimSpace(testPath)
	if clean == "" {
		return "", fmt.Errorf("adapter %s: empty test path; test_selector %q has nothing to render", a.name(), a.TestSelector)
	}
	base := path.Base(clean)
	vars := map[string]string{
		"file": clean,
		"dir":  path.Dir(clean),
		"name": strings.TrimSuffix(base, path.Ext(base)),
	}
	// substitute is the same one-pass expander every other template goes through, so an
	// unknown placeholder is impossible here — validateTemplates rejected it at load.
	return a.substitute(a.TestSelector, vars)
}
