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
