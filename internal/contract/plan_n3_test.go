package contract

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The N3 plan document is a contract, not prose: issue #477 makes it the first child of
// PRD #411 and every sibling (#478-#482) is implemented against it. N3 rewrites the text
// every agent reads and the command that installs it, so a plan that leaves the protocol
// wording, the forbidden-word list, the config rtdd init writes or the v0.2 migration to
// the implementer hands five lanes five answers — two of which edit the same file. All
// asserted here. planTasksOf and planSection live in plan_n2_test.go.
const planN3 = "docs/plans/11-skill-install.md"

// Issue #477 AC1: the header block of docs/plans/08-one-pipeline.md, naming the spec
// sections PRD #411 is built from.
func TestN3PlanOpensWithTheHeaderBlockNamingTheSpecSections(t *testing.T) {
	src := readRepoFile(t, planN3)
	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planN3)
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	}
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planN3, field)
		}
	}
	spec := regexp.MustCompile(`(?m)^\*\*Spec:\*\*.*$`).FindString(head)
	for _, want := range []string{"docs/specs/2026-10-07-node-graph.md", "§8", "§10", "§11", "#411"} {
		if !strings.Contains(spec, want) {
			t.Errorf("the **Spec:** line does not name %q: %q", want, spec)
		}
	}
}

// Issue #477 AC2: the global constraints, each stated where an implementer reads them
// before starting a task.
func TestN3PlanGlobalConstraintsCarryThePRDConstraints(t *testing.T) {
	block := planSection(t, planN3, readRepoFile(t, planN3), "## Global Constraints")
	for _, want := range []struct{ what, token string }{
		{"rtdd running no test", "runs no test"},
		{"front-ends naming no language", "no language"},
		{"front-ends naming no test framework", "no test framework"},
		{"graphify never run by rtdd", "never runs graphify"},
		{"the git shell-out guard", "TestOnlyGitctxShellsOutToGit"},
		{"the git package", "internal/gitctx"},
		{"no new dependency", "gopkg.in/yaml.v3"},
		{"no cgo", "cgo"},
		{"the PRD owning the graph", "#409"},
		{"the PRD owning rounds and commands", "#410"},
		{"the release being a human action", "#418"},
		{"no tag pushed", "v*"},
		{"the PRD owning rtdd-bench", "#412"},
		{"spec §12 settled", "§12"},
	} {
		if !strings.Contains(block, want.token) {
			t.Errorf("`## Global Constraints` does not state %s (no %q)", want.what, want.token)
		}
	}
}

// Issue #477 AC3, first half: every task names its files and interfaces and moves in
// checkboxed steps.
func TestN3PlanTasksCarryFilesInterfacesAndCheckboxedSteps(t *testing.T) {
	for _, task := range planTasksOf(t, planN3, readRepoFile(t, planN3)) {
		for _, want := range []string{"**Files:**", "**Interfaces:**", "- [ ] **Step 1:", "- [ ] **Step 2:"} {
			if !strings.Contains(task.body, want) {
				t.Errorf("Task %d has no %s", task.num, want)
			}
		}
	}
}

// Issue #477 AC3, second half: step 1 is literal, compilable failing test source and
// step 2 names the command and the failure it produces.
func TestN3PlanStepOneIsLiteralTestSourceAndStepTwoNamesTheFailure(t *testing.T) {
	for _, task := range planTasksOf(t, planN3, readRepoFile(t, planN3)) {
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

// Issue #477 AC4: tasks are the sibling issues #478-#482 in order, and every PRD #411
// acceptance criterion 1-8 is discharged by at least one task, on a line that says so.
func TestN3PlanTasksAreTheSiblingIssuesAndDischargeEveryCriterion(t *testing.T) {
	src := readRepoFile(t, planN3)
	tasks := planTasksOf(t, planN3, src)
	if len(tasks) != 5 {
		t.Errorf("%d tasks, want 5 — one per sibling issue #478-#482", len(tasks))
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
		} else if got, want := m[1], strconv.Itoa(477+task.num); got != want {
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
	for ac := 1; ac <= 8; ac++ {
		if !claimed[ac] {
			t.Errorf("PRD #411 AC%d is discharged by no task", ac)
		}
	}
	planSection(t, planN3, src, "## Acceptance-criterion map")
}

// Issue #477 "What to build": the decisions the siblings must not each make for
// themselves are made here, in so many words.
func TestN3PlanSettlesTheSharedDecisions(t *testing.T) {
	src := readRepoFile(t, planN3)
	for _, want := range []struct{ what, token string }{
		{"the five steps, verbatim", "1. Edit code (test first, per TDD)."},
		{"the process section", "<!-- rtdd:section id=process"},
		{"the graphify section", "<!-- rtdd:section id=graphify"},
		{"rtdd running no tests, as the front-ends say it", "rtdd runs no tests"},
		{"the project's own test command", "the project's own test command"},
		{"empty rounds", "no linked test"},
		{"graphify used when present", "used when `graphify-out/graph.json` exists"},
		{"graphify never trusted for changed files", "never trusted for changed files"},
		{"graphify never run by rtdd", "never run by rtdd"},
		{"how the dist/ is regenerated", "go run ./cmd/rtdd-gen render"},
		{"how the goldens are regenerated", "go test ./internal/protocol -run TestGolden -update"},
		{"the embedded protocol copy", "internal/install/protocol.md"},
		{"the dist/ guard test", "TestFrontEndsUseNoV02Vocabulary"},
		{"the guard's allowlist", "vocabularyAllowed"},
		{"the Cursor rule's globs", "MdcGlobs"},
		{"the config writer", "func DefaultConfig() string"},
		{"the config key test_files", "test_files"},
		{"the config key scan_exclude", "scan_exclude"},
		{"the config key graphify_path", "graphify_path"},
		{"the config key max_stale_ratio", "max_stale_ratio"},
		{"the fate of --force", "`--force` is removed"},
		{"the gitignore entry", ".rtdd/graph.json"},
		{"the migration planner", "func PlanMigration(root string) ([]Step, error)"},
		{"the gitattributes line", "merge=union"},
		{"the no-reference guard's allowlist", "v02StateAllowed"},
		{"the superseded-spec banner", "> **Superseded** by"},
		{"the v0.2 record markers in README", "<!-- rtdd:v0.2-record -->"},
		{"the README twin", "README.positive.md"},
	} {
		if !strings.Contains(src, want.token) {
			t.Errorf("%s does not settle %s (no %q)", planN3, want.what, want.token)
		}
	}
}

// Issue #477 AC5: the plan ends by saying what it does not do.
func TestN3PlanClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planN3)
	heads := regexp.MustCompile(`(?m)^## .*$`).FindAllString(src, -1)
	if len(heads) == 0 || strings.TrimSpace(heads[len(heads)-1]) != "## What this plan deliberately leaves undone" {
		t.Errorf("%s must close with `## What this plan deliberately leaves undone` as its last section", planN3)
	}
}
