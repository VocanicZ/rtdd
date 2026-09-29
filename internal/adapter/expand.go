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

// Expand substitutes placeholders into a command template and returns argv.
//
// Those are the two names the engine's callers supply, not a set enforced here: Expand
// resolves whatever keys the vars map holds and rejects any placeholder it cannot
// resolve, so the caller's map is what decides which names a host adapter may use.
//
// The template is tokenised on whitespace BEFORE substitution, so a substituted value is
// never re-split: a log path containing a space stays one argv element.
func (a *Adapter) Expand(tmpl string, vars map[string]string) ([]string, error) {
	toks := strings.Fields(tmpl)
	if len(toks) == 0 {
		return nil, fmt.Errorf("adapter %s: empty command template", a.name())
	}
	out := make([]string, 0, len(toks))
	for _, tok := range toks {
		s, err := a.substitute(tok, vars)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
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
