package protocol

import (
	"errors"
	"strings"
	"testing"
)

const renderSample = `<!-- rtdd:meta version=1 -->

<!-- rtdd:section id=setup title="Setting up a repository" targets=global,global-agents order=5 -->
setup body
<!-- rtdd:endsection -->

<!-- rtdd:section id=what title="What rtdd reports" targets=skill,agents,mdc,global,global-agents order=10 -->
what body
<!-- rtdd:endsection -->

<!-- rtdd:section id=which title="rtdd which" targets=skill,agents,mdc,global,global-agents order=20 -->
which body
<!-- rtdd:endsection -->

<!-- rtdd:section id=run title="rtdd run" targets=skill,agents,mdc,global,global-agents order=30 -->
run body
<!-- rtdd:endsection -->

<!-- rtdd:section id=uncovered title="The uncovered report" targets=skill,agents,mdc,global,global-agents order=40 -->
uncovered body
<!-- rtdd:endsection -->

<!-- rtdd:section id=empty title="An empty selection" targets=skill,agents,mdc,global,global-agents order=50 -->
empty body
<!-- rtdd:endsection -->

<!-- rtdd:section id=fidelity title="Selection fidelity" targets=skill,agents,mdc,global,global-agents order=55 -->
fidelity body
<!-- rtdd:endsection -->

<!-- rtdd:section id=json title="JSON output" targets=skill,global order=60 -->
json body
<!-- rtdd:endsection -->

<!-- rtdd:section id=commands title="The rest of the commands" targets=skill,global order=70 -->
commands body
<!-- rtdd:endsection -->

<!-- rtdd:section id=limits title="What it cannot see" targets=skill,mdc,global order=80 -->
limits body
<!-- rtdd:endsection -->

<!-- rtdd:section id=map title="The map file" targets=skill,global order=90 -->
map body
<!-- rtdd:endsection -->
`

func sampleDoc(t *testing.T) *Doc {
	t.Helper()
	d, err := Parse(renderSample)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return d
}

func TestTargetByNameFindsRegisteredTargets(t *testing.T) {
	for _, name := range []string{"skill", "agents", "mdc"} {
		if _, ok := TargetByName(name); !ok {
			t.Fatalf("TargetByName(%q) not found", name)
		}
	}
	if _, ok := TargetByName("vscode"); ok {
		t.Fatal("TargetByName(vscode) found, want not found")
	}
}

func TestRenderAllKeysByOutPath(t *testing.T) {
	out, err := RenderAll(sampleDoc(t))
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	for _, path := range []string{"dist/SKILL.md", "dist/AGENTS.md", "dist/cursor/rules/rtdd.mdc"} {
		if _, ok := out[path]; !ok {
			t.Fatalf("RenderAll output missing key %q", path)
		}
	}
}

func TestRenderAllTargetsGetDifferentSections(t *testing.T) {
	out, err := RenderAll(sampleDoc(t))
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	skill := out["dist/SKILL.md"]
	agents := out["dist/AGENTS.md"]
	mdc := out["dist/cursor/rules/rtdd.mdc"]

	if !strings.Contains(skill, "map body") {
		t.Error("SKILL.md missing skill-only section body")
	}
	if strings.Contains(agents, "map body") {
		t.Error("AGENTS.md must not contain the skill-only section body")
	}
	if strings.Contains(mdc, "map body") {
		t.Error(".mdc must not contain the skill-only section body")
	}
	if !strings.Contains(mdc, "limits body") {
		t.Error(".mdc missing shared section body")
	}
}

func TestRenderAllIsByteStableAcrossCalls(t *testing.T) {
	d := sampleDoc(t)
	out1, err := RenderAll(d)
	if err != nil {
		t.Fatalf("RenderAll (1): %v", err)
	}
	out2, err := RenderAll(d)
	if err != nil {
		t.Fatalf("RenderAll (2): %v", err)
	}
	for path, want := range out1 {
		if got := out2[path]; got != want {
			t.Fatalf("%s not byte-stable:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", path, want, got)
		}
	}
}

func TestRenderAllReturnsErrOverBudgetNamingTheTarget(t *testing.T) {
	huge := strings.Repeat("x", agentsMaxBytes*2)
	d, err := Parse(renderSample + "\n<!-- rtdd:section id=huge title=\"Huge\" targets=agents order=5 -->\n" + huge + "\n<!-- rtdd:endsection -->\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = RenderAll(d)
	if err == nil {
		t.Fatal("RenderAll: want error for over-budget target")
	}
	if !errors.Is(err, ErrOverBudget) {
		t.Fatalf("RenderAll error = %v, want it to wrap ErrOverBudget", err)
	}
	if !strings.Contains(err.Error(), "dist/AGENTS.md") {
		t.Fatalf("RenderAll error %q does not name the offending target", err.Error())
	}
}

func TestRenderAllFailsWhenARequiredSectionIsMissing(t *testing.T) {
	d, err := Parse(`<!-- rtdd:meta version=1 -->
<!-- rtdd:section id=what title="What" targets=skill,agents,mdc order=10 -->
body
<!-- rtdd:endsection -->
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := RenderAll(d); err == nil {
		t.Fatal("RenderAll: want error when a target's required sections are missing")
	}
}

func TestRenderSkillFrontmatterAndGeneratedHeader(t *testing.T) {
	out, err := RenderAll(sampleDoc(t))
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	skill := out["dist/SKILL.md"]
	if !strings.HasPrefix(skill, "---\nname: rtdd\n") {
		t.Fatalf("SKILL.md does not open with expected frontmatter:\n%s", skill)
	}
	if !strings.Contains(skill, Generated) {
		t.Error("SKILL.md missing generated header")
	}
}

func TestRenderMDCFrontmatterHasGlobsAndAlwaysApply(t *testing.T) {
	out, err := RenderAll(sampleDoc(t))
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	mdc := out["dist/cursor/rules/rtdd.mdc"]
	if !strings.HasPrefix(mdc, "---\n") {
		t.Fatalf(".mdc does not open with frontmatter:\n%s", mdc)
	}
	if !strings.Contains(mdc, "\nglobs: ") || !strings.Contains(mdc, "\nalwaysApply: ") {
		t.Fatalf(".mdc frontmatter missing globs/alwaysApply:\n%s", mdc)
	}
}

func TestRenderAgentsHasBeginEndMarkers(t *testing.T) {
	out, err := RenderAll(sampleDoc(t))
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	agents := out["dist/AGENTS.md"]
	if !strings.HasPrefix(agents, BeginMarker) {
		t.Fatalf("AGENTS.md does not open with BeginMarker:\n%s", agents)
	}
	if !strings.HasSuffix(agents, EndMarker+"\n") {
		t.Fatalf("AGENTS.md does not close with EndMarker:\n%s", agents)
	}
}

func TestRenderAllSucceedsOnTheRealProtocol(t *testing.T) {
	src, err := readProtocolMD(t)
	if err != nil {
		t.Fatalf("read PROTOCOL.md: %v", err)
	}
	d, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := RenderAll(d); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
}
