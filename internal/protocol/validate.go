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

// withinBudget asserts that out fits the target's byte budget.
//
// RenderAll enforces the same budget against freshly rendered bytes, which
// makes it impossible to *render* an over-budget file. That check cannot see a
// file on disk: `rtdd-gen verify` reads dist/ and never renders, so without
// this a 5593-byte .mdc against a 4000-byte budget passes verify with exit 0.
func withinBudget(t Target, out string) error {
	if t.MaxBytes > 0 && len(out) > t.MaxBytes {
		return fmt.Errorf("%w: %d bytes exceeds budget %d", ErrOverBudget, len(out), t.MaxBytes)
	}
	return nil
}

// requiredBodiesPresent asserts that every one of t.Required's sections has its
// body present verbatim in out. It is the counterpart of
// requiredHeadingsPresent for a target that renders no headings of its own:
// AGENTS.md carries bare bodies under a single "## rtdd", so a heading check
// would pass on a block whose content had been emptied out.
func requiredBodiesPresent(d *Doc, t Target, out string) error {
	bodies := map[string]string{}
	for _, s := range d.For(t.Name) {
		bodies[s.ID] = s.BodyFor(t.Name)
	}
	var missing []string
	for _, id := range t.Required {
		body, ok := bodies[id]
		if !ok || body == "" || !strings.Contains(out, body) {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("target %q output is missing the content of required section(s) %v", t.Name, missing)
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
	if err := withinBudget(t, out); err != nil {
		return fmt.Errorf("SKILL.md: %w", err)
	}
	if err := hasFrontmatter(out, "name: rtdd", "description: "); err != nil {
		return fmt.Errorf("SKILL.md: %w", err)
	}
	return requiredHeadingsPresent(d, t, out)
}

// validateMDC fails when the frontmatter block is missing or lacks globs /
// alwaysApply, or when a required section's heading is missing.
func validateMDC(d *Doc, t Target, out string) error {
	if err := withinBudget(t, out); err != nil {
		return fmt.Errorf(".mdc: %w", err)
	}
	if err := hasFrontmatter(out, "description: ", "globs: ", "alwaysApply: "); err != nil {
		return fmt.Errorf(".mdc: %w", err)
	}
	return requiredHeadingsPresent(d, t, out)
}

// validateAgents fails when the output exceeds the agents byte budget, when the
// rtdd block is not marker-delimited, when a required section's content is
// absent, or when it contains the body of a section that is not targeted at
// agents — the "AGENTS.md swallowed the skill body" failure mode a
// byte-comparison drift check cannot see.
func validateAgents(d *Doc, t Target, out string) error {
	if err := withinBudget(t, out); err != nil {
		return fmt.Errorf("AGENTS.md: %w", err)
	}
	// The block is delimited because AGENTS.md is a host-owned file and
	// `rtdd init` only ever rewrites what is between the markers. A file that
	// lost them is unmergeable, and an empty or truncated one loses them first.
	begin := strings.Index(out, BeginMarker)
	if begin < 0 {
		return fmt.Errorf("AGENTS.md: missing the begin marker %q", BeginMarker)
	}
	end := strings.Index(out[begin:], EndMarker)
	if end < 0 {
		return fmt.Errorf("AGENTS.md: missing the end marker %q after the begin marker", EndMarker)
	}
	if err := requiredBodiesPresent(d, t, out[begin:begin+end]); err != nil {
		return fmt.Errorf("AGENTS.md: %w", err)
	}
	// Any section the source does not target at agents is wrong here, not only
	// a skill-only one: a section shared by skill and mdc swallowed into
	// AGENTS.md is the same bug wearing a second target.
	for _, s := range d.Sections {
		if s.HasTarget("agents") {
			continue
		}
		for _, target := range s.Targets {
			if body := s.BodyFor(target); body != "" && strings.Contains(out, body) {
				return fmt.Errorf("AGENTS.md contains section %q, which is not targeted at agents", s.ID)
			}
		}
	}
	return nil
}
