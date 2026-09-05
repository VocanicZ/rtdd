package main

import (
	"flag"
	"fmt"
	"io"
	"os/exec"
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

// FidelityRow is one resolved adapter as doctor reports it: where it came from, the
// selection fidelity THIS repository can achieve with it, and why it landed there.
//
// Fidelity without Why is not an honesty surface. An agent cannot calibrate on a verdict
// with no cause attached, and a bare "static" reads as a failure rather than as the known
// ceiling of a toolchain RTDD cannot instrument (spec §6).
type FidelityRow struct {
	Name     string
	Src      string // repo-relative path for a host adapter, the embedded name otherwise
	Host     bool   // read from the repo's .rtdd/adapters/, not from the binary
	Override bool   // a host adapter that replaced a built-in of the same name
	Fidelity adapter.Fidelity
	Why      string // one clause naming what determined the fidelity
}

// fidelityRows describes the adapters resolved for repoRoot, in the order
// adapter.AvailableReport returns them so repeated runs print the same table.
func fidelityRows(repoRoot string, all []*adapter.Adapter) []FidelityRow {
	builtin := map[string]bool{}
	if bs, err := adapter.Builtin(); err == nil {
		for _, b := range bs {
			builtin[b.Name] = true
		}
	}
	rows := make([]FidelityRow, 0, len(all))
	for _, a := range all {
		host := adapter.IsHostAuthored(repoRoot, a)
		src := a.Src
		if host {
			src = relToRoot(repoRoot, a.Src)
		}
		rows = append(rows, FidelityRow{
			Name:     a.Name,
			Src:      src,
			Host:     host,
			Override: host && builtin[a.Name],
			Fidelity: a.Fidelity(),
			Why:      fidelityWhy(a),
		})
	}
	return rows
}

// fidelityWhy states what determined this adapter's fidelity, in the adapter's own keys.
// It reads `selection` and `coverage` rather than any separate assertion, because that
// derivation is the guarantee: an adapter declaring it records nothing can never report
// execution-derived selection (spec §4.2, §6).
func fidelityWhy(a *adapter.Adapter) string {
	switch a.Fidelity() {
	case adapter.FidelityExecution:
		return fmt.Sprintf("selection: %s with coverage: %s — tests are chosen from per-test coverage recorded by a real run",
			a.Selection, a.Coverage)
	case adapter.FidelityStatic:
		var have []string
		if n := len(a.TestFor); n > 0 {
			have = append(have, fmt.Sprintf("%d test_for %s", n, plural(n, "template", "templates")))
		}
		if a.Importscan != nil {
			have = append(have, "an importscan command")
		}
		return fmt.Sprintf("declares selection: static with %s, and coverage: %s — nothing is recorded, so tests are chosen from declared correspondence",
			strings.Join(have, " and "), a.Coverage)
	default:
		return "declares selection: static but no test_for templates and no importscan command"
	}
}

// RenderFidelity formats the selection-fidelity block printed above the fan-out table.
// Pure.
//
// It is the one place §4.5's override rule and §6's fidelity report meet: a repo whose
// shipped adapter has been replaced by its own YAML has to be able to see that, a host
// file that failed to load has to be named along with the field that failed — doctor is
// the command you run to find out what is wrong, so it reports a broken adapter rather
// than dying on it — and every row states the fidelity that repository can actually reach
// and why.
func RenderFidelity(rows []FidelityRow, invalid []adapter.Invalid) string {
	var b strings.Builder
	b.WriteString("selection fidelity\n\n")

	// A repo with no adapter at all is the state `rtdd init --force` leaves behind. It is
	// still a fidelity, and spec §6 says the value is never blank or omitted.
	if len(rows) == 0 {
		b.WriteString("  none resolved  none\n")
		b.WriteString("      no adapter resolves for this repository, so RTDD cannot select anything\n")
		b.WriteString("      narrower than the full suite.\n")
		b.WriteString("      Fix: add .rtdd/adapters/<language>.yaml declaring an adapter for this\n")
		b.WriteString("      toolchain, then re-run rtdd doctor.\n")
	}

	nameW, srcW := 0, 0
	for _, r := range rows {
		nameW = max(nameW, len(r.Name))
		srcW = max(srcW, len(r.Src))
	}
	for i, r := range rows {
		if i > 0 {
			b.WriteString("\n")
		}
		origin := "built-in"
		if r.Host {
			origin = "host-authored"
			if r.Override {
				origin += ", overrides built-in"
			}
		}
		fmt.Fprintf(&b, "  %-*s  %-*s  (%s)  %s\n", nameW, r.Name, srcW, r.Src, origin, r.Fidelity)
		fmt.Fprintf(&b, "      %s\n", r.Why)
		// `none` is never left as a bare verdict: it names the operational consequence the
		// agent has to act on — the full suite — and the edit that lifts it.
		if r.Fidelity == adapter.FidelityNone {
			b.WriteString("      so RTDD cannot select anything narrower than the full suite.\n")
			b.WriteString("      Fix: add a test_for template that resolves to a real test file, or an\n")
			b.WriteString("      importscan command, then re-run rtdd doctor.\n")
		}
	}

	// A file that did not load is reported by path AND by the error naming its field:
	// "one of your adapters is broken" is not something anyone can act on.
	for _, bad := range invalid {
		fmt.Fprintf(&b, "\n  not loaded: %s\n      %v\n", bad.Path, bad.Err)
	}

	if needsStaticCaveat(rows) {
		b.WriteString("\n")
		b.WriteString(doctor.StaticCaveat + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// needsStaticCaveat reports whether this block prints a fidelity weaker than
// execution-derived. A repo with no adapter at all is `none`, so it needs it too.
func needsStaticCaveat(rows []FidelityRow) bool {
	if len(rows) == 0 {
		return true
	}
	for _, r := range rows {
		if r.Fidelity != adapter.FidelityExecution {
			return true
		}
	}
	return false
}

// RenderRequirements formats the unmet-prerequisite findings. Pure; no findings renders
// the empty string, because a heading over an empty list reads as a problem.
//
// Spec §4.3: an unmet prerequisite surfaces here, naming the binary, the adapter that
// needs it and the declared reason — never as a mid-run parse failure against a report
// file that was never written.
func RenderRequirements(findings []adapter.UnmetFinding) string {
	if len(findings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("prerequisites\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "  %s is not on PATH — needed by adapter %s: %s\n", f.Req.Bin, f.Adapter, f.Req.Reason)
	}
	b.WriteString("\n")
	return b.String()
}

// unmetFindings collects every declared prerequisite that does not resolve on this
// machine, adapter by adapter, in declaration order.
func unmetFindings(all []*adapter.Adapter, lookPath func(string) (string, error)) []adapter.UnmetFinding {
	var out []adapter.UnmetFinding
	for _, a := range all {
		for _, r := range a.Unmet(lookPath) {
			out = append(out, adapter.UnmetFinding{Adapter: a.Name, Req: r})
		}
	}
	return out
}

// cmdDoctor implements `rtdd doctor`: what selection fidelity this repository can actually
// achieve and why, what its adapters need installed first, and which files the most tests
// reach — with the spec §9 caveat that keeps the ranking from being read as an escalation
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
	fmt.Fprint(stdout, RenderFidelity(fidelityRows(e.root, all), invalid))
	fmt.Fprint(stdout, RenderRequirements(unmetFindings(all, exec.LookPath)))
	fmt.Fprint(stdout, RenderDoctor(doctor.Hubs(e.m), e.m.Len(), *limit))
	return 0
}
