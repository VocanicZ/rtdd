package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// readmeBlock returns the body of README.md's fenced block that opens with first, up to
// its closing fence.
func readmeBlock(t *testing.T, first string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	i := strings.Index(src, "```\n"+first+"\n")
	if i < 0 {
		t.Fatalf("README.md has no fenced block opening with %q", first)
	}
	body := src[i+len("```\n"+first+"\n"):]
	return body[:strings.Index(body, "```\n")]
}

// PRD #411 AC7: README's `rtdd which` example is real output — this repository, this
// edit, this binary — so the README cannot describe output rtdd no longer prints.
func TestREADMEWhichExampleIsRealOutput(t *testing.T) {
	want := readmeBlock(t, "$ rtdd which")
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add, total\n\n\ndef test_add():\n    assert add(1, 2) == 3\n\n\ndef test_total():\n    assert total([1, 2]) == 3\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return b + a\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n")
	code, got, errOut := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	// The commit is the one thing that differs between runs: README shows one, the test
	// makes another.
	sha := regexp.MustCompile(`built at [0-9a-f]+,`)
	got, want = sha.ReplaceAllString(got, "built at <HEAD>,"), sha.ReplaceAllString(want, "built at <HEAD>,")
	if got != want {
		t.Errorf("README.md's `$ rtdd which` example is not what rtdd prints\n--- README ---\n%s--- rtdd which ---\n%s", want, got)
	}
}

// PRD #411 AC7: every `rtdd <command>` README shows in a code block is a command rtdd has.
func TestREADMEShowsOnlyCommandsRtddHas(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if i, j := strings.Index(src, "<!-- rtdd:v0.2-record -->"), strings.Index(src, "<!-- /rtdd:v0.2-record -->"); i >= 0 && j > i {
		src = src[:i] + src[j:]
	}
	fence := regexp.MustCompile("(?s)```[a-z]*\n(.*?)```")
	cmd := regexp.MustCompile(`(?m)^\$?\s*rtdd\s+([a-z-]+)`)
	for _, block := range fence.FindAllStringSubmatch(src, -1) {
		for _, m := range cmd.FindAllStringSubmatch(block[1], -1) {
			if !strings.Contains(usage, "rtdd "+m[1]) {
				t.Errorf("README.md shows `rtdd %s`, which `rtdd --help` does not list", m[1])
			}
		}
	}
}
