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

// ExpandTests is Expand for a template containing the {tests} placeholder. It splices the
// ids in the one shape the adapter declares — ids are data throughout, so no substitution
// is ever attempted inside one:
//
//	neither key: one bare argv element per id, at the token that is exactly "{tests}"
//	TestFlag:    TestFlag, id, TestFlag, id, ... at that same token
//	TestJoin:    the ids joined by TestJoin, substituted wherever {tests} appears in a
//	             token, so `-Dtest={tests}` becomes one argument
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
	if a.TestJoin != "" {
		return a.expandJoined(tmpl, toks, vars, tests)
	}
	out := make([]string, 0, len(toks)+2*len(tests))
	sawTests := false
	for _, tok := range toks {
		if tok == "{tests}" {
			sawTests = true
			for _, id := range tests {
				// An adapter declaring no flag emits the bare id; one declaring a flag
				// emits it before EACH id, because `gradle test --tests A B` reads B as a
				// task name and fails with "Task 'B' not found" — a broken-repo message
				// for what is really a one-selector-too-many command.
				if a.TestFlag != "" {
					out = append(out, a.TestFlag)
				}
				out = append(out, id)
			}
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

// expandJoined is ExpandTests under TestJoin: every id becomes part of ONE argv token, so
// {tests} is a substring of a token rather than the whole of it. That is the only way
// Surefire's single `-Dtest=` argument can be expressed at all.
//
// The joined string is substituted BEFORE the normal placeholder pass would run, and the
// pass is then skipped for the ids themselves: an id is data and may legitimately contain
// a brace group, which substitute would reject as an unknown placeholder.
func (a *Adapter) expandJoined(tmpl string, toks []string, vars map[string]string, tests []string) ([]string, error) {
	// An id already holding the separator would split back into two selectors inside the
	// joined token: the runner would select something nobody asked for, silently drop the
	// test that was asked for, and still exit 0. Name it here instead.
	for i, id := range tests {
		if strings.Contains(id, a.TestJoin) {
			return nil, fmt.Errorf("adapter %s: test id %q at index %d contains the test_join separator %q; it would split into two selectors", a.name(), id, i, a.TestJoin)
		}
	}
	joined := strings.Join(tests, a.TestJoin)

	out := make([]string, 0, len(toks))
	sawTests := false
	for _, tok := range toks {
		// The whole-token rule cannot decide this shape, so the substring does. Losing
		// the check would let a typo'd template run the whole suite in silence.
		if !strings.Contains(tok, "{tests}") {
			s, err := a.substitute(tok, vars)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
			continue
		}
		sawTests = true
		// Substitute the rest of the token first, so the ids — spliced last — are never
		// themselves scanned for placeholders.
		s, err := a.substitute(strings.ReplaceAll(tok, "{tests}", testsSentinel), vars)
		if err != nil {
			return nil, err
		}
		out = append(out, strings.ReplaceAll(s, testsSentinel, joined))
	}
	if !sawTests {
		return nil, fmt.Errorf("adapter %s: template %q has no {tests} placeholder; test ids would be silently dropped", a.name(), tmpl)
	}
	return out, nil
}

// testsSentinel stands in for {tests} across the one substitute pass expandJoined makes.
// It contains no brace, so placeholderRe cannot match it and substitute leaves it alone;
// and no adapter template or vars value can produce it, because every placeholder the
// engine resolves is written {like this}.
const testsSentinel = "\x00rtdd-tests\x00"

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
