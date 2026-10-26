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

// cmdInit implements `rtdd init`: it sets up the REPO ROOT by writing .rtdd/config.yaml and
// .rtdd/.gitignore, and nothing outside .rtdd/ (#509). The agent instructions live in
// rtdd-owned directories `rtdd skill install` writes once per machine; a team that wants
// them committed opts in with --project-skill (rtdd's own whole files) or names a shared
// file with --append-to (rtdd's block between its markers, nothing outside them touched).
// A whole-file front-end is replaced only when an earlier rtdd wrote it; one rtdd did not
// write is a conflict no flag overrides. What an older init wrote outside .rtdd/ is
// reported, never touched. It detects nothing and refuses no git repository: the node
// graph serves every language the scanner reads (spec §8).
//
// The root is resolved with findRepoRoot, so running init from a subdirectory installs
// where the other commands will look. The working directory is only a fallback for the
// one case where there is no root to find: installing before `git init`.
func cmdInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "print the plan and change nothing")
	var opts install.PlanOptions
	fs.BoolVar(&opts.ProjectSkill, "project-skill", false, "also write the Claude Code skill and the Cursor rule into the repository")
	fs.Func("append-to", "also merge rtdd's marker block into this shared file (repeatable)", func(v string) error {
		opts.AppendTo = append(opts.AppendTo, v)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd init [--dry-run] [--project-skill] [--append-to <file>]...")
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
	steps, err := install.Plan(root, files, opts)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 2
	}
	fmt.Fprint(stdout, RenderInit(append(steps, globalLegacy()...)))

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
		fmt.Fprintf(stderr, "rtdd init: %d conflict(s); nothing written; move the named file(s) aside and re-run\n", conflicts)
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
