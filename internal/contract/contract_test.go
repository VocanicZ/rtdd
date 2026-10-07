// Package contract holds repo-level guard tests: the module declaration, the ignore
// rules, the interface contract amendments, and the CI pipeline. It contains no non-test
// code.
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
	// Both `go 1.24` and the patch-qualified `go 1.24.0` are the 1.24 language version;
	// anything outside the 1.24 line is a real change.
	if !regexp.MustCompile(`(?m)^go\s+1\.24(\.\d+)?\s*$`).MatchString(src) {
		t.Errorf("go.mod must declare the go 1.24 language version, got:\n%s", src)
	}
}

// TestGoModRequiresExactlyYAML pins the engine's direct dependencies. The one-pipeline
// spec §9 permits exactly one: gopkg.in/yaml.v3 for adapter definitions. Adding another
// needs a spec amendment (DEVELOPMENT.md, "Dependencies").
func TestGoModRequiresExactlyYAML(t *testing.T) {
	got := directRequires(t, readRepoFile(t, "go.mod"))

	want := []string{"gopkg.in/yaml.v3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("go.mod direct requirements = %v, want exactly %v", got, want)
	}
}

// directRequires returns the module paths go.mod requires DIRECTLY — sorted, and
// excluding every `// indirect` line, which is the module graph the direct dependency
// drags in rather than a choice this repo made.
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

// TestCompiledModulesAreYAMLOnly pins the "zero non-stdlib dependencies in the engine
// except yaml.v3" constraint against what the toolchain actually COMPILES, which is the
// claim that matters — `go list -m all` reports the whole module graph, including modules
// that contribute only a go.mod file.
func TestCompiledModulesAreYAMLOnly(t *testing.T) {
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
	}

	var extra []string
	for _, m := range mods {
		if !permitted[m] {
			extra = append(extra, m)
		}
	}
	if len(extra) > 0 {
		t.Errorf("modules compiled into the engine beyond yaml.v3 = %v (full set: %v)", extra, mods)
	}
	if !seen["gopkg.in/yaml.v3"] {
		t.Errorf("gopkg.in/yaml.v3 is not compiled into the engine at all; compiled set = %v", mods)
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
		`func (c Class) String() string // "covered" | "uncovered"`,
		"func Summarize(reports []FileReport) Summary",
		"func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport",
		"func (r FileReport) UncoveredLines() int",
		// internal/gitctx — the one raw diff every changed line derives from.
		"func RawDiff(repoRoot, base string) (string, error)",
		// internal/doctor — the mandatory spec §9 caveat, an immutable const.
		"const Caveat = ",
		// internal/initrepo — merge into existing front-ends, never clobber.
		"func Run(repoRoot string) ([]Action, error)",
		// The --json schema v2: the contract the agent front-ends bind to.
		"### The schema, version 2",
		`"schema": 2,`,
		"| `schema` | int | Always `2` for this version",
		// The never-narrow-silently guarantee, carried in the document rather than on
		// stderr a --json consumer discards.
		"| `complete` | bool |",
		"| `warnings` | array of string |",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md must record the M2 addition %q", want)
		}
	}
	for _, pkg := range []string{"internal/uncovered/", "internal/doctor/", "internal/initrepo/"} {
		if !strings.Contains(src, pkg) {
			t.Errorf("00-interfaces.md must list %s in the package layout", pkg)
		}
	}
}

// fieldContractRow matches one markdown table row whose first cell is a backticked field name.
var fieldContractRow = regexp.MustCompile("(?m)^\\| `([^`]+)` \\|")

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

// commentMarker matches the `//` opening a Go doc-comment line inside a fenced block.
var commentMarker = regexp.MustCompile(`(?m)^[ \t]*//[ \t]?`)
