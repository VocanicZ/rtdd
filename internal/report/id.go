package report

import (
	"errors"
	"fmt"
	"strings"
)

// id_template rendering. The template arrives as a string, never as an *adapter.Adapter:
// internal/report keeps its single dependency direction, and internal/adapter stays free
// of parsing.
//
// Plan 06-m6c Task 3 owns this file. What is here is what Task 5's ReadJUnitReport needs
// to turn a parsed <testcase> into an Outcome.Test — the renderer and the two errors it
// can produce. ParseID, RenderID's inverse, lands with the rest of Task 3.
var (
	// ErrNoFileAttr is a template naming {file} against a runner that does not emit the
	// attribute — Maven Surefire, go-junit-report and jest-junit by default. Deriving the
	// path would produce an id the runner does not recognise, so the subset would select
	// nothing and report green (decision 2).
	ErrNoFileAttr = errors.New("junit-xml: testcase has no file attribute")
	// ErrAmbiguousTemplate is an id_template whose placeholders are adjacent, e.g.
	// "{classname}{name}": it renders, but nothing can read it back, so the round-trip
	// PRD #231 AC3 requires cannot hold.
	ErrAmbiguousTemplate = errors.New("junit-xml: id_template placeholders need a literal separator")
)

// idPlaceholders is the vocabulary internal/adapter's validateTemplates already enforces
// at load time. {class} is deliberately not an alias for {classname}: two spellings for
// one key is worse than one rejection with a message (decision 1).
var idPlaceholders = map[string]bool{"{file}": true, "{classname}": true, "{name}": true}

// idSegment is one piece of a split template: a literal, or a placeholder.
type idSegment struct {
	text        string // the literal text, or the placeholder including its braces
	placeholder bool
}

// splitTemplate splits an id_template into its alternating literals and placeholders. It
// is shared so RenderID and ParseID reject exactly the same templates.
func splitTemplate(tmpl string) ([]idSegment, error) {
	var segs []idSegment
	rest := tmpl
	for {
		open := strings.Index(rest, "{")
		if open < 0 {
			break
		}
		close := strings.Index(rest[open:], "}")
		if close < 0 {
			break
		}
		close += open
		ph := rest[open : close+1]
		if !idPlaceholders[ph] {
			return nil, fmt.Errorf("report: id_template %q: unknown placeholder %s", tmpl, ph)
		}
		if open > 0 {
			segs = append(segs, idSegment{text: rest[:open]})
		} else if len(segs) > 0 {
			// The previous segment was a placeholder and this one begins where it ended:
			// the rendered id carries no literal to split on again.
			return nil, fmt.Errorf("report: %w: %q", ErrAmbiguousTemplate, tmpl)
		}
		segs = append(segs, idSegment{text: ph, placeholder: true})
		rest = rest[close+1:]
	}
	if rest != "" {
		segs = append(segs, idSegment{text: rest})
	}
	if len(segs) == 0 {
		return nil, fmt.Errorf("report: id_template %q names no placeholder", tmpl)
	}
	return segs, nil
}

// RenderID renders one parsed case into the runner's own selector syntax, expanding
// {file}, {classname} and {name}. The result is ONE argv token: ExpandTests splices each
// id as its own argument, so a template that renders a space produces one argument
// containing a space, which is what the runner then receives.
func RenderID(tmpl string, c JUnitCase) (string, error) {
	segs, err := splitTemplate(tmpl)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, s := range segs {
		if !s.placeholder {
			b.WriteString(s.text)
			continue
		}
		switch s.text {
		case "{file}":
			// A missing file= is never derived from classname or the suite name: a wrong
			// path renders an id the runner does not recognise, and the subset then
			// reports green having selected nothing (decision 2).
			if c.File == "" {
				return "", fmt.Errorf("report: %w: id_template %q names {file}: suite %q case %q: this runner does not emit file=; use an id_template that does not name {file}",
					ErrNoFileAttr, tmpl, c.Suite, c.Name)
			}
			b.WriteString(c.File)
		case "{classname}":
			// An absent classname is not an error: plenty of runners omit it on a
			// top-level test, and "" is a truthful rendering of what the report said.
			b.WriteString(c.Classname)
		case "{name}":
			b.WriteString(c.Name)
		}
	}
	return b.String(), nil
}
