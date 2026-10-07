package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/VocanicZ/rtdd/internal/gitctx"
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

// cmdInit implements `rtdd init`: it installs the generated front-ends and the config
// into the REPO ROOT. AGENTS.md and CLAUDE.md are merged between rtdd's own markers;
// nothing outside them is ever touched, and nothing outside a marker-delimited target is
// overwritten without --force. It detects nothing and refuses no git repository: the
// node graph serves every language the scanner reads (spec §8).
//
// The root is resolved with findRepoRoot, so running init from a subdirectory installs
// where the other commands will look. The working directory is only a fallback for the
// one case where there is no root to find: installing before `git init`.
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
	steps, err := install.Plan(root, files, *force, nil)
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
	fmt.Fprint(stdout, RenderNextStep())
	return 0
}

// RenderNextStep is the line `rtdd init` closes with. v0.3.0 has no seed step: the graph
// is built on first use, so the next step is to edit code and ask `rtdd which`.
func RenderNextStep() string {
	return "\nNext: edit code, then run `rtdd which` for the tests to run, in rounds.\n"
}

// findRepoRoot walks up from start to the working tree that contains it.
//
// The rule lives in gitctx so this command and rtdd-gen cannot drift: a `.git`
// DIRECTORY counts only when it holds HEAD, a `.git` FILE only when its `gitdir:`
// target does. A stray marker is skipped and the walk continues upward.
func findRepoRoot(start string) (string, error) {
	return gitctx.FindRepoRoot(start)
}
