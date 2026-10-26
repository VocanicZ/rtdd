package scan

import (
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// refRule names the path-token rule below; it is part of Fingerprint, so a cache scanned
// under another rule is a miss.
const refRule = "refs/1: runs of [A-Za-z0-9_.-/], not followed by (, with a / or a lower-case extension of at most 16"

// maxRefLen is the longest token kept as a path, maxExtLen the longest extension
// (`.gitattributes`'s is 13).
const (
	maxRefLen = 200
	maxExtLen = 16
)

// pathTokens adds to set every token of line that may name a file: a maximal run of path
// characters, less trailing dots, that is not a call (followed by `(`) and either holds a
// `/` before a non-empty last segment or ends in a lower-case extension (`app.yaml`,
// `.gitattributes`). `n.ID` and `fmt.Println(` are none; `self.name` is one, and links to
// nothing unless a file is so named.
func pathTokens(line string, set map[string]bool) {
	for i := 0; i < len(line); {
		if !isPathByte(line[i]) {
			i++
			continue
		}
		j := i
		for j < len(line) && isPathByte(line[j]) {
			j++
		}
		tok := strings.TrimRight(line[i:j], ".")
		if j < len(line) && line[j] == '(' || len(tok) > maxRefLen {
			i = j
			continue
		}
		if namesFile(tok) {
			set[tok] = true
		}
		i = j
	}
}

func isPathByte(c byte) bool {
	return isWord(c) && c != '$' || c == '.' || c == '-' || c == '/'
}

// namesFile is pathTokens' shape rule for one token.
func namesFile(tok string) bool {
	base := tok
	if i := strings.LastIndexByte(tok, '/'); i >= 0 {
		return strings.Trim(tok[i+1:], ".-") != ""
	}
	dot := strings.LastIndexByte(base, '.')
	if dot < 0 || dot == len(base)-1 || len(base)-dot-1 > maxExtLen {
		return false
	}
	for _, c := range []byte(base[dot+1:]) {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// sortedSet is set's members, sorted; nil when it has none.
func sortedSet(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// ScanProse scans a file read for its file node only (definition_exclude, #504): no
// definition, every line's path tokens attributed to the file itself.
func ScanProse(rel string, src []byte) FileResult {
	lines := splitLines(src)
	res := FileResult{Path: rel, Lines: len(lines), Calls: map[string][]string{}, Supers: map[string][]Super{}}
	set := map[string]bool{}
	for _, l := range lines {
		pathTokens(l, set)
	}
	if toks := sortedSet(set); toks != nil {
		res.Refs = map[string][]string{rel: toks}
	}
	return res
}

// References makes the references edges of #504: from each scanned node, or file (its
// lines in no definition), to the node of every other file one of its path tokens names.
// A token names the file at its path or at the path of a trailing run of its segments
// (`$ROOT/scripts/cfg.json` names scripts/cfg.json); failing that, every file whose base
// name, holding a dot, is its last segment (`"impl.md"`) — same-named files over-link, as
// same-named calls do. files are the paths that have file nodes.
func References(files []string, results []FileResult) []graph.Edge {
	isFile := make(map[string]bool, len(files))
	byBase := map[string][]string{}
	for _, f := range files {
		isFile[f] = true
		base := f[strings.LastIndexByte(f, '/')+1:]
		if strings.Contains(base, ".") {
			byBase[base] = append(byBase[base], f)
		}
	}
	memo := map[string][]string{}
	named := func(tok string) []string {
		if out, ok := memo[tok]; ok {
			return out
		}
		var out []string
		s := tok
		for {
			i := strings.IndexByte(s, '/')
			if i < 0 {
				out = byBase[s]
				break
			}
			if isFile[s] {
				out = []string{s}
				break
			}
			s = s[i+1:]
		}
		memo[tok] = out
		return out
	}
	var out []graph.Edge
	for _, r := range results {
		for from, toks := range r.Refs {
			for _, tok := range toks {
				for _, f := range named(tok) {
					if f != r.Path {
						out = append(out, graph.Edge{From: from, To: f, Relation: graph.RelReferences})
					}
				}
			}
		}
	}
	return out
}
