package main

import (
	"io"

	"github.com/VocanicZ/rtdd/internal/install"
)

// globalLegacy reports what an older `rtdd skill install` merged into the home directory's
// shared files, with "~/" before each path. It is a report on the side of another command,
// so a home directory it cannot resolve or read yields no lines rather than a failure.
func globalLegacy() []install.Step {
	home, err := homeDir()
	if err != nil {
		return nil
	}
	steps, err := install.PlanGlobalLegacyFiles(home)
	if err != nil {
		return nil
	}
	return install.ReportLegacy(steps, "~/")
}

// repoLegacy reports what an older `rtdd init` wrote outside .rtdd/ in the repository at
// root; like globalLegacy, trouble reading it yields no lines.
func repoLegacy(root string) []install.Step {
	// A home directory that cannot be found cannot be the root either (#554).
	home, _ := homeDir()
	steps, err := install.PlanLegacyFiles(root, install.PlanOptions{Home: home})
	if err != nil {
		return nil
	}
	return install.ReportLegacy(steps, "")
}

// homeShown returns steps with "~/" before each path, for printing a plan whose paths are
// relative to the home directory beside one whose paths are relative to the repository.
func homeShown(steps []install.Step) []install.Step {
	out := make([]install.Step, len(steps))
	for i, s := range steps {
		s.Path = "~/" + s.Path
		out[i] = s
	}
	return out
}

// renderLegacyList writes a legacy report as an indented list, "none" when it is empty.
func renderLegacyList(w io.Writer, steps []install.Step) {
	if len(steps) == 0 {
		io.WriteString(w, "  none\n")
	}
	for _, s := range steps {
		io.WriteString(w, "  "+s.Path+"  — "+s.Note+"\n")
	}
}
