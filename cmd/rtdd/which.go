package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// noLinkedTest is what an empty Round 1 or Round 2 says. It is never a pass: rtdd ran
// nothing, and an empty round means only that no test links to the change (spec §10).
const noLinkedTest = "no linked test"

// roundThree is Round 3, always (spec §7).
const roundThree = "Round 3 — the full suite, once, at the end"

// cmdWhich answers "which tests does this change need, in what order" from the node
// graph (spec §7, §8). It runs nothing: no test, no toolchain. Every answer exits 0.
func cmdWhich(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("which", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "HEAD", "the ref changes are measured from")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON (schema 3)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd which [--base <ref>] [--json]")
		return 2
	}
	root, err := graphRoot()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 3
	}
	changes, err := changedSet(root, *base)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		if errors.Is(err, gitctx.ErrUnknownBase) {
			return 2
		}
		return 3
	}
	cfg, res, code, err := buildGraph(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return code
	}
	ranges, warnings := changedRanges(changes)
	r := rounds.Rounds(res.Graph, ranges)
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false) // "<module>" stays readable
		enc.SetIndent("", "  ")
		if err := enc.Encode(buildWhichJSON(*base, res, changes, r, warnings)); err != nil {
			fmt.Fprintf(stderr, "rtdd which: %v\n", err)
			return 3
		}
		return 0
	}
	for _, w := range warnings {
		fmt.Fprintf(stderr, "rtdd which: warning: %s\n", w)
	}
	fmt.Fprint(stdout, renderWhich(res, cfg, r))
	return 0
}

// changedRanges is the changed set as rounds' input: each file's new-side line ranges
// (gitctx.ChangedSet's Lines, verbatim). A deleted file has no lines left to own a node,
// so it selects nothing; the warning says so rather than leaving it silent.
func changedRanges(changes []gitctx.Change) (map[string][]rounds.LineRange, []string) {
	ranges := map[string][]rounds.LineRange{}
	warnings := []string{}
	for _, c := range changes {
		if c.Status == gitctx.Deleted {
			warnings = append(warnings, fmt.Sprintf("%s was deleted: tests that called it are linked to nothing now; Round 3 runs them", c.Path))
			continue
		}
		for _, l := range c.Lines {
			ranges[c.Path] = append(ranges[c.Path], rounds.LineRange{Start: l.Start, End: l.End})
		}
	}
	return ranges, warnings
}

// renderWhich is the human answer: the graph, what changed, the three rounds, untested.
func renderWhich(res *graphbuild.Result, cfg graph.Config, r rounds.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "graph: %s\n", graphLine(res, cfg))
	b.WriteString("changed nodes:\n")
	if len(r.ChangedNodes) == 0 {
		b.WriteString("  none\n")
	}
	for _, n := range r.ChangedNodes {
		fmt.Fprintf(&b, "  %s  (lines %d-%d)\n", n.ID, n.Start, n.End)
	}
	for _, rd := range []struct {
		head  string
		tests []graph.Node
	}{
		{"Round 1 — run these first:", r.Round1},
		{"Round 2 — then these:", r.Round2},
	} {
		b.WriteString(rd.head + "\n")
		if len(rd.tests) == 0 {
			b.WriteString("  " + noLinkedTest + "\n")
		}
		for _, t := range rd.tests {
			fmt.Fprintf(&b, "  %s\n", t.ID)
		}
	}
	b.WriteString(roundThree + "\n")
	b.WriteString("untested:\n")
	if len(r.Untested) == 0 {
		b.WriteString("  none\n")
	}
	for _, n := range r.Untested {
		fmt.Fprintf(&b, "  %s\n", n.ID)
	}
	return b.String()
}

// graphLine states where the graph came from: the source, the commit it describes and
// how many files graphify was not trusted for — or, when graphify was ignored, why.
func graphLine(res *graphbuild.Result, cfg graph.Config) string {
	s := fmt.Sprintf("%s, built at %s, %d stale %s", res.Source, res.BuiltAtCommit,
		len(res.StaleFiles), plural(len(res.StaleFiles), "file", "files"))
	if res.GraphifyIgnored != "" {
		s += fmt.Sprintf(" (graphify ignored — %s; run `graphify --update` to use it again)", ignoredWhy(res, cfg))
	}
	return s
}

