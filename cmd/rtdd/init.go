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
	// AvailableReport rather than Available: the files that did NOT load are the other
	// half of the answer. `init` is the command that refuses because of them, so a
	// refusal that drops the report tells the user to write the adapter they already
	// wrote (PRD #229 AC5).
	all, invalid, err := adapter.AvailableReport(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		warnInvalidAdapters(stderr, root, invalid)
		return 2
	}
	detected, err := adapter.DetectAll(root, all)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 3
	}
	if len(detected) == 0 && !*force {
		writeNoAdapterRefusal(stderr, root, invalid)
		return 2
	}
	// A repo that IS served can still carry a half-written adapter beside the working
	// one. Skipping it silently would let a typo in a host override read as the built-in
	// simply winning, exactly as it would for `which` and `run`.
	warnInvalidAdapters(stderr, root, invalid)

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
	steps, err := install.Plan(root, files, *force, adapterRecords(detected))
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
	// The prerequisite block spec §4.3 requires at init time as well as at doctor time,
	// scoped to the DETECTED adapters (#249) so nothing sends an agent to install a
	// binary for a toolchain this repository does not use.
	//
	// It is a report, not a refusal, and the exit code stays 0: the repo HAS an adapter,
	// so §5's gate is satisfied, and a binary missing from this machine is no reason to
	// decline to install — the CI that runs the suite may install it later, and refusing
	// would break `rtdd init` on every such repo.
	fmt.Fprint(stderr, RenderRequirements(adapter.UnmetFindings(detected, lookPath)))
	fmt.Fprintln(stdout, "\nNext: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it.")
	return 0
}

// adapterRecords is what spec §5 has init record in a newly created .rtdd/config.yaml:
// each detected adapter's name, its declared selection, and the fidelity that selection
// derives. Fidelity is derived here rather than declared by the adapter, and `rtdd
// doctor` re-derives it live — the config is a record of the install, never an input.
func adapterRecords(detected []*adapter.Adapter) []install.AdapterRecord {
	recs := make([]install.AdapterRecord, 0, len(detected))
	for _, a := range detected {
		recs = append(recs, install.AdapterRecord{
			Name:      a.Name,
			Selection: a.Selection,
			Fidelity:  string(a.Fidelity()),
		})
	}
	return recs
}

// writeNoAdapterRefusal explains a refusal in the terms the user can act on: what RTDD
// found, why installing anyway would be a promise it cannot keep, and the two ways out.
// Naming .rtdd/adapters/ is the load-bearing half — authoring an adapter is the remedy,
// and a message that omits it reads as "this repo is unsupported, full stop".
func writeNoAdapterRefusal(stderr io.Writer, root string, invalid []adapter.Invalid) {
	fmt.Fprintf(stderr, "rtdd init: no adapter detected in %s\n", root)
	if found := adapter.UnsupportedToolchains(root); len(found) > 0 {
		fmt.Fprintf(stderr, "  found, but served by no adapter: %s\n", strings.Join(found, ", "))
	}
	// An adapter that failed to load is the likeliest reason a repo with one still has
	// none, and both the file and the failing field are already known here. "Write an
	// adapter" is unusable advice to someone who wrote one — name the file and the field
	// so the remedy below reads as "fix this", not "start over".
	for _, bad := range invalid {
		fmt.Fprintf(stderr, "  not loaded: %s\n      %v\n", relToRoot(root, bad.Path), bad.Err)
	}
	fmt.Fprintln(stderr, "  Installing agent instructions here would promise a selection RTDD cannot make:")
	fmt.Fprintln(stderr, "  with no adapter there is no map to seed, so every answer would be \"run the full suite\".")
	fmt.Fprintf(stderr, "  Write an adapter in %s/<language>.yaml (see docs/specs for the contract),\n", adapter.HostAdapterDir)
	fmt.Fprintln(stderr, "  or re-run with --force to install anyway.")
}
