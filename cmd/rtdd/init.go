package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/VocanicZ/rtdd/internal/initrepo"
)

// RenderInit formats what `rtdd init` installed. It is the only place this command's
// output is composed; initrepo itself never prints.
func RenderInit(acts []initrepo.Action) string {
	var b strings.Builder
	b.WriteString("rtdd init\n")
	for _, a := range acts {
		fmt.Fprintf(&b, "  %-9s  %s\n", a.Kind, a.Path)
	}
	b.WriteString("\n")
	b.WriteString("Next: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it.\n")
	return b.String()
}

// cmdInit implements `rtdd init`: it installs the union merge driver, the config and the
// agent front-ends into the working directory. Nothing is ever clobbered.
func cmdInit(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: rtdd init")
		return 2
	}
	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 3
	}
	acts, err := initrepo.Run(repoRoot)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 2
	}
	fmt.Fprint(stdout, RenderInit(acts))
	return 0
}
