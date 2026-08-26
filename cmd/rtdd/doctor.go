package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/VocanicZ/rtdd/internal/doctor"
)

// doctorDefaultLimit caps the table at a length a human reads in one glance. Fan-out has
// a long tail of files covered by one test; the head is the whole diagnostic.
const doctorDefaultLimit = 20

// RenderDoctor formats the fan-out table. The spec §9 caveat is ALWAYS included, empty
// map included: a fan-out number without it actively misleads, because anything executed
// once per process is attributed to whichever test happened to run first.
//
// A limit of zero or less means no limit.
func RenderDoctor(hubs []doctor.Hub, total, limit int) string {
	var b strings.Builder

	if len(hubs) == 0 {
		b.WriteString("fan-out: the map is empty. Run `rtdd seed` first.\n")
		b.WriteString("\n")
		b.WriteString(doctor.Caveat + "\n")
		return b.String()
	}

	shown := hubs
	if limit > 0 && limit < len(hubs) {
		shown = hubs[:limit]
	}
	// "top N of M" is a claim that something was left out; only make it when it is true.
	if len(shown) < len(hubs) {
		fmt.Fprintf(&b, "fan-out over %d %s (top %d of %d %s)\n",
			total, plural(total, "test", "tests"),
			len(shown), len(hubs), plural(len(hubs), "file", "files"))
	} else {
		fmt.Fprintf(&b, "fan-out over %d %s (%d %s)\n",
			total, plural(total, "test", "tests"),
			len(hubs), plural(len(hubs), "file", "files"))
	}
	b.WriteString("\n")
	b.WriteString("  tests  share  file\n")
	for _, h := range shown {
		fmt.Fprintf(&b, "  %5d  %4.0f%%  %s\n", h.TestCount, h.Fraction*100, h.Path)
	}
	b.WriteString("\n")
	b.WriteString(doctor.Caveat + "\n")
	return b.String()
}

// cmdDoctor implements `rtdd doctor`: which files the most tests reach, ranked, with the
// spec §9 caveat that keeps the ranking from being read as an escalation trigger. It runs
// no tests, so its only non-zero exits are usage and environment errors.
func cmdDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("limit", doctorDefaultLimit, "show at most n files (0 for all)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd doctor [--limit <n>]")
		return 2
	}

	e, code, err := loadEnv("")
	if err != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", err)
		return code
	}

	fmt.Fprint(stdout, RenderDoctor(doctor.Hubs(e.m), e.m.Len(), *limit))
	return 0
}
