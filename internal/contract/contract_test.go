// Package contract holds repo-level guard tests for the M1a bootstrap: the module
// declaration, the dogfooding git attributes, the ignore rules, the interface
// contract amendment, and the CI pipeline. It contains no non-test code.
package contract

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// repoRoot walks up from the test's working directory to the directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found above %s", dir)
		}
		dir = parent
	}
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func TestGoModDeclaresModulePathAndGoVersion(t *testing.T) {
	src := readRepoFile(t, "go.mod")

	if !regexp.MustCompile(`(?m)^module\s+github\.com/VocanicZ/rtdd\s*$`).MatchString(src) {
		t.Errorf("go.mod must declare module github.com/VocanicZ/rtdd, got:\n%s", src)
	}
	// modernc.org/sqlite and its libc declare `go 1.24.0`, and the module graph's
	// minimum language version is the maximum of those — so the directive is the
	// patch-qualified `go 1.24.0`, not the bare `go 1.24`. Both are the 1.24
	// language version; anything outside the 1.24 line is a real change.
	if !regexp.MustCompile(`(?m)^go\s+1\.24(\.\d+)?\s*$`).MatchString(src) {
		t.Errorf("go.mod must declare the go 1.24 language version, got:\n%s", src)
	}
}

// TestGoModRequiresExactlyYAMLAndSQLite pins the engine's direct dependencies.
// Spec §4 permits exactly two: gopkg.in/yaml.v3 for adapter definitions, and
// modernc.org/sqlite to read .coverage directly. Adding a third needs a spec
// amendment (DEVELOPMENT.md, "Dependencies").
func TestGoModRequiresExactlyYAMLAndSQLite(t *testing.T) {
	got := directRequires(t, readRepoFile(t, "go.mod"))

	want := []string{"gopkg.in/yaml.v3", "modernc.org/sqlite"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("go.mod direct requirements = %v, want exactly %v", got, want)
	}
}

// directRequires returns the module paths go.mod requires DIRECTLY — sorted, and
// excluding every `// indirect` line, which is the module graph the two direct
// dependencies drag in rather than a choice this repo made.
func directRequires(t *testing.T, src string) []string {
	t.Helper()
	req := regexp.MustCompile(`(?m)^\s*(?:require\s+)?([\w.\-]+\.[\w.\-]+/[^\s]+)\s+v[^\s]+`)
	var got []string
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "require (":
			inBlock = true
			continue
		case inBlock && trimmed == ")":
			inBlock = false
			continue
		case strings.HasPrefix(trimmed, "module ") || strings.HasPrefix(trimmed, "go ") ||
			strings.HasPrefix(trimmed, "toolchain "):
			continue
		case strings.Contains(trimmed, "// indirect"):
			continue
		}
		if m := req.FindStringSubmatch(trimmed); m != nil {
			got = append(got, m[1])
		}
	}
	sort.Strings(got)
	return got
}

// sqliteClosure is modernc.org/sqlite's own dependency closure: the modules it
// pulls in that are actually compiled into the binary. They are pure Go — that is
// the whole reason sqlite is modernc's and not mattn's (DEVELOPMENT.md) — and they
// are listed here so that a new module appearing in the build is a test failure
// and not a silent widening of the "two dependencies" rule.
var sqliteClosure = []string{
	"github.com/dustin/go-humanize",
	"github.com/google/uuid",
	"github.com/mattn/go-isatty",
	"github.com/ncruces/go-strftime",
	"github.com/remyoudompheng/bigfft",
	"golang.org/x/exp",
	"golang.org/x/sys",
	"modernc.org/libc",
	"modernc.org/mathutil",
	"modernc.org/memory",
}

