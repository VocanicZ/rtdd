// releasing_doc_test.go asserts docs/RELEASING.md is a SEQUENCE, not a list of topics.
// The order is the whole content: publishing the draft before the tag exists is
// impossible, and flipping visibility after the release is published leaks nothing but
// also helps nobody. A document that names the three steps in the wrong order is worse
// than none, because it will be followed.
package contract

import (
	"strings"
	"testing"
)

const releasingDoc = "docs/RELEASING.md"

// PRD #368 AC9: the exact human sequence, in order.
func TestReleasingDocStatesTheHumanSequenceInOrder(t *testing.T) {
	src := readRepoFile(t, releasingDoc)

	steps := []struct{ what, token string }{
		{"flip the repository visibility (#10)", "visibility"},
		{"push the v* tag", "v*"},
		{"publish the GoReleaser draft", "publish"},
	}
	prev := -1
	for _, s := range steps {
		i := strings.Index(src, s.token)
		if i < 0 {
			t.Errorf("%s never mentions %s (no %q)", releasingDoc, s.what, s.token)
			continue
		}
		if i < prev {
			t.Errorf("%s puts %s before the step that must precede it", releasingDoc, s.what)
		}
		prev = i
	}
	if !strings.Contains(src, "#10") {
		t.Errorf("%s does not attribute the visibility flip to issue #10", releasingDoc)
	}
}

// PRD #368 AC9, second half: the failure mode a first release walks into is named, with
// the config line that causes it and the symptom it produces.
func TestReleasingDocNamesTheDraftReleaseTrap(t *testing.T) {
	src := readRepoFile(t, releasingDoc)
	for _, want := range []struct{ what, token string }{
		{"the config that drafts the release", "draft: true"},
		{"the file that sets it", ".goreleaser.yaml"},
		{"the endpoint that stays 404", "releases/latest"},
		{"the status code a stranger sees", "404"},
		{"the script that cannot work until the draft is published", "install.sh"},
	} {
		if !strings.Contains(src, want.token) {
			t.Errorf("%s does not name %s (no %q)", releasingDoc, want.what, want.token)
		}
	}
	// The claim has to stay true of the config, not merely be written down once.
	if !strings.Contains(readRepoFile(t, ".goreleaser.yaml"), "draft: true") {
		t.Errorf(".goreleaser.yaml no longer sets `draft: true`, so %s is now wrong", releasingDoc)
	}
}

// Issue #373: the document is read by a human deciding whether to act and by an agent
// deciding whether it may. Both open human decisions have to be named as such, and the
// automated half has to be pointed at, so the split between "run this" and "decide this"
// survives in the doc rather than only in the plan.
func TestReleasingDocNamesTheHumanDecisionsAndThePreflight(t *testing.T) {
	src := readRepoFile(t, releasingDoc)
	for _, want := range []struct{ what, token string }{
		{"the visibility decision's issue", "#10"},
		{"the pre-registration signature decision's issue", "#8"},
		{"the script that runs the automated checks", "scripts/release-preflight.sh"},
		{"the block that script stops at", "DECISION REQUIRED"},
	} {
		if !strings.Contains(src, want.token) {
			t.Errorf("%s does not name %s (no %q)", releasingDoc, want.what, want.token)
		}
	}
	// "no agent performs these" is the whole reason the document exists: an agent that
	// reads it must not read the three steps as a runbook it may execute.
	if !strings.Contains(src, "no agent") {
		t.Errorf("%s does not state that no agent performs the three steps", releasingDoc)
	}
}
