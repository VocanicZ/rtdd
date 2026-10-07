package contract

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The N4 plan document is a contract, not prose: issue #483 makes it the first child of
// PRD #412 and every sibling (#484-#486) is implemented against it. N4 moves rtdd-bench
// onto the node graph and publishes what it measures, so a plan that leaves the schema it
// consumes, the id expansion, the arm names or the results page's path to the implementer
// hands three lanes three answers — and a bench whose arms are named twice is a results
// page nobody can reproduce. All asserted here.
const planN4 = "docs/plans/12-bench-rounds.md"

// Issue #483 AC1: the header block of docs/plans/08-one-pipeline.md, naming the spec
// section PRD #412 measures.
func TestN4PlanOpensWithTheHeaderBlockNamingTheSpecSection(t *testing.T) {
	src := readRepoFile(t, planN4)
	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planN4)
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	}
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planN4, field)
		}
	}
	spec := regexp.MustCompile(`(?m)^\*\*Spec:\*\*.*$`).FindString(head)
	for _, want := range []string{"docs/specs/2026-10-07-node-graph.md", "§1", "#412"} {
		if !strings.Contains(spec, want) {
			t.Errorf("the **Spec:** line does not name %q: %q", want, spec)
		}
	}
}

// Issue #483 AC3: the global constraints repeat the PRD's — the pre-registration fields
// and tag are human decisions, and the driver cache is never touched.
func TestN4PlanGlobalConstraintsCarryThePRDConstraints(t *testing.T) {
	block := planSection(t, planN4, readRepoFile(t, planN4), "## Global Constraints")
	for _, want := range []struct{ what, token string }{
		{"the pre-registration file", "bench/PREREGISTRATION.md"},
		{"no signed_at", "`signed_at`"},
		{"no signed_by", "`signed_by`"},
		{"no stratified_recall_floor", "`stratified_recall_floor`"},
		{"no prereg tag", "`prereg-*`"},
		{"the driver cache", "~/.cache/rtdd-bench"},
		{"the cache never deleted", "deleted"},
		{"the cache never re-keyed", "re-keyed"},
		{"the cache never invalidated", "invalidated"},
		{"the gate", "scripts/ci-local.sh"},
		{"the bench suite", "uv run pytest"},
	} {
		if !strings.Contains(block, want.token) {
			t.Errorf("`## Global Constraints` does not state %s (no %q)", want.what, want.token)
		}
	}
}

// Issue #483 AC1, the task shape: every task names its files and interfaces and moves in
// checkboxed steps.
func TestN4PlanTasksCarryFilesInterfacesAndCheckboxedSteps(t *testing.T) {
	for _, task := range planTasksOf(t, planN4, readRepoFile(t, planN4)) {
		for _, want := range []string{"**Files:**", "**Interfaces:**", "- [ ] **Step 1:", "- [ ] **Step 2:"} {
			if !strings.Contains(task.body, want) {
				t.Errorf("Task %d has no %s", task.num, want)
			}
		}
	}
}

// Issue #483 AC1: step 1 is literal failing test source — the bench is Python, so a
// pytest module — and step 2 names the command and the failure it produces.
func TestN4PlanStepOneIsLiteralTestSourceAndStepTwoNamesTheFailure(t *testing.T) {
	for _, task := range planTasksOf(t, planN4, readRepoFile(t, planN4)) {
		one := stepBody(task.body, "1")
		sawTest := false
		for _, b := range fencedBlocks(one, "python") {
			if strings.Contains(b, "import ") && strings.Contains(b, "def test_") {
				sawTest = true
			}
		}
		if !sawTest {
			t.Errorf("Task %d step 1 has no ```python block with an import and a `def test_`", task.num)
		}
		two := stepBody(task.body, "2")
		cmds := fencedBlocks(two, "bash")
		if len(cmds) == 0 || !strings.Contains(cmds[0], "uv run pytest") {
			t.Errorf("Task %d step 2 has no ```bash block running `uv run pytest`", task.num)
		}
		if !strings.Contains(two, "Expected:") {
			t.Errorf("Task %d step 2 does not state the expected failure (no `Expected:`)", task.num)
		}
	}
}

// Issue #483 AC2: tasks are the sibling issues #484-#486 in order, and every PRD #412
// acceptance criterion 1-5 is discharged by at least one task, on a line that says so.
func TestN4PlanTasksAreTheSiblingIssuesAndDischargeEveryCriterion(t *testing.T) {
	src := readRepoFile(t, planN4)
	tasks := planTasksOf(t, planN4, src)
	if len(tasks) != 3 {
		t.Errorf("%d tasks, want 3 — one per sibling issue #484-#486", len(tasks))
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
		} else if got, want := m[1], strconv.Itoa(483+task.num); got != want {
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
	for ac := 1; ac <= 5; ac++ {
		if !claimed[ac] {
			t.Errorf("PRD #412 AC%d is discharged by no task", ac)
		}
	}
	planSection(t, planN4, src, "## Acceptance-criterion map")
}

// Issue #483 "What to build": the decisions the siblings must not each make for
// themselves are made here, in so many words.
func TestN4PlanSettlesTheSharedDecisions(t *testing.T) {
	src := readRepoFile(t, planN4)
	for _, want := range []struct{ what, token string }{
		{"the consumed schema", "SCHEMA_VERSION = 3"},
		{"the one graph build", "rtdd graph"},
		{"no seeding", "rtdd seed"},
		{"the id expansion function", "def expand_graph_ids("},
		{"the schema-3 result type", "class WhichResult"},
		{"the Round 1 arm's strategy id", `id = "rtdd"`},
		{"the Rounds 1+2 arm's strategy id", `"rtdd-r12"`},
		{"the baseline", "/tdd"},
		{"the baseline's strategy id", "`full`"},
		{"the results page", "docs/results/n4-bench-rounds.md"},
		{"the v0.2 fence going away", "bench/tests/v02binary.py"},
	} {
		if !strings.Contains(src, want.token) {
			t.Errorf("%s does not settle %s (no %q)", planN4, want.what, want.token)
		}
	}
}

// Issue #483 AC1 and AC4: the plan ends by saying what it does not do, and selection —
// PRD #410's — is the first thing it does not do.
func TestN4PlanClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planN4)
	heads := regexp.MustCompile(`(?m)^## .*$`).FindAllString(src, -1)
	if len(heads) == 0 || strings.TrimSpace(heads[len(heads)-1]) != "## What this plan deliberately leaves undone" {
		t.Errorf("%s must close with `## What this plan deliberately leaves undone` as its last section", planN4)
	}
	undone := planSection(t, planN4, src, "## What this plan deliberately leaves undone")
	for _, want := range []string{"selection", "#410"} {
		if !strings.Contains(undone, want) {
			t.Errorf("`## What this plan deliberately leaves undone` does not name %q: changing selection to improve a bench number is a non-goal", want)
		}
	}
}
