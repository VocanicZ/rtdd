package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/selfupdate"
)

// RenderUninstall formats what `rtdd uninstall` planned or did. Like RenderInit, this is
// the only place the command's output is composed; internal/install never prints.
func RenderUninstall(steps []install.Step) string {
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

// cmdUninstall implements `rtdd uninstall`: the inverse of `rtdd init`.
//
// It removes what init wrote into THIS REPOSITORY — the Claude Code skill, the Cursor
// rule, rtdd's marker block in AGENTS.md and CLAUDE.md, and the .gitattributes line — and
// stops there. Two things it leaves unless asked: .rtdd/, because the recorded map is the
// expensive thing to rebuild, and the binary, because a repository is not where the binary
// lives.
//
// AGENTS.md and CLAUDE.md belong to the host project. A file rtdd cannot read
// unambiguously is a conflict, and a conflict stops the whole run before anything is
// written, exactly as it does on the way in.
func cmdUninstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "print the plan and change nothing")
	state := fs.Bool("state", false, "also remove .rtdd/ — the config and the recorded map")
	binary := fs.Bool("binary", false, "also delete the installed rtdd binary")
	global := fs.Bool("global", false, "also remove the machine-wide agent front-ends `rtdd skill install` wrote")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd uninstall [--dry-run] [--state] [--binary] [--global]")
		return 2
	}

	root, err := findRepoRoot(".")
	if err != nil {
		if root, err = os.Getwd(); err != nil {
			fmt.Fprintf(stderr, "rtdd uninstall: %v\n", err)
			return 3
		}
	}

	steps, err := install.PlanUninstall(root, install.UninstallOptions{State: *state})
	if err != nil {
		fmt.Fprintf(stderr, "rtdd uninstall: %v\n", err)
		return 3
	}

	// The machine-wide front-ends are opt-in: this command's subject is THIS repository,
	// and someone uninstalling rtdd from one project usually still wants it on the machine.
	//
	// Both plans are computed BEFORE anything is printed or applied, so --dry-run shows the
	// whole of what the command would do. A dry run that printed only half the plan would
	// be the same defect `rtdd init` avoids by gating ahead of its own plan: a plan is the
	// one output a user reads as a promise.
	var globalSteps []install.Step
	var home string
	if *global {
		home, err = homeDir()
		if err != nil {
			fmt.Fprintf(stderr, "rtdd uninstall: %v\n", err)
			return 3
		}
		if globalSteps, err = install.PlanGlobalUninstall(home); err != nil {
			fmt.Fprintf(stderr, "rtdd uninstall: %v\n", err)
			return 3
		}
	}

	fmt.Fprint(stdout, RenderUninstall(steps))
	fmt.Fprint(stdout, RenderUninstall(globalSteps))

	if *dryRun {
		fmt.Fprintln(stdout, "\ndry run — nothing was changed")
		return 0
	}
	if err := install.ApplyUninstall(root, steps); err != nil {
		fmt.Fprintf(stderr, "rtdd uninstall: %v\n", err)
		return 2
	}
	if *global {
		if err := install.ApplyUninstall(home, globalSteps); err != nil {
			fmt.Fprintf(stderr, "rtdd uninstall: %v\n", err)
			return 2
		}
	}

	// The binary goes last. Removing it first would leave a half-uninstalled repository
	// behind with no command left to finish the job.
	if *binary {
		path, err := selfupdate.RemoveInstalled("")
		if err != nil {
			fmt.Fprintf(stderr, "rtdd uninstall: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%-14s %s\n", "delete", path)
	}
	return 0
}
