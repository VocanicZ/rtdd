package scan

import (
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// maxCandidates is how many same-named definitions a call may still match once every
// narrowing rule has run: past it the call links to none and is counted as ambiguous
// (spec §4.3, issue #508). A name defined that widely carries no signal about which.
const maxCandidates = 5

// Super is one supertype a class definition line names.
type Super struct {
	Name     string
	Relation graph.Relation // RelInherits or RelImplements
}

// Refs is what Link resolves into edges: every scanned file's calls, imports and
// supertypes. A file or node that no scan produced (a graphify node) has none.
type Refs struct {
	Calls   map[string][]string // node ID -> call tokens (see ScanFile)
	Imports map[string][]string // file -> import specs, relative ones rooted at "/"
	Supers  map[string][]Super  // class node ID -> what it extends or implements
}

// RefsOf gathers the refs of every result.
func RefsOf(results []FileResult) Refs {
	refs := Refs{Calls: map[string][]string{}, Imports: map[string][]string{}, Supers: map[string][]Super{}}
	for _, r := range results {
		for id, toks := range r.Calls {
			refs.Calls[id] = toks
		}
		if len(r.Imports) > 0 {
			refs.Imports[r.Path] = r.Imports
		}
		for id, s := range r.Supers {
			refs.Supers[id] = s
		}
	}
	return refs
}

// Stats is what Link resolved besides its edges, for `rtdd graph` (spec §8).
type Stats struct {
	Imports        int // file -> file imports, each importing file and imported file once
	AmbiguousCalls int // calls left matching more than maxCandidates definitions, linked to none
}

// Link makes the calls, inherits and implements edges (spec §4.3). A call token is `X`
// (a bare or self/this call), `T#X` (a call on a receiver declared of type T), `R.X` (on
// a receiver R with no declared type — a module, a class, a field), `super.X`, or `.X`
// (on a receiver nothing names). It resolves, first rule that finds a definition wins:
//
//  1. receiver type — `T#X` links to class T's own X, else its supertypes', else the X of
//     every class below T (an interface's implementations), and goes on only when none
//     has one; when the caller's file imports T from outside the repository (an import
//     no file resolves), T is a library's type and the call links to nothing; when it
//     imports T from a file that holds no class T, only that file's X. `R.X` links to class R's X the same way when R is a class, and
//     otherwise goes on;
//  2. scope — a bare `X` links to the X defined in the innermost enclosing node that has
//     one (its own class first), or in that class's supertypes; `super.X` only to the
//     supertypes', and to nothing when none is in the graph;
//  3. same file, then imported files and the caller's directory, then the same package (a
//     directory with the same last two path segments) — narrowing the same-named
//     definitions of the caller's own file type, or, when it has none, of the extensions
//     that share a directory with it (`.ts` -> `.js`, `.c` -> `.h`; a builtin such as
//     `len(` is not captured elsewhere). A call on a receiver (`R.X`, `.X`) never links to
//     a member of the caller's own class this way;
//  4. whatever is left. Past the caller's own file, a set of more than maxCandidates
//     definitions links to none, counted in Stats.AmbiguousCalls.
//
// A supertype name resolves to class nodes by rules 3-4. nodes is the whole graph's node
// set, whichever source produced each node. The caller itself is never a target.
func Link(nodes []graph.Node, refs Refs) ([]graph.Edge, Stats) {
	edges, st := link(nodes, refs)
	g := graph.Graph{Edges: edges}
	graph.Sort(&g)
	return g.Edges, st
}

type extName struct{ ext, name string }

type linker struct {
	byID      map[string]graph.Node
	byName    map[string][]string
	byExtName map[extName][]string
	dirs      map[string]map[string]bool // extension -> directories holding a node of it
	together  map[[2]string]bool
	children  map[string]map[string][]string // enclosing node ID -> name -> nested node IDs
	supers    map[string][]string            // class ID -> resolved supertype class IDs
	subs      map[string][]string            // class ID -> the class IDs naming it a supertype
	imported  map[string]map[string]bool     // file -> files it imports
	external  map[string]map[string]bool     // file -> last segments of its imports no file resolves
	typeFile  map[string]map[string][]string // file -> last segment of an import -> the files it resolves to
	pkg       map[string]string              // file -> package key
}

// link is Link unsorted, for a caller that sorts the whole graph once afterwards.
func link(nodes []graph.Node, refs Refs) ([]graph.Edge, Stats) {
	l := &linker{byID: map[string]graph.Node{}, byName: map[string][]string{}, byExtName: map[extName][]string{},
		dirs: map[string]map[string]bool{}, together: map[[2]string]bool{}, children: map[string]map[string][]string{},
		supers: map[string][]string{}, subs: map[string][]string{}, imported: map[string]map[string]bool{}, external: map[string]map[string]bool{}, typeFile: map[string]map[string][]string{}, pkg: map[string]string{}}
	for _, n := range nodes {
		ext := filepath.Ext(n.File)
		l.byID[n.ID] = n
		l.byName[n.Name] = append(l.byName[n.Name], n.ID)
		k := extName{ext, n.Name}
		l.byExtName[k] = append(l.byExtName[k], n.ID)
		if l.dirs[ext] == nil {
			l.dirs[ext] = map[string]bool{}
		}
		l.dirs[ext][pathpkg.Dir(n.File)] = true
		if parent := enclosing(n.ID, n.File); parent != "" {
			if l.children[parent] == nil {
				l.children[parent] = map[string][]string{}
			}
			l.children[parent][n.Name] = append(l.children[parent][n.Name], n.ID)
		}
		if _, ok := l.pkg[n.File]; !ok {
			l.pkg[n.File] = packageKey(pathpkg.Dir(n.File))
		}
	}

	var st Stats
	files := newFileIndex(l.pkg)
	for from, specs := range refs.Imports {
		for _, spec := range specs {
			resolved := files.resolve(spec)
			if len(resolved) == 0 {
				if l.external[from] == nil {
					l.external[from] = map[string]bool{}
				}
				l.external[from][pathpkg.Base(spec)] = true
			} else {
				if l.typeFile[from] == nil {
					l.typeFile[from] = map[string][]string{}
				}
				l.typeFile[from][pathpkg.Base(spec)] = resolved
			}
			for _, f := range resolved {
				if f == from || l.imported[from][f] {
					continue
				}
				if l.imported[from] == nil {
					l.imported[from] = map[string]bool{}
				}
				l.imported[from][f] = true
				st.Imports++
			}
		}
	}

	var out []graph.Edge
	for id, sups := range refs.Supers {
		n, ok := l.byID[id]
		if !ok {
			continue
		}
		for _, s := range sups {
			for _, c := range l.classesNamed(n, s.Name) {
				if c != id {
					out = append(out, graph.Edge{From: id, To: c, Relation: s.Relation})
					l.supers[id] = append(l.supers[id], c)
					l.subs[c] = append(l.subs[c], id)
				}
			}
		}
	}

	for from, toks := range refs.Calls {
		n, ok := l.byID[from]
		if !ok {
			continue
		}
		for _, tok := range toks {
			targets, ambiguous := l.resolve(n, tok)
			if ambiguous {
				st.AmbiguousCalls++
			}
			for _, to := range targets {
				if to != from {
					out = append(out, graph.Edge{From: from, To: to, Relation: graph.RelCalls})
				}
			}
		}
	}
	return out, st
}

// resolve is the rule cascade of Link's doc for one call token of node n. n itself is
// never a target: a scope rule that finds only n (a delegating `load()` inside a method
// `load`) falls through to the next rule.
func (l *linker) resolve(n graph.Node, tok string) ([]string, bool) {
	recv, name := "", tok
	if i := strings.LastIndexAny(tok, ".#"); i >= 0 {
		recv, name = tok[:i], tok[i+1:]
	}
	bare := recv == "" && !strings.Contains(tok, ".")
	switch {
	case strings.Contains(tok, "#"):
		classes := l.classesNamed(n, recv)
		if len(classes) == 0 && l.external[n.File][recv] {
			return nil, false // a type the file imports from outside the repository
		}
		var out []string
		for _, c := range classes {
			ids := l.member(c, name)
			if len(ids) == 0 {
				ids = l.below(c, name)
			}
			out = append(out, without(ids, n.ID)...)
		}
		if len(out) > 0 {
			return out, false
		}
		if fs := l.typeFile[n.File][recv]; len(classes) == 0 && len(fs) > 0 {
			// T is imported from a file the graph holds no class T in (a Java record):
			// X is defined there or nowhere this graph sees.
			in := map[string]bool{}
			for _, f := range fs {
				in[f] = true
			}
			for _, id := range l.candidates(n, name) {
				if in[l.byID[id].File] && id != n.ID {
					out = append(out, id)
				}
			}
			return out, false
		}
		// T holds no body for X (a Go struct, whose methods are not nested in it): go on.
	case recv == superReceiver:
		for _, scope := range scopes(n) {
			for _, sup := range l.supers[scope] {
				if ids := without(l.member(sup, name), n.ID); len(ids) > 0 {
					return ids, false
				}
			}
		}
		return nil, false // a supertype outside the graph: nothing here is the target
	case recv != "":
		for _, c := range l.classesNamed(n, recv) {
			if ids := without(l.member(c, name), n.ID); len(ids) > 0 {
				return ids, false
			}
		}
	case bare:
		for _, scope := range scopes(n) {
			if ids := without(l.member(scope, name), n.ID); len(ids) > 0 {
				return ids, false
			}
		}
	}
	cands := without(l.candidates(n, name), n.ID)
	if !bare {
		// A call on another object is not to the caller's own class: `this.sock.close()`
		// inside Conn.close is the socket's close, not Conn's.
		own := map[string]bool{}
		for _, scope := range scopes(n) {
			own[scope] = true
		}
		var keep []string
		for _, id := range cands {
			if !own[enclosing(id, l.byID[id].File)] {
				keep = append(keep, id)
			}
		}
		cands = keep
	}
	return l.narrow(n.File, cands)
}

// without is ids less id.
func without(ids []string, id string) []string {
	for i, x := range ids {
		if x == id {
			return append(append([]string{}, ids[:i]...), ids[i+1:]...)
		}
	}
	return ids
}

// member is the nodes named name nested directly in scope, or else in scope's
// supertypes, nearest first.
func (l *linker) member(scope, name string) []string {
	seen := map[string]bool{}
	queue := []string{scope}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if seen[s] {
			continue
		}
		seen[s] = true
		if ids := l.children[s][name]; len(ids) > 0 {
			return ids
		}
		queue = append(queue, l.supers[s]...)
	}
	return nil
}