// TestCompiledModulesAreYAMLAndSQLiteOnly pins the "zero non-stdlib dependencies
// in the engine except yaml.v3 and sqlite" constraint against what the toolchain
// actually COMPILES, which is the claim that matters — `go list -m all` reports the
// whole module graph, including modules that contribute only a go.mod file.
//
// The permitted set is the main module, the two direct dependencies, and sqlite's
// own closure. Anything else means a third dependency crept into the engine.
func TestCompiledModulesAreYAMLAndSQLiteOnly(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}
	root := repoRoot(t)

	list := func(args ...string) []string {
		cmd := exec.Command("go", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		var vals []string
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				vals = append(vals, line)
			}
		}
		return vals
	}

	pkgs := list("list", "-deps", "./...")
	args := append([]string{"list", "-f", "{{if .Module}}{{.Module.Path}}{{end}}"}, pkgs...)
	seen := map[string]bool{}
	var mods []string
	for _, m := range list(args...) {
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		mods = append(mods, m)
	}
	sort.Strings(mods)

	permitted := map[string]bool{
		"github.com/VocanicZ/rtdd": true,
		"gopkg.in/yaml.v3":         true,
		"modernc.org/sqlite":       true,
	}
	for _, m := range sqliteClosure {
		permitted[m] = true
	}

	var extra []string
	for _, m := range mods {
		if !permitted[m] {
			extra = append(extra, m)
		}
	}
	if len(extra) > 0 {
		t.Errorf("modules compiled into the engine beyond yaml.v3, sqlite and sqlite's closure = %v (full set: %v)",
			extra, mods)
	}
	for _, m := range []string{"gopkg.in/yaml.v3", "modernc.org/sqlite"} {
		if !seen[m] {
			t.Errorf("%s is not compiled into the engine at all; compiled set = %v", m, mods)
		}
	}
}

// TestSQLiteDriverIsPureGo guards spec D4: the binary must stay static, so the
// SQLite driver must never become a cgo one. go.sum naming mattn/go-sqlite3 is the
// signal that someone swapped it.
func TestSQLiteDriverIsPureGo(t *testing.T) {
	sum := readRepoFile(t, "go.sum")
	if strings.Contains(sum, "github.com/mattn/go-sqlite3") {
		t.Error("go.sum names github.com/mattn/go-sqlite3, which needs cgo; the driver must be modernc.org/sqlite so CGO_ENABLED=0 still links (spec D4)")
	}
}

func TestGitAttributesDeclaresUnionMergeForMap(t *testing.T) {
	src := readRepoFile(t, ".gitattributes")
	if !regexp.MustCompile(`(?m)^\.rtdd/map\.jsonl\s+merge=union\s*$`).MatchString(src) {
		t.Errorf(".gitattributes must declare `.rtdd/map.jsonl merge=union`, got:\n%s", src)
	}
}

func TestGitIgnoreIgnoresBuildOutput(t *testing.T) {
	src := readRepoFile(t, ".gitignore")
	for _, want := range []string{"/rtdd", "/dist/bin/"} {
		found := false
		for _, line := range strings.Split(src, "\n") {
			if strings.TrimSpace(line) == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf(".gitignore must ignore %q, got:\n%s", want, src)
		}
	}
}

func TestInterfaceContractCarriesM1aAmendments(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	idx := strings.Index(src, "## M1a amendments")
	if idx < 0 {
		t.Fatal("docs/plans/00-interfaces.md must end with the `## M1a amendments` section")
	}
	if strings.Contains(src[idx+1:], "## M1a amendments") {
		t.Error("`## M1a amendments` must appear exactly once")
	}

	// The amendment must be appended at the end: nothing but the section follows it.
	if head := strings.TrimSpace(src[:idx]); !strings.HasSuffix(head, "---") {
		t.Errorf("`## M1a amendments` must be appended after a `---` rule, got tail: %q",
			head[max(0, len(head)-40):])
	}

	for _, want := range []string{
		"func MatchGlob(pattern, rel string) bool",
		"func LoadWith(path string, older func(a, b string) string) (*Map, error)",
		"type Meta struct {",
		"func LoadMeta(path string) (Meta, error)",
		"func SaveMeta(path string, m Meta) error",
		"func RepoRoot(start string) (string, error)",
		"func (s Status) String() string",
		"type Inputs struct {",
		"ImportOnly func(rel string) []string",
		"rtdd status [--adapter <path>]",
		"rtdd which  [--base <ref>] [--json] [--adapter <path>]",
	} {
		if !strings.Contains(src[idx:], want) {
			t.Errorf("M1a amendments must contain %q", want)
		}
	}
}

func TestCIWorkflowRunsTheGoPipeline(t *testing.T) {
	src := readRepoFile(t, ".github/workflows/ci.yml")

	if strings.Contains(src, "autodetect") {
		t.Error("ci.yml must no longer be the language-autodetect stub")
	}
	for _, want := range append([]string{"pull_request", "branches: [main]"}, ciCommands...) {
		if !strings.Contains(src, want) {
			t.Errorf("ci.yml must run/contain %q", want)
		}
	}
}

