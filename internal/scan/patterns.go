package scan

import (
	"regexp"
	"strings"
)

// This file is the ONLY place in rtdd where the shape of a programming language appears
// (spec §4.2; PRD #409 global constraint). Everything else in the scanner — spans,
// ownership, edges — is language-agnostic. A language the table misreads is fixed HERE,
// with a fixture under testdata/ that proves it; never with a branch on a file extension.

// shape is what a definition line declares. Whether a callable is a func or a method is
// not decided here: it is a method when its innermost enclosing node is a class.
type shape int

const (
	shapeCallable shape = iota
	shapeClass
	shapeTest
)

// ident is one identifier segment; a dotted or colon-separated path names its last
// segment (spec §4.2: `function M.load(` names `load`, `func (s *S) Load(` names `Load`).
const ident = `[A-Za-z_$][\w$]*`
const path = ident + `(?:(?:\.|::?)` + ident + `)*`

// modifiers are the words that may precede a definition keyword.
const modifiers = `(?:(?:export|default|pub(?:\([^)]*\))?|async|local|static|unsafe|abstract|final|sealed|data|public|private|protected|internal|override|inline|suspend|open)\s+)*`

type pattern struct {
	shape shape
	re    *regexp.Regexp // group 1 is the name
}

var patterns = []pattern{
	// test: it('label' / test("label" / describe(`label` / context('label'
	{shapeTest, regexp.MustCompile("^\\s*(?:it|test|describe|context)\\s*\\(\\s*['\"`]([^'\"`]*)")},
	// func: def / fn / func / function / fun / sub / proc, optional Go receiver
	{shapeCallable, regexp.MustCompile(`^\s*` + modifiers + `(?:def|fn|func|function|fun|sub|proc)\s+(?:\([^)]*\)\s*)?(` + path + `)`)},
	// class: class / struct / interface / trait / impl / module / object / enum
	{shapeClass, regexp.MustCompile(`^\s*` + modifiers + `(?:class|struct|interface|trait|module|object|enum)\s+(` + path + `)`)},
	{shapeClass, regexp.MustCompile(`^\s*` + modifiers + `impl(?:<[^>]*>)?\s+(?:` + path + `(?:<[^>]*>)?\s+for\s+)?(` + path + `)`)},
	// JS/TS assigned function: const Name = (async) function / (...) => / x =>
	{shapeCallable, regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+(` + ident + `)\s*=\s*(?:async\s+)?(?:function\b|\([^)]*\)\s*=>|` + ident + `\s*=>)`)},
	// shell: Name() {
	{shapeCallable, regexp.MustCompile(`^\s*(` + ident + `)\s*\(\)\s*\{`)},
	// method: <type tokens> Name(<params>) <modifiers> {? — no trailing ';'
	{shapeCallable, regexp.MustCompile(`^\s*((?:[\w$<>\[\],.?*&:]+\s+)+)[*&]*(` + ident + `)\s*\([^;]*$`)},
}

// controlKeywords never name a definition and never open a method-pattern line: they
// are how a call statement (`return add(x)`, `defer close(ch)`) would otherwise read as
// `<type> Name(`.
var controlKeywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`if else elif for foreach while do switch case catch try
		finally return throw throws new delete yield await defer go goto raise assert print
		puts echo local not and or in is del lambda when unless until then sizeof typeof
		with import from using package require`) {
		controlKeywords[k] = true
	}
}

// methodTail is what may follow a method's parameter list on its definition line.
var methodTail = regexp.MustCompile(`\)\s*(?:[\w$<>\[\],.?&:]+\s*)*\{?\s*$`)

// matchDefinition reports whether line defines a node, and its shape and name.
func matchDefinition(line string) (string, shape, bool) {
	for i, p := range patterns {
		m := p.re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if i == len(patterns)-1 { // the method row
			for _, tok := range strings.Fields(m[1]) {
				if controlKeywords[tok] {
					return "", 0, false
				}
			}
			if controlKeywords[m[2]] || !methodTail.MatchString(line) {
				return "", 0, false
			}
			return m[2], p.shape, true
		}
		if p.shape == shapeTest {
			return m[1], p.shape, true
		}
		return lastSegment(m[1]), p.shape, true
	}
	return "", 0, false
}

func lastSegment(s string) string {
	if i := strings.LastIndexAny(s, ".:"); i >= 0 {
		return s[i+1:]
	}
	return s
}