// below is the nodes named name nested directly in any class beneath scope — every
// implementation of an interface method, every override of an abstract one.
func (l *linker) below(scope, name string) []string {
	var out []string
	seen := map[string]bool{scope: true}
	queue := append([]string{}, l.subs[scope]...)
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, l.children[s][name]...)
		queue = append(queue, l.subs[s]...)
	}
	return out
}

// classesNamed is the class nodes named name that n's file can see, narrowed as a call is
// (rule 3); none when more than maxCandidates remain.
func (l *linker) classesNamed(n graph.Node, name string) []string {
	var classes []string
	for _, id := range l.candidates(n, name) {
		if l.byID[id].Kind == graph.KindClass {
			classes = append(classes, id)
		}
	}
	out, _ := l.narrow(n.File, classes)
	return out
}

// candidates is today's name match: the nodes named name in files of n's extension, or,
// when there are none, in the extensions that share a directory with it.
func (l *linker) candidates(n graph.Node, name string) []string {
	ext := filepath.Ext(n.File)
	if ids := l.byExtName[extName{ext, name}]; len(ids) > 0 {
		return ids
	}
	var out []string
	for _, id := range l.byName[name] {
		if l.sharesDir(ext, filepath.Ext(l.byID[id].File)) {
			out = append(out, id)
		}
	}
	return out
}

