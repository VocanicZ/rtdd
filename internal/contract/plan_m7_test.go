package contract

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The M7 plan document is a contract, not prose: issue #370 makes it the first child of
// PRD #368 and nothing else in that milestone starts before it merges, so every later M7
// issue is checked against this file. A plan whose tasks sketch their tests instead of
// writing them hands the implementer the design work the plan exists to have already
// done, and a plan that leaves the PRD's irreversible actions unnamed hands them a
// repository-visibility flip. Both are asserted here rather than reviewed by eye.
const planM7 = "docs/plans/07-m7-release-readiness.md"

// m7TaskHeading matches a numbered task heading, e.g. "## Task 3 — `docs/RELEASING.md`".
var m7TaskHeading = regexp.MustCompile(`(?m)^#{2,3} Task (\d+) — `)

// m7ACRef matches an acceptance-criterion reference on a **Discharges:** line, e.g. "AC7".
var m7ACRef = regexp.MustCompile(`\bAC(\d+)\b`)

// m7Task is one task section: its number and the body between its heading and the next.
type m7Task struct {
	num  string
	body string
}

// m7Tasks splits the plan into its numbered task sections.
func m7Tasks(t *testing.T, src string) []m7Task {
	t.Helper()
	locs := m7TaskHeading.FindAllStringSubmatchIndex(src, -1)
	if len(locs) == 0 {
		t.Fatalf("%s has no `## Task N — ` sections", planM7)
	}
	var out []m7Task
	for i, loc := range locs {
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, m7Task{num: src[loc[2]:loc[3]], body: src[loc[0]:end]})
	}
	return out
}

// Issue #370 AC1: the same top-level section structure as docs/plans/04-m3-replay-benchmark.md.
func TestM7PlanOpensWithTheHeaderBlockAndClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planM7)

	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planM7)
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	} else {
		t.Errorf("%s must carry a `## Global Constraints` block", planM7)
	}
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planM7, field)
		}
	}
	if !regexp.MustCompile(`(?m)^## What this plan deliberately leaves undone\s*$`).MatchString(src) {
		t.Errorf("%s must close with a `## What this plan deliberately leaves undone` section", planM7)
	}
}

// Issue #370 AC2, first half: every task names its files and its interfaces and moves in
// checkboxed steps.
func TestM7PlanTasksCarryFilesInterfacesAndCheckboxedSteps(t *testing.T) {
	src := readRepoFile(t, planM7)
	for _, task := range m7Tasks(t, src) {
		if !strings.Contains(task.body, "**Files:**") {
			t.Errorf("Task %s has no **Files:** line", task.num)
		}
		if !strings.Contains(task.body, "**Interfaces:**") {
			t.Errorf("Task %s has no **Interfaces:** line", task.num)
		}
		if !strings.Contains(task.body, "- [ ] ") {
			t.Errorf("Task %s has no checkboxed steps", task.num)
		}
	}
}

// Issue #370 AC2, second half: step 1 is the test itself. A description of a test leaves
// the implementer to invent the assertion, and an invented assertion is the one the
// implementation already passes.
func TestM7PlanStepOneIsLiteralFailingTestSource(t *testing.T) {
	src := readRepoFile(t, planM7)
	for _, task := range m7Tasks(t, src) {
		one := stepBody(task.body, "1")
		if one == "" {
			t.Errorf("Task %s has no Step 1", task.num)
			continue
		}
		sawTestFunc := false
		for _, b := range fencedBlocks(one, "go") {
			if strings.Contains(b, "func Test") && strings.Contains(b, "package ") {
				sawTestFunc = true
			}
		}
		if !sawTestFunc {
			t.Errorf("Task %s step 1 has no compilable `package ...` + `func Test...` source: step 1 must be literal failing test source", task.num)
		}
	}
}

// Issue #370 AC2, third half: step 2 names the command and the failure it produces, so
// "red" is a verdict the implementer reads rather than one they assume.
func TestM7PlanStepTwoNamesCommandAndExpectedFailure(t *testing.T) {
	src := readRepoFile(t, planM7)
	for _, task := range m7Tasks(t, src) {
		two := stepBody(task.body, "2")
		if two == "" {
			t.Errorf("Task %s has no Step 2", task.num)
			continue
		}
		cmds := fencedBlocks(two, "bash")
		if len(cmds) == 0 {
			t.Errorf("Task %s step 2 has no ```bash block naming the command to run", task.num)
			continue
		}
		if !strings.Contains(cmds[0], "go test") && !strings.Contains(cmds[0], "scripts/ci-local.sh") {
			t.Errorf("Task %s step 2 command block runs neither `go test` nor `scripts/ci-local.sh`: %q", task.num, cmds[0])
		}
		if !strings.Contains(two, "Expected:") {
			t.Errorf("Task %s step 2 does not state the expected failure (no `Expected:` line)", task.num)
		}
	}
}

