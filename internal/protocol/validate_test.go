package protocol

import (
	"strings"
	"testing"
)

func skillTarget(t *testing.T) Target {
	t.Helper()
	tgt, ok := TargetByName("skill")
	if !ok {
		t.Fatal("skill target not registered")
	}
	return tgt
}

func agentsTarget(t *testing.T) Target {
	t.Helper()
	tgt, ok := TargetByName("agents")
	if !ok {
		t.Fatal("agents target not registered")
	}
	return tgt
}

func mdcTarget(t *testing.T) Target {
	t.Helper()
	tgt, ok := TargetByName("mdc")
	if !ok {
		t.Fatal("mdc target not registered")
	}
	return tgt
}

func TestValidateSkillPassesOnItsOwnRender(t *testing.T) {
	d := sampleDoc(t)
	tgt := skillTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if err := validateSkill(d, tgt, out); err != nil {
		t.Fatalf("validateSkill on a correct render: %v", err)
	}
}

func TestValidateSkillFailsWhenFrontmatterMissing(t *testing.T) {
	d := sampleDoc(t)
	tgt := skillTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	corrupted := strings.TrimPrefix(out, "---\nname: rtdd\ndescription: "+SkillDescription+"\n---\n\n")
	if err := validateSkill(d, tgt, corrupted); err == nil {
		t.Fatal("validateSkill: want error when frontmatter block is stripped")
	}
}

func TestValidateSkillFailsWhenRequiredSectionHeadingMissing(t *testing.T) {
	d := sampleDoc(t)
	tgt := skillTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	corrupted := strings.Replace(out, "## The map file\n\n", "", 1)
	if err := validateSkill(d, tgt, corrupted); err == nil {
		t.Fatal("validateSkill: want error when a required section heading is missing")
	}
}

func TestValidateMDCPassesOnItsOwnRender(t *testing.T) {
	d := sampleDoc(t)
	tgt := mdcTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if err := validateMDC(d, tgt, out); err != nil {
		t.Fatalf("validateMDC on a correct render: %v", err)
	}
}

func TestValidateMDCFailsWhenFrontmatterStripped(t *testing.T) {
	d := sampleDoc(t)
	tgt := mdcTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	i := strings.Index(out, "\n\n")
	corrupted := out[i+2:] // drop the whole frontmatter block, keep the body
	if err := validateMDC(d, tgt, corrupted); err == nil {
		t.Fatal("validateMDC: want error when frontmatter block is stripped")
	}
}

func TestValidateMDCFailsWhenGlobsMissing(t *testing.T) {
	d := sampleDoc(t)
	tgt := mdcTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	corrupted := strings.Replace(out, "globs: "+MdcGlobs+"\n", "", 1)
	if err := validateMDC(d, tgt, corrupted); err == nil {
		t.Fatal("validateMDC: want error when globs is missing from frontmatter")
	}
}

func TestValidateMDCFailsWhenAlwaysApplyMissing(t *testing.T) {
	d := sampleDoc(t)
	tgt := mdcTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	corrupted := strings.Replace(out, "alwaysApply: false\n", "", 1)
	if err := validateMDC(d, tgt, corrupted); err == nil {
		t.Fatal("validateMDC: want error when alwaysApply is missing from frontmatter")
	}
}

func TestValidateAgentsPassesOnItsOwnRender(t *testing.T) {
	d := sampleDoc(t)
	tgt := agentsTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if err := validateAgents(d, tgt, out); err != nil {
		t.Fatalf("validateAgents on a correct render: %v", err)
	}
}

func TestValidateAgentsFailsWhenOverBudget(t *testing.T) {
	d := sampleDoc(t)
	tgt := agentsTarget(t)
	huge := strings.Repeat("x", agentsMaxBytes*2)
	if err := validateAgents(d, tgt, huge); err == nil {
		t.Fatal("validateAgents: want error for an oversized AGENTS.md")
	}
}

// TestValidateAgentsFailsWhenItSwallowsTheSkillBody is the flagship regression
// this issue exists to prevent: a byte-comparison drift check cannot see that
// AGENTS.md has swallowed a section that belongs only to the skill target.
func TestValidateAgentsFailsWhenItSwallowsTheSkillBody(t *testing.T) {
	d := sampleDoc(t)
	tgt := agentsTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var skillOnlyBody string
	for _, s := range d.Sections {
		if len(s.Targets) == 1 && s.Targets[0] == "skill" {
			skillOnlyBody = s.BodyFor("skill")
			break
		}
	}
	if skillOnlyBody == "" {
		t.Fatal("test fixture has no skill-only section to swallow")
	}
	corrupted := out + skillOnlyBody + "\n"
	if err := validateAgents(d, tgt, corrupted); err == nil {
		t.Fatal("validateAgents: want error when output contains a skill-only section's body")
	}
}
