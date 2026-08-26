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
