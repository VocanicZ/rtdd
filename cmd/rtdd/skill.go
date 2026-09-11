package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
)

const skillUsage = `usage:
  rtdd skill install   [--dry-run] [--force]
  rtdd skill uninstall [--dry-run]
  rtdd skill prompt
`

// homeDir resolves the directory the machine-wide front-ends live in.
//
// RTDD_HOME overrides it. The override exists because os.UserHomeDir reads a different
// variable per platform — HOME on Unix, USERPROFILE on Windows — so a test that set one of
// those directly would only redirect the install on one OS. It is also the escape hatch for
// anyone whose agent configuration lives somewhere unusual.
func homeDir() (string, error) {
	if h := os.Getenv("RTDD_HOME"); h != "" {
		return h, nil
	}
	return os.UserHomeDir()
}

// cmdSkill implements `rtdd skill`: the MACHINE-WIDE counterpart of `rtdd init`.
//
// `rtdd init` installs into one repository and is gated on an adapter matching it. This
// installs into the user's home directory and is gated on nothing, because there is no
// repository to detect: the front-ends it writes are read in every repository on the
// machine, and their whole job is to tell an agent to run `rtdd init` in the ones rtdd has
// never touched.
//
// It is a separate command rather than `rtdd init --global` because a flag that moved
// init's root AND disabled its adapter gate would make one command mean two different
// things.
func cmdSkill(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, skillUsage)
		return 2
	}
	switch args[0] {
	case "install":
		return cmdSkillInstall(args[1:], stdout, stderr)
	case "uninstall":
		return cmdSkillUninstall(args[1:], stdout, stderr)
	case "prompt":
		return cmdSkillPrompt(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "rtdd skill: unknown subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, skillUsage)
		return 2
	}
}

func cmdSkillInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skill install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "print the plan and change nothing")
	force := fs.Bool("force", false, "overwrite a hand-edited front-end")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprint(stderr, skillUsage)
		return 2
	}

	home, err := homeDir()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd skill install: %v\n", err)
		return 3
	}
	files, err := install.Files()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd skill install: %v\n", err)
		return 2
	}
	steps, err := install.PlanGlobal(home, files, *force)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd skill install: %v\n", err)
		return 2
	}
	fmt.Fprint(stdout, RenderInit(steps))

	if *dryRun {
		fmt.Fprintln(stdout, "\ndry run — nothing was changed")
		return 0
	}

	conflicts := 0
	for _, s := range steps {
		if s.Action == install.Conflict {
			conflicts++
		}
	}
	if conflicts > 0 {
		fmt.Fprintf(stderr, "rtdd skill install: %d conflict(s); nothing written; re-run with --force to overwrite\n", conflicts)
		return 2
	}
	if err := install.Apply(home, steps); err != nil {
		fmt.Fprintf(stderr, "rtdd skill install: %v\n", err)
		return 2
	}
	fmt.Fprint(stdout, "\nFor any agent not listed above, run `rtdd skill prompt` and give its output to that agent.\n")
	return 0
}

func cmdSkillUninstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skill uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "print the plan and change nothing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprint(stderr, skillUsage)
		return 2
	}

	home, err := homeDir()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd skill uninstall: %v\n", err)
		return 3
	}
	steps, err := install.PlanGlobalUninstall(home)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd skill uninstall: %v\n", err)
		return 3
	}
	fmt.Fprint(stdout, RenderUninstall(steps))

	if *dryRun {
		fmt.Fprintln(stdout, "\ndry run — nothing was changed")
		return 0
	}
	if err := install.ApplyUninstall(home, steps); err != nil {
		fmt.Fprintf(stderr, "rtdd skill uninstall: %v\n", err)
		return 2
	}
	return 0
}

// cmdSkillPrompt prints a paste-anywhere copy of the machine-wide skill.
//
// It exists for the agents rtdd cannot write to itself: Cursor, whose user-level rules live
// inside the application's settings rather than in a file, and whatever the user happens to
// run that rtdd has never heard of. It is deliberately self-contained — instruction and full
// document in one block — because it is pasted into a chat window that may have no shell
// behind it to fetch the rest.
func cmdSkillPrompt(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprint(stderr, skillUsage)
		return 2
	}
	files, err := install.Files()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd skill prompt: %v\n", err)
		return 2
	}
	body, ok := files[protocol.GlobalSkillPath]
	if !ok {
		fmt.Fprintf(stderr, "rtdd skill prompt: the embedded protocol rendered no %s\n", protocol.GlobalSkillPath)
		return 2
	}
	fmt.Fprint(stdout, RenderSkillPrompt(body))
	return 0
}

// RenderSkillPrompt wraps the machine-wide skill in the instruction that tells an agent
// what to do with it. Like RenderInit, composing output is the command layer's job.
//
// The fence is four backticks so the document's own fenced code blocks survive the paste
// intact; a three-backtick wrapper would be closed by the first example inside it.
func RenderSkillPrompt(body string) string {
	var b strings.Builder
	b.WriteString("rtdd is installed on this machine. rtdd runs only the tests that cover the code\n")
	b.WriteString("you changed, using coverage recorded from real runs.\n\n")
	b.WriteString("Save the document below verbatim wherever you keep your global, user-level agent\n")
	b.WriteString("instructions — the place whose contents apply in every project, not just this one.\n")
	b.WriteString("Then follow it whenever you are about to run tests after an edit.\n\n")
	b.WriteString("````markdown\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("````\n")
	return b.String()
}

// refreshGlobalSkill rewrites the machine-wide front-ends after a successful `rtdd update`.
//
// The rendered front-ends are embedded in the binary, so replacing the binary makes whatever
// is in the home directory stale. Left alone, the skill an agent reads keeps describing the
// release the user first installed while the binary it describes moves on.
//
// It never fails the caller. `rtdd update` has already replaced the binary by the time this
// runs, so the update genuinely succeeded; reporting a home directory rtdd could not write
// as an update failure would send the user to re-run something that already worked. Trouble
// is reported as a note on stderr and nothing more, and the surfaces PlanGlobal skips are
// silent by construction.
func refreshGlobalSkill(stdout, stderr io.Writer) {
	home, err := homeDir()
	if err != nil {
		fmt.Fprintf(stderr, "note: could not refresh the agent skill: %v\n", err)
		return
	}
	files, err := install.Files()
	if err != nil {
		fmt.Fprintf(stderr, "note: could not refresh the agent skill: %v\n", err)
		return
	}
	steps, err := install.PlanGlobal(home, files, false)
	if err != nil {
		fmt.Fprintf(stderr, "note: could not refresh the agent skill: %v\n", err)
		return
	}
	// A conflict here is a front-end the user has hand-written. Updating the binary is not
	// permission to overwrite it, so it is left exactly as it is and reported.
	kept := make([]install.Step, 0, len(steps))
	for _, s := range steps {
		if s.Action == install.Conflict {
			fmt.Fprintf(stderr, "note: left %s alone — %s\n", s.Path, s.Note)
			continue
		}
		kept = append(kept, s)
	}
	if err := install.Apply(home, kept); err != nil {
		fmt.Fprintf(stderr, "note: could not refresh the agent skill: %v\n", err)
		return
	}
	for _, s := range kept {
		if s.Action != install.Skip {
			fmt.Fprintf(stdout, "refreshed the agent skill at %s\n", s.Path)
		}
	}
}