// Issue #370 AC3: acceptance criteria 3-11 of PRD #368 are each claimed by exactly one
// task, and the claim is written down per task rather than inferred from the prose.
func TestM7PlanClaimsEveryPRDAcceptanceCriterionExactlyOnce(t *testing.T) {
	src := readRepoFile(t, planM7)
	claimedBy := map[int][]string{}
	for _, task := range m7Tasks(t, src) {
		line := regexp.MustCompile(`(?m)^\*\*Discharges:\*\*.*(?:\n(?:  |\t).*)*`).FindString(task.body)
		if line == "" {
			t.Errorf("Task %s has no **Discharges:** line naming the PRD acceptance criteria it covers", task.num)
			continue
		}
		found := m7ACRef.FindAllStringSubmatch(line, -1)
		if len(found) == 0 {
			t.Errorf("Task %s **Discharges:** line names no `ACn` of PRD #368: %q", task.num, line)
		}
		for _, m := range found {
			n := 0
			for _, c := range m[1] {
				n = n*10 + int(c-'0')
			}
			claimedBy[n] = append(claimedBy[n], task.num)
		}
	}
	for ac := 3; ac <= 11; ac++ {
		switch len(claimedBy[ac]) {
		case 0:
			t.Errorf("PRD #368 AC%d is claimed by no task", ac)
		case 1:
		default:
			sort.Strings(claimedBy[ac])
			t.Errorf("PRD #368 AC%d is claimed by %d tasks (%s); exactly one task must own it", ac, len(claimedBy[ac]), strings.Join(claimedBy[ac], ", "))
		}
	}
	for _, ac := range []int{1, 2} {
		if len(claimedBy[ac]) != 0 {
			t.Errorf("PRD #368 AC%d is re-filed by Task %s; it belongs to issue #369 and must not be a task here", ac, strings.Join(claimedBy[ac], ", "))
		}
	}
	if !strings.Contains(src, "#369") {
		t.Errorf("%s must name issue #369 as the already-filed owner of AC1 and AC2", planM7)
	}
}

// Issue #370 AC4: the PRD's global constraint arrives verbatim in the plan's own, naming
// all three irreversible actions plus the bench cache. A constraint that is not written
// down is one an implementer discovers by performing it.
func TestM7PlanGlobalConstraintsNameEveryIrreversibleAction(t *testing.T) {
	src := readRepoFile(t, planM7)
	i := strings.Index(src, "## Global Constraints")
	if i < 0 {
		t.Fatalf("%s has no `## Global Constraints` section", planM7)
	}
	block := src[i:]
	if j := m7TaskHeading.FindStringIndex(block); j != nil {
		block = block[:j[0]]
	}
	for _, want := range []struct{ what, token string }{
		{"the repository visibility decision", "#10"},
		{"the visibility flip itself", "visibility"},
		{"the release tag", "v*"},
		{"publishing a release or a draft", "draft"},
		{"the pre-registration signature decision", "#8"},
		{"the signature fields", "signed_at"},
		{"the second signature field", "signed_by"},
		{"the pre-registered floor", "stratified_recall_floor"},
		{"the pre-registration file", "bench/PREREGISTRATION.md"},
		{"the pre-registration tag", "prereg-m4"},
		{"the bench cache decision", "#218"},
		{"the bench cache itself", "~/.cache/rtdd-bench"},
		{"the cache key", "cache.key()"},
	} {
		if !strings.Contains(block, want.token) {
			t.Errorf("`## Global Constraints` does not name %s (no %q)", want.what, want.token)
		}
	}
}

// Issue #370 AC5: the two facts the later tasks turn on are stated, not assumed. A plan
// that does not say the workflow is disabled reads as if CI has been green all along.
func TestM7PlanStatesTheTwoFactsItsTasksTurnOn(t *testing.T) {
	src := readRepoFile(t, planM7)
	for _, want := range []struct{ what, token string }{
		{"that .github/workflows/ci.yml is currently disabled", "disabled_manually"},
		{"the workflow it is talking about", ".github/workflows/ci.yml"},
		{"that .goreleaser.yaml drafts its releases", "draft: true"},
		{"the release config it is talking about", ".goreleaser.yaml"},
	} {
		if !strings.Contains(src, want.token) {
			t.Errorf("%s does not state %s (no %q)", planM7, want.what, want.token)
		}
	}
}
