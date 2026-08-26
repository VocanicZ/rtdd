// Package contract holds repo-level guard tests for the M1a bootstrap: the module
// declaration, the dogfooding git attributes, the ignore rules, the interface
// contract amendment, and the CI pipeline. It contains no non-test code.
package contract

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	if !regexp.MustCompile(`(?m)^go\s+1\.24\s*$`).MatchString(src) {
		t.Errorf("go.mod must declare `go 1.24`, got:\n%s", src)
	}
}

func TestGoModRequiresExactlyYAMLv3(t *testing.T) {
	src := readRepoFile(t, "go.mod")

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
		case strings.HasPrefix(trimmed, "module ") || strings.HasPrefix(trimmed, "go "):
			continue
		}
		if m := req.FindStringSubmatch(trimmed); m != nil {
			got = append(got, m[1])
		}
	}

	want := []string{"gopkg.in/yaml.v3"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("go.mod requirements = %v, want exactly %v", got, want)
	}
}

// TestModuleGraphIsModuleAndYAMLOnly pins the "exactly one non-stdlib dependency"
// constraint against `go list -m all`.
//
// `go list -m all` also prints gopkg.in/check.v1: gopkg.in/yaml.v3's own go.mod declares
// `go 1.11`, so module-graph pruning does not apply to it and its test-only requirement
// stays in the graph. It contributes no source — go.sum carries only its /go.mod hash,
// never an h1: content hash — so the build list that actually compiles is still exactly
// the main module plus yaml.v3. This test asserts that: the two real modules are present,
// and every other module in the graph is source-less.
func TestModuleGraphIsModuleAndYAMLOnly(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}
	root := repoRoot(t)
	cmd := exec.Command("go", "list", "-m", "all")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -m all: %v\n%s", err, out)
	}
	var mods []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			mods = append(mods, strings.Fields(line)[0])
		}
	}
	if len(mods) < 2 || mods[0] != "github.com/VocanicZ/rtdd" {
		t.Fatalf("go list -m all = %v, want it to start with the main module", mods)
	}

	sum := readRepoFile(t, "go.sum")
	// A go.sum line "<mod> <version> h1:..." records module source; "<mod> <version>/go.mod
	// h1:..." records only the go.mod file, which is graph metadata, not compiled code.
	hasSource := func(mod string) bool {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(mod) + `\s+(\S+)\s+h1:`)
		for _, m := range re.FindAllStringSubmatch(sum, -1) {
			if !strings.HasSuffix(m[1], "/go.mod") {
				return true
			}
		}
		return false
	}

	var withSource []string
	for _, m := range mods[1:] {
		if hasSource(m) {
			withSource = append(withSource, m)
		}
	}
	want := []string{"gopkg.in/yaml.v3"}
	if len(withSource) != len(want) || withSource[0] != want[0] {
		t.Errorf("non-stdlib modules contributing source = %v, want exactly %v (full graph: %v)",
			withSource, want, mods)
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
	for _, want := range []string{
		"pull_request",
		"branches: [main]",
		"go build ./...",
		"go vet ./...",
		"gofmt -l .",
		"go test ./... -count=1",
		"CGO_ENABLED=0 go build -o /tmp/rtdd ./cmd/rtdd",
		"statically linked",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("ci.yml must run/contain %q", want)
		}
	}
}
