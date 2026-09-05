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
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/doctor"
	"github.com/VocanicZ/rtdd/internal/selector"
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
	// The generated front-ends: check catches drift from PROTOCOL.md, verify
	// catches a dist/ file that is wrong for its target, and the diff catches
	// an embedded protocol copy that `rtdd init` would ship stale. ci.yml ran
	// all three while ci-local.sh ran none, so a green local gate did not imply
	// a green CI — the parity this list exists to enforce.
	"go run ./cmd/rtdd-gen check",
	"go run ./cmd/rtdd-gen verify",
	"diff -u protocol/PROTOCOL.md internal/install/protocol.md",
	"CGO_ENABLED=0 go build -o /tmp/rtdd ./cmd/rtdd",
	"statically linked",
	// The host build above proves one of the four released artifacts is static. Issue
	// #212: the other three (linux/arm64, darwin/amd64, darwin/arm64) are only ever
	// inspected by this test, which cross-builds the whole .goreleaser.yaml matrix. It
	// runs inside `go test ./...` too; naming it as its own step means a red build points
	// straight at the release artifacts instead of at the suite in general.
	"go test -count=1 -run '^TestReleaseArtifactsAreStaticallyLinked$' .",
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

// M6a Task 4 adds the host-adapter loaders. §4.5 calls user-authorable adapters the
// load-bearing part of the multi-language design, so the precedence rule — the host wins —
// belongs in the interface of record, not only in the code that implements it.
func TestInterfaceContractRecordsTheHostAdapterLoaders(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	for _, want := range []string{
		`const HostAdapterDir = ".rtdd/adapters"`,
		"func LoadHost(repoRoot string) ([]*Adapter, error)",
		"func LoadHostReport(repoRoot string) ([]*Adapter, []Invalid, error)",
		"func Available(repoRoot string) ([]*Adapter, error)",
		"func AvailableReport(repoRoot string) ([]*Adapter, []Invalid, error)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md must record %q", want)
		}
	}
	if !strings.Contains(src, "REPLACES it") {
		t.Error("00-interfaces.md must state the precedence rule: a host adapter replaces the built-in of the same name")
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

// M2 Task 4 adds internal/importscan to the contract. The M1a planning sketch declared a
// `Scanner` struct with exported RepoRoot/Python fields and a per-target `Scan` method; the
// shipped package is a single package-level Scan over all targets at once, because one
// Python subprocess walking the tree once is the whole point of shelling out. Both shapes in
// one document would say the shipped one is wrong.
func TestInterfaceContractRecordsImportscan(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	if !strings.Contains(src, "internal/importscan/") {
		t.Error("00-interfaces.md must list internal/importscan/ in the package layout")
	}
	for _, want := range []string{
		"func Scan(repoRoot string, targets, tests []string) (map[string][]string, error)",
		// M2 Task 5: the memoising Scanner that satisfies selector.Inputs.ImportOnly.
		"func NewScanner(repoRoot string, tests []string) *Scanner",
		"func (s *Scanner) TestsImporting(rel string) []string",
		"func (s *Scanner) Err() error",
		"ImportOnly func(rel string) []string // importscan.Scanner.TestsImporting",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md must record %q", want)
		}
	}
	for _, stale := range []string{
		"type Scanner struct{ RepoRoot string; Python string }",
		"func (s *Scanner) Scan(target string, testFiles []string) ([]string, error)",
	} {
		if strings.Contains(src, stale) {
			t.Errorf("00-interfaces.md still carries the superseded importscan signature %q", stale)
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
		{"internal/uncovered/hunk.go", "WithLines"},
		{"internal/uncovered/hunk.go", "ParseHunks"},
		{"internal/uncovered/classify.go", "Summarize"},
		{"internal/gitctx/rawdiff.go", "RawDiff"},
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
		{"internal/uncovered/classify.go", "Summary"},
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

// M2 Task 6 adds internal/doctor to the contract. Spec §9 requires the fan-out caveat to
// appear in the tool's OWN output, so it ships as an exported constant, not as prose in a
// plan. The Additions block sketched it as `func Caveat() string`; the shipped shape is a
// const, and both shapes in one document would say the shipped one is wrong.
func TestInterfaceContractRecordsDoctorCaveat(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	if !strings.Contains(src, "internal/doctor/") {
		t.Error("00-interfaces.md must list internal/doctor/ in the package layout")
	}
	for _, want := range []string{
		"const Caveat = ",
		"func Hubs(m *mapstore.Map) []Hub",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md must record %q", want)
		}
	}
	if strings.Contains(src, "func Caveat() string") {
		t.Error("00-interfaces.md still carries the superseded signature \"func Caveat() string\"")
	}
}

// The caveat is worthless if it is a vague warning: it has to name the mechanisms that
// produce a fan-out of 1 so a reader can recognise one in their own repo.
func TestDoctorCaveatNamesTheOncePerProcessMechanisms(t *testing.T) {
	for _, want := range []string{
		"lru_cache",
		"module singleton",
		"DI container",
		"session-scoped fixture",
		"fan-out of 1",
		"most coupled",
		"cleanest",
	} {
		if !strings.Contains(doctor.Caveat, want) {
			t.Errorf("doctor.Caveat must name %q (spec §9)", want)
		}
	}
}

// M2's Definition of Done names every contract addition the milestone makes. This is the
// roll-up guard for it: one test that fails if any of them stopped being recorded, so a
// later edit to 00-interfaces.md cannot quietly drop half the milestone's interface.
// Per-symbol shape guards live in the tests above; this one guards presence.
func TestInterfaceContractCarriesEveryM2Addition(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	for _, want := range []string{
		// internal/uncovered — hunk parsing, the authoritative line source, the classes.
		"func ParseHunks(diff string) map[string][]gitctx.LineRange",
		"func WithLines(repoRoot string, changes []gitctx.Change, rawDiff string) ([]gitctx.Change, error)",
		`func (c Class) String() string // "covered" | "uncovered" | "import-time"`,
		"func Summarize(reports []FileReport) Summary",
		"func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport",
		"func (r FileReport) UncoveredLines() int",
		// internal/gitctx — the one raw diff every changed line derives from.
		"func RawDiff(repoRoot, base string) (string, error)",
		// internal/importscan — the static fallback for import-time-only files.
		"func Scan(repoRoot string, targets, tests []string) (map[string][]string, error)",
		"func NewScanner(repoRoot string, tests []string) *Scanner",
		"func (s *Scanner) TestsImporting(rel string) []string",
		"func (s *Scanner) Err() error",
		// internal/doctor — the mandatory spec §9 caveat, an immutable const.
		"const Caveat = ",
		// internal/initrepo — merge into existing front-ends, never clobber.
		"func Run(repoRoot string) ([]Action, error)",
		// The --json schema v1: the contract the agent front-ends bind to.
		"### The schema, version 1",
		`"schema": 1,`,
		"| `schema` | int | Always `1` for this version.",
		// The never-narrow-silently guarantee, carried in the document rather than on
		// stderr a --json consumer discards.
		"| `complete` | bool |",
		"| `warnings` | array of string |",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md must record the M2 addition %q", want)
		}
	}
	for _, pkg := range []string{"internal/uncovered/", "internal/importscan/", "internal/doctor/", "internal/initrepo/"} {
		if !strings.Contains(src, pkg) {
			t.Errorf("00-interfaces.md must list %s in the package layout", pkg)
		}
	}
}

// TestJSONSchemaV1KeySetMatchesTheInterfaceContract is the drift guard between the frozen
// --json schema and the document that specifies it.
//
// The two have already disagreed once: an amendment in 00-interfaces.md mandated
// `complete` and `warnings`, the `Output` struct never grew them, and nothing failed
// (issue #112). Prose and struct are checked against each other in BOTH directions here,
// so a key added to one and not the other is a red test rather than a silent divergence.
func TestJSONSchemaV1KeySetMatchesTheInterfaceContract(t *testing.T) {
	doc := readRepoFile(t, "docs/plans/00-interfaces.md")
	src := readRepoFile(t, "cmd/rtdd/jsonout.go")

	inStruct := topLevelOutputKeys(t, src)
	if len(inStruct) < 5 {
		t.Fatalf("could not read the Output struct's json tags, got %v", inStruct)
	}
	inDoc := fieldContractTopLevelKeys(t, doc)
	if len(inDoc) < 5 {
		t.Fatalf("could not read the field contract table, got %v", inDoc)
	}

	sample := jsonSchemaSample(t, doc)
	for _, k := range inStruct {
		if !inDoc[k] {
			t.Errorf("Output emits top-level key %q, but 00-interfaces.md's field contract "+
				"does not document it — the schema and the contract have drifted apart", k)
		}
		if !strings.Contains(sample, `"`+k+`":`) {
			t.Errorf("the schema v1 sample document must show top-level key %q", k)
		}
	}
	for k := range inDoc {
		if !slices.Contains(inStruct, k) {
			t.Errorf("00-interfaces.md documents top-level key %q, but cmd/rtdd/jsonout.go's "+
				"Output struct never emits it — the contract promises a field the code omits", k)
		}
	}
}

// topLevelOutputKeys reads the json tags of `type Output struct` in source order, which is
// also the emitted key order.
func topLevelOutputKeys(t *testing.T, src string) []string {
	t.Helper()
	body, ok := blockAfter(src, "type Output struct {")
	if !ok {
		t.Fatal("cmd/rtdd/jsonout.go no longer declares `type Output struct {`")
	}
	var out []string
	for _, m := range regexp.MustCompile("`json:\"([a-z_]+)\"`").FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	return out
}

// fieldContractRow matches one markdown table row whose first cell is a backticked field name.
var fieldContractRow = regexp.MustCompile("(?m)^\\| `([^`]+)` \\|")

// fieldContractTopLevelKeys reads the "Field contract." table and returns the TOP-LEVEL key
// each row belongs to. A composite value is documented through its members rather than a row
// of its own — `changed[].path`, `selection.count`, `run.passed`/`failed`/... — so the root
// before the first `.` or `[` is what names the top-level key. A scalar row is its own root.
func fieldContractTopLevelKeys(t *testing.T, doc string) map[string]bool {
	t.Helper()
	table, ok := between(doc, "**Field contract.**", "**Invariant, and it is tested:**")
	if !ok {
		t.Fatal("00-interfaces.md no longer contains the field contract table")
	}
	out := map[string]bool{}
	for _, m := range fieldContractRow.FindAllStringSubmatch(table, -1) {
		root := m[1]
		if i := strings.IndexAny(root, ".["); i >= 0 {
			root = root[:i]
		}
		out[root] = true
	}
	return out
}

// jsonSchemaSample is the fenced example document under "### The schema, version 1".
func jsonSchemaSample(t *testing.T, doc string) string {
	t.Helper()
	sample, ok := between(doc, "### The schema, version 1", "**Field contract.**")
	if !ok {
		t.Fatal("00-interfaces.md no longer contains the schema v1 sample document")
	}
	return sample
}

// blockAfter returns the text from marker up to the first line that is exactly "}".
func blockAfter(src, marker string) (string, bool) {
	i := strings.Index(src, marker)
	if i < 0 {
		return "", false
	}
	rest := src[i+len(marker):]
	end := strings.Index(rest, "\n}")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func between(src, start, end string) (string, bool) {
	i := strings.Index(src, start)
	if i < 0 {
		return "", false
	}
	rest := src[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return "", false
	}
	return rest[:j], true
}

// Issue #222: scripts/release-preflight.sh was committed 100644 while its two siblings were
// 100755, so the invocation DEVELOPMENT.md documents (`scripts/release-preflight.sh`, no
// interpreter prefix) died with "Permission denied". Nothing caught it, because the one test
// that execs the script chmods its own temp copy first.
//
// This guard therefore reads the mode of the files that are actually in the repository, and
// globs scripts/*.sh so a script added tomorrow is covered without editing this list. Both
// modes are asserted: the working-tree bit is what an operator's shell honours, and the git
// index mode is what a fresh clone gets — a `chmod +x` that was never staged fixes only the
// first.
func TestEveryRepoScriptIsExecutable(t *testing.T) {
	root := repoRoot(t)

	scripts, err := filepath.Glob(filepath.Join(root, "scripts", "*.sh"))
	if err != nil {
		t.Fatalf("glob scripts/*.sh: %v", err)
	}
	if len(scripts) == 0 {
		t.Fatalf("no scripts/*.sh found under %s", root)
	}

	for _, abs := range scripts {
		rel := filepath.ToSlash(mustRel(t, root, abs))
		info, err := os.Stat(abs)
		if err != nil {
			t.Errorf("stat %s: %v", rel, err)
			continue
		}
		if info.Mode().Perm()&0o100 == 0 {
			t.Errorf("%s must be executable, working-tree mode is %v; run `chmod +x %s`",
				rel, info.Mode().Perm(), rel)
		}
	}

	// internal/contract is exempt from TestOnlyGitctxShellsOutToGit, so the index mode -
	// the mode a fresh clone materialises - can be read directly from git here.
	out, err := exec.Command("git", "-C", root, "ls-files", "-s", "--", "scripts/*.sh").Output()
	if err != nil {
		t.Fatalf("git ls-files -s scripts/*.sh: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			t.Errorf("unparseable `git ls-files -s` line %q", line)
			continue
		}
		mode, path := fields[0], fields[len(fields)-1]
		if mode != "100755" {
			t.Errorf("%s is committed with mode %s, want 100755; run `git update-index --chmod=+x %s`",
				path, mode, path)
		}
	}
}

// mustRel is filepath.Rel with the test-fatal error handling every caller here wants.
func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatalf("rel %s %s: %v", base, target, err)
	}
	return rel
}

