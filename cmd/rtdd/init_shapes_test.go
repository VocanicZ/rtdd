package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

// PRD #411 AC5: `rtdd init` sets up every git repository — the shapes v0.2 refused or
// never met — writes the front-ends and a config the graph loads, and names no adapter.
func TestInitSucceedsOnEveryRepositoryShape(t *testing.T) {
	for _, shape := range []struct {
		name  string
		files map[string]string
	}{
		{"empty", nil},
		{"prose only", map[string]string{"README.md": "# notes\n", "docs/guide.rst": "Guide\n=====\n"}},
		{"a language the scanner does not know", map[string]string{
			"src/main.zig": "pub fn main() void {}\n", "build.zig": "const std = @import(\"std\");\n"}},
		{"polyglot", map[string]string{
			"src/calc.py":        "def add(a, b):\n    return a + b\n",
			"tests/test_calc.py": "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n",
			"pkg/add.go":         "package pkg\n\nfunc Add(a, b int) int { return a + b }\n",
			"pkg/add_test.go":    "package pkg\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { Add(1, 2) }\n",
			"web/app.ts":         "export function greet(n: string) { return n }\n",
			"web/app.test.ts":    "it('greets', () => { greet('a') })\n",
		}},
	} {
		t.Run(shape.name, func(t *testing.T) {
			dir := gittest.Init(t)
			for rel, body := range shape.files {
				gittest.Write(t, dir, rel, body)
			}
			if len(shape.files) > 0 {
				gittest.Commit(t, dir, "init")
			}
			code, out, errOut := rtdd(t, dir, "init")
			if code != 0 {
				t.Fatalf("rtdd init = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
			}
			for _, bad := range []string{"adapter", "detect", "refus", "--force"} {
				if strings.Contains(strings.ToLower(out+errOut), bad) {
					t.Errorf("rtdd init output says %q:\n%s%s", bad, out, errOut)
				}
			}
			for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc", "AGENTS.md", ".rtdd/config.yaml", ".gitignore"} {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
					t.Errorf("rtdd init did not write %s: %v", rel, err)
				}
			}
			cfg, err := graph.LoadConfig(dir)
			if err != nil {
				t.Fatalf("the graph cannot load the config init wrote: %v", err)
			}
			if !reflect.DeepEqual(cfg, graph.DefaultConfig()) {
				t.Errorf("the config init wrote loads as %+v, want graph.DefaultConfig()", cfg)
			}
			raw := readRepoFileForTest(t, dir, ".rtdd/config.yaml")
			for _, key := range []string{"test_files:", "scan_exclude:", "graphify_path:", "max_stale_ratio:"} {
				if !strings.Contains(raw, key) {
					t.Errorf(".rtdd/config.yaml has no %s\n%s", key, raw)
				}
			}
			if strings.Contains(raw, "adapters") {
				t.Errorf(".rtdd/config.yaml still has an adapters key:\n%s", raw)
			}
		})
	}
}

// PRD #411 AC5: two runs leave `.rtdd/graph.json` in .gitignore exactly once, and the
// second run changes nothing.
func TestInitTwiceIgnoresTheGraphCacheExactlyOnce(t *testing.T) {
	dir := graphRepo(t)
	gittest.Write(t, dir, ".gitignore", "node_modules/\n")
	gittest.Commit(t, dir, "ignore")
	if code, _, errOut := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("first rtdd init = %d, stderr %q", code, errOut)
	}
	first := gitRun(t, dir, "status", "--porcelain", "-uall")
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("second rtdd init = %d, stderr %q", code, errOut)
	}
	if n := strings.Count(readRepoFileForTest(t, dir, ".gitignore"), ".rtdd/graph.json\n"); n != 1 {
		t.Errorf(".gitignore names .rtdd/graph.json %d times, want 1", n)
	}
	if again := gitRun(t, dir, "status", "--porcelain", "-uall"); again != first {
		t.Errorf("the second init changed the tree:\nafter first:\n%s\nafter second:\n%s", first, again)
	}
	for _, line := range strings.Split(strings.TrimSpace(strings.SplitN(out, "\n\n", 2)[0]), "\n") {
		if !strings.HasPrefix(line, "skip") {
			t.Errorf("the second init planned %q, want every step skipped", line)
		}
	}
}

// PRD #411 AC5: `--force` is removed — init --help does not offer it and passing it is a
// usage error.
func TestInitHasNoForceFlag(t *testing.T) {
	dir := graphRepo(t)
	code, _, errOut := rtdd(t, dir, "init", "--force")
	if code != 2 || !strings.Contains(errOut, "flag provided but not defined: -force") {
		t.Errorf("rtdd init --force = %d, stderr %q; want 2 naming the undefined flag", code, errOut)
	}
	_, _, help := rtdd(t, dir, "init", "--help")
	if strings.Contains(help, "force") {
		t.Errorf("rtdd init --help still offers --force:\n%s", help)
	}
	_, usage, _ := rtdd(t, dir, "--help")
	if !regexp.MustCompile(`(?m)^\s*rtdd init\s+\[--dry-run\]$`).MatchString(usage) {
		t.Errorf("rtdd --help does not document `rtdd init [--dry-run]`:\n%s", usage)
	}
}
