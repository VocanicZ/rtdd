package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/VocanicZ/rtdd/internal/install"
)

// RenderInit formats what `rtdd init` planned or did. It is the only place this
// command's output is composed; internal/install itself never prints.
func RenderInit(steps []install.Step) string {
	var b []byte
	for _, s := range steps {
		line := fmt.Sprintf("%-14s %s", s.Action, s.Path)
		if s.Note != "" {
			line += "  — " + s.Note
		}
		b = append(b, line+"\n"...)
	}
	return string(b)
}

// cmdInit implements `rtdd init`: it installs the generated front-ends, the union
// merge driver and the config into the REPO ROOT. AGENTS.md and CLAUDE.md are merged
// between rtdd's own markers; nothing outside them is ever touched, and nothing
// outside a marker-delimited target is overwritten without --force.
//
// The root is resolved with findRepoRoot, exactly as `rtdd run` and `rtdd seed` do, so
// running init from a subdirectory installs where the other commands will look. The
// working directory is only a fallback for the one case where there is no root to find:
// installing before `git init`. Getting this wrong is silent — `.gitattributes` patterns
// are directory-scoped, so a copy under sub/deep/ binds `merge=union` to a path that does
// not exist and leaves the real .rtdd/map.jsonl with no union merge driver at all.
func cmdInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "print the plan and change nothing")
	force := fs.Bool("force", false, "overwrite whole-file front-ends that exist and differ")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd init [--dry-run] [--force]")
		return 2
	}

	root, err := findRepoRoot(".")
	if err != nil {
		if root, err = os.Getwd(); err != nil {
			fmt.Fprintf(stderr, "rtdd init: %v\n", err)
			return 3
		}
	}

	files, err := install.Files()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 2
	}
	steps, err := install.Plan(root, files, *force)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 2
	}
	fmt.Fprint(stdout, RenderInit(steps))

	if *dryRun {
		return 0
	}

	conflicts := 0
	for _, s := range steps {
		if s.Action == install.Conflict {
			conflicts++
		}
	}
	if conflicts > 0 {
		fmt.Fprintf(stderr, "rtdd init: %d conflict(s); nothing written; re-run with --force to overwrite\n", conflicts)
		return 2
	}
	if err := install.Apply(root, steps); err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 2
	}
	fmt.Fprintln(stdout, "\nNext: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it.")
	return 0
}