// ciCommands are the checks that define "green" for this repo. They must appear in both
// the hosted workflow and the local entrypoint, so the two cannot drift apart.
var ciCommands = []string{
	"go build ./...",
	"go vet ./...",
	"gofmt -l .",
	"go test ./... -count=1",
	"CGO_ENABLED=0 go build -o /tmp/rtdd ./cmd/rtdd",
	"statically linked",
}

func TestLocalCIEntrypointIsExecutableAndRunsTheSameChecks(t *testing.T) {
	rel := "scripts/ci-local.sh"
	info, err := os.Stat(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("%s must exist: %v", rel, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("%s must be executable, mode is %v", rel, info.Mode().Perm())
	}

	src := readRepoFile(t, rel)
	if !strings.Contains(src, "set -e") {
		t.Errorf("%s must fail fast (set -e)", rel)
	}
	for _, want := range ciCommands {
		if !strings.Contains(src, want) {
			t.Errorf("%s must run/contain %q", rel, want)
		}
	}
}

// M1b Task 1 adds two loaders to the contract. The doc is the interface of record, so a
// signature that exists only in code is a drift the next task would inherit.
func TestInterfaceContractRecordsTheAdapterLoaders(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	for _, want := range []string{
		"func LoadFS(fsys fs.FS, dir string) ([]*Adapter, error)",
		"func Builtin() ([]*Adapter, error)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md must record %q", want)
		}
	}
	// The superseded single-adapter forms must be gone, not merely outnumbered.
	for _, stale := range []string{
		"func LoadFS(fsys fs.FS, name string) (*Adapter, error)",
		"func Builtin(name string) (*Adapter, error)",
	} {
		if strings.Contains(src, stale) {
			t.Errorf("00-interfaces.md still carries the superseded signature %q", stale)
		}
	}
}

// M2 Task 13 adds internal/initrepo to the contract. The planning sketch in the
// Additions block named the enum `Action` and the record `Block`, with a `force` flag;
// the shipped package inverts the two names and has no force, because `rtdd init` never
// clobbers and there is therefore nothing to force. Both shapes in one document would
// say the shipped one is wrong.
func TestInterfaceContractRecordsInitrepo(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	if !strings.Contains(src, "internal/initrepo/") {
		t.Error("00-interfaces.md must list internal/initrepo/ in the package layout")
	}
	for _, want := range []string{
		"func MergeManagedBlock(existing, block string) string",
		"func EnsureGitAttributes(repoRoot string) (Action, error)",
		"func EnsureConfig(repoRoot string) (Action, error)",
		"func EnsureFrontEnd(repoRoot, rel, block string) (Action, error)",
		"func Block() string",
		"func Run(repoRoot string) ([]Action, error)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md must record %q", want)
		}
	}
	for _, stale := range []string{
		"func Install(repoRoot string, force bool) ([]Block, error)",
		"type Action int",
	} {
		if strings.Contains(src, stale) {
			t.Errorf("00-interfaces.md still carries the superseded initrepo signature %q", stale)
		}
	}
}

// M1b Task 4 adds ExpandTests to the contract, and narrows Expand to reject {tests}. The
// doc is the interface of record: a signature that exists only in code is a drift the next
// task would inherit, and here the drift is silent — a caller that reached Expand with a
// {tests} template would join every id into one argument.
func TestInterfaceContractRecordsExpandTests(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	for _, want := range []string{
		"func (a *Adapter) Expand(tmpl string, vars map[string]string) ([]string, error)",
		"func (a *Adapter) ExpandTests(tmpl string, vars map[string]string, tests []string) ([]string, error)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md must record %q", want)
		}
	}
	// The superseded form promised {tests} substitution inside Expand itself.
	stale := "// Expand substitutes {tests} {src} {out} {log} into a command template and returns argv."
	if strings.Contains(src, stale) {
		t.Errorf("00-interfaces.md still carries the superseded Expand doc line %q", stale)
	}
}

// signatureRe finds every declaration of name in a document, whether it stands as Go
// source or inside a `//` comment. Both forms are read as contract by a human.
func signatureRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^[ \t]*(?://[ \t]*)?func\b[^\n]*\b` + name + `\(`)
}

// implSignature returns the one-line declaration of name from a Go source file, with the
// trailing " {" removed, so it can be compared against the contract document verbatim.
func implSignature(t *testing.T, rel, name string) string {
	t.Helper()
	m := signatureRe(name).FindStringIndex(readRepoFile(t, rel))
	if m == nil {
		t.Fatalf("%s declares no func %s", rel, name)
	}
	src := readRepoFile(t, rel)
	line := src[m[0]:]
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSuffix(strings.TrimSpace(line), " {")
}

