package contract

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The N1 plan document is a contract, not prose: issue #419 makes it the first child of
// PRD #409 and every sibling (#420-#429) is implemented against it. A plan that leaves the
// node ID, the package boundaries, the git functions or the cache's on-disk shape to the
// implementer hands ten parallel lanes ten different answers; a plan whose step 1 sketches
// a test instead of writing it hands them the assertion too. Both are asserted here.
const planN1 = "docs/plans/09-node-graph-core.md"

// n1TaskHeading matches a numbered task heading, e.g. "### Task 3: Ruby, Lua and Bash".
var n1TaskHeading = regexp.MustCompile(`(?m)^### Task (\d+): `)

type n1Task struct {
	num  int
	body string
}

// n1Tasks splits the plan into its numbered task sections; the last one ends where the
// next level-2 section begins.
func n1Tasks(t *testing.T, src string) []n1Task {
	t.Helper()
	locs := n1TaskHeading.FindAllStringSubmatchIndex(src, -1)
	if len(locs) == 0 {
		t.Fatalf("%s has no `### Task N: ` sections", planN1)
	}
	var out []n1Task
	for i, loc := range locs {
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		} else if j := strings.Index(src[loc[1]:], "\n## "); j >= 0 {
			end = loc[1] + j
		}
		n, _ := strconv.Atoi(src[loc[2]:loc[3]])
		out = append(out, n1Task{num: n, body: src[loc[0]:end]})
	}
	return out
}

// globalConstraintsN1 is the `## Global Constraints` block, up to the next level-2 heading.
func globalConstraintsN1(t *testing.T, src string) string {
	t.Helper()
	i := strings.Index(src, "## Global Constraints")
	if i < 0 {
		t.Fatalf("%s has no `## Global Constraints` block", planN1)
	}
	block := src[i+len("## Global Constraints"):]
	if j := strings.Index(block, "\n## "); j >= 0 {
		block = block[:j]
	}
	return block
}

// Issue #419 AC1: the header block of docs/plans/08-one-pipeline.md, naming the spec
// sections PRD #409 is built from.
func TestN1PlanOpensWithTheHeaderBlockNamingTheSpecSections(t *testing.T) {
	src := readRepoFile(t, planN1)
	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planN1)
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	}
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planN1, field)
		}
	}
	spec := regexp.MustCompile(`(?m)^\*\*Spec:\*\*.*$`).FindString(head)
	for _, want := range []string{"docs/specs/2026-10-07-node-graph.md", "§3", "§4", "§5", "§6", "§8", "§12", "#409"} {
		if !strings.Contains(spec, want) {
			t.Errorf("the **Spec:** line does not name %q: %q", want, spec)
		}
	}
}

// Issue #419 AC2: PRD #409's global constraints, each stated where an implementer reads
// them before starting a task.
func TestN1PlanGlobalConstraintsCarryThePRDConstraints(t *testing.T) {
	block := globalConstraintsN1(t, readRepoFile(t, planN1))
	for _, want := range []struct{ what, token string }{
		{"the single regex table", "§4.2"},
		{"the one Go file holding it", "internal/scan/patterns.go"},
		{"the git shell-out guard", "TestOnlyGitctxShellsOutToGit"},
		{"the git package", "internal/gitctx"},
		{"no tree-sitter", "tree-sitter"},
		{"no cgo", "cgo"},
		{"no new dependency", "gopkg.in/yaml.v3"},
		{"rtdd never invoking graphify", "never invokes graphify"},
		{"the PRD that owns removal", "#410"},
	} {
		if !strings.Contains(block, want.token) {
			t.Errorf("`## Global Constraints` does not state %s (no %q)", want.what, want.token)
		}
	}
}

// Issue #419 AC3, first half: every task names its files and interfaces and moves in
// checkboxed steps.
func TestN1PlanTasksCarryFilesInterfacesAndCheckboxedSteps(t *testing.T) {
	for _, task := range n1Tasks(t, readRepoFile(t, planN1)) {
		for _, want := range []string{"**Files:**", "**Interfaces:**", "- [ ] **Step 1:", "- [ ] **Step 2:"} {
			if !strings.Contains(task.body, want) {
				t.Errorf("Task %d has no %s", task.num, want)
			}
		}
	}
}

// Issue #419 AC3, second half: step 1 is literal, compilable failing test source and
// step 2 names the command and the failure it produces.
func TestN1PlanStepOneIsLiteralTestSourceAndStepTwoNamesTheFailure(t *testing.T) {
	for _, task := range n1Tasks(t, readRepoFile(t, planN1)) {
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

// Issue #419 AC4: tasks are the sibling issues #420-#429 in order, and every PRD #409
// acceptance criterion 1-11 is discharged by at least one task, on a line that says so.
func TestN1PlanTasksAreTheSiblingIssuesAndDischargeEveryCriterion(t *testing.T) {
	tasks := n1Tasks(t, readRepoFile(t, planN1))
	if len(tasks) != 10 {
		t.Errorf("%d tasks, want 10 — one per sibling issue #420-#429", len(tasks))
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
		} else if got, want := m[1], strconv.Itoa(419+task.num); got != want {
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
	for ac := 1; ac <= 11; ac++ {
		if !claimed[ac] {
			t.Errorf("PRD #409 AC%d is discharged by no task", ac)
		}
	}
}

// Issue #419 "What to build": the decisions the siblings must not each make for
// themselves are made here, in so many words.
func TestN1PlanSettlesTheSharedDecisions(t *testing.T) {
	src := readRepoFile(t, planN1)
	for _, want := range []struct{ what, token string }{
		{"the node ID section", "## Node.ID"},
		{"the ID separator", "file::"},
		{"the duplicate-ID suffix", "@<start>"},
		{"the model package", "internal/graph"},
		{"the scanner package", "internal/scan"},
		{"the graphify loader package", "internal/graphify"},
		{"the build/cache/overlay package", "internal/graphbuild"},
		{"the blob-id git function", "BlobIDs"},
		{"the diff-since git function", "DiffNamesSince"},
		{"the commit-known git function", "CommitKnown"},
		{"where untracked files come from", "ChangedSet"},
		{"the config loader", "LoadConfig"},
		{"the scan_exclude key", "scan_exclude"},
		{"the test_files key", "test_files"},
		{"the graphify_path key", "graphify_path"},
		{"the max_stale_ratio key", "max_stale_ratio"},
		{"the cache file", ".rtdd/graph.json"},
		{"the cache's per-file blob ids", "rtdd_files"},
		{"the cache's version key", "rtdd_cache"},
		{"the cache's commit", "built_at_commit"},
	} {
		if !strings.Contains(src, want.token) {
			t.Errorf("%s does not settle %s (no %q)", planN1, want.what, want.token)
		}
	}
}

// Issue #419 AC5: the plan ends by saying what it does not do.
func TestN1PlanClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planN1)
	heads := regexp.MustCompile(`(?m)^## .*$`).FindAllString(src, -1)
	if len(heads) == 0 || strings.TrimSpace(heads[len(heads)-1]) != "## What this plan deliberately leaves undone" {
		t.Errorf("%s must close with `## What this plan deliberately leaves undone` as its last section", planN1)
	}
}
