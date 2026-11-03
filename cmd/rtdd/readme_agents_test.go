package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
)

// supportedAgentRows returns the rows of the `## Supported agents` table in the named
// repository file, one slice of cells per agent, and the section's text.
func supportedAgentRows(t *testing.T, rel string) ([][]string, string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(b), "\n## Supported agents\n")
	if !ok {
		t.Fatalf("%s has no `## Supported agents` section", rel)
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
	return rows, section
}

// PRD #511 AC2: the "Supported agents" table lists exactly the matrix's columns, in its
// order, each with the table's Verified cell. The README carries the compact table; the
// full one, with every limitation stated, is docs/reference.md's (#568).
func TestREADMESupportedAgentsAreTheMatrixColumns(t *testing.T) {
	for _, rel := range []string{"README.md", "docs/reference.md"} {
		rows, section := supportedAgentRows(t, rel)
		if len(rows) != len(matrixAgents) {
			t.Fatalf("%s lists %d agents, the matrix has %d columns", rel, len(rows), len(matrixAgents))
		}
		for i, a := range matrixAgents {
			ag := agentRow(t, a.id)
			if rows[i][0] != a.name {
				t.Errorf("%s row %d is %q, want %q (the matrix's order)", rel, i+1, rows[i][0], a.name)
			}
			verified := ag.Verified
			if verified == "" {
				verified = "not yet verified"
			}
			if !strings.Contains(strings.Join(rows[i], " | "), verified) {
				t.Errorf("%s row %s does not say %q", rel, a.name, verified)
			}
			if rel != "docs/reference.md" {
				continue
			}
			for _, lim := range ag.Limits {
				if !strings.Contains(strings.Join(rows[i], " | "), lim) {
					t.Errorf("%s row %s does not state the limitation %q", rel, a.name, lim)
				}
			}
		}
		if !strings.Contains(section, "reads only a shared file") || !strings.Contains(section, "--append-to") {
			t.Errorf("%s: `## Supported agents` must say that an agent which reads only a shared file gets only `--append-to`", rel)
		}
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
