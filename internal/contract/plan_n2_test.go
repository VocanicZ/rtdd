package contract

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The N2 plan document is a contract, not prose: issue #445 makes it the first child of
// PRD #410 and every sibling (#446-#454) is implemented against it. N2 is the cutover —
// rounds replace the coverage map and the pipeline is deleted — so a plan that leaves the
// package owning Rounds, the changed-range source, the synthetic module node, the schema-3
// types or the deletion order to the implementer hands nine lanes nine answers, and a
// deletion order nobody fixed is a red main between two of them. All asserted here.
const planN2 = "docs/plans/10-rounds-cutover.md"

type planTask struct {
	num  int
	body string
}

// planTasksOf splits a plan into its numbered `### Task N: ` sections; the last one ends
// where the next level-2 section begins.
func planTasksOf(t *testing.T, plan, src string) []planTask {
	t.Helper()
	locs := regexp.MustCompile(`(?m)^### Task (\d+): `).FindAllStringSubmatchIndex(src, -1)
	if len(locs) == 0 {
		t.Fatalf("%s has no `### Task N: ` sections", plan)
	}
	var out []planTask
	for i, loc := range locs {
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		} else if j := strings.Index(src[loc[1]:], "\n## "); j >= 0 {
			end = loc[1] + j
		}
		n, _ := strconv.Atoi(src[loc[2]:loc[3]])
		out = append(out, planTask{num: n, body: src[loc[0]:end]})
	}
	return out
}

// planSection is the body of the level-2 section `heading`, up to the next level-2 heading.
func planSection(t *testing.T, plan, src, heading string) string {
	t.Helper()
	i := strings.Index(src, "\n"+heading+"\n")
	if i < 0 {
		t.Fatalf("%s has no `%s` section", plan, heading)
	}
	block := src[i+len(heading)+2:]
	if j := strings.Index(block, "\n## "); j >= 0 {
		block = block[:j]
	}
	return block
}

// Issue #445 AC1: the header block of docs/plans/08-one-pipeline.md, naming the spec
// sections PRD #410 is built from.
func TestN2PlanOpensWithTheHeaderBlockNamingTheSpecSections(t *testing.T) {
	src := readRepoFile(t, planN2)
	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planN2)
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	}
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planN2, field)
		}
	}
	spec := regexp.MustCompile(`(?m)^\*\*Spec:\*\*.*$`).FindString(head)
	for _, want := range []string{"docs/specs/2026-10-07-node-graph.md", "§7", "§8", "§9", "§11", "§12", "#410"} {
		if !strings.Contains(spec, want) {
			t.Errorf("the **Spec:** line does not name %q: %q", want, spec)
		}
	}
}

// Issue #445 AC2: the global constraints, each stated where an implementer reads them
// before starting a task.
func TestN2PlanGlobalConstraintsCarryThePRDConstraints(t *testing.T) {
	block := planSection(t, planN2, readRepoFile(t, planN2), "## Global Constraints")
	for _, want := range []struct{ what, token string }{
		{"rtdd running no test", "runs no test"},
		{"rtdd running no toolchain", "no toolchain"},
		{"the scanner's single regex table", "internal/scan/patterns.go"},
		{"the git shell-out guard", "TestOnlyGitctxShellsOutToGit"},
		{"the git package", "internal/gitctx"},
		{"no new dependency", "gopkg.in/yaml.v3"},
		{"no cgo", "cgo"},
		{"the exit codes", "0 / 2 / 3"},
		{"no exit code reflecting a test result", "test result"},
		{"the PRD owning protocol, init migration and README", "#411"},
		{"the PRD owning rtdd-bench", "#412"},
		{"spec §12 settled", "§12"},
	} {
		if !strings.Contains(block, want.token) {
			t.Errorf("`## Global Constraints` does not state %s (no %q)", want.what, want.token)
		}
	}
}

// Issue #445 AC3, first half: every task names its files and interfaces and moves in
// checkboxed steps.
func TestN2PlanTasksCarryFilesInterfacesAndCheckboxedSteps(t *testing.T) {
	for _, task := range planTasksOf(t, planN2, readRepoFile(t, planN2)) {
		for _, want := range []string{"**Files:**", "**Interfaces:**", "- [ ] **Step 1:", "- [ ] **Step 2:"} {
			if !strings.Contains(task.body, want) {
				t.Errorf("Task %d has no %s", task.num, want)
			}
		}
	}
}

