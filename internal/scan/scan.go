// Package scan is rtdd's built-in, language-agnostic code scanner (spec §4): definitions
// by the regex table in patterns.go, spans by braces or indentation, edges by name.
package scan

import (
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// FileResult is one file's scan: its nodes (IsTest unset), the method edges inside it,
// the calls each node makes, the modules the file imports, what each class extends, its
// line count and the paths it names. Calls, inherits, implements and references edges are
// made over every file at once (Link, References), because each names a definition or a
// file that may live anywhere.
type FileResult struct {
	Path    string
	Nodes   []graph.Node
	Edges   []graph.Edge
	Calls   map[string][]string // node ID -> sorted, de-duplicated call tokens (see Link)
	Imports []string            // import specs in line order; a relative one rooted at "/"
	Supers  map[string][]Super  // class node ID -> what its definition line extends or implements
	Lines   int                 // the file's line count: its file node's end (#504)
	// Refs is node ID — or Path, for the lines in no definition — -> the sorted path
	// tokens in its code (not its comments), see References.
	Refs map[string][]string
}

// callSite is `X(` with X a whole word.
var callSite = regexp.MustCompile(`[A-Za-z_$][\w$]*\(`)

// commandSite is a word in command position — the first word of a statement, followed by
// an argument or nothing — as in `add 1 2` (shell) or `helper x` (Ruby). An assignment
// (`x = 1`, `x := f()`), a member access or a literal (`Calc { total: 0 }`) is not one.
var commandSite = regexp.MustCompile(`(?:^|;|&&|\|\||\||\$\(|` + "`" + `)\s*([A-Za-z_][\w]*)(?:\s+[^\s=:+\-*/%<>!&|^.,;)\]{}]|\s*$)`)

// declared is what follows a declaration's leading word — `let c = …`, `let c: T`,
// `Calc c = …` — so the word is a keyword or a type, not a command.
var declared = regexp.MustCompile(`^\s+[A-Za-z_]\w*\s*(?::|=(?:[^=~]|$))`)

type def struct {
	name   string
	shape  shape
	start  int // 0-based
	end    int // 0-based, inclusive
	indent int
	parent int // index into defs, -1 at top level
}

// ScanFile scans one file's source. rel is its repo-relative, slash-separated path.
func ScanFile(rel string, src []byte) FileResult {
	lines := splitLines(src)

	var defs []def
	var open []int // defs whose span holds the current line, outermost first
	codeLines := code(lines)
	for i, line := range codeLines {
		for len(open) > 0 && defs[open[len(open)-1]].end < i {
			open = open[:len(open)-1]
		}
		inBody := len(open) > 0 && defs[open[len(open)-1]].shape != shapeClass
		name, sh, ok := matchDefinition(line, inBody)
		if !ok && !inBody {
			if h, wrapped := header(codeLines, i); wrapped {
				name, sh, ok = matchDefinition(h, false)
			}
		}
		if !ok {
			continue
		}
		defs = append(defs, def{name: name, shape: sh, start: i, end: endLine(codeLines, i), indent: indentOf(lines[i]), parent: -1})
		open = append(open, len(defs)-1)
	}

	// Nesting: a node's parent is the nearest earlier node whose span holds its start;
	// a child never outlives its parent.
	var stack []int
	for i := range defs {
		for len(stack) > 0 && defs[stack[len(stack)-1]].end < defs[i].start {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			p := stack[len(stack)-1]
			defs[i].parent = p
			if defs[i].end > defs[p].end {
				defs[i].end = defs[p].end
			}
		}
		stack = append(stack, i)
	}

	res := FileResult{Path: rel, Calls: map[string][]string{}, Supers: map[string][]Super{}, Lines: len(lines)}
	dir := pathpkg.Dir(rel)
	for _, spec := range imports(lines) {
		if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
			spec = pathpkg.Join(dir, spec)
			if spec == ".." || strings.HasPrefix(spec, "../") {
				continue // above the repository root
			}
			spec = "/" + spec
		}
		res.Imports = append(res.Imports, spec)
	}
	ids := make([]string, len(defs))
	seen := map[string]bool{}
	for i, d := range defs {
		qual := d.name
		for p := d.parent; p >= 0; p = defs[p].parent {
			qual = defs[p].name + "::" + qual
		}
		id := rel + "::" + qual
		if seen[id] {
			id += "@" + strconv.Itoa(d.start+1)
		}
		seen[id] = true
		ids[i] = id

		kind := graph.KindFunc
		switch {
		case d.shape == shapeClass:
			kind = graph.KindClass
		case d.shape == shapeTest:
			kind = graph.KindTest
		case d.parent >= 0 && defs[d.parent].shape == shapeClass:
			kind = graph.KindMethod
		}
		res.Nodes = append(res.Nodes, graph.Node{ID: id, File: rel, Name: d.name, Kind: kind, Start: d.start + 1, End: d.end + 1})
		if kind == graph.KindClass {
			if sups := supertypes(lines[d.start]); len(sups) > 0 {
				res.Supers[id] = sups
			}
		}
		if d.parent >= 0 && defs[d.parent].shape == shapeClass {
			res.Edges = append(res.Edges, graph.Edge{From: ids[d.parent], To: id, Relation: graph.RelMethod})
		}
	}

	// Ownership: a line belongs to the innermost node containing it. Parents precede
	// their children in defs, so a later write is always the more inner node.
	owner := make([]int, len(lines))
	for i := range owner {
		owner[i] = -1
	}
	for i, d := range defs {
		for l := d.start; l <= d.end; l++ {
			owner[l] = i
		}
	}
	// Each node's declared locals, for a call's receiver type (spec §4.3): its own lines,
	// definition line included, so parameters count. Read only for a node a receiver is
	// looked up in.
	lineOf := make([][]int, len(defs))
	for l := range lines {
		if o := owner[l]; o >= 0 {
			lineOf[o] = append(lineOf[o], l)
		}
	}
	decls := make([]map[string]string, len(defs)) // node -> receiver -> its type, "" for none
	declOf := func(d int, recv string) string {
		if decls[d] == nil {
			decls[d] = map[string]string{}
		}
		if t, ok := decls[d][recv]; ok {
			return t
		}
		t := ""
		for _, l := range lineOf[d] {
			if !strings.Contains(lines[l], recv) {
				continue
			}
			found := map[string]string{}
			declarations(lines[l], found)
			if x, ok := found[recv]; ok {
				t = x
				break
			}
		}
		decls[d][recv] = t
		return t
	}
	// token is the call token of name called on recv from node o: `X` bare or on self,
	// `T#X` with T recv's declared type, `recv.X` when recv has none, `.X` on no named
	// receiver.
	token := func(o int, recv string, member bool, name string) string {
		switch {
		case !member || selfReceivers[recv]:
			return name
		case recv == "":
			return "." + name
		}
		for d := o; d >= 0; d = defs[d].parent {
			if t := declOf(d, recv); t != "" {
				return t + "#" + name
			}
		}
		return recv + "." + name
	}
	named := map[int]map[string]bool{} // owner (-1: none) -> path tokens
	called := map[int]map[string]bool{}
	for l, line := range codeLines {
		o := owner[l]
		if named[o] == nil {
			named[o] = map[string]bool{}
		}
		pathTokens(line, named[o])
		if o < 0 || defs[o].start == l {
			continue // top-level code, or the node's own definition line
		}
		for _, loc := range callSite.FindAllStringIndex(line, -1) {
			if loc[0] > 0 && isWord(line[loc[0]-1]) {
				continue
			}
			name := line[loc[0] : loc[1]-1]
			// After `.` or `:` a keyword is a member or path segment (`Calc::new()`), a call.
			qualified := loc[0] > 0 && (line[loc[0]-1] == '.' || line[loc[0]-1] == ':')
			if controlKeywords[name] && !qualified || strings.Trim(name, "$") == "" {
				continue
			}
			if called[o] == nil {
				called[o] = map[string]bool{}
			}
			recv, member := receiverOf(line, loc[0])
			called[o][token(o, recv, member, name)] = true
		}
		for _, m := range commandSite.FindAllStringSubmatchIndex(line, -1) {
			name := line[m[2]:m[3]]
			if controlKeywords[name] || declared.MatchString(line[m[3]:]) {
				continue
			}
			if called[o] == nil {
				called[o] = map[string]bool{}
			}
			called[o][name] = true
		}
	}
	for o, names := range called {
		var list []string
		for n := range names {
			list = append(list, n)
		}
		sort.Strings(list)
		res.Calls[ids[o]] = list
	}
	for o, set := range named {
		if toks := sortedSet(set); toks != nil {
			if res.Refs == nil {
				res.Refs = map[string][]string{}
			}
			if o < 0 {
				res.Refs[rel] = toks
			} else {
				res.Refs[ids[o]] = toks
			}
		}
	}
	return res
}