// narrow keeps, of cands, those in file, else those in the files file imports or in its
// directory, else those in its package; then, unless what is kept is in file itself, none
// when that is more than maxCandidates (ambiguous).
func (l *linker) narrow(file string, cands []string) ([]string, bool) {
	if len(cands) <= 1 {
		return cands, false
	}
	dir := pathpkg.Dir(file)
	for i, keep := range []func(string) bool{
		func(f string) bool { return f == file },
		func(f string) bool { return l.imported[file][f] || pathpkg.Dir(f) == dir },
		func(f string) bool { return l.pkg[f] == l.pkg[file] },
		func(string) bool { return true },
	} {
		var sub []string
		for _, id := range cands {
			if keep(l.byID[id].File) {
				sub = append(sub, id)
			}
		}
		switch {
		case len(sub) == 0:
			continue
		case i > 0 && len(sub) > maxCandidates:
			return nil, true
		}
		return sub, false
	}
	return nil, false
}

func (l *linker) sharesDir(a, b string) bool {
	k := [2]string{a, b}
	if v, ok := l.together[k]; ok {
		return v
	}
	v := false
	for d := range l.dirs[a] {
		if l.dirs[b][d] {
			v = true
			break
		}
	}
	l.together[k] = v
	return v
}

// enclosing is the ID of the node a scanner node is nested in — its ID less the last
// `::` segment — or "" for a top-level node, or a node whose ID the scanner did not make.
func enclosing(id, file string) string {
	if !strings.HasPrefix(id, file+"::") {
		return ""
	}
	if i := strings.LastIndex(id, "::"); i > len(file) {
		return id[:i]
	}
	return ""
}

