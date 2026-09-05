package adapter

import (
	"errors"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/report"
)

// The id_template vocabulary is declared here and consumed there: internal/adapter rejects
// an unknown placeholder at load time (exit 2), and internal/report expands the ones that
// survive. Two hand-maintained lists would drift silently — a spelling this loader admitted
// and the renderer did not would fail mid-run, against a report the runner did write, which
// is the shape of failure the whole contract exists to move to load time.
//
// So the agreement is asserted by COMPUTING both verdicts over the same probes rather than
// by restating either list. The probes are the two shipped vocabularies plus the spellings
// an adapter author plausibly reaches for, including PRD #231's own {class}.
//
// internal/report imports nothing from internal/adapter, so this test-only edge is the
// whole of the coupling and the dependency direction is unchanged.
func TestIDTemplateVocabularyAgreesWithTheRenderer(t *testing.T) {
	probes := map[string]bool{}
	for ph := range idTemplatePlaceholders {
		probes[ph] = true
	}
	// test_for's vocabulary answers a different question, so every spelling in it that
	// id_template does not share must be rejected by BOTH sides here.
	for ph := range testForPlaceholders {
		probes[ph] = true
	}
	for _, ph := range []string{"{class}", "{classname }", "{suite}", "{path}", "{test}", "{File}", "{}"} {
		probes[ph] = true
	}

	sample := report.JUnitCase{Suite: "s", Classname: "calc.CalcTest", Name: "adds", File: "src/Calc.java"}
	accepted := 0
	for ph := range probes {
		a := &Adapter{Name: "probe", IDTemplate: ph}
		loads := a.validateTemplates() == nil

		_, rerr := report.RenderID(ph, sample)
		renders := rerr == nil
		if loads != renders {
			t.Errorf("id_template %q: loader accepts=%v but RenderID accepts=%v (%v); a template that loads must render, and a template that renders must load", ph, loads, renders, rerr)
		}

		// ParseID shares the splitter, so the reader's vocabulary is the renderer's; an
		// id that can be rendered but not read back is not an identifier.
		_, perr := report.ParseID(ph, "calc.CalcTest")
		if parses := perr == nil; parses != loads {
			t.Errorf("id_template %q: loader accepts=%v but ParseID accepts=%v (%v)", ph, loads, parses, perr)
		}

		if loads {
			accepted++
		}
	}
	// A renderer that accepted everything, or a loader that rejected everything, would
	// satisfy the equality above vacuously.
	if accepted != len(idTemplatePlaceholders) {
		t.Errorf("%d of %d probes were accepted by both sides, want exactly the %d shipped placeholders", accepted, len(probes), len(idTemplatePlaceholders))
	}
}

// Decision 1 of plan 06-m6c: {classname} is the one spelling, and PRD #231's {class} is
// not an alias for it. Two spellings for one key is worse than one rejection that names
// the placeholder, so the rejection is asserted on both sides.
func TestTheClassSpellingIsRejectedByTheLoaderAndTheRenderer(t *testing.T) {
	a := &Adapter{Name: "probe", IDTemplate: "{class}#{name}"}
	err := a.validateTemplates()
	if err == nil {
		t.Fatal("validateTemplates accepted {class}; the shipped spelling is {classname} and an alias would make both valid")
	}
	if !strings.Contains(err.Error(), "{class}") {
		t.Errorf("loader error %q does not name the rejected placeholder", err)
	}

	if _, rerr := report.RenderID("{class}#{name}", report.JUnitCase{Classname: "c", Name: "n"}); rerr == nil {
		t.Error("RenderID accepted {class}; the renderer's vocabulary must be the loader's")
	}

	// And the spelling that wins renders, through the loader and the renderer alike.
	ok := &Adapter{Name: "probe", IDTemplate: "{classname}#{name}"}
	if err := ok.validateTemplates(); err != nil {
		t.Fatalf("validateTemplates(%q): %v", ok.IDTemplate, err)
	}
	got, err := report.RenderID(ok.IDTemplate, report.JUnitCase{Classname: "calc.CalcTest", Name: "adds"})
	if err != nil {
		t.Fatalf("RenderID: %v", err)
	}
	if got != "calc.CalcTest#adds" {
		t.Errorf("RenderID = %q, want %q", got, "calc.CalcTest#adds")
	}
}

// The one place the two sides deliberately disagree, recorded rather than discovered:
// adjacency is a STRUCTURAL rule, not a vocabulary one. "{classname}{name}" names only
// shipped placeholders, so this loader admits it; it renders an id nothing can split
// again, so internal/report rejects it on both the way out and the way back. Plan 06-m6c
// gives internal/adapter exactly one new rule this milestone (report_path may not be a
// glob), so the check stays where the round trip is until a plan moves it.
func TestAdjacentPlaceholdersAreAStructuralRuleTheLoaderDoesNotPolice(t *testing.T) {
	const tmpl = "{classname}{name}"
	a := &Adapter{Name: "probe", IDTemplate: tmpl}
	if err := a.validateTemplates(); err != nil {
		t.Fatalf("validateTemplates(%q) = %v; every placeholder in it is in the shipped vocabulary", tmpl, err)
	}
	if _, err := report.RenderID(tmpl, report.JUnitCase{Classname: "a", Name: "b"}); !errors.Is(err, report.ErrAmbiguousTemplate) {
		t.Errorf("RenderID error = %v, want errors.Is(_, report.ErrAmbiguousTemplate)", err)
	}
	if _, err := report.ParseID(tmpl, "ab"); !errors.Is(err, report.ErrAmbiguousTemplate) {
		t.Errorf("ParseID error = %v, want errors.Is(_, report.ErrAmbiguousTemplate)", err)
	}
}
