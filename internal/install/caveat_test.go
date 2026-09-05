package install

import (
	"strings"
	"testing"
)

// The caveat has to be the first thing an agent reads: an agent that reads "run
// `rtdd which`" and nothing else believes it has a selection tool it does not have.
func TestWithNoAdapterCaveatPutsTheCaveatInTheFirstParagraph(t *testing.T) {
	files, err := Files()
	if err != nil {
		t.Fatal(err)
	}

	got := WithNoAdapterCaveat(files)["dist/SKILL.md"]
	if !strings.Contains(got, "no adapter detected") {
		t.Fatalf("SKILL.md carries no caveat:\n%s", got)
	}
	if !strings.Contains(got, ".rtdd/adapters/") {
		t.Errorf("the caveat does not name the remedy:\n%s", got)
	}

	_, rest, ok := strings.Cut(got, "\n# rtdd\n")
	if !ok {
		t.Fatalf("the generated skill lost its title:\n%s", got)
	}
	if !strings.HasPrefix(strings.TrimLeft(rest, "\n"), "**Caveat: no adapter detected") {
		t.Errorf("the caveat is not the first paragraph under the title:\n%s", rest[:min(len(rest), 400)])
	}
	// Everything the front-end said before is still said.
	if !strings.Contains(got, files["dist/SKILL.md"][strings.Index(files["dist/SKILL.md"], "## "):]) {
		t.Error("splicing the caveat dropped part of the generated skill")
	}
}

// Only the Claude Code skill is rewritten, and the caller's map is never mutated: the
// map install.Files returned is reused by the same command.
func TestWithNoAdapterCaveatLeavesTheOtherFrontEndsAndTheInputAlone(t *testing.T) {
	files, err := Files()
	if err != nil {
		t.Fatal(err)
	}
	before := files["dist/SKILL.md"]

	got := WithNoAdapterCaveat(files)
	if files["dist/SKILL.md"] != before {
		t.Error("WithNoAdapterCaveat mutated its input")
	}
	for _, key := range []string{"dist/AGENTS.md", "dist/cursor/rules/rtdd.mdc"} {
		if got[key] != files[key] {
			t.Errorf("%s was rewritten; only the Claude Code skill carries the caveat", key)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
