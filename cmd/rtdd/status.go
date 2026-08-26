package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// cmdStatus answers "is the map worth trusting?" without running anything. Every line it
// prints is a fact the selector will act on, stated in the terms the selector uses.
func cmdStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	adapterPath := fs.String("adapter", "", "path to the adapter YAML (default .rtdd/adapter.yaml)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	e, code, err := loadEnv(*adapterPath)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd status: %v\n", err)
		return code
	}

	cfg := selector.DefaultConfig()

	fmt.Fprintf(stdout, "repo:    %s\n", e.root)
	if e.ad != nil {
		fmt.Fprintf(stdout, "adapter: %s (%s)\n", e.ad.Name, e.adapterSource())
	} else {
		fmt.Fprintf(stdout, "adapter: none (%s) - file classification is disabled\n", e.noAdapterReason())
	}
	fmt.Fprintf(stdout, "map:     .rtdd/map.jsonl - %d tests, %d files\n", e.m.Len(), len(e.m.FanOut()))

	switch {
	case e.m.Len() == 0:
		fmt.Fprintf(stdout, "seed:    UNSEEDED - every selection escalates to T2 (full suite)\n")
	case e.meta.SeededAt == "":
		fmt.Fprintf(stdout, "seed:    unknown - no .rtdd/meta.json\n")
	default:
		d, derr := gitctx.CommitDistance(e.root, e.meta.SeededAt)
		switch {
		case derr != nil:
			fmt.Fprintf(stdout, "seed:    %s (age unknown: %v)\n", e.meta.SeededAt, derr)
		case d < 0:
			fmt.Fprintf(stdout, "seed:    %s (UNREACHABLE - rebased, squashed, or shallow clone; "+
				"treated as stale, never as fresh)\n", e.meta.SeededAt)
		default:
			fmt.Fprintf(stdout, "seed:    %s (%d commits ago, stale_commits=%d)\n",
				e.meta.SeededAt, d, cfg.StaleCommits)
		}
	}

	fmt.Fprintf(stdout, "cycles:  %d / %d drift guard\n", e.meta.Cycles, cfg.DriftGuard)

	if head, herr := gitctx.HeadSHA(e.root); herr == nil {
		merge, _ := gitctx.IsMergeCommit(e.root, "HEAD")
		if merge {
			fmt.Fprintf(stdout, "head:    %s (merge commit - selections escalate to T1)\n", head)
		} else {
			fmt.Fprintf(stdout, "head:    %s\n", head)
		}
	} else {
		fmt.Fprintf(stdout, "head:    none - the repository has no commits yet\n")
	}
	return 0
}
