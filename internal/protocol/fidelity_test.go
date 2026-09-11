package protocol

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/doctor"
)

// fidelitySectionID is the section of protocol/PROTOCOL.md that carries the
// selection-fidelity contract. It is named here rather than inline because two
// tests and one Required list all have to agree on it.
const fidelitySectionID = "fidelity"

// TestFidelitySectionTargetsEveryFrontEnd pins the fidelity contract to all
// three generated front-ends. An agent that reads only AGENTS.md, or only the
// Cursor rule, has to learn what kind of evidence a selection is before it
// reads one — so a section carried by SKILL.md alone is not the contract.
func TestFidelitySectionTargetsEveryFrontEnd(t *testing.T) {
	d := parseRealProtocol(t)
	var sec *Section
	for i := range d.Sections {
		if d.Sections[i].ID == fidelitySectionID {
			sec = &d.Sections[i]
		}
	}
	if sec == nil {
		t.Fatalf("protocol/PROTOCOL.md has no section %q", fidelitySectionID)
	}
	for _, name := range []string{"skill", "agents", "mdc"} {
		if !sec.HasTarget(name) {
			t.Errorf("section %q does not target %q", fidelitySectionID, name)
		}
	}
}

// TestEveryFrontEndStatesTheFidelityContract is the acceptance test for the
// text itself: whatever wording each target uses, all three have to say that
// selection_fidelity exists and what its three values are, that a static
// selection is weaker evidence, and that rtdd doctor is where you find out
// which fidelity this repository can reach.
//
// It asserts against the fidelity constants and the doctor caveat rather than
// against copies of their strings, because a protocol that describes a
// vocabulary the binary does not emit is worse than one that says nothing.
func TestEveryFrontEndStatesTheFidelityContract(t *testing.T) {
	out, err := RenderAll(parseRealProtocol(t))
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	want := []string{
		"selection_fidelity",
		string(adapter.FidelityExecution),
		string(adapter.FidelityStatic),
		string(adapter.FidelityNone),
		"weaker evidence",
		"rtdd doctor",
	}
	for _, tgt := range Targets {
		body := out[tgt.OutPath]
		for _, w := range want {
			if !strings.Contains(body, w) {
				t.Errorf("%s does not state %q", tgt.OutPath, w)
			}
		}
	}
}

// TestFidelityTextAgreesWithTheDoctorCaveat keeps the protocol's wording and
// rtdd doctor's printed caveat from drifting apart. doctor.StaticSelectionCaveat
// exists so one static selection is not described one way on doctor's table and
// another in the document an agent parses; the skill carries it verbatim.
func TestFidelityTextAgreesWithTheDoctorCaveat(t *testing.T) {
	out, err := RenderAll(parseRealProtocol(t))
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	skill := out["dist/SKILL.md"]
	// The caveat is one sentence pair wrapped across source lines, so compare
	// on whitespace-collapsed text.
	if !strings.Contains(collapse(skill), collapse(doctor.StaticSelectionCaveat)) {
		t.Errorf("dist/SKILL.md does not carry doctor.StaticSelectionCaveat verbatim:\n%s", skill)
	}
}

// TestEveryTargetedSectionIsRequired closes the gap issue #223 closed: a
// section added to PROTOCOL.md but left out of a target's Required list is
// rendered into that target and then never asserted on, so gutting its body
// would pass `rtdd-gen verify`. Requiring every section a target carries makes
// the body assertions cover new sections automatically.
func TestEveryTargetedSectionIsRequired(t *testing.T) {
	d := parseRealProtocol(t)
	for _, tgt := range Targets {
		required := map[string]bool{}
		for _, id := range tgt.Required {
			required[id] = true
		}
		for _, s := range d.For(tgt.Name) {
			if !required[s.ID] {
				t.Errorf("target %q renders section %q but does not require it", tgt.Name, s.ID)
			}
		}
	}
}

func parseRealProtocol(t *testing.T) *Doc {
	t.Helper()
	src, err := readProtocolMD(t)
	if err != nil {
		t.Fatalf("read PROTOCOL.md: %v", err)
	}
	d, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return d
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// TestEveryTargetRejectsAGuttedFidelitySection is the per-target demonstration
// that the body assertions actually cover the new section. Issue #223's gap was
// that only AGENTS.md was body-checked, so a SKILL.md or .mdc whose section had
// been emptied to its heading still validated; each target is asserted here, not
// just the one that has no headings of its own.
func TestEveryTargetRejectsAGuttedFidelitySection(t *testing.T) {
	d := parseRealProtocol(t)
	out, err := RenderAll(d)
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	for _, tgt := range Targets {
		var body string
		for _, s := range d.For(tgt.Name) {
			if s.ID == fidelitySectionID {
				body = tgt.BodyOf(s)
			}
		}
		if body == "" {
			t.Fatalf("target %q renders no %q body", tgt.Name, fidelitySectionID)
		}
		gutted := strings.Replace(out[tgt.OutPath], body, "", 1)
		if gutted == out[tgt.OutPath] {
			t.Fatalf("target %q output does not contain its own %q body", tgt.Name, fidelitySectionID)
		}
		err := tgt.Validate(d, tgt, gutted)
		if err == nil {
			t.Errorf("target %q validated an output with the %q body removed", tgt.Name, fidelitySectionID)
			continue
		}
		if !strings.Contains(err.Error(), fidelitySectionID) {
			t.Errorf("target %q: error = %v, want it to name %q", tgt.Name, err, fidelitySectionID)
		}
	}
}
