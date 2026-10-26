package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
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
	// test, block form: it "label" do / describe 'label' do |x| — the trailing `do` keeps a
	// shell `test "$x" = y` out
	{shapeTest, regexp.MustCompile(`^\s*(?:it|test|describe|context)\s+['"]([^'"]*)['"].*\bdo\s*(?:\|[^|]*\|)?\s*$`)},
	// func: def / fn / func / function / fun / sub / proc, optional Go receiver
	{shapeCallable, regexp.MustCompile(`^\s*` + modifiers + `(?:def|fn|func|function|fun|sub|proc)\s+(?:\([^)]*\)\s*)?(` + path + `)`)},
	// class: class / struct / interface / trait / impl / module / object / enum
	{shapeClass, regexp.MustCompile(`^\s*` + modifiers + `(?:class|struct|interface|trait|module|object|enum)\s+(` + path + `)`)},
	{shapeClass, regexp.MustCompile(`^\s*` + modifiers + `impl(?:<[^>]*>)?\s+(?:` + path + `(?:<[^>]*>)?\s+for\s+)?(` + path + `)`)},
	// record: Java `record Name(<components>)`, C# `record (class|struct)? Name(` / `{` / `<`
	{shapeClass, regexp.MustCompile(`^\s*` + modifiers + `record\s+(?:class\s+|struct\s+)?(` + path + `)\s*[<({]`)},
	// JS/TS assigned function: const Name = (async) function / (...) => / x =>
	{shapeCallable, regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+(` + ident + `)\s*=\s*(?:async\s+)?(?:function\b|\([^)]*\)\s*=>|` + ident + `\s*=>)`)},
	// shell: Name() {
	{shapeCallable, regexp.MustCompile(`^\s*(` + ident + `)\s*\(\)\s*\{`)},
	// JS/TS class member: indented `Name(<params>)(: Type)? {` with no type tokens before it
	{shapeCallable, regexp.MustCompile(`^\s+(?:(?:static|async|get|set|public|private|protected|readonly|override)\s+)*(` + ident + `)\s*\([^)]*\)\s*(?::\s*[^={;]+)?\{\s*$`)},
	// method: @Annotation(…) <type tokens> Scope::Name(<params>) <modifiers> {? — no ';'
	// after the '(', unless the line is a whole one-line definition (see methodHead)
	{shapeCallable, regexp.MustCompile(`^\s*(?:@` + path + `(?:\([^()]*\))?\s+)*((?:[\w$<>\[\],.?*&:]+\s+)+)[*&]*(?:` + ident + `::)*(` + ident + `)\s*\(`)},
}

// controlKeywords never name a definition and never open a method-pattern line: they
// are how a call statement (`return add(x)`, `defer close(ch)`) would otherwise read as
// `<type> Name(`.
var controlKeywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`if else elif for foreach while do switch case catch try
		finally return throw throws new delete yield await defer go goto raise assert print
		puts echo local not and or in is del lambda when unless until then sizeof typeof
		with import from using package require end fi done esac`) {
		controlKeywords[k] = true
	}
	for _, k := range strings.Fields(`def fn func function fun sub proc class struct interface
		trait impl module object enum`) {
		definitionKeywords[k] = true
	}
}

// definitionKeywords are §4.2's definition keywords. One captured as a name is an
// anonymous literal (`func(x int) {`, `onDone: function (err) {`), not a node: its body
// belongs to the node that encloses it.
var definitionKeywords = map[string]bool{}

// methodTail is what may follow a method's parameter list on its definition line.
var methodTail = regexp.MustCompile(`\)\s*(?:[\w$<>\[\],.?&:]+\s*)*\{?\s*$`)

// methodHead reports whether line, a method row match whose parameter list opens just
// before at, is a definition: no ';' from there on and the line ends as methodTail says,
// or a one-line definition — a head that does, closing the parameter list, then a body
// { … } closing on the line or an expression body `=> …`.
func methodHead(line string, at int) bool {
	if !strings.Contains(line[at:], ";") && methodTail.MatchString(line) {
		return true
	}
	head := func(end int) bool {
		params := line[at:end]
		return !strings.Contains(params, ";") && strings.Count(params, ")")-strings.Count(params, "(") == 1 &&
			methodTail.MatchString(strings.TrimRight(line[:end], " \t")+"{")
	}
	if a := strings.Index(line[at:], "=>"); a >= 0 && head(at+a) {
		return true
	}
	b := strings.IndexByte(line[at:], '{') + at
	if b < at || !head(b) {
		return false
	}
	var t tokenizer
	t.line(line[b:])
	return t.brace == 0 && t.paren == 0 && strings.HasSuffix(strings.TrimSpace(line), "}")
}

// bodyOpener is a line after a definition's header that opens its body: `{` alone
// (Allman style) or after a trailing modifier, as in `throws IOException {` (Java) or
// `const override {` (C++).
var bodyOpener = regexp.MustCompile(`^\s*(?:(?:throws|const|noexcept|override|final)\b[^{};=]*)?\{\s*$`)

// matchDefinition reports whether line defines a node, and its shape and name. inBody is
// whether line lies inside a callable's or test's body, where a `<type> Name(` line is a
// statement (`x, len(y), z(w),`), so the method row does not apply; a nested definition
// is still found by the other rows.
func matchDefinition(line string, inBody bool) (string, shape, bool) {
	for i, p := range patterns {
		if inBody && i == len(patterns)-1 {
			continue
		}
		m := p.re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if i == len(patterns)-2 && controlKeywords[m[1]] { // the member row
			return "", 0, false
		}
		if i == len(patterns)-1 { // the method row
			for _, tok := range strings.Fields(m[1]) {
				if controlKeywords[tok] {
					return "", 0, false
				}
			}
			if controlKeywords[m[2]] || definitionKeywords[m[2]] || !methodHead(line, len(m[0])) {
				return "", 0, false
			}
			return m[2], p.shape, true
		}
		if p.shape == shapeTest {
			return m[1], p.shape, true
		}
		// After a definition keyword a control keyword is a real name (`fn new(`,
		// `function M.print(`); only another definition keyword is not.
		name := lastSegment(m[1])
		if definitionKeywords[name] {
			return "", 0, false
		}
		return name, p.shape, true
	}
	return "", 0, false
}

func lastSegment(s string) string {
	if i := strings.LastIndexAny(s, ".:"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// spanRules versions the language-agnostic rules in scan.go. Bump it with any change to
// how spans, ownership or edges are computed, so every cached graph is rebuilt.
const spanRules = "5"

// Fingerprint identifies this scanner: the pattern table, the keyword set, the call-site
// expressions and the span rules. The graph cache is valid only for the fingerprint that
// wrote it.
func Fingerprint() string {
	h := sha256.New()
	for _, p := range patterns {
		fmt.Fprintf(h, "%d %s\n", p.shape, p.re)
	}
	fmt.Fprintf(h, "%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n", methodTail, bodyOpener, callSite, commandSite, declared, sortedKeys(controlKeywords), sortedKeys(definitionKeywords), spanRules)
	for _, r := range declarationRows {
		fmt.Fprintf(h, "decl %d %d %s\n", r.name, r.typ, r.re)
	}
	for _, r := range supertypeRows {
		fmt.Fprintf(h, "super %s %s\n", r.rel, r.re)
	}
	for _, r := range importRows {
		fmt.Fprintf(h, "import %s %s\n", r.sep, r.re)
	}
	fmt.Fprintf(h, "%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n", strings.Join(importWords, " "), goImportBlock, goImportLine, generics, sortedKeys(supertypeWords), sortedKeys(selfReceivers), sortedKeys(bindingKeywords), superReceiver)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func sortedKeys(set map[string]bool) string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, " ")
}

// selfReceivers name the enclosing object: `self.X(` / `this.X(` resolve as a bare `X(`.
var selfReceivers = map[string]bool{"self": true, "this": true, "Self": true, "cls": true}

// superReceiver names the enclosing class's supertypes: `super.X(` / `super().X(`.
const superReceiver = "super"

// declarationRows find a local's declared type, for `obj.X(` (spec §4.3, receiver type
// hint). Each is (pattern, index of the name group, index of the type group).
var declarationRows = []struct {
	re        *regexp.Regexp
	name, typ int
}{
	// Type obj / final Type<T> obj = / Type[] obj; / (Type obj, — C-family declarations
	{regexp.MustCompile(`(?:^|[\s(,;{])(` + ident + `(?:\.` + ident + `)*)(?:<[^;=()]*>)?(?:\[\])*\s+(` + ident + `)\s*(?:[=;,)]|$)`), 2, 1},
	// obj: Type / obj: &mut Type — Python, TypeScript, Kotlin, Swift, Rust annotations
	// — but not Lua's `obj:method(`, a call
	{regexp.MustCompile(`(?:^|[^\w$:])(` + ident + `)\??\s*:\s*(?:&\s*)?(?:mut\s+)?(` + ident + `(?:\.` + ident + `)*)\s*(?:[^\w$.(\s]|$)`), 1, 2},
	// obj = new Type( / obj := &Type{ / obj = Type( / let obj = Type::new( — a construction
	{regexp.MustCompile(`(?:^|[^\w$.])(` + ident + `)\s*:?=\s*(?:new\s+|&)?([A-Z][\w$]*(?:\.[A-Z][\w$]*)*)(?:<[^;=()]*>)?(?:::\w+)?\s*[({]`), 1, 2},
}

// bindingKeywords introduce a local without naming its type (`const s = new Store()`), so
// one is never a declared type; a later row reads the construction instead.
var bindingKeywords = map[string]bool{"var": true, "let": true, "const": true, "val": true, "mut": true, "auto": true, "dim": true, "my": true, "our": true}

// declarations reports the locals line declares, name -> type (the type's last segment).
func declarations(line string, into map[string]string) {
	for _, row := range declarationRows {
		for _, m := range row.re.FindAllStringSubmatch(line, -1) {
			name, typ := m[row.name], lastSegment(m[row.typ])
			if controlKeywords[name] || controlKeywords[typ] || definitionKeywords[typ] || bindingKeywords[typ] || selfReceivers[name] {
				continue
			}
			if _, ok := into[name]; !ok {
				into[name] = typ
			}
		}
	}
}

// supertypeRows read what a class line extends or implements (spec §4.3). Group 1 is a
// comma-separated list of type names.
var supertypeRows = []struct {
	re  *regexp.Regexp
	rel graph.Relation
}{
	// Java, TypeScript, PHP: class A extends B implements C, D / interface A extends B, C
	{regexp.MustCompile(`\bextends\s+(.+?)\s*(?:\bimplements\b|\{|$)`), graph.RelInherits},
	{regexp.MustCompile(`\bimplements\s+(.+?)\s*(?:\{|$)`), graph.RelImplements},
	// Python: class A(B, C):
	{regexp.MustCompile(`^\s*class\s+` + ident + `\s*\(([^)]*)\)\s*:\s*(?:#.*|pass\s*)?$`), graph.RelInherits},
	// C++, C#, Kotlin, Swift: class A : public B, C / struct A : B / class A(x: Int) : B()
	{regexp.MustCompile(`^\s*` + modifiers + `(?:class|struct|interface|object|enum)\s+` + ident + `(?:<[^>]*>)?(?:\([^)]*\))?\s*:\s*([^{:][^{]*)`), graph.RelInherits},
	// Ruby: class A < B
	{regexp.MustCompile(`^\s*class\s+` + path + `\s*<\s*(` + path + `)`), graph.RelInherits},
	// Rust: impl Trait for Type
	{regexp.MustCompile(`^\s*` + modifiers + `impl(?:<[^>]*>)?\s+(` + path + `)(?:<[^>]*>)?\s+for\s+`), graph.RelImplements},
}

// generics strips one level of <…> and (…) groups, so a list splits on its own commas.
var generics = regexp.MustCompile(`<[^<>]*>|\([^()]*\)`)

var identOnly = regexp.MustCompile(`^` + ident + `$`)

// supertypeWords are the access and virtual specifiers a supertype list may carry.
var supertypeWords = map[string]bool{"public": true, "private": true, "protected": true, "virtual": true, "object": true, "pass": true}

// supertypes reports what the class defined on line extends or implements.
func supertypes(line string) []Super {
	var out []Super
	for _, row := range supertypeRows {
		m := row.re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		list := m[1]
		for prev := ""; prev != list; {
			prev, list = list, generics.ReplaceAllString(list, "")
		}
		for _, item := range strings.Split(list, ",") {
			if strings.Contains(item, "=") { // a keyword argument: metaclass=ABCMeta
				continue
			}
			var name string
			for _, w := range strings.Fields(item) {
				if !supertypeWords[w] {
					name = w
				}
			}
			name = lastSegment(strings.TrimRight(name, "{ "))
			if identOnly.MatchString(name) && !supertypeWords[name] {
				out = append(out, Super{Name: name, Relation: row.rel})
			}
		}
	}
	return out
}

// importRows read an import line's module spec (spec §4.3, imported files). sep is the
// spec's path separator, turned into "/"; a spec beginning "./" or "../" is relative to
// the importing file. Group 1 is the spec; a group 2, when present, lists names the spec
// imports from, each of which may itself be a module.
var importRows = []struct {
	re  *regexp.Regexp
	sep string
}{
	// Java, Kotlin, Scala, Python: import a.b.C / import static a.b.C.m; / import a.b.*; /
	// import a.b as c, d — each item of the list is a spec
	{regexp.MustCompile(`^\s*import\s+(?:static\s+)?([A-Za-z_][\w.]*(?:\.\*)?(?:\s+as\s+\w+)?(?:\s*,\s*[A-Za-z_][\w.]*(?:\s+as\s+\w+)?)*)\s*;?\s*$`), "list"},
	// Python: from a.b import c, d / from . import c / from ..a import b
	{regexp.MustCompile(`^\s*from\s+(\.*[\w.]*)\s+import\s+\(?([\w\s,]+)`), "."},
	// JavaScript, TypeScript: import x from './y' / export { x } from 'y' / } from './y'
	{regexp.MustCompile(`\bfrom\s+['"]([^'"]+)['"]`), ""},
	// JavaScript, TypeScript, Go: import './y' / import "a/b"
	{regexp.MustCompile(`^\s*import\s+['"]([^'"]+)['"]`), ""},
	// require('./y') / require "a.b" (Lua) / require 'a/b' (Ruby) / import('./y')
	{regexp.MustCompile(`\b(?:require|import)\s*\(?\s*['"]([^'"]+)['"]`), "."},
	// Ruby: require_relative 'x'
	{regexp.MustCompile(`\brequire_relative\s*\(?\s*['"]([^'"]+)['"]`), "rel"},
	// Go: import x "a/b"; a block's lines are read by imports below
	{regexp.MustCompile(`^\s*import\s+[\w.]+\s+"([^"]+)"`), ""},
	// Rust: use crate::a::b::C; / use a::{b, c}; PHP: use A\B\C;
	{regexp.MustCompile(`^\s*` + modifiers + `use\s+([\w:\\]+?)(?:::\{([^}]*)\}|::\*)?(?:\s+as\s+\w+)?\s*;`), "::"},
	// Rust: mod x;
	{regexp.MustCompile(`^\s*` + modifiers + `mod\s+(` + ident + `)\s*;`), "rel"},
	// C, C++: #include "x.h"
	{regexp.MustCompile(`^\s*#\s*include\s+"([^"]+)"`), "include"},
	// C#: using A.B;
	{regexp.MustCompile(`^\s*using\s+(?:static\s+)?([A-Za-z_][\w.]*)\s*;`), "."},
	// shell: source x / . x
	{regexp.MustCompile(`^\s*(?:source|\.)\s+['"]?([^'"\s;|&=][^'"\s;|&]*)`), "path"},
}

// importWords: one is on every line an import row can match (or the line starts with a
// shell `. `), so a line with none is skipped before the rows are tried.
var importWords = []string{"import", "from", "require", "use", "mod", "include", "using", "source"}

func mayImport(line string) bool {
	for _, w := range importWords {
		if strings.Contains(line, w) {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), ". ")
}

// goImportBlock opens and goImportLine is one line of a Go `import ( … )` block.
var (
	goImportBlock = regexp.MustCompile(`^\s*import\s*\(\s*$`)
	goImportLine  = regexp.MustCompile(`^\s*(?:[\w.]+\s+)?"([^"]+)"`)
)

// imports reports the module specs lines import, each "/"-separated: relative ones begin
// "./" or "../", the rest are module paths matched against the repository's files.
func imports(lines []string) []string {
	var out []string
	add := func(s string) {
		if s != "" && s != "." && s != "./" {
			out = append(out, s)
		}
	}
	inGoBlock := false
	for _, line := range lines {
		if inGoBlock {
			if strings.HasPrefix(strings.TrimSpace(line), ")") {
				inGoBlock = false
			} else if m := goImportLine.FindStringSubmatch(line); m != nil {
				add(m[1])
			}
			continue
		}
		if !mayImport(line) {
			continue
		}
		if goImportBlock.MatchString(line) {
			inGoBlock = true
			continue
		}
		for _, row := range importRows {
			m := row.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			spec := m[1]
			switch row.sep {
			case "list":
				for _, item := range strings.Split(spec, ",") {
					item = strings.TrimSuffix(strings.Fields(item)[0], ".*")
					add(strings.ReplaceAll(item, ".", "/"))
				}
			case "rel":
				add("./" + spec)
			case "include":
				add("./" + spec)
				add(spec)
			case "path":
				var keep []string
				for _, seg := range strings.Split(spec, "/") {
					if !strings.Contains(seg, "$") {
						keep = append(keep, seg)
					}
				}
				spec = strings.Join(keep, "/")
				if strings.HasPrefix(spec, "/") {
					spec = strings.TrimPrefix(spec, "/")
				}
				add(spec)
			case "::":
				spec = strings.ReplaceAll(strings.ReplaceAll(spec, "::", "/"), `\`, "/")
				spec = strings.TrimSuffix(spec, "/")
				switch {
				case strings.HasPrefix(spec, "crate/"):
					spec = strings.TrimPrefix(spec, "crate/")
				case strings.HasPrefix(spec, "self/"):
					spec = "./" + strings.TrimPrefix(spec, "self/")
				case strings.HasPrefix(spec, "super/"):
					spec = "../" + strings.TrimPrefix(spec, "super/")
				}
				add(spec)
				if len(m) > 2 {
					for _, n := range strings.Split(m[2], ",") {
						if n = strings.TrimSpace(n); n != "" && n != "self" && n != "*" {
							add(spec + "/" + n)
						}
					}
				}
			case ".":
				if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.Contains(spec, "/") {
					add(spec)
					break
				}
				dots := len(spec) - len(strings.TrimLeft(spec, "."))
				mod := strings.ReplaceAll(spec[dots:], ".", "/")
				if dots > 0 { // Python relative: one dot is this package, each more one up
					mod = "./" + strings.Repeat("../", dots-1) + mod
				}
				add(mod)
				if len(m) > 2 && m[2] != "" {
					for _, n := range strings.Split(m[2], ",") {
						if n = strings.TrimSpace(n); n != "" && n != "*" {
							add(strings.TrimSuffix(mod, "/") + "/" + strings.Fields(n)[0])
						}
					}
				}
			default:
				add(spec)
			}
			break
		}
	}
	return out
}
