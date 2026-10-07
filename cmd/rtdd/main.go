// Command rtdd reports which tests cover the code that just changed.
// This package is the only place in the engine that prints.
package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `rtdd - relational test-driven development

usage:
  rtdd init
  rtdd which  [--base <ref>] [--json]
  rtdd explain <file[:line]|name>
  rtdd graph  [--json]
  rtdd doctor
  rtdd update [--check] [--version <tag>]
  rtdd skill install   [--dry-run] [--force]
  rtdd skill uninstall [--dry-run]
  rtdd skill prompt
  rtdd uninstall [--dry-run] [--state] [--binary]
  rtdd --version

exit codes:
  0  success - an empty selection is a signal, not a failure
  2  usage or configuration error
  3  fatal environment error (not a git repository, git unavailable, a graph that cannot be built)
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run is main's whole body, parameterised on its writers so every command is testable
// with in-memory buffers and an asserted exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "init":
		return cmdInit(args[1:], stdout, stderr)
	case "seed", "run", "verify", "status", "map":
		return removedCommand(args, stderr)
	case "which":
		return cmdWhich(args[1:], stdout, stderr)
	case "explain":
		return cmdExplain(args[1:], stdout, stderr)
	case "graph":
		return cmdGraph(args[1:], stdout, stderr)
	case "doctor":
		return cmdDoctor(args[1:], stdout, stderr)
	case "update":
		return cmdUpdate(args[1:], stdout, stderr)
	case "skill":
		return cmdSkill(args[1:], stdout, stderr)
	case "uninstall":
		return cmdUninstall(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "--version", "version":
		return cmdVersion(stdout)
	default:
		fmt.Fprintf(stderr, "rtdd: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

// removedCommand answers a v0.2 command v0.3.0 removed (spec §8): a usage error naming
// the release and what replaces it. It writes nothing.
func removedCommand(args []string, stderr io.Writer) int {
	name := args[0]
	if name == "map" && len(args) > 1 {
		name += " " + args[1]
	}
	fmt.Fprintf(stderr, "rtdd %s: removed in v0.3.0 — rtdd no longer runs tests or keeps a coverage map.\n"+
		"Run `rtdd which` and run its rounds with the project's own test command.\n", name)
	return 2
}
