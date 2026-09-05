package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// Spec §4.3 / PRD #229 AC9: the finding names the missing binary, the adapter needing it,
// and the reason — at doctor time, not as a mid-run parse failure against a report file
// that was never written.
func TestRenderRequirementsNamesBinAdapterAndReason(t *testing.T) {
	got := RenderRequirements([]adapter.UnmetFinding{{
		Adapter: "vitest",
		Req:     adapter.Requirement{Bin: "npx", Reason: "resolves the vitest binary from the lockfile"},
	}})

	for _, want := range []string{"npx", "vitest", "resolves the vitest binary from the lockfile", "not on PATH"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

// No findings must print nothing at all: a heading over an empty list reads as a problem.
func TestRenderRequirementsIsSilentWhenNothingIsMissing(t *testing.T) {
	if got := RenderRequirements(nil); got != "" {
		t.Errorf("RenderRequirements(nil) = %q, want the empty string", got)
	}
}

// Every declared prerequisite is reported, in declaration order, one finding per line.
func TestRenderRequirementsReportsEveryUnmetEntry(t *testing.T) {
	got := RenderRequirements([]adapter.UnmetFinding{
		{Adapter: "vitest", Req: adapter.Requirement{Bin: "node", Reason: "runs vitest"}},
		{Adapter: "vitest", Req: adapter.Requirement{Bin: "npx", Reason: "resolves the vitest binary"}},
	})
	for _, want := range []string{"node", "npx", "runs vitest", "resolves the vitest binary"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}