// docSignatures returns every declaration of name found in the contract document, each
// normalised to bare Go source: leading whitespace and any "// " comment marker removed.
func docSignatures(t *testing.T, doc, name string) []string {
	t.Helper()
	var out []string
	for _, m := range signatureRe(name).FindAllStringIndex(doc, -1) {
		line := doc[m[0]:]
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimPrefix(line, "//"))
		out = append(out, line)
	}
	return out
}

// The Amendments/Additions block declares itself as overriding everything above it, so a
// signature that appears twice in the document with two different shapes does not merely
// duplicate: it says the shipped implementation is wrong. Every declaration of a name must
// agree with every other, and all of them must agree with the code.
func TestInterfaceContractSignaturesDoNotContradictTheImplementation(t *testing.T) {
	doc := readRepoFile(t, "docs/plans/00-interfaces.md")

	for _, tc := range []struct{ rel, name string }{
		{"internal/adapter/expand.go", "ExpandTests"},
		{"internal/adapter/expand.go", "Expand"},
	} {
		want := implSignature(t, tc.rel, tc.name)
		got := docSignatures(t, doc, tc.name)
		if len(got) == 0 {
			t.Errorf("00-interfaces.md declares %s nowhere; %s has %q", tc.name, tc.rel, want)
			continue
		}
		for _, g := range got {
			if g != want {
				t.Errorf("00-interfaces.md declares %s as\n  %q\nbut %s implements\n  %q",
					tc.name, g, tc.rel, want)
			}
		}
	}
}

// The M1b amendment dropped {src} from Expand's variable set: a bare --cov honours the
// host's own [run] source for both seed and subset, and an RTDD-guessed {src} makes the
// two disagree on scope. The doc's own Expand entry must not still promise it.
func TestInterfaceContractDropsSrcFromExpand(t *testing.T) {
	doc := readRepoFile(t, "docs/plans/00-interfaces.md")

	for _, stale := range []string{
		"// Expand substitutes {src} {out} {log} into a command template and returns argv.",
		"// Expand substitutes {tests} {src} {out} {log} into a command template and returns argv.",
	} {
		if strings.Contains(doc, stale) {
			t.Errorf("00-interfaces.md still documents {src} for Expand: %q", stale)
		}
	}
	// Expand resolves whatever key the caller passes — there is no closed variable set in
	// internal/adapter to enforce the drop. The contract has to say where the enforcement
	// actually lives, or a host adapter reintroduces --cov={src} unopposed.
	if !strings.Contains(doc, "Expand has no closed variable set") {
		t.Error("00-interfaces.md must record that Expand has no closed variable set")
	}
	if !strings.Contains(doc, "the var map the caller supplies") {
		t.Error("00-interfaces.md must record that the caller's var map is what keeps {src} out of expanded argv")
	}
	// The contract document is not the only place the promise survives. Expand's own
	// godoc is the first thing a reader of the public API sees, and it outlived the
	// amendment that the document above was already policed for: {src} is unresolvable
	// at runtime, so a comment still naming it sends a host adapter author to a
	// placeholder no template can expand. expand.go must not name it at all — the
	// negative assertions live in expand_test.go, not in the shipped source.
	impl := readRepoFile(t, "internal/adapter/expand.go")
	for i, line := range strings.Split(impl, "\n") {
		if strings.Contains(line, "{src}") {
			t.Errorf("internal/adapter/expand.go:%d still names {src}: %q", i+1, strings.TrimSpace(line))
		}
	}
}

// structDeclRe finds every declaration of a struct type named name, whether it stands as
// Go source or inside a `//` comment, and whether its fields are braced on one line or
// spread over many. Both forms are read as contract by a human.
func structDeclRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^[ \t]*(?://[ \t]*)?type[ \t]+` + name + `[ \t]+struct[ \t]*\{`)
}

// structFieldSets returns the sorted field names of every declaration of struct name in
// src. Comment markers are stripped, so a declaration quoted inside a `//` block compares
// byte-for-byte against one in Go source.
func structFieldSets(t *testing.T, src, name string) [][]string {
	t.Helper()
	var out [][]string
	for _, m := range structDeclRe(name).FindAllStringIndex(src, -1) {
		body, ok := braceBody(src[m[1]-1:])
		if !ok {
			t.Fatalf("declaration of %s at offset %d has no closing brace", name, m[0])
		}
		out = append(out, fieldNames(body))
	}
	return out
}

