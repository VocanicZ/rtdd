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

	changes, err := gitctx.ChangedSet(e.root, *base)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 3
	}

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
		Cycles:   e.meta.Cycles,
		Merge:    merge,
		Distance: distance,
	})

	if *asJSON {
		return emitWhichJSON(stdout, stderr, e, *base, changes, sel)
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
	if sel.Tier == selector.TierEmpty {
		fmt.Fprintf(stdout, "\nNOTE: an empty selection is not a pass. Nothing was checked.\n")
	}
	if sel.Tier == selector.TierT2 && len(sel.Tests) == 0 {
		fmt.Fprintf(stdout, "\nNOTE: T2 means the full suite. rtdd does not enumerate it in M1a "+
			"(adapter.List is M1b), so no test ids are listed.\n")
	}
	return 0
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
}

func emitWhichJSON(stdout, stderr io.Writer, e *env, base string, changes []gitctx.Change, sel selector.Selection) int {
	head, _ := gitctx.HeadSHA(e.root)
	out := whichJSON{
		Base:     base,
		Head:     head,
		Tier:     sel.Tier.String(),
		Reason:   sel.Reason,
		Direct:   nonNilStrings(sel.Direct),
		Tests:    nonNilStrings(sel.Tests),
		Changed:  make([]jsonChange, 0, len(changes)),
		MapTests: e.m.Len(),
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
