package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
)

// PRD #511 AC2: the README's "Supported agents" table lists exactly the matrix's columns,
// in its order, each with the table's Verified cell and every limitation stated.
func TestREADMESupportedAgentsAreTheMatrixColumns(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(b), "\n## Supported agents\n")
	if !ok {
		t.Fatal("README.md has no `## Supported agents` section")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	var rows [][]string
	for _, l := range strings.Split(section, "\n") {
		if !strings.HasPrefix(l, "| ") || strings.HasPrefix(l, "| agent ") || strings.HasPrefix(l, "| ---") {
			continue
		}
		var cells []string
		for _, c := range strings.Split(strings.Trim(l, "|"), " | ") {
			cells = append(cells, strings.TrimSpace(c))
		}
		rows = append(rows, cells)
	}
	if len(rows) != len(matrixAgents) {
		t.Fatalf("README lists %d agents, the matrix has %d columns", len(rows), len(matrixAgents))
	}
	for i, a := range matrixAgents {
		ag := agentRow(t, a.id)
		if rows[i][0] != a.name {
			t.Errorf("README row %d is %q, want %q (the matrix's order)", i+1, rows[i][0], a.name)
		}
		verified := ag.Verified
		if verified == "" {
			verified = "not yet verified"
		}
		if !strings.Contains(strings.Join(rows[i], " | "), verified) {
			t.Errorf("README row %s does not say %q", a.name, verified)
		}
		for _, lim := range ag.Limits {
			if !strings.Contains(strings.Join(rows[i], " | "), lim) {
				t.Errorf("README row %s does not state the limitation %q", a.name, lim)
			}
		}
	}
	if !strings.Contains(section, "reads only a shared file") || !strings.Contains(section, "--append-to") {
		t.Error("`## Supported agents` must say that an agent which reads only a shared file gets only `--append-to`")
	}
}

// Every user-level skill an agent reads fits every agent's frontmatter rules: OpenCode and
// Cursor require name to equal the directory, Zcode drops a description over 1024
// characters and shows the first 250.
func TestEverySkillFrontmatterFitsEveryAgent(t *testing.T) {
	files, err := install.Files()
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{"dist/GLOBAL-SKILL.md", "dist/SKILL.md"} {
		front, _, _ := strings.Cut(strings.TrimPrefix(files[src], "---\n"), "\n---\n")
		var name, desc string
		for _, l := range strings.Split(front, "\n") {
			if v, ok := strings.CutPrefix(l, "name: "); ok {
				name = v
			}
			if v, ok := strings.CutPrefix(l, "description: "); ok {
				desc = v
			}
		}
		if name != "rtdd" {
			t.Errorf("%s name = %q, want rtdd — the directory every agent's copy lives in", src, name)
		}
		if len(desc) > 1024 {
			t.Errorf("%s description is %d characters; Zcode drops a skill over 1024", src, len(desc))
		}
		if first, _, _ := strings.Cut(desc, ". "); len(first) > 250 {
			t.Errorf("%s description's first sentence is %d characters; Zcode shows 250", src, len(first))
		}
	}
}
