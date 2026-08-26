package adapter

// Command templates become argv here, and only here. The engine never hands a shell a
// string: a measured pytest id such as `tests/test_a.py::test_param[1-one two]` contains
// a space, a '-', a '|' and brackets, and it round-trips as a valid selector only when it
// is passed as its own argv element with no quoting and no escaping applied.

import (
	"fmt"
	"regexp"
	"strings"
)

// placeholderRe matches any {...} group, not just the known names. A brace group RTDD
// does not recognise is a typo in the adapter; letting it through as a literal would
// reach the runner as a nonsense argument and surface as an unreadable bad-selector exit.
var placeholderRe = regexp.MustCompile(`\{[^{}]*\}`)

// Expand substitutes {out} and {log} into a command template and returns argv.
//
// Those are the two names the engine's callers supply, not a set enforced here: Expand
// resolves whatever keys the vars map holds and rejects any placeholder it cannot
// resolve, so the caller's map is what decides which names a host adapter may use.
//
// The template is tokenised on whitespace BEFORE substitution, so a substituted value is
// never re-split: a log path containing a space stays one argv element.
//
// A template containing {tests} is an error — use ExpandTests. Splicing ids is not
// substitution, and a caller that reached the wrong function would join every id into a
// single argument.
func (a *Adapter) Expand(tmpl string, vars map[string]string) ([]string, error) {
	toks := strings.Fields(tmpl)
	if len(toks) == 0 {
		return nil, fmt.Errorf("adapter %s: empty command template", a.name())
	}
	out := make([]string, 0, len(toks))
	for _, tok := range toks {
		if strings.Contains(tok, "{tests}") {
			return nil, fmt.Errorf("adapter %s: template %q contains {tests}; use ExpandTests", a.name(), tmpl)
		}
		s, err := a.substitute(tok, vars)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// ExpandTests is Expand for a template containing the {tests} placeholder. Each test id
// is spliced in as its own argv element at that position, verbatim — ids are data, so no
// substitution is attempted inside one.
//
// An empty tests slice is an error rather than a command with the ids dropped: `pytest
// --cov` with no selectors collects the whole suite, which is the most expensive possible
// way to be wrong about an empty selection.
func (a *Adapter) ExpandTests(tmpl string, vars map[string]string, tests []string) ([]string, error) {
	if len(tests) == 0 {
		return nil, fmt.Errorf("adapter %s: ExpandTests with no test ids; the command would run the whole suite", a.name())
	}
	for i, id := range tests {
		if strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("adapter %s: empty test id at index %d", a.name(), i)
		}
	}

	toks := strings.Fields(tmpl)
	if len(toks) == 0 {
		return nil, fmt.Errorf("adapter %s: empty command template", a.name())
	}
	out := make([]string, 0, len(toks)+len(tests))
	sawTests := false
	for _, tok := range toks {
		if tok == "{tests}" {
			sawTests = true
			out = append(out, tests...)
			continue
		}
		s, err := a.substitute(tok, vars)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if !sawTests {
		return nil, fmt.Errorf("adapter %s: template %q has no {tests} placeholder; test ids would be silently dropped", a.name(), tmpl)
	}
	return out, nil
}

// substitute replaces every placeholder in one argv token, and reports the first one it
// cannot resolve.
func (a *Adapter) substitute(tok string, vars map[string]string) (string, error) {
	var bad string
	s := placeholderRe.ReplaceAllStringFunc(tok, func(m string) string {
		v, ok := vars[m[1:len(m)-1]]
		if !ok {
			if bad == "" {
				bad = m
			}
			return m
		}
		return v
	})
	if bad != "" {
		return "", fmt.Errorf("adapter %s: unknown placeholder %s in %q", a.name(), bad, tok)
	}
	return s, nil
}

// name keeps every message readable for an adapter that has not been through validate.
func (a *Adapter) name() string {
	if a == nil || a.Name == "" {
		return "?"
	}
	return a.Name
}