// whichJSON is `rtdd which --json`, schema 3 (spec §9): exactly §9's nine top-level keys,
// in §9's order. Every array is [], never null.
type whichJSON struct {
	Schema       int           `json:"schema"`
	Command      string        `json:"command"`
	Base         string        `json:"base"`
	Graph        graphObject   `json:"graph"`
	Changed      []changedFile `json:"changed"`
	ChangedNodes []nodeJSON    `json:"changed_nodes"`
	Rounds       []any         `json:"rounds"`   // testRound{1}, testRound{2}, fullSuiteRound{3}
	Untested     []string      `json:"untested"` // node IDs
	Warnings     []string      `json:"warnings"`
}

type changedFile struct {
	Path  string      `json:"path"`
	Lines []lineRange `json:"lines"` // [] for a deleted file
}

type lineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type nodeJSON struct {
	ID    string `json:"id"`
	File  string `json:"file"`
	Name  string `json:"name"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type testJSON struct {
	ID   string `json:"id"`
	File string `json:"file"`
	Name string `json:"name"`
}

type testRound struct {
	Round int        `json:"round"`
	Tests []testJSON `json:"tests"`
	Files []string   `json:"files"` // the round's test files, de-duplicated, in test order
}

type fullSuiteRound struct {
	Round     int  `json:"round"`
	FullSuite bool `json:"full_suite"`
}

// buildWhichJSON is the schema-3 document (spec §9) for one answer.
func buildWhichJSON(base string, res *graphbuild.Result, changes []gitctx.Change, r rounds.Result, warnings []string) whichJSON {
	doc := whichJSON{Schema: 3, Command: "which", Base: base, Graph: graphObjectOf(res),
		Changed: []changedFile{}, ChangedNodes: []nodeJSON{}, Untested: []string{}, Warnings: nonNilStrings(warnings)}
	for _, c := range changes {
		f := changedFile{Path: c.Path, Lines: []lineRange{}}
		for _, l := range c.Lines {
			f.Lines = append(f.Lines, lineRange{Start: l.Start, End: l.End})
		}
		doc.Changed = append(doc.Changed, f)
	}
	for _, n := range r.ChangedNodes {
		doc.ChangedNodes = append(doc.ChangedNodes, nodeJSON{ID: n.ID, File: n.File, Name: n.Name, Start: n.Start, End: n.End})
	}
	for i, tests := range [][]graph.Node{r.Round1, r.Round2} {
		rd := testRound{Round: i + 1, Tests: []testJSON{}, Files: []string{}}
		seen := map[string]bool{}
		for _, t := range tests {
			rd.Tests = append(rd.Tests, testJSON{ID: t.ID, File: t.File, Name: t.Name})
			if !seen[t.File] {
				seen[t.File] = true
				rd.Files = append(rd.Files, t.File)
			}
		}
		doc.Rounds = append(doc.Rounds, rd)
	}
	doc.Rounds = append(doc.Rounds, fullSuiteRound{Round: 3, FullSuite: true})
	for _, n := range r.Untested {
		doc.Untested = append(doc.Untested, n.ID)
	}
	return doc
}

// graphObjectOf is the `graph` object `rtdd graph --json` and `rtdd which --json` share.
func graphObjectOf(res *graphbuild.Result) graphObject {
	obj := graphObject{Source: res.Source, BuiltAtCommit: res.BuiltAtCommit, StaleFiles: len(res.StaleFiles),
		GraphifyIgnored: res.GraphifyIgnored, Nodes: len(res.Graph.Nodes), Edges: len(res.Graph.Edges)}
	for _, n := range res.Graph.Nodes {
		if n.IsTest {
			obj.Tests++
		}
	}
	return obj
}

// nonNilStrings guarantees a JSON array rather than null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
