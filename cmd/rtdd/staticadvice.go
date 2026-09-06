package main

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// The seed-advice surfaces (issue #279). `rtdd seed` is advice that only applies to an
// adapter that RECORDS coverage: a `selection: static` adapter declares `coverage: none`,
// so no map is ever built and seeding is a command that can do nothing for it. An empty
// map is that repository's steady state, not a missing setup step.
//
// PRD #230 AC9 fixed the three surfaces it enumerated — init's next-step line, seed's
// refusal, and the selection reason. This file is the same question asked by the surfaces
// AC9 did not enumerate: doctor's fan-out line, explain's empty-map branch, and which's
// no-map-row notice. They share one partition so the four cannot drift apart, which is
// how the defect survived on three of them in the first place.

// detectedAdapters resolves the adapter set that serves repoRoot, for the commands that
// need the WHOLE detected set rather than the single adapter loadEnv selects.
//
// Every failure resolves to "nothing detected". These are advisory surfaces: a repository
// whose adapters cannot be read is one where seed advice is as unfounded as anywhere
// else, and `doctor` — the command you run to find out what is wrong — already reports the
// broken files itself. Failing the command instead would take the diagnostic away.
func detectedAdapters(repoRoot string) []*adapter.Adapter {
	all, _, err := adapter.AvailableReport(repoRoot)
	if err != nil {
		return nil
	}
	detected, err := adapter.DetectAll(repoRoot, all)
	if err != nil {
		return nil
	}
	return detected
}

// selectionSplit partitions the DETECTED adapters into the ones that record nothing and
// the ones seeding is for, preserving the order they were detected in.
//
// It is a partition, not a flag: a polyglot repository has a coverage adapter to seed AND
// a static one seeding cannot help, and it needs to be told both — suppressing the
// guidance would strand the map the coverage adapter needs, and printing it unscoped would
// send the caller to seed a toolchain that records nothing.
func selectionSplit(detected []*adapter.Adapter) (static, coverage []*adapter.Adapter) {
	for _, a := range detected {
		if a == nil {
			continue
		}
		if a.Selection == adapter.SelectionStatic {
			static = append(static, a)
			continue
		}
		coverage = append(coverage, a)
	}
	return static, coverage
}

// adapterNames renders a partition for a human sentence.
func adapterNames(as []*adapter.Adapter) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}

// staticEvidence names what actually answers "which tests touch this file" for a static
// adapter: the correspondence templates it declares, and — only when it declares one —
// its import scan.
//
// The importscan half is conditional on the DECLARATION for the reason issue #275 gave
// for the `reason` string: a level that was never supplied is a level SKIPPED, and prose
// that names it tells the reader evidence was weighed that never was. Returns "" for an
// adapter that declares neither, which is fidelity `none`.
func staticEvidence(as []*adapter.Adapter) string {
	corr, imports := false, false
	for _, a := range as {
		if len(a.TestFor) > 0 {
			corr = true
		}
		if a.Importscan != nil {
			imports = true
		}
	}
	switch {
	case corr && imports:
		return "its declared test_for correspondence and the tests its importscan reports as importing it"
	case corr:
		return "its declared test_for correspondence"
	case imports:
		return "the tests its importscan reports as importing it"
	default:
		return ""
	}
}

// staticNothingRecorded is the clause every one of these surfaces shares: which adapters
// record nothing, and therefore why the map being empty is not a problem to fix. It is
// the sibling of init.go's staticClause and reads in the same voice.
func staticNothingRecorded(static []*adapter.Adapter) string {
	return fmt.Sprintf("%s, so no map is ever built", staticClause(adapterNames(static)))
}

// seedScope names the adapters `rtdd seed` does apply to, for the mixed case.
func seedScope(coverage []*adapter.Adapter) string {
	names := adapterNames(coverage)
	return fmt.Sprintf("the %s %s", strings.Join(names, ", "),
		plural(len(names), "adapter", "adapters"))
}

// seedPlan is decision 6 of docs/plans/06-m6d-shipped-adapters.md: which adapters `rtdd
// seed` runs in this repository, and the sentence that reports what it did about the rest.
//
// A mixed repository is the case #257 did not cover. Its refusal is for a repository whose
// coverage half is EMPTY — there, seeding genuinely cannot do anything — and applying it
// to a repository that also has a Python package strands the map that half needs, which is
// the defect rather than the safeguard. So:
//
//   - coverage half non-empty: seed exactly those adapters, and return one line naming the
//     static ones and why seeding cannot help them. The caller prints it and exits on the
//     run's own code.
//   - coverage half empty: return nothing to seed and #257's exit-2 refusal, which the
//     caller prints to stderr.
//
// The message is empty for a repository with no static half at all: it exists to scope the
// guidance away from an adapter that records nothing, and there is none to scope away from.
func seedPlan(detected []*adapter.Adapter) ([]*adapter.Adapter, string) {
	static, coverage := selectionSplit(detected)
	if len(coverage) == 0 {
		return nil, staticSeedRefusalFor(static)
	}
	if len(static) == 0 {
		return coverage, ""
	}
	return coverage, fmt.Sprintf("%s; seeding %s.\n",
		staticNothingRecorded(static), seedScope(coverage))
}

// staticSeedRefusalFor is #257's refusal over the whole static set.
//
// One static adapter renders the string #257 shipped, byte for byte — every existing
// static repository already reads it, and a plural rendering that churned it would be a
// gratuitous output change. Several render the same sentence in the plural, naming them
// all: a repository resolves a set since #309, and a refusal that named one of three
// leaves the reader believing the other two were seeded.
func staticSeedRefusalFor(static []*adapter.Adapter) string {
	names := adapterNames(static)
	if len(names) == 0 {
		return ""
	}
	return fmt.Sprintf("rtdd seed: the %s %s selection: static, so %s nothing "+
		"and there is no map to build.\n"+
		"  Nothing was written; seeding applies only to an adapter that records coverage.\n"+
		"  Run `rtdd which` instead: it selects from declared test_for correspondence and imports.\n",
		strings.Join(names, ", "),
		plural(len(names), "adapter declares", "adapters declare"),
		plural(len(names), "it records", "they record"))
}
