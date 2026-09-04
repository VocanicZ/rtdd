package protocol

import (
	"errors"
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

// TestValidateSkillFailsWhenOverBudget pins the budget to the validator, not
// only to RenderAll: `rtdd-gen verify` reads a file off disk and never renders
// it, so a budget checked only at render time cannot see an over-budget file.
func TestValidateSkillFailsWhenOverBudget(t *testing.T) {
	d := sampleDoc(t)
	tgt := skillTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	bloated := out + strings.Repeat("x", tgt.MaxBytes)
	err = validateSkill(d, tgt, bloated)
	if err == nil {
		t.Fatal("validateSkill: want error for an over-budget SKILL.md")
	}
	if !errors.Is(err, ErrOverBudget) {
		t.Errorf("validateSkill error = %v, want it to wrap ErrOverBudget", err)
	}
}

// TestValidateMDCFailsWhenOverBudget is the second failure the review
// reproduced: the whole skill body with .mdc frontmatter grafted on, far over
// the .mdc budget, passing verify with exit 0.
func TestValidateMDCFailsWhenOverBudget(t *testing.T) {
	d := sampleDoc(t)
	tgt := mdcTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	bloated := out + strings.Repeat("x", tgt.MaxBytes)
	err = validateMDC(d, tgt, bloated)
	if err == nil {
		t.Fatal("validateMDC: want error for an over-budget .mdc")
	}
	if !errors.Is(err, ErrOverBudget) {
		t.Errorf("validateMDC error = %v, want it to wrap ErrOverBudget", err)
	}
}

func TestValidateAgentsFailsWhenOverBudgetWrapsErrOverBudget(t *testing.T) {
	d := sampleDoc(t)
	tgt := agentsTarget(t)
	err := validateAgents(d, tgt, strings.Repeat("x", tgt.MaxBytes+1))
	if err == nil {
		t.Fatal("validateAgents: want error for an oversized AGENTS.md")
	}
	if !errors.Is(err, ErrOverBudget) {
		t.Errorf("validateAgents error = %v, want it to wrap ErrOverBudget", err)
	}
}

// TestValidateAgentsFailsWhenBeginMarkerMissing: the block AGENTS.md carries is
// marker-delimited because `rtdd init` only ever rewrites what is between the
// markers. A file that lost them is wrong for the target.
func TestValidateAgentsFailsWhenBeginMarkerMissing(t *testing.T) {
	d := sampleDoc(t)
	tgt := agentsTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	corrupted := strings.Replace(out, BeginMarker+"\n", "", 1)
	if err := validateAgents(d, tgt, corrupted); err == nil {
		t.Fatal("validateAgents: want error when the begin marker is missing")
	}
}

func TestValidateAgentsFailsWhenEndMarkerMissing(t *testing.T) {
	d := sampleDoc(t)
	tgt := agentsTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	corrupted := strings.Replace(out, EndMarker+"\n", "", 1)
	if err := validateAgents(d, tgt, corrupted); err == nil {
		t.Fatal("validateAgents: want error when the end marker is missing")
	}
}

// TestValidateAgentsFailsWhenEmpty is the first failure the review reproduced:
// a dist/AGENTS.md holding nothing but "hello" passed verify with exit 0.
func TestValidateAgentsFailsWhenEmpty(t *testing.T) {
	d := sampleDoc(t)
	tgt := agentsTarget(t)
	if err := validateAgents(d, tgt, "hello\n"); err == nil {
		t.Fatal("validateAgents: want error for an AGENTS.md with no rtdd block at all")
	}
}

// TestValidateAgentsFailsWhenRequiredSectionContentMissing covers the
// truncated-block case: markers intact, bodies gone.
func TestValidateAgentsFailsWhenRequiredSectionContentMissing(t *testing.T) {
	d := sampleDoc(t)
	tgt := agentsTarget(t)
	empty := BeginMarker + "\n## rtdd\n\n" + EndMarker + "\n"
	err := validateAgents(d, tgt, empty)
	if err == nil {
		t.Fatal("validateAgents: want error when a required section's content is absent")
	}
	if !strings.Contains(err.Error(), tgt.Required[0]) {
		t.Errorf("error = %v, want it to name the missing section %q", err, tgt.Required[0])
	}
}

// TestValidateAgentsFailsWhenItSwallowsASharedSection widens the containment
// check: a section shared by skill and mdc (never targeted at agents) swallowed
// into AGENTS.md is just as wrong as a skill-only one, and the old
// `Targets == ["skill"]` test could not see it.
func TestValidateAgentsFailsWhenItSwallowsASharedSection(t *testing.T) {
	d := sampleDoc(t)
	tgt := agentsTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var shared Section
	for _, s := range d.Sections {
		if !s.HasTarget("agents") && len(s.Targets) > 1 {
			shared = s
			break
		}
	}
	if shared.ID == "" {
		t.Fatal("test fixture has no multi-target non-agents section to swallow")
	}
	corrupted := out + shared.BodyFor(shared.Targets[0]) + "\n"
	if err := validateAgents(d, tgt, corrupted); err == nil {
		t.Fatalf("validateAgents: want error when output contains section %q, which is not targeted at agents", shared.ID)
	}
}

// guttedBodies returns out with the body of every section this target renders
// deleted, leaving the frontmatter and the bare "## " headings behind. It is
// the on-disk mutation from issue #223: a heading-only check passes on it.
func guttedBodies(d *Doc, t Target, out string) string {
	gutted := out
	for _, s := range d.For(t.Name) {
		gutted = strings.Replace(gutted, s.BodyFor(t.Name)+"\n", "", 1)
	}
	return gutted
}

// TestValidateSkillFailsWhenRequiredSectionBodyMissing pins validateSkill to
// the package contract `verify` advertises: it must catch content that is wrong
// even when the headings all survive. A SKILL.md reduced to its frontmatter
// plus bare "## " headings passed verify with exit 0 before this.
func TestValidateSkillFailsWhenRequiredSectionBodyMissing(t *testing.T) {
	d := sampleDoc(t)
	tgt := skillTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	gutted := guttedBodies(d, tgt, out)
	err = validateSkill(d, tgt, gutted)
	if err == nil {
		t.Fatal("validateSkill: want error when every required section's body is deleted")
	}
	if !strings.Contains(err.Error(), tgt.Required[0]) {
		t.Errorf("error = %v, want it to name the missing section %q", err, tgt.Required[0])
	}
}

// TestValidateMDCFailsWhenRequiredSectionBodyMissing is the .mdc half of the
// same gap: headings intact, bodies gone.
func TestValidateMDCFailsWhenRequiredSectionBodyMissing(t *testing.T) {
	d := sampleDoc(t)
	tgt := mdcTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	gutted := guttedBodies(d, tgt, out)
	err = validateMDC(d, tgt, gutted)
	if err == nil {
		t.Fatal("validateMDC: want error when every required section's body is deleted")
	}
	if !strings.Contains(err.Error(), tgt.Required[0]) {
		t.Errorf("error = %v, want it to name the missing section %q", err, tgt.Required[0])
	}
}

// TestValidateMDCFailsWhenItSwallowsASkillOnlySection is the "swallowed the
// skill body" failure mode wearing the mdc target: appending the skill-only
// `map` section keeps the file under the 4000-byte budget, so nothing else
// caught it.
func TestValidateMDCFailsWhenItSwallowsASkillOnlySection(t *testing.T) {
	d := sampleDoc(t)
	tgt := mdcTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var foreign Section
	for _, s := range d.Sections {
		if !s.HasTarget("mdc") {
			foreign = s
			break
		}
	}
	if foreign.ID == "" {
		t.Fatal("test fixture has no non-mdc section to swallow")
	}
	corrupted := out + foreign.BodyFor(foreign.Targets[0]) + "\n"
	err = validateMDC(d, tgt, corrupted)
	if err == nil {
		t.Fatalf("validateMDC: want error when output contains section %q, which is not targeted at mdc", foreign.ID)
	}
	if !strings.Contains(err.Error(), foreign.ID) {
		t.Errorf("error = %v, want it to name the foreign section %q", err, foreign.ID)
	}
}

// skillForeignSample is renderSample plus one section targeted at mdc alone.
// Every section of the real protocol/PROTOCOL.md targets skill, so the
// foreign-section scan for skill cannot be exercised by sampleDoc — it needs a
// purpose-built Doc. The scan still belongs on validateSkill: it guards the
// first non-skill section anyone adds.
const skillForeignSample = renderSample + `
<!-- rtdd:section id=cursoronly title="Cursor only" targets=mdc order=100 -->
cursor only body
<!-- rtdd:endsection -->
`

// TestValidateSkillFailsWhenItSwallowsAnMDCOnlySection covers the skill half of
// the foreign-section scan.
func TestValidateSkillFailsWhenItSwallowsAnMDCOnlySection(t *testing.T) {
	d, err := Parse(skillForeignSample)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tgt := skillTarget(t)
	out, err := tgt.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if err := validateSkill(d, tgt, out); err != nil {
		t.Fatalf("validateSkill on a correct render of the purpose-built doc: %v", err)
	}
	corrupted := out + "cursor only body\n"
	err = validateSkill(d, tgt, corrupted)
	if err == nil {
		t.Fatal(`validateSkill: want error when output contains section "cursoronly", which is not targeted at skill`)
	}
	if !strings.Contains(err.Error(), "cursoronly") {
		t.Errorf("error = %v, want it to name the foreign section %q", err, "cursoronly")
	}
}
