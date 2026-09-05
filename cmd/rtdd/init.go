package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
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

	// The gate (spec §5), before install.Files and therefore before install.Plan, so
	// --dry-run refuses too: printing a plan the command would refuse to execute is a
	// lie, and a plan is the one output a user reads as a promise.
	//
	// Detection consults the RESOLVED set — the built-ins overlaid with the host's
	// .rtdd/adapters/*.yaml — so a repo that supplies its own adapter passes the gate
	// without an RTDD release (§4.5).
	all, err := adapter.Available(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 2
	}
	detected, err := adapter.DetectAll(root, all)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 3
	}
	if len(detected) == 0 && !*force {
		writeNoAdapterRefusal(stderr, root)
		return 2
	}

	files, err := install.Files()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 2
	}
	// --force in a repo nothing matched installs, and then the front-end has to say so
	// itself: the file outlives this terminal, and the agent that reads it never saw
	// the warning printed here.
	if len(detected) == 0 {
		files = install.WithNoAdapterCaveat(files)
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

// writeNoAdapterRefusal explains a refusal in the terms the user can act on: what RTDD
// found, why installing anyway would be a promise it cannot keep, and the two ways out.
// Naming .rtdd/adapters/ is the load-bearing half — authoring an adapter is the remedy,
// and a message that omits it reads as "this repo is unsupported, full stop".
func writeNoAdapterRefusal(stderr io.Writer, root string) {
	fmt.Fprintf(stderr, "rtdd init: no adapter detected in %s\n", root)
	if found := adapter.UnsupportedToolchains(root); len(found) > 0 {
		fmt.Fprintf(stderr, "  found, but served by no adapter: %s\n", strings.Join(found, ", "))
	}
	fmt.Fprintln(stderr, "  Installing agent instructions here would promise a selection RTDD cannot make:")
	fmt.Fprintln(stderr, "  with no adapter there is no map to seed, so every answer would be \"run the full suite\".")
	fmt.Fprintf(stderr, "  Write an adapter in %s/<language>.yaml (see docs/specs for the contract),\n", adapter.HostAdapterDir)
	fmt.Fprintln(stderr, "  or re-run with --force to install anyway.")
}
