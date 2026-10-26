package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
)

func TestHomeShownPrefixesEveryPathAndLeavesTheInputAlone(t *testing.T) {
	in := []install.Step{{Path: ".codex/AGENTS.md", Action: install.StripBlock}, {Path: ".gemini/GEMINI.md", Action: install.Delete}}
	got := homeShown(in)
	if got[0].Path != "~/.codex/AGENTS.md" || got[1].Path != "~/.gemini/GEMINI.md" || got[1].Action != install.Delete {
		t.Errorf("homeShown = %+v", got)
	}
	if in[0].Path != ".codex/AGENTS.md" {
		t.Errorf("homeShown rewrote its input: %+v", in)
	}
}

func TestRenderLegacyListSaysNoneOrOneLinePerStep(t *testing.T) {
	var b strings.Builder
	renderLegacyList(&b, nil)
	if b.String() != "  none\n" {
		t.Errorf("empty list = %q, want %q", b.String(), "  none\n")
	}
	b.Reset()
	renderLegacyList(&b, []install.Step{{Path: "AGENTS.md", Note: "old block"}, {Path: "~/.codex/AGENTS.md", Note: "old too"}})
	if want := "  AGENTS.md  — old block\n  ~/.codex/AGENTS.md  — old too\n"; b.String() != want {
		t.Errorf("list = %q, want %q", b.String(), want)
	}
}
