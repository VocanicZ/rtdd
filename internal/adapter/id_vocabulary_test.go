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
// internal/report imports nothing from internal/adapter — the edge runs one way, from the
// loader to the splitter it defers the structural verdict to (report.ValidateIDTemplate),
// so there is one parser and no direction to reverse.
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

// Adjacency is a STRUCTURAL rule about the id_template, and spec §4.3 puts it at load
// time with the rest of them: "{classname}{name}" renders an id nothing can split again,
// so the round trip PRD #231 AC3 requires cannot hold. Left to render time the refusal
// arrives only after `rtdd run` has cleared the report path and executed the whole subset
// command — the mid-run parse failure load-time validation exists to prevent — while
// `rtdd doctor`, `rtdd explain` and `rtdd init` all call the adapter healthy.
//
// The loader does not grow a second brace parser for it: internal/report's splitter is
// the one that decides, through the exported report.ValidateIDTemplate, so the two
// verdicts cannot drift.
func TestAdjacentPlaceholdersAreRejectedAtLoad(t *testing.T) {
	base := `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
`
	for _, tmpl := range []string{"{classname}{name}", "{file}{name}", "{file}{classname}{name}"} {
		t.Run(tmpl, func(t *testing.T) {
			a := &Adapter{Name: "probe", IDTemplate: tmpl}
			err := a.validateTemplates()
			if !errors.Is(err, report.ErrAmbiguousTemplate) {
				t.Fatalf("validateTemplates(%q) = %v, want errors.Is(_, report.ErrAmbiguousTemplate)", tmpl, err)
			}
			if !strings.Contains(err.Error(), tmpl) {
				t.Errorf("error = %q, want it to name the rejected template %q", err, tmpl)
			}

			// And through Load, so a host-authored .rtdd/adapters/*.yaml is refused at
			// exit 2 rather than mid-run.
			p := writeAdapter(t, t.TempDir(), "a.yaml", base+"id_template: \""+tmpl+"\"\n")
			if _, lerr := Load(p); !errors.Is(lerr, report.ErrAmbiguousTemplate) {
				t.Fatalf("Load(id_template %q) = %v, want errors.Is(_, report.ErrAmbiguousTemplate)", tmpl, lerr)
			}

			// RenderID is exported and must stay safe against a template that never went
			// through Load, so the render-time check stays too.
			if _, rerr := report.RenderID(tmpl, report.JUnitCase{Classname: "a", Name: "b", File: "f"}); !errors.Is(rerr, report.ErrAmbiguousTemplate) {
				t.Errorf("RenderID error = %v, want errors.Is(_, report.ErrAmbiguousTemplate)", rerr)
			}
			if _, perr := report.ParseID(tmpl, "ab"); !errors.Is(perr, report.ErrAmbiguousTemplate) {
				t.Errorf("ParseID error = %v, want errors.Is(_, report.ErrAmbiguousTemplate)", perr)
			}
		})
	}
}

// An id_template names at least one placeholder, or it is not a template: "classname#name"
// — the braces simply forgotten — renders that same constant id for every <testcase> in the
// report, de-duplication collapses the suite to one row whose status is whichever case came
// last, and a run whose first test failed reports green, exit 0. There is no error at load,
// none at parse, and nothing in the output that looks wrong, so the rejection belongs here,
// where the rest of the id_template vocabulary is already enforced (exit 2).
//
// An unterminated "{" is rejected on the same grounds: it is a typo for a placeholder, not
// a runner selector that happens to contain a brace, and left as a literal it fails exactly
// as silently.
func TestAnIDTemplateThatNamesNoPlaceholderIsRejectedAtLoad(t *testing.T) {
	base := `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
`
	sample := report.JUnitCase{Suite: "s", Classname: "calc.CalcTest", Name: "adds", File: "src/Calc.java"}
	for _, tmpl := range []string{"classname#name", "{name", "prefix{"} {
		t.Run(tmpl, func(t *testing.T) {
			p := writeAdapter(t, t.TempDir(), "a.yaml", base+"id_template: \""+tmpl+"\"\n")
			_, err := Load(p)
			if err == nil {
				t.Fatalf("Load accepted id_template %q; it renders one constant id for every test in the report", tmpl)
			}
			for _, want := range []string{"id_template", tmpl, "placeholder"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), want)
				}
			}

			// The renderer's vocabulary is the loader's, structural rules included.
			if _, rerr := report.RenderID(tmpl, sample); rerr == nil {
				t.Errorf("RenderID(%q) = nil error; the loader rejects it, so the renderer must too", tmpl)
			}
			if _, perr := report.ParseID(tmpl, "calc.CalcTest#adds"); perr == nil {
				t.Errorf("ParseID(%q) = nil error; the loader rejects it, so the reader must too", tmpl)
			}
		})
	}
}

// The rejection above is a rule about STRUCTURE, so it must not cost the shipped templates
// anything: every id_template the junit fixtures render through (internal/report's
// idFixtures) still loads and still renders.
func TestTheShippedIDTemplatesStillLoadAndRender(t *testing.T) {
	base := `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
`
	sample := report.JUnitCase{Suite: "s", Classname: "calc.CalcTest", Name: "adds", File: "src/Calc.java"}
	for _, tmpl := range []string{"{classname}", "{name}", "{classname}#{name}", "{file}", "{classname}::{name}", "{file}::{name}"} {
		t.Run(tmpl, func(t *testing.T) {
			p := writeAdapter(t, t.TempDir(), "a.yaml", base+"id_template: \""+tmpl+"\"\n")
			if _, err := Load(p); err != nil {
				t.Fatalf("Load(id_template %q): %v", tmpl, err)
			}
			if _, err := report.RenderID(tmpl, sample); err != nil {
				t.Fatalf("RenderID(%q): %v", tmpl, err)
			}
		})
	}
}
