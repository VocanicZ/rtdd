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

// Every declared prerequisite is reported, in declaration order, ONE FINDING PER LINE —
// and this asserts both, because "contains npx" is satisfied by a renderer that drops a
// finding, reorders them, or runs them together into one paragraph.
func TestRenderRequirementsReportsEveryUnmetEntry(t *testing.T) {
	got := RenderRequirements([]adapter.UnmetFinding{
		{Adapter: "vitest", Req: adapter.Requirement{Bin: "npx", Reason: "resolves the vitest binary"}},
		{Adapter: "vitest", Req: adapter.Requirement{Bin: "node", Reason: "runs vitest"}},
		{Adapter: "python", Req: adapter.Requirement{Bin: "pytest", Reason: "runs the suite"}},
	})

	// One line per finding, in the order given. Everything else the block prints — the
	// heading and its blank lines — is not a finding line.
	var findings []string
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "not on PATH") {
			findings = append(findings, line)
		}
	}
	if len(findings) != 3 {
		t.Fatalf("got %d finding lines, want 3, one per unmet entry:\n%s", len(findings), got)
	}
	wantIn := [][]string{
		{"npx", "vitest", "resolves the vitest binary"},
		{"node", "vitest", "runs vitest"},
		{"pytest", "python", "runs the suite"},
	}
	for i, wants := range wantIn {
		for _, want := range wants {
			if !strings.Contains(findings[i], want) {
				t.Errorf("finding line %d = %q, want it to name %q (declaration order)", i, findings[i], want)
			}
		}
	}
}