// Issue #445 AC3, second half: step 1 is literal, compilable failing test source and
// step 2 names the command and the failure it produces.
func TestN2PlanStepOneIsLiteralTestSourceAndStepTwoNamesTheFailure(t *testing.T) {
	for _, task := range planTasksOf(t, planN2, readRepoFile(t, planN2)) {
		one := stepBody(task.body, "1")
		sawTest := false
		for _, b := range fencedBlocks(one, "go") {
			if strings.Contains(b, "package ") && strings.Contains(b, "func Test") {
				sawTest = true
			}
		}
		if !sawTest {
			t.Errorf("Task %d step 1 has no ```go block with a `package` clause and a `func Test`", task.num)
		}
		two := stepBody(task.body, "2")
		cmds := fencedBlocks(two, "bash")
		if len(cmds) == 0 || !strings.Contains(cmds[0], "go test") {
			t.Errorf("Task %d step 2 has no ```bash block running `go test`", task.num)
		}
		if !strings.Contains(two, "Expected:") {
			t.Errorf("Task %d step 2 does not state the expected failure (no `Expected:`)", task.num)
		}
	}
}

// Issue #445 AC4: tasks are the sibling issues #446-#454 in order, and every PRD #410
// acceptance criterion 1-9 is discharged by at least one task, on a line that says so.
func TestN2PlanTasksAreTheSiblingIssuesAndDischargeEveryCriterion(t *testing.T) {
	src := readRepoFile(t, planN2)
	tasks := planTasksOf(t, planN2, src)
	if len(tasks) != 9 {
		t.Errorf("%d tasks, want 9 — one per sibling issue #446-#454", len(tasks))
	}
	issueLine := regexp.MustCompile(`(?m)^\*\*Issue:\*\* #(\d+)\b`)
	acRef := regexp.MustCompile(`\bAC(\d+)\b`)
	claimed := map[int]bool{}
	for i, task := range tasks {
		if task.num != i+1 {
			t.Errorf("task %d is numbered %d", i+1, task.num)
		}
		m := issueLine.FindStringSubmatch(task.body)
		if m == nil {
			t.Errorf("Task %d has no `**Issue:** #N` line", task.num)
		} else if got, want := m[1], strconv.Itoa(445+task.num); got != want {
			t.Errorf("Task %d is issue #%s, want #%s", task.num, got, want)
		}
		line := regexp.MustCompile(`(?m)^\*\*Discharges:\*\*.*$`).FindString(task.body)
		if line == "" {
			t.Errorf("Task %d has no **Discharges:** line", task.num)
			continue
		}
		for _, ac := range acRef.FindAllStringSubmatch(line, -1) {
			n, _ := strconv.Atoi(ac[1])
			claimed[n] = true
		}
	}
	for ac := 1; ac <= 9; ac++ {
		if !claimed[ac] {
			t.Errorf("PRD #410 AC%d is discharged by no task", ac)
		}
	}
	planSection(t, planN2, src, "## Acceptance-criterion map")
}

// Issue #445 "What to build": the decisions the siblings must not each make for
// themselves are made here, in so many words.
func TestN2PlanSettlesTheSharedDecisions(t *testing.T) {
	src := readRepoFile(t, planN2)
	for _, want := range []struct{ what, token string }{
		{"the package owning Rounds", "internal/rounds"},
		{"Rounds' exact signature", "func Rounds(g graph.Graph, changed map[string][]LineRange) Result"},
		{"Rounds' result type", "type Result struct"},
		{"where changed ranges come from", "gitctx.ChangedSet"},
		{"the --base flag", "--base <ref>"},
		{"the synthetic module node's name", "<module>"},
		{"the synthetic module node's kind", "KindModule"},
		{"the schema-3 document type", "type whichJSON struct"},
		{"the golden fixture repository", "cmd/rtdd/testdata/which-golden/"},
		{"the deletion order section", "## Deletion order"},
		{"a package left with no importer", "internal/coverage"},
		{"the doctor package", "internal/doctor"},
		{"the pytest fixture package", "internal/pytestfixture"},
		{"the adapters directory", "adapters/"},
		{"the shipped-adapter -list guard", "TestEveryShippedAdapterIsFullySpecified"},
		{"the pipeline -list guard", "^TestPipeline"},
		{"the installtest shipped-path list", "trackedDirs"},
		{"the no-reference guard test", "TestNoGoCodeReferencesTheCoveragePipeline"},
		{"the removed-command release", "v0.3.0"},
	} {
		if !strings.Contains(src, want.token) {
			t.Errorf("%s does not settle %s (no %q)", planN2, want.what, want.token)
		}
	}
}

// Issue #445 AC5: the plan ends by saying what it does not do.
func TestN2PlanClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planN2)
	heads := regexp.MustCompile(`(?m)^## .*$`).FindAllString(src, -1)
	if len(heads) == 0 || strings.TrimSpace(heads[len(heads)-1]) != "## What this plan deliberately leaves undone" {
		t.Errorf("%s must close with `## What this plan deliberately leaves undone` as its last section", planN2)
	}
}
