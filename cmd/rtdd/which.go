package main

import (
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
	// Until schema 3 lands (docs/plans/10-rounds-cutover.md Task 3), --json is still the
	// v0.2 document, answered by the v0.2 path.
	for _, a := range args {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			return cmdWhichV02(args, stdout, stderr)
		}
	}
	fs := flag.NewFlagSet("which", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "HEAD", "the ref changes are measured from")
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
