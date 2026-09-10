package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/selector"
)

func plainCapable() []*adapter.Adapter {
	return []*adapter.Adapter{{Name: "python", Subset: "pytest {tests} --cov", SubsetPlain: "pytest {tests}"}}
}

func recordOnly() []*adapter.Adapter {
	return []*adapter.Adapter{{Name: "python", Subset: "pytest {tests} --cov"}}
}

func TestTheDefaultRecordsEveryCycle(t *testing.T) {
	// The shipped default must be byte-identical behaviour to every release before the
	// flag existed, whatever the tier or the map say.
	for _, tier := range []selector.Tier{selector.TierT0, selector.TierT1, selector.TierT2, selector.TierTS} {
		sel := selector.Selection{Tier: tier}
		if got, _ := shouldRecord(recordAlways, sel, nil, plainCapable()); !got {
			t.Errorf("--record=always did not record at tier %v", tier)
		}
	}
}

func TestAutoSkipsRecordingWhenTheMapAlreadyAnsweredTheChange(t *testing.T) {
	sel := selector.Selection{Tier: selector.TierT0, Tests: []string{"tests/test_auth.py::test_login"}}
	got, why := shouldRecord(recordAuto, sel, nil, plainCapable())
	if got {
		t.Fatal("--record=auto recorded a T0 cycle with nothing unmapped")
	}
	if !strings.Contains(why, "no uncovered report") {
		t.Errorf("reason = %q, want it to state that no uncovered report is available", why)
	}
}

func TestAutoRecordsWhenTheMapHasNothingForAChangedFile(t *testing.T) {
	// An unmapped changed file is precisely what a recording run would teach the map.
	sel := selector.Selection{Tier: selector.TierT0}
	if got, _ := shouldRecord(recordAuto, sel, []string{"src/brand_new.py"}, plainCapable()); !got {
		t.Fatal("--record=auto skipped recording while a changed file had no covering row")
	}
}

func TestAutoRecordsOnAFullRun(t *testing.T) {
	// T2 already pays full-suite cost, and it is the map's one chance to refresh wholesale.
	sel := selector.Selection{Tier: selector.TierT2}
	if got, _ := shouldRecord(recordAuto, sel, nil, plainCapable()); !got {
		t.Fatal("--record=auto skipped recording on a T2 full run")
	}
}

func TestAutoIsInertForAnAdapterWithNoPlainCommand(t *testing.T) {
	// adapters/python.yaml is byte-frozen (PRD #232, M6d global constraints) and declares
	// no subset_plain, so --record=auto must degrade to recording rather than fail or
	// silently run the instrumented command while claiming it did not.
	sel := selector.Selection{Tier: selector.TierT0}
	if got, _ := shouldRecord(recordAuto, sel, nil, recordOnly()); !got {
		t.Fatal("--record=auto skipped recording for an adapter with no subset_plain")
	}
}

func TestAutoRecordsWhenOnlySomeAdaptersCanRunPlain(t *testing.T) {
	// A cycle where half the adapters recorded and half did not is a map whose freshness
	// nobody could reason about afterwards.
	mixed := append(plainCapable(), recordOnly()...)
	sel := selector.Selection{Tier: selector.TierT0}
	if got, _ := shouldRecord(recordAuto, sel, nil, mixed); !got {
		t.Fatal("--record=auto ran plain with a mixed adapter set")
	}
}

func TestAPlainCycleIsNotReportedAsAnAdapterThatCannotInstrument(t *testing.T) {
	// The two are different claims: one is a per-cycle choice, the other a permanent
	// incapacity, and reporting the first as the second would libel a working adapter.
	skipped := notRecordedReason("python")
	cannot := noCoverageReason("python")
	if skipped == cannot {
		t.Fatal("a skipped cycle and a coverage: none adapter share one reason")
	}
	if !strings.Contains(skipped, "this cycle") {
		t.Errorf("reason = %q, want it scoped to this cycle", skipped)
	}
	if strings.Contains(skipped, "declares coverage: none") {
		t.Errorf("reason = %q claims the adapter cannot instrument", skipped)
	}
}