// scopes is n's enclosing nodes, innermost first.
func scopes(n graph.Node) []string {
	var out []string
	for id := enclosing(n.ID, n.File); id != ""; id = enclosing(id, n.File) {
		out = append(out, id)
	}
	return out
}

// packageKey is a directory's last two segments: `src/main/java/a/b` and
// `src/test/java/a/b` are one package, as a same-package test and its subject are.
func packageKey(dir string) string {
	parts := strings.Split(dir, "/")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return strings.Join(parts, "/")
}

// fileIndex resolves an import spec to the node-bearing files it names.
type fileIndex struct {
	bySuffix map[string][]string // path or extension-less path, and each of its trailing segment runs -> files
	dirs     map[string][]string // directory, and each of its trailing segment runs -> directories
	inDir    map[string][]string // directory -> its files
	exact    map[string][]string // path or extension-less path -> files
}

func newFileIndex(files map[string]string) *fileIndex {
	ix := &fileIndex{bySuffix: map[string][]string{}, dirs: map[string][]string{}, inDir: map[string][]string{}, exact: map[string][]string{}}
	for f := range files {
		stem := strings.TrimSuffix(f, pathpkg.Ext(f))
		ix.exact[f] = append(ix.exact[f], f)
		if stem != f {
			ix.exact[stem] = append(ix.exact[stem], f)
		}
		for _, p := range []string{f, stem} {
			for _, s := range suffixes(p) {
				ix.bySuffix[s] = append(ix.bySuffix[s], f)
			}
		}
		dir := pathpkg.Dir(f)
		if len(ix.inDir[dir]) == 0 {
			for _, s := range suffixes(dir) {
				ix.dirs[s] = append(ix.dirs[s], dir)
			}
		}
		ix.inDir[dir] = append(ix.inDir[dir], f)
	}
	return ix
}

// suffixes is p and each run of its trailing segments: a/b/c -> c, b/c, a/b/c.
func suffixes(p string) []string {
	out := []string{p}
	for i := strings.IndexByte(p, '/'); i >= 0; i = strings.IndexByte(p, '/') {
		p = p[i+1:]
		out = append(out, p)
	}
	return out
}

// resolve: a rooted spec (`/a/b`, a relative import already joined to its file's
// directory) names the file a/b, a/b.<ext> or the directory a/b. Any other spec names
// the files whose path ends with it, else the directories that do, else the directory a
// trailing run of its segments is (Go's `module/path/pkg`), else — once — its parent
// (`import a.b.C.member`).
func (ix *fileIndex) resolve(spec string) []string {
	if strings.HasPrefix(spec, "/") {
		p := spec[1:]
		for _, q := range []string{p, strings.TrimSuffix(p, pathpkg.Ext(p))} {
			if fs := ix.exact[q]; len(fs) > 0 {
				return fs
			}
		}
		return ix.inDir[p]
	}
	if !strings.Contains(spec, "/") {
		// One segment (`import utils`, Go's `import "errors"`) names a top-level module —
		// a file or directory at the root or one directory down — not any file so named.
		var out []string
		for _, f := range ix.bySuffix[spec] {
			if strings.Count(f, "/") <= 1 {
				out = append(out, f)
			}
		}
		for _, d := range ix.dirs[spec] {
			if strings.Count(d, "/") <= 1 {
				out = append(out, ix.inDir[d]...)
			}
		}
		return out
	}
	for try := 0; try < 2 && spec != "" && spec != "."; try++ {
		if fs := ix.bySuffix[spec]; len(fs) > 0 {
			return fs
		}
		if ds := ix.dirs[spec]; len(ds) > 0 {
			var out []string
			for _, d := range ds {
				out = append(out, ix.inDir[d]...)
			}
			return out
		}
		if try == 0 {
			for _, s := range suffixes(spec)[1:] {
				if fs := ix.inDir[s]; len(fs) > 0 {
					return fs
				}
			}
		}
		spec = pathpkg.Dir(spec)
	}
	return nil
}