// --- M6b: the TS static selection tier -------------------------------------------------

// What this protects: the SPELLING of the tier, not its existence. `Tier.String()` is a
// wire value — `rtdd which --json` reports it verbatim and the sibling milestones bind
// their front-end text to it — while internal/selector's own tests compare tiers as
// constants and would keep passing through a rename to "static", "ts" or "T-S".
// PRD #230 AC2 fixes it as exactly "TS".
func TestTierTSStringIsExactlyTS(t *testing.T) {
	if got := selector.TierTS.String(); got != "TS" {
		t.Errorf(`selector.TierTS.String() = %q, want exactly "TS"`, got)
	}
}

// What this protects: the ORDER of the tier constants. They are ranked by confidence and
// compared with `<` in the escalation logic, so TierTS's position is behaviour, not
// cosmetics: moved above T1 it would override a usable map, moved below T2 it would
// outrank the full suite. TierEmpty stays the zero value so a zero Selection is an
// explicit empty rather than whichever tier happened to land on 0.
func TestTierTSIsOrderedBetweenT1AndT2(t *testing.T) {
	if selector.TierEmpty != 0 {
		t.Errorf("selector.TierEmpty = %d, want 0: the zero Selection must be an explicit empty", selector.TierEmpty)
	}
	if !(selector.TierT1 < selector.TierTS) {
		t.Errorf("selector.TierT1 (%d) must sort below TierTS (%d): TS never overrides a usable map (spec §4.1)",
			selector.TierT1, selector.TierTS)
	}
	if !(selector.TierTS < selector.TierT2) {
		t.Errorf("selector.TierTS (%d) must sort below TierT2 (%d): TS is narrower than the full suite",
			selector.TierTS, selector.TierT2)
	}
}

