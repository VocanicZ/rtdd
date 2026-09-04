package main

import (
	"fmt"
	"io"
)

// version, commit, and date are set by GoReleaser's ldflags (.goreleaser.yaml) at build
// time. A `go build` with no ldflags leaves them at these defaults.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// cmdVersion prints the build identity install.sh checks for after installing.
func cmdVersion(stdout io.Writer) int {
	fmt.Fprintf(stdout, "rtdd %s (commit %s, built %s)\n", version, commit, date)
	return 0
}
