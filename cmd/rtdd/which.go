package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// cmdWhich answers "what should I run?" without running anything. It costs one map load
// and one git diff, and it is the primary agent integration point. Every tier exits 0 —
// an empty selection is a signal, and the human output says so in as many words.
func cmdWhich(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("which", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "HEAD", "base ref for the changed set")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	adapterPath := fs.String("adapter", "", "path to the adapter YAML (default .rtdd/adapter.yaml)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	e, code, err := loadEnv(*adapterPath)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return code
	}

	// changedSet, not gitctx.ChangedSet: rtdd's own .rtdd/ writes must not select.
	changes, err := changedSet(e.root, *base)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 3
	}

	// allTests is the enumerated suite. adapter.List lands in M1b, so in M1a it is always
	// empty and every T2 selection is a partial list — which is exactly what `complete`
	// and the note below report.
	var allTests []string

	merge, _ := gitctx.IsMergeCommit(e.root, "HEAD")
	distance := func(sha string) int {
		d, derr := gitctx.CommitDistance(e.root, sha)
		if derr != nil {
			return -1 // unknown, never fresh
		}
		return d
	}

	sel := selector.Select(selector.Inputs{
		Map:      e.m,
		Changes:  changes,
		Adapter:  e.ad,
		Cfg:      selector.DefaultConfig(),
		AllTests: allTests,
		Cycles:   e.meta.Cycles,
		Merge:    merge,
		Distance: distance,
	})

	warnings := whichWarnings(e)

	if *asJSON {
		return emitWhichJSON(stdout, stderr, e, *base, changes, sel, allTests, warnings)
	}

	fmt.Fprintf(stdout, "base:     %s\n", *base)
	fmt.Fprintf(stdout, "changed:  %d files\n", len(changes))
	for _, c := range changes {
		if c.OldPath != "" {
			fmt.Fprintf(stdout, "  %-9s %s (from %s)\n", c.Status.String(), c.Path, c.OldPath)
		} else {
			fmt.Fprintf(stdout, "  %-9s %s\n", c.Status.String(), c.Path)
		}
	}
	fmt.Fprintf(stdout, "tier:     %s - %s\n", sel.Tier.String(), sel.Reason)
	fmt.Fprintf(stdout, "direct:   %d\n", len(sel.Direct))
	for _, id := range sel.Direct {
		fmt.Fprintf(stdout, "  %s\n", id)
	}
	fmt.Fprintf(stdout, "selected: %d\n", len(sel.Tests))
	for _, id := range sel.Tests {
		fmt.Fprintf(stdout, "  %s\n", id)
	}
	for _, w := range warnings {
		fmt.Fprintf(stdout, "\nWARNING: %s\n", w)
	}
	if sel.Tier == selector.TierEmpty {
		fmt.Fprintf(stdout, "\nNOTE: an empty selection is not a pass. Nothing was checked.\n")
	}
	// Gated on the suite being unenumerated, not on the selection being empty: a T2
	// selection that also carries a direct test is still a partial list of the full suite,
	// and reading it as "T2 satisfied by one test" is exactly the under-run this note exists
	// to prevent.
	if sel.Tier == selector.TierT2 && len(allTests) == 0 {
		fmt.Fprintf(stdout, "\nNOTE: T2 means the full suite. rtdd does not enumerate it in M1a "+
			"(adapter.List is M1b), so the %d test id(s) listed above are not the whole run.\n",
			len(sel.Tests))
	}
	return 0
}

// whichWarnings are the conditions under which the selection is narrower than it looks.
// `status` already reports a missing adapter; `which` is the command agents actually call,
// and it used to run with file classification silently disabled.
func whichWarnings(e *env) []string {
	var out []string
	if e.ad == nil {
		out = append(out, fmt.Sprintf("no adapter (%s not found) - file classification is disabled: "+
			"no changed file can be recognised as a test file, so the direct tier is empty", e.adPath))
	}
	return out
}

type jsonLineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type jsonChange struct {
	Path    string          `json:"path"`
	OldPath string          `json:"old_path"`
	Status  string          `json:"status"`
	Lines   []jsonLineRange `json:"lines"`
}

// whichJSON is the machine-readable form of a selection. PROVISIONAL in M1a:
// plan M2, task "JSON output", freezes this schema, and the agent front-ends bind to
// the frozen version.
type whichJSON struct {
	Base     string       `json:"base"`
	Head     string       `json:"head"`
	Tier     string       `json:"tier"`
	Reason   string       `json:"reason"`
	Direct   []string     `json:"direct"`
	Tests    []string     `json:"tests"`
	Changed  []jsonChange `json:"changed"`
	MapTests int          `json:"map_tests"`
	// Adapter is the loaded adapter's name, or "" when no adapter file was found.
	Adapter string `json:"adapter"`
	// Complete reports whether Tests is the whole set to run. It is false for a T2
	// selection whose suite was not enumerated: the list is then a partial one.
	Complete bool     `json:"complete"`
	Warnings []string `json:"warnings"`
}

func emitWhichJSON(stdout, stderr io.Writer, e *env, base string, changes []gitctx.Change,
	sel selector.Selection, allTests, warnings []string) int {
	head, _ := gitctx.HeadSHA(e.root)
	adapterName := ""
	if e.ad != nil {
		adapterName = e.ad.Name
	}
	out := whichJSON{
		Base:     base,
		Head:     head,
		Tier:     sel.Tier.String(),
		Reason:   sel.Reason,
		Direct:   nonNilStrings(sel.Direct),
		Tests:    nonNilStrings(sel.Tests),
		Changed:  make([]jsonChange, 0, len(changes)),
		MapTests: e.m.Len(),
		Adapter:  adapterName,
		Complete: !(sel.Tier == selector.TierT2 && len(allTests) == 0),
		Warnings: nonNilStrings(warnings),
	}
	for _, c := range changes {
		jc := jsonChange{
			Path:    c.Path,
			OldPath: c.OldPath,
			Status:  c.Status.String(),
			Lines:   make([]jsonLineRange, 0, len(c.Lines)),
		}
		for _, r := range c.Lines {
			jc.Lines = append(jc.Lines, jsonLineRange{Start: r.Start, End: r.End})
		}
		out.Changed = append(out.Changed, jc)
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 2
	}
	return 0
}

// nonNilStrings guarantees a JSON array rather than null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