// commentMarker matches the `//` opening a Go doc-comment line inside a fenced block.
var commentMarker = regexp.MustCompile(`(?m)^[ \t]*//[ \t]?`)

// What this protects: that the tier is WRITTEN DOWN where the repo records its contracts.
// M6c and M6d bind to these names, and a name that lives only in the implementation is a
// name the next milestone has to reverse-engineer — which is what 00-interfaces.md's own
// "Rule for future additions" exists to prevent.
func TestInterfaceContractRecordsTheStaticTier(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	const heading = "## M6b amendments"
	idx := strings.Index(src, heading)
	if idx < 0 {
		t.Fatalf("docs/plans/00-interfaces.md has no %q section", heading)
	}
	// The section is wrapped markdown carrying wrapped Go doc comments, so a sentence the
	// contract fixes can be split across lines — and the second line then opens with the
	// `//` marker. Dropping the markers and collapsing runs of whitespace asserts the WORDS
	// are recorded, without pinning where the reflow happens to break them.
	tail := commentMarker.ReplaceAllString(src[idx:], "")
	tail = strings.Join(strings.Fields(tail), " ")

	for _, want := range []struct{ name, text string }{
		{"the tier itself", "TierTS"},
		{`its String() value, so the document and the wire agree`, `Its String()`},
		{"the resolution order it sits in", "T2 escalations, T1 escalations, T0, TS, empty"},
		{"the gate that reaches it", "in.Adapter.Selection == adapter.SelectionStatic"},
		{"the injected existence check level 1 resolves against", "Exists func(rel string) bool"},
		{"the injected hop counts level 2 ranks by", "ImportDistance func(changed string) map[string]int"},
		{"test_for resolution", "func (a *Adapter) TestForCandidate(rel string, exists func(string) bool) (string, bool)"},
		{"init's fidelity-aware closing line", "func RenderNextStep(detected []*adapter.Adapter) string"},
	} {
		if !strings.Contains(tail, want.text) {
			t.Errorf("the M6b amendments must record %s — no %q", want.name, want.text)
		}
	}
}

