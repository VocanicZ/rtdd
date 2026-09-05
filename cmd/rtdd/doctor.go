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

// AdapterRow is one resolved adapter as doctor reports it.
type AdapterRow struct {
	Name     string
	Src      string // repo-relative path for a host adapter, the embedded name otherwise
	Host     bool   // read from the repo's .rtdd/adapters/, not from the binary
	Override bool   // a host adapter that replaced a built-in of the same name
}

// adapterRows describes the adapters resolved for repoRoot, sorted the way
// adapter.AvailableReport returns them so repeated runs print the same table.
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
		})
	}
	return rows
}

// hostContributed reports whether the host repo's .rtdd/adapters/ affected this run,
// either by resolving an adapter or by failing to.
func hostContributed(rows []AdapterRow, invalid []adapter.Invalid) bool {
	if len(invalid) > 0 {
		return true
	}
	for _, r := range rows {
		if r.Host {
			return true
		}
	}
	return false
}

// RenderAdapters formats the adapter block printed above the fan-out table. Pure.
//
// It exists because §4.5's override rule must never be silent: a repo whose shipped
// adapter has been replaced by its own YAML has to be able to see that, and a host file
// that failed to load has to be named along with the field that failed — doctor is the
// command you run to find out what is wrong, so it reports a broken adapter rather than
// dying on it.
func RenderAdapters(rows []AdapterRow, invalid []adapter.Invalid) string {
	var b strings.Builder
	b.WriteString("adapters\n\n")

	if len(rows) == 0 {
		b.WriteString("  none resolved\n")
	}
	for _, r := range rows {
		origin := "built-in"
		if r.Host {
			origin = "host-authored"
			if r.Override {
				origin += ", overrides built-in"
			}
		}
		fmt.Fprintf(&b, "  %s  %s  (%s)\n", r.Name, r.Src, origin)
	}

	// A file that did not load is reported by path AND by the error naming its field:
	// "one of your adapters is broken" is not something anyone can act on.
	for _, bad := range invalid {
		fmt.Fprintf(&b, "\n  not loaded: %s\n      %v\n", bad.Path, bad.Err)
	}

	b.WriteString("\n")
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
	// The block is printed only when .rtdd/adapters/ actually contributed something. On
	// a repo with no host adapters the resolved set IS the shipped set, which the rest of
	// doctor's output already implies; printing it there would be noise on every repo
	// that exists today.
	if rows := adapterRows(e.root, all); hostContributed(rows, invalid) {
		fmt.Fprint(stdout, RenderAdapters(rows, invalid))
	}
	fmt.Fprint(stdout, RenderDoctor(doctor.Hubs(e.m), e.m.Len(), *limit))
	return 0
}
