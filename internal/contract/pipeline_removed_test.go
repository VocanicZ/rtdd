package contract

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// removedByN2 is every package PRD #410 deletes (docs/plans/10-rounds-cutover.md,
// "Deletion order"): the six the spec names, adapters/, and the three their deletion
// leaves with no importer.
var removedByN2 = []string{
	"internal/adapter", "internal/covfmt", "internal/runner", "internal/mapstore",
	"internal/selector", "internal/uncovered",
	"adapters",
	"internal/coverage", "internal/doctor", "internal/pytestfixture",
}

// v02State are the paths of the v0.2 coverage pipeline's state in a host repository.
var v02State = []string{".rtdd/map.jsonl", ".rtdd/meta.json", ".rtdd/adapters/"}

// v02StateAllowed are the files that may still spell a v0.2 state path, each because
// removing v0.2 state from a host repository or rewriting the front-ends is PRD #411's.
// An entry that no longer spells one is an error: the list only shrinks.
var v02StateAllowed = map[string]string{
	"internal/protocol/targets.go":               "front-end descriptions; PRD #411 rewrites them",
	"internal/protocol/global_test.go":           "pins those descriptions; PRD #411 rewrites them",
	"internal/install/uninstall.go":              "removes the v0.2 merge-driver line from a host repository",
	"internal/install/uninstall_test.go":         "proves uninstall leaves v0.2 state it does not own",
	"internal/contract/pipeline_removed_test.go": "this guard",
}

// goFiles is every .go file in the module, repo-relative and slash-separated.
func goFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "build", "work", ".harness":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// PRD #410 AC5: the packages are gone.
func TestCoveragePipelinePackagesAreGone(t *testing.T) {
	root := repoRoot(t)
	for _, p := range removedByN2 {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s still exists", p)
		}
	}
}

// PRD #410 AC5: no Go file — test files included — imports a removed package, and none
// outside v02StateAllowed reads, writes or names .rtdd/map.jsonl, .rtdd/meta.json or
// .rtdd/adapters/.
func TestNoGoCodeReferencesTheCoveragePipeline(t *testing.T) {
	root := repoRoot(t)
	spelled := map[string]bool{}
	for _, rel := range goFiles(t) {
		src, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(token.NewFileSet(), rel, src, parser.ImportsOnly)
		if err != nil {
			continue // testdata fixtures need not parse
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, gone := range removedByN2 {
				if p == "github.com/VocanicZ/rtdd/"+gone || strings.HasPrefix(p, "github.com/VocanicZ/rtdd/"+gone+"/") {
					t.Errorf("%s imports the removed package %s", rel, p)
				}
			}
		}
		for _, s := range v02State {
			if !strings.Contains(string(src), s) {
				continue
			}
			spelled[rel] = true
			if _, ok := v02StateAllowed[rel]; !ok {
				t.Errorf("%s still names %s", rel, s)
			}
		}
	}
	for rel, why := range v02StateAllowed {
		if !spelled[rel] {
			t.Errorf("%s is allowed to name v0.2 state (%s) but no longer does: remove it from v02StateAllowed", rel, why)
		}
	}
}