// What this protects: a reader of the tier enum finds TS. The enum is documented once, in
// the M1a-era `internal/selector` section, and the amendment that adds TierTS is 700 lines
// below it — so the enum on its own reads as a complete set that has no static tier, which
// is exactly the reverse-engineering the contract document exists to prevent.
func TestTheTierEnumPointsAtTheStaticTierAmendment(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	head, _, ok := strings.Cut(src, "## M6b amendments")
	if !ok {
		t.Fatal("docs/plans/00-interfaces.md has no `## M6b amendments` section")
	}
	i := strings.Index(head, "func (t Tier) String() string")
	if i < 0 {
		t.Fatal("docs/plans/00-interfaces.md documents no `func (t Tier) String() string`")
	}
	near := strings.Join(strings.Fields(commentMarker.ReplaceAllString(head[i:], "")), " ")
	for _, want := range []string{"TierTS", "T2 escalations, T1 escalations, T0, TS, empty"} {
		if !strings.Contains(near, want) {
			t.Errorf("the tier enum must point at the static tier — no %q beside it", want)
		}
	}
}

// What this protects: the two documents that describe the resolution order cannot drift
// apart. select.go's own header comment is what an implementer reads; 00-interfaces.md is
// what the next milestone binds to. PRD #230 AC1 makes the order part of the contract, so
// a step reordered in one place and not the other is a red test rather than two documents
// quietly disagreeing about when TS is reached.
func TestDocumentedResolutionOrderMatchesTheSelector(t *testing.T) {
	impl := readRepoFile(t, "internal/selector/select.go")

	head, _, ok := strings.Cut(impl, "func Select(in Inputs) Selection {")
	if !ok {
		t.Fatal("internal/selector/select.go declares no `func Select(in Inputs) Selection`")
	}
	var at []int
	for _, step := range []string{"T2 escalations", "T1 escalations", "4. T0", "5. TS", "6. TierEmpty"} {
		i := strings.Index(head, step)
		if i < 0 {
			t.Fatalf("Select's header comment names no %q step; the documented order is "+
				"T2 escalations, T1 escalations, T0, TS, empty", step)
		}
		at = append(at, i)
	}
	if !slices.IsSorted(at) {
		t.Errorf("Select's header comment lists the steps out of order (offsets %v); the "+
			"documented order is T2 escalations, T1 escalations, T0, TS, empty", at)
	}
}

// What this protects: the `selection_fidelity` vocabulary stays in ONE document. It is
// PRD #233's — the front-end honesty milestone owns the field, its three values and which
// surfaces carry it — and a second copy here is a copy that goes stale the first time that
// PRD refines it. The TS tier needs no part of it: a tier name and a fidelity are answers
// to different questions.
//
// If ownership ever moves, delete this guard in the same commit that writes the vocabulary
// here, and say in the message which PRD licenses the move.
func TestInterfaceContractDoesNotDuplicateTheSelectionFidelityVocabulary(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	if strings.Contains(src, "selection_fidelity") {
		t.Error("00-interfaces.md documents the `selection_fidelity` wire field; it belongs " +
			"to PRD #233 alone, and two copies of a vocabulary drift")
	}
}
