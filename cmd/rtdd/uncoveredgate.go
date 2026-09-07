package main

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// recordsCoverage reports whether this adapter's run produces coverage at all.
//
// It is the gate on both halves of issue #345, because both defects have the same
// cause: `rtdd run` printed the coverage vocabulary over an adapter that declares
// `coverage: none`. Nothing is instrumented, so no changed line can be KNOWN to be
// uncovered — with `coverage: none` EVERY changed line reported UNCOVERED
// unconditionally — and no map row can be recorded either.
//
// It reads the coverage axis rather than the selection axis on purpose. Both spell the
// same adapters today (Validate rejects `coverage: none` outside `selection: static`,
// and `selection: static` outside `coverage: none`), but what makes an uncovered report
// dishonest is that nothing was instrumented, and coverage is the field that says so.
//
// A nil adapter keeps the coverage reading — nothing declared otherwise, and that is the
// reading every existing repository has. This is the convention unmappedNoticeApplies
// already follows on the `which` surface.
func recordsCoverage(ad *adapter.Adapter) bool {
	return ad == nil || ad.Coverage != adapter.CoverageNone
}

// coverageWasRecorded reports whether ANY DETECTED adapter records coverage.
//
// It is the repository-level reading of recordsCoverage, and it gates the summary line's
// map-rows clause. The quantifier is "any", not "all": in a mixed repository the Python
// half's rows really are in the map, and dropping the count because a `coverage: none`
// adapter sits beside it would hide a number that was measured.
//
// It reads the DETECTED set rather than the blocks that ran, so a coverage repository
// whose subset invocation failed still reports its map the way it always has — what the
// clause is about is the repository, not this run's luck.
//
// An empty set keeps the coverage reading, for the same reason recordsCoverage's nil
// adapter does.
func coverageWasRecorded(ads []*adapter.Adapter) bool {
	if len(ads) == 0 {
		return true
	}
	for _, a := range ads {
		if recordsCoverage(a) {
			return true
		}
	}
	return false
}

// noCoverageReason is the one sentence that says why an adapter's run carries no
// uncovered report. The text surface and the --json document both render it, so the
// terminal and the machine-readable document can never explain the same absence
// differently.
func noCoverageReason(name string) string {
	return fmt.Sprintf("the %s adapter declares coverage: none, so nothing was "+
		"instrumented and no changed line can be known to be uncovered", name)
}

// RenderUncoveredFor is the whole post-run uncovered surface: the classified report for
// the adapters that recorded coverage, and one stated reason for each adapter that
// declares it records none.
//
// reports must already be filtered to coverage-recording adapters — a `coverage: none`
// block's classification is fabricated by construction, since Classify reports every
// changed line of an absent file as Uncovered and an uninstrumented run's coverage is
// empty for every file. noCoverage names the adapters whose report is absent, and the
// suppression is stated rather than silent for the reason spec §2 splits fidelity into
// two axes at all: an agent reads UNCOVERED as an instruction to go write a test, so an
// absent report that says nothing is read as "everything is covered".
//
// With no suppressed adapter the output is BYTE-IDENTICAL to RenderUncovered: the gate
// may only remove a claim that was never true, never reword the one that is.
func RenderUncoveredFor(reports []uncovered.FileReport, noCoverage []string) string {
	var b strings.Builder
	b.WriteString(RenderUncovered(reports))
	for _, name := range noCoverage {
		fmt.Fprintf(&b, "  no uncovered report: %s\n", noCoverageReason(name))
	}
	return b.String()
}
