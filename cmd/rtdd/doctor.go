package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/doctor"
)

// doctorDefaultLimit caps the table at a length a human reads in one glance. Fan-out has
// a long tail of files covered by one test; the head is the whole diagnostic.
const doctorDefaultLimit = 20

// RenderDoctor formats the fan-out table. The spec §9 caveat accompanies every fan-out
// this command computes, an empty map included: a fan-out number without it misleads.
//
// A limit of zero or less means no limit.
func RenderDoctor(hubs []doctor.Hub, total, limit int) string {
	var b strings.Builder

	if len(hubs) == 0 {
		b.WriteString("fan-out: the map is empty. Run `rtdd seed` first.\n\n")
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

// AdapterRow is one adapter as doctor reports it: where it came from and which of its
// declared prerequisites this machine lacks.
type AdapterRow struct {
	Name     string
	Src      string // repo-relative path for a host adapter, the embedded name otherwise
	Host     bool   // read from the repo's .rtdd/adapters/, not from the binary
	Override bool   // a host adapter that replaced a built-in of the same name
	Markers  []string
	Unmet    []adapter.Requirement
}

// adapterRows describes the given adapters, in the order they were passed — the order
// adapter.AvailableReport returns them, so repeated runs print the same table.
func adapterRows(repoRoot string, all []*adapter.Adapter) []AdapterRow {
	builtin := map[string]bool{}
	if bs, err := adapter.Builtin(); err == nil {
		for _, b := range bs {
			builtin[b.Name] = true
		}
	}
	rows := make([]AdapterRow, 0, len(all))
	for _, a := range all {
		host := adapter.IsHostAuthored(repoRoot, a)
		src := a.Src
		if host {
			src = relToRoot(repoRoot, a.Src)
		}
		rows = append(rows, AdapterRow{
			Name:     a.Name,
			Src:      src,
			Host:     host,
			Override: host && builtin[a.Name],
			Markers:  a.Detect,
			Unmet:    a.Unmet(lookPath),
		})
	}
	return rows
}

// origin is the row's provenance: built-in, host-authored, or a host override.
func (r AdapterRow) origin() string {
	switch {
	case r.Override:
		return "host-authored, overrides built-in"
	case r.Host:
		return "host-authored"
	}
	return "built-in"
}

// RenderAdapters formats the detected-adapter block: one row per adapter with its source
// and every declared prerequisite this machine lacks. Pure.
//
// A host file that failed to load is named along with the field that failed — doctor is
// the command you run to find out what is wrong, so it reports a broken adapter rather
// than dying on it.
func RenderAdapters(rows []AdapterRow, invalid []adapter.Invalid) string {
	var b strings.Builder
	b.WriteString("adapters\n\n")
	if len(rows) == 0 {
		b.WriteString("  none detected\n")
		b.WriteString("      no adapter's markers match this repository, so RTDD cannot select anything.\n")
		b.WriteString("      Fix: add .rtdd/adapters/<language>.yaml whose detect: globs match a file this\n")
		b.WriteString("      repository contains, then re-run rtdd doctor.\n")
	}
	nameW, srcW := 0, 0
	for _, r := range rows {
		nameW = max(nameW, len(r.Name))
		srcW = max(srcW, len(r.Src))
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "  %-*s  %-*s  (%s)\n", nameW, r.Name, srcW, r.Src, r.origin())
		for _, q := range r.Unmet {
			fmt.Fprintf(&b, "      %s is not on PATH: %s\n", q.Bin, q.Reason)
		}
	}
	for _, bad := range invalid {
		fmt.Fprintf(&b, "\n  not loaded: %s\n      %v\n", bad.Path, bad.Err)
	}
	b.WriteString("\n")
	return b.String()
}

// undetectedHeading labels the adapters that resolved for this repository but whose
// markers match nothing in it.
const undetectedHeading = "not detected in this repository"

// RenderUndetected formats the resolved-but-undetected adapters, with the markers that
// would have matched, so an author can tell a host file that never loaded from one whose
// detect: globs simply miss. Pure; nothing undetected renders the empty string.
func RenderUndetected(rows []AdapterRow) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(undetectedHeading + "\n\n")

	nameW, srcW := 0, 0
	for _, r := range rows {
		nameW = max(nameW, len(r.Name))
		srcW = max(srcW, len(r.Src))
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "  %-*s  %-*s  (%s)\n", nameW, r.Name, srcW, r.Src, r.origin())
		if len(r.Markers) == 0 {
			b.WriteString("      declares no detect: markers, so it can never be detected\n")
			continue
		}
		fmt.Fprintf(&b, "      no file matches its markers: %s\n", strings.Join(r.Markers, ", "))
	}
	b.WriteString("\n")
	return b.String()
}

// undetected returns the adapters in all that detection did not match, in the order of
// all. Pointer identity is the test: DetectAll returns elements of the slice it was given.
func undetected(all, detected []*adapter.Adapter) []*adapter.Adapter {
	hit := make(map[*adapter.Adapter]bool, len(detected))
	for _, a := range detected {
		hit[a] = true
	}
	var out []*adapter.Adapter
	for _, a := range all {
		if !hit[a] {
			out = append(out, a)
		}
	}
	return out
}

// cmdDoctor implements `rtdd doctor`: which adapters serve this repository and what they
// need installed first, and which files the most tests reach — with the spec §9 caveat that keeps the ranking from being read as an escalation
// trigger. It runs no tests, so its only non-zero exits are usage and environment errors.
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

	e, code, err := loadEnv("", stderr)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", err)
		return code
	}

	// A repo whose adapters cannot be resolved at all is still worth a fan-out table, so
	// the failure is reported and doctor carries on rather than exiting non-zero.
	all, invalid, aerr := adapter.AvailableReport(e.root)
	if aerr != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", aerr)
	}
	for i := range invalid {
		invalid[i].Path = relToRoot(e.root, invalid[i].Path)
	}

	// Rows are per DETECTED adapter: the resolved set still includes the built-in python
	// in a TypeScript repo. A detection-walk failure is reported and doctor carries on
	// with nothing detected — doctor is diagnostic, never fatal.
	detected, derr := adapter.DetectAll(e.root, all)
	if derr != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", derr)
	}
	fmt.Fprint(stdout, RenderAdapters(adapterRows(e.root, detected), invalid))
	fmt.Fprint(stdout, RenderUndetected(adapterRows(e.root, undetected(all, detected))))
	fmt.Fprint(stdout, RenderDoctor(doctor.Hubs(e.m), e.m.Len(), *limit))
	return 0
}
