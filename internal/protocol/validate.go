package protocol

import (
	"fmt"
	"strings"
)

// hasFrontmatter checks that out opens with a `---`-delimited block and that
// every field is present somewhere inside it.
func hasFrontmatter(out string, fields ...string) error {
	if !strings.HasPrefix(out, "---\n") {
		return fmt.Errorf("missing frontmatter block")
	}
	end := strings.Index(out[4:], "\n---\n")
	if end < 0 {
		return fmt.Errorf("frontmatter block is not closed")
	}
	block := out[:4+end]
	var missing []string
	for _, f := range fields {
		if !strings.Contains(block, f) {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("frontmatter is missing %v", missing)
	}
	return nil
}

// requiredHeadingsPresent asserts that every one of t.Required's sections is
// present as a "## <title>" heading in out. A section can be present in the
// Doc (requireSections already checked that) yet still be dropped from the
// rendered string by a rendering bug — that gap is what this catches.
func requiredHeadingsPresent(d *Doc, t Target, out string) error {
	titles := map[string]string{}
	for _, s := range d.For(t.Name) {
		titles[s.ID] = s.Title
	}
	var missing []string
	for _, id := range t.Required {
		title, ok := titles[id]
		if !ok || !strings.Contains(out, "## "+title) {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("target %q rendered output is missing the heading for required section(s) %v", t.Name, missing)
	}
	return nil
}

// validateSkill fails when the skill frontmatter is missing or when a
// required section's heading did not make it into the rendered output.
func validateSkill(d *Doc, t Target, out string) error {
	if err := hasFrontmatter(out, "name: rtdd", "description: "); err != nil {
		return fmt.Errorf("SKILL.md: %w", err)
	}
	return requiredHeadingsPresent(d, t, out)
}

// validateMDC fails when the frontmatter block is missing or lacks globs /
// alwaysApply, or when a required section's heading is missing.
func validateMDC(d *Doc, t Target, out string) error {
	if err := hasFrontmatter(out, "description: ", "globs: ", "alwaysApply: "); err != nil {
		return fmt.Errorf(".mdc: %w", err)
	}
	return requiredHeadingsPresent(d, t, out)
}

// validateAgents fails when the output exceeds the agents byte budget, or
// when it contains the body of a section that belongs only to the skill
// target — the "AGENTS.md swallowed the whole skill body" failure mode a
// byte-comparison drift check cannot see.
func validateAgents(d *Doc, t Target, out string) error {
	if len(out) > agentsMaxBytes {
		return fmt.Errorf("AGENTS.md: %d bytes exceeds budget %d", len(out), agentsMaxBytes)
	}
	for _, s := range d.Sections {
		if len(s.Targets) != 1 || s.Targets[0] != "skill" {
			continue
		}
		if body := s.BodyFor("skill"); body != "" && strings.Contains(out, body) {
			return fmt.Errorf("AGENTS.md contains section %q, which belongs only to the skill target", s.ID)
		}
	}
	return nil
}
