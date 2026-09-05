package selector

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// Issue #275: the T2 reason a static adapter falls back to used to claim BOTH levels had
// been consulted, whatever was actually wired. A CLI that never supplied Exists therefore
// printed "neither declared correspondence nor imports reach the changed set" about a
// correspondence nobody ever evaluated. A skipped level must say it was skipped.
func TestUnanswerableReasonNamesOnlyTheLevelsThatWereConsulted(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Inputs)
		wantHas []string
		wantNot []string
	}{
		{
			name: "both levels consulted, both found nothing",
			mutate: func(in *Inputs) {
				in.Exists = existsIn()
				in.ImportDistance = func(string) map[string]int { return nil }
			},
			wantHas: []string{"typescript", "correspondence", "imports"},
			wantNot: []string{"was not consulted", "run rtdd seed"},
		},
		{
			name: "correspondence consulted, no importscan declared",
			mutate: func(in *Inputs) {
				in.Exists = existsIn()
				in.ImportDistance = nil
			},
			wantHas: []string{"typescript", "correspondence", "importscan", "not consulted"},
		},
		{
			name: "imports consulted, no test_for declared",
			mutate: func(in *Inputs) {
				in.Adapter.TestFor = nil
				in.Adapter.Importscan = &adapter.Importscan{Command: "node {script}", Script: "scan.js"}
				in.Exists = existsIn()
				in.ImportDistance = func(string) map[string]int { return nil }
			},
			wantHas: []string{"typescript", "imports", "test_for", "not consulted"},
			wantNot: []string{"declared correspondence names"},
		},
		{
			name: "templates declared but no resolver was supplied",
			mutate: func(in *Inputs) {
				in.Exists = nil
				in.ImportDistance = func(string) map[string]int { return nil }
			},
			wantHas: []string{"typescript", "correspondence", "not consulted"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := staticInputs()
			tc.mutate(&in)

			got := Select(in)

			if got.Tier != TierT2 {
				t.Fatalf("Tier = %v (%s), want TierT2", got.Tier, got.Reason)
			}
			for _, want := range tc.wantHas {
				if !contains(got.Reason, want) {
					t.Errorf("Reason = %q, want it to mention %q", got.Reason, want)
				}
			}
			for _, never := range tc.wantNot {
				if contains(got.Reason, never) {
					t.Errorf("Reason = %q, must not contain %q", got.Reason, never)
				}
			}
			if contains(got.Reason, "recorded coverage") {
				t.Errorf("Reason = %q, must not claim recorded coverage", got.Reason)
			}
		})
	}
}

// A level that was skipped must not be described as one that reached nothing. This is the
// sentence issue #275 filed as a lie, asserted directly.
func TestUnanswerableReasonDoesNotClaimASkippedCorrespondenceReachedNothing(t *testing.T) {
	in := staticInputs()
	in.Exists = nil
	in.ImportDistance = nil

	got := Select(in)

	if contains(got.Reason, "neither declared correspondence nor imports reach") {
		t.Errorf("Reason = %q: correspondence was never evaluated, so it may not be reported as having found nothing", got.Reason)
	}
}