// splitLines is src's lines, CRLF read as LF; none for an empty file.
func splitLines(src []byte) []string {
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// receiverOf reports what the call at line[at:] is made on: after `.`, `?.`, `::`, `:` or
// `->`, the word before it ("" when that is no word, as in `f().x(`; "super" for
// `super().x(`), and member true; otherwise member false.
func receiverOf(line string, at int) (string, bool) {
	j := at
	switch {
	case j > 0 && line[j-1] == '.':
		j--
		if j > 0 && line[j-1] == '?' {
			j--
		}
	case j > 1 && line[j-1] == '>' && line[j-2] == '-':
		j -= 2
	case j > 0 && line[j-1] == ':':
		j--
		if j > 0 && line[j-1] == ':' {
			j--
		}
	default:
		return "", false
	}
	if strings.HasSuffix(line[:j], superReceiver+"()") {
		return superReceiver, true
	}
	k := j
	for k > 0 && isWord(line[k-1]) {
		k--
	}
	return strings.Trim(line[k:j], "$"), true
}

func isWord(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func blank(line string) bool { return strings.TrimSpace(line) == "" }

// header joins line i to the lines its open ( or [ carries on to, until they close (at
// most 50 lines on), so a signature whose parameters wrap reads as one line. wrapped is
// false when line i leaves none open, or they never close.
func header(lines []string, i int) (string, bool) {
	var t tokenizer
	t.line(lines[i])
	if t.paren <= 0 {
		return "", false
	}
	parts := []string{lines[i]}
	for j := i + 1; j < len(lines) && j-i <= 50; j++ {
		t.line(lines[j])
		parts = append(parts, strings.TrimSpace(lines[j]))
		if t.paren <= 0 {
			return strings.Join(parts, " "), true
		}
	}
	return "", false
}

// endLine is spec §4.3's end rule for the definition on line i (0-based). The header is
// the definition line plus any lines its open ( or [ carries on to; when the header (or
// the line after it, when that is a bodyOpener) leaves a { open the node ends where
// braces balance, otherwise at the last non-blank line before the next non-blank line
// indented at or below the definition (or EOF).
func endLine(lines []string, i int) int {
	var t tokenizer
	header := i
	for j := i; j < len(lines); j++ {
		t.line(lines[j])
		header = j
		if t.paren <= 0 || j-i >= 50 {
			break
		}
	}
	// The body opens on the line after the header — Allman-style `{`, or `throws X {`.
	if t.brace == 0 && header+1 < len(lines) && indentOf(lines[header+1]) >= indentOf(lines[i]) && bodyOpener.MatchString(lines[header+1]) {
		header++
		t.line(lines[header])
	}
	if t.brace > 0 {
		for j := header + 1; j < len(lines); j++ {
			t.line(lines[j])
			if t.brace <= 0 {
				return j
			}
		}
		return len(lines) - 1
	}
	in := indentOf(lines[i])
	end := len(lines) - 1
	for j := header + 1; j < len(lines); j++ {
		if !blank(lines[j]) && indentOf(lines[j]) <= in {
			end = j - 1
			break
		}
	}
	for end > header && blank(lines[end]) {
		end--
	}
	return end
}

// tokenizer counts brackets outside string literals and comments, best-effort (spec
// §4.3). A comment is // or # (at the line's start or after a space) or -- (opening the
// line, before a space, -, [ or the end) to end of line — not a shell `cmd -- path` — or
// a /* block */ (see blockOpens). A block comment, a `backtick` string and a string
// opened by three " or three ' may span lines; any other string ends at end of line. A '
// opens a string only when another ' closes it later on the line, so a lone one (a Rust
// lifetime such as &'static, an apostrophe) does not swallow the line's brackets.
type tokenizer struct {
	paren, brace int
	block        bool
	quote        byte // the quote of a string still open at end of line, 0 for none
	triple       bool // whether that string's quote is tripled

	row   int             // the line being read, for code
	open  [2]int          // row and column of the comment or string open at end of line
	stray map[[2]int]bool // row and column of each opener that is none (see code)
}

func (t *tokenizer) line(s string) { t.scan(s, nil) }

// code is lines with every comment, and every string literal that spans lines, blanked
// to spaces, so no definition is read inside one; each line keeps its length. A comment
// or string still open at the end of the file opens none — a shell `ls /*`, or a backtick
// in a comment syntax the tokenizer does not know — so its opener is blanked alone and
// the lines from it are read again.
func code(lines []string) []string {
	t := tokenizer{stray: map[[2]int]bool{}}
	out := make([]string, len(lines))
	before := make([]tokenizer, len(lines)) // the state each line is read in
	for i := 0; i < len(lines); i++ {
		before[i] = t
		t.row = i
		mask := []byte(lines[i])
		t.scan(lines[i], mask)
		out[i] = string(mask)
		if i == len(lines)-1 && (t.block || t.quote != 0) {
			t.stray[t.open] = true
			i = t.open[0]
			t = before[i]
			i--
		}
	}
	return out
}

// scan reads line s. When mask is not nil, each byte of s in a comment, or in a string
// literal open at the start or the end of s, is blanked in mask.
func (t *tokenizer) scan(s string, mask []byte) {
	wipe := func(from, to int) {
		for ; mask != nil && from < to; from++ {
			mask[from] = ' '
		}
	}
	quote, triple := t.quote, t.triple
	from := 0 // where the open comment or string began on this line
	opened := t.open
	for k := 0; k < len(s); k++ {
		c := s[k]
		switch {
		case t.block:
			if c == '*' && k+1 < len(s) && s[k+1] == '/' {
				t.block = false
				k++
				wipe(from, k+1)
			}
		case quote != 0:
			if c == '\\' {
				k++
			} else if c == quote && (!triple || tripled(s, k)) {
				if triple {
					k += 2
				}
				if t.quote != 0 { // opened on an earlier line
					wipe(0, k+1)
					t.quote = 0
				}
				quote = 0
			}
		case t.stray[[2]int{t.row, k}]:
			n := 1
			if c == '/' {
				n = 2
			} else if c != '`' {
				n = 3
			}
			wipe(k, k+n)
			k += n - 1
		case (c == '"' || c == '\'') && tripled(s, k):
			quote, triple, from, opened = c, true, k, [2]int{t.row, k}
			k += 2
		case c == '"' || c == '`' || c == '\'' && closes(s, k):
			quote, triple, from, opened = c, false, k, [2]int{t.row, k}
		case c == '/' && k+1 < len(s) && s[k+1] == '/':
			wipe(k, len(s))
			return
		case c == '/' && k+1 < len(s) && s[k+1] == '*' && blockOpens(s, k):
			t.block, from, opened = true, k, [2]int{t.row, k}
			k++
		case c == '#' && (k == 0 || s[k-1] == ' ' || s[k-1] == '\t'),
			c == '-' && strings.HasPrefix(s[k:], "--") && blank(s[:k]) &&
				(k+2 == len(s) || strings.IndexByte(" \t-[", s[k+2]) >= 0):
			wipe(k, len(s))
			return
		case c == '(' || c == '[':
			t.paren++
		case c == ')' || c == ']':
			t.paren--
		case c == '{':
			t.brace++
		case c == '}':
			t.brace--
		}
	}
	switch {
	case t.block:
		wipe(from, len(s))
		t.open = opened
	case quote == '`' || triple && quote != 0:
		wipe(from, len(s))
		t.quote, t.triple, t.open = quote, triple, opened
	}
}

// blockOpens reports whether the /* at s[k] opens a comment: after the line's start, a
// space or punctuation but not after a comparison, and before a space, *, ! or the
// line's end — so a path glob such as `"$D"/parts/*.pck`, `dir/*` or a shell
// `[[ $p == /* ]]` opens none.
func blockOpens(s string, k int) bool {
	prev := strings.TrimRight(s[:k], " \t")
	return (k == 0 || strings.IndexByte(" \t(,;=:{", s[k-1]) >= 0) &&
		!strings.HasSuffix(prev, "==") && !strings.HasSuffix(prev, "!=") &&
		(k+2 == len(s) || strings.IndexByte(" \t*!", s[k+2]) >= 0)
}

// tripled reports whether s[k] begins three of itself.
func tripled(s string, k int) bool {
	return k+2 < len(s) && s[k+1] == s[k] && s[k+2] == s[k]
}

// closes reports whether the quote at s[k] has an unescaped match later in s.
func closes(s string, k int) bool {
	for j := k + 1; j < len(s); j++ {
		if s[j] == '\\' {
			j++
		} else if s[j] == s[k] {
			return true
		}
	}
	return false
}

// ScanFiles scans each of files (repo-relative, already Filtered) under root, in order,
// over one worker per CPU. A file that cannot be read is skipped.
func ScanFiles(root string, files []string) []FileResult {
	return ScanFilesProse(root, files, nil)
}

// ScanFilesProse is ScanFiles, reading each file prose reports true for with ScanProse.
func ScanFilesProse(root string, files []string, prose func(string) bool) []FileResult {
	scanned := make([]*FileResult, len(files))
	var next atomic.Int64
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), len(files)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := int(next.Add(1)) - 1; i < len(files); i = int(next.Add(1)) - 1 {
				b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(files[i])))
				if err != nil {
					continue
				}
				var r FileResult
				if prose != nil && prose(files[i]) {
					r = ScanProse(files[i], b)
				} else {
					r = ScanFile(files[i], b)
				}
				scanned[i] = &r
			}
		}()
	}
	wg.Wait()
	var out []FileResult
	for _, r := range scanned {
		if r != nil {
			out = append(out, *r)
		}
	}
	return out
}

// Assemble is the scanner's whole graph: every file's nodes and method edges, plus the
// edges Link makes across all of them.
func Assemble(results []FileResult) graph.Graph {
	g, _ := AssembleStats(results)
	return g
}

// AssembleStats is Assemble and what its Link resolved.
func AssembleStats(results []FileResult) (graph.Graph, Stats) {
	var g graph.Graph
	for _, r := range results {
		g.Nodes = append(g.Nodes, r.Nodes...)
		g.Edges = append(g.Edges, r.Edges...)
	}
	edges, st := link(g.Nodes, RefsOf(results))
	g.Edges = append(g.Edges, edges...)
	graph.Sort(&g)
	return g, st
}
