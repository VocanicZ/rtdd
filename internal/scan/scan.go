// Package scan is rtdd's built-in, language-agnostic code scanner (spec §4): definitions
// by the regex table in patterns.go, spans by braces or indentation, edges by name.
package scan

import (
	"os"
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
// and the names each node calls. Calls edges are made by Link, over every file at once,
// because a call names a definition that may live anywhere.
type FileResult struct {
	Path  string
	Nodes []graph.Node
	Edges []graph.Edge
	Calls map[string][]string // node ID -> sorted, de-duplicated called names
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
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if text == "" {
		lines = nil
	}

	var defs []def
	var open []int // defs whose span holds the current line, outermost first
	for i, line := range lines {
		for len(open) > 0 && defs[open[len(open)-1]].end < i {
			open = open[:len(open)-1]
		}
		inBody := len(open) > 0 && defs[open[len(open)-1]].shape != shapeClass
		name, sh, ok := matchDefinition(line, inBody)
		if !ok {
			continue
		}
		defs = append(defs, def{name: name, shape: sh, start: i, end: endLine(lines, i), indent: indentOf(lines[i]), parent: -1})
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

	res := FileResult{Path: rel, Calls: map[string][]string{}}
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
	called := map[int]map[string]bool{}
	for l, line := range lines {
		o := owner[l]
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
			called[o][name] = true
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
	return res
}

// Link makes the calls edges: from each caller to EVERY node bearing a name it calls
// (spec §4.3 — over-linking is accepted; a direct call is never missed). nodes is the
// whole graph's node set, whichever source produced each node. No self edges.
func Link(nodes []graph.Node, calls map[string][]string) []graph.Edge {
	g := graph.Graph{Edges: link(nodes, calls)}
	graph.Sort(&g)
	return g.Edges
}

// link is Link unsorted, for a caller that sorts the whole graph once afterwards.
func link(nodes []graph.Node, calls map[string][]string) []graph.Edge {
	byName := map[string][]string{}
	for _, n := range nodes {
		byName[n.Name] = append(byName[n.Name], n.ID)
	}
	var out []graph.Edge
	for from, names := range calls {
		for _, name := range names {
			for _, to := range byName[name] {
				if to != from {
					out = append(out, graph.Edge{From: from, To: to, Relation: graph.RelCalls})
				}
			}
		}
	}
	return out
}

func isWord(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func blank(line string) bool { return strings.TrimSpace(line) == "" }

// endLine is spec §4.3's end rule for the definition on line i (0-based). The header is
// the definition line plus any lines its open ( or [ carries on to; when the header
// leaves a { open the node ends where braces balance, otherwise at the last non-blank
// line before the next non-blank line indented at or below the definition (or EOF).
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
// §4.3). String state ends at end of line; a /* block */ comment may span lines. A '
// opens a string only when another ' closes it later on the line, so a lone one (a
// Rust lifetime such as &'static, an apostrophe) does not swallow the line's brackets.
type tokenizer struct {
	paren, brace int
	block        bool
}

func (t *tokenizer) line(s string) {
	var quote byte
	for k := 0; k < len(s); k++ {
		c := s[k]
		switch {
		case t.block:
			if c == '*' && k+1 < len(s) && s[k+1] == '/' {
				t.block = false
				k++
			}
		case quote != 0:
			if c == '\\' {
				k++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '`' || c == '\'' && closes(s, k):
			quote = c
		case c == '/' && k+1 < len(s) && s[k+1] == '/':
			return
		case c == '/' && k+1 < len(s) && s[k+1] == '*':
			t.block = true
			k++
		case c == '#' && (k == 0 || s[k-1] == ' ' || s[k-1] == '\t'):
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
				r := ScanFile(files[i], b)
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
// calls edges Link makes across all of them.
func Assemble(results []FileResult) graph.Graph {
	var g graph.Graph
	calls := map[string][]string{}
	for _, r := range results {
		g.Nodes = append(g.Nodes, r.Nodes...)
		g.Edges = append(g.Edges, r.Edges...)
		for id, names := range r.Calls {
			calls[id] = names
		}
	}
	g.Edges = append(g.Edges, link(g.Nodes, calls)...)
	graph.Sort(&g)
	return g
}