// braceBody returns the contents between s's leading '{' and its matching '}'.
func braceBody(s string) (string, bool) {
	depth := 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[1:i], true
			}
		}
	}
	return "", false
}

// fieldNames extracts the declared names from a struct body. Fields separated by ';' on
// one line and fields on their own lines are the same declaration written two ways, so
// both split the same; a grouped `Covered, Uncovered int` contributes both names.
func fieldNames(body string) []string {
	var out []string
	for _, decl := range strings.FieldsFunc(body, func(r rune) bool { return r == '\n' || r == ';' }) {
		decl = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(decl), "//"))
		if i := strings.Index(decl, "//"); i >= 0 {
			decl = strings.TrimSpace(decl[:i])
		}
		toks := strings.Fields(decl)
		if len(toks) < 2 {
			continue
		}
		for _, name := range toks[:len(toks)-1] {
			out = append(out, strings.TrimSuffix(name, ","))
		}
	}
	sort.Strings(out)
	return out
}

// A struct in the contract binds exactly as hard as a func signature, and the
// Amendments/Additions block declares itself as overriding everything above it — so an
// amended field list that disagrees with the shipped struct does not merely duplicate:
// it says the implementation is wrong, and it is the half that formally wins. Comparing
// signatures alone let FatalExitError drift, because the divergence was in its fields.
func TestInterfaceContractStructFieldsDoNotContradictTheImplementation(t *testing.T) {
	doc := readRepoFile(t, "docs/plans/00-interfaces.md")

	for _, tc := range []struct{ rel, name string }{
		{"internal/runner/errors.go", "FatalExitError"},
	} {
		impl := structFieldSets(t, readRepoFile(t, tc.rel), tc.name)
		if len(impl) != 1 {
			t.Fatalf("%s declares struct %s %d times; want exactly one", tc.rel, tc.name, len(impl))
		}
		want := impl[0]
		got := structFieldSets(t, doc, tc.name)
		if len(got) == 0 {
			t.Errorf("00-interfaces.md declares struct %s nowhere; %s has fields %v", tc.name, tc.rel, want)
			continue
		}
		for _, g := range got {
			if !reflect.DeepEqual(g, want) {
				t.Errorf("00-interfaces.md declares %s with fields\n  %v\nbut %s implements\n  %v",
					tc.name, g, tc.rel, want)
			}
		}
	}
	// Spelled out separately from the field-set comparison: `Meaning` is the specific
	// field the superseded amendment invented, and it must be gone from the document
	// rather than merely outnumbered by correct declarations elsewhere in it.
	for _, m := range structDeclRe("FatalExitError").FindAllStringIndex(doc, -1) {
		body, ok := braceBody(doc[m[1]-1:])
		if ok && strings.Contains(body, "Meaning") {
			t.Errorf("00-interfaces.md still declares FatalExitError with a Meaning field: {%s}", body)
		}
	}
}

// internal/adapter/classify.go narrows IsTestFile with a FullEscalate exclusion that the
// plan never wrote down. The behaviour is right — naming conftest.py as a selector is a
// fatal exit 5 — but an undocumented narrowing of a contract predicate is drift, and this
// one has a side effect on IsInstrumentable.
func TestInterfaceContractDocumentsTheFullEscalateExclusion(t *testing.T) {
	doc := readRepoFile(t, "docs/plans/00-interfaces.md")

	entry := precedingComment(t, doc, "func (a *Adapter) IsTestFile(rel string) bool")
	for _, want := range []string{"FullEscalate", "IsInstrumentable"} {
		if !strings.Contains(entry, want) {
			t.Errorf("IsTestFile's contract entry must mention %q; its doc comment is:\n%s", want, entry)
		}
	}
}

// precedingComment returns the contiguous block of `//` lines immediately above decl in
// doc. A qualification three paragraphs away is not a qualification of the entry.
func precedingComment(t *testing.T, doc, decl string) string {
	t.Helper()
	i := strings.Index(doc, decl)
	if i < 0 {
		t.Fatalf("00-interfaces.md contains no %q", decl)
	}
	lines := strings.Split(doc[:i], "\n")
	var block []string
	for j := len(lines) - 2; j >= 0; j-- {
		if !strings.HasPrefix(strings.TrimSpace(lines[j]), "//") {
			break
		}
		block = append([]string{lines[j]}, block...)
	}
	return strings.Join(block, "\n")
}
