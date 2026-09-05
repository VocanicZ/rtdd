package contract

import (
	"regexp"
	"strings"
	"testing"
)

// The M6b plan document is a contract, not prose: issue #253 makes it the first child of
// PRD #230 and nothing else in that PRD starts before it merges, so every sibling issue
// is checked against it. The same shape assertions the M6a plan carries apply here, plus
// the three things this plan had to DECIDE — a plan that leaves them open hands the
// under-determination back to the implementer it was written to protect.
const planM6b = "docs/plans/06-m6b-static-tier.md"

// specSectionM6b matches a reference to one of the two spec sections this plan discharges.
var specSectionM6b = regexp.MustCompile(`§(4\.1|2)\b`)

// taskHeadingM6b matches a numbered task heading, e.g. "### Task 3 — test_for resolution".
var taskHeadingM6b = regexp.MustCompile(`(?m)^### Task (\d+) — `)

type m6bTask struct {
	num  string
	body string
}

func m6bTasks(t *testing.T, src string) []m6bTask {
	t.Helper()
	locs := taskHeadingM6b.FindAllStringSubmatchIndex(src, -1)
	if len(locs) == 0 {
		t.Fatalf("%s has no `### Task N — ` sections", planM6b)
	}
	var out []m6bTask
	for i, loc := range locs {
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, m6bTask{num: src[loc[2]:loc[3]], body: src[loc[0]:end]})
	}
	return out
}

func TestM6bPlanOpensWithTheHeaderBlock(t *testing.T) {
	src := readRepoFile(t, planM6b)

	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planM6b)
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	}
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planM6b, field)
		}
	}
}

func TestM6bPlanHasGlobalConstraints(t *testing.T) {
	src := readRepoFile(t, planM6b)
	if !regexp.MustCompile(`(?m)^## Global Constraints\s*$`).MatchString(src) {
		t.Errorf("%s must carry a `## Global Constraints` block", planM6b)
	}
}

func TestM6bPlanTasksCarryFilesAndInterfaces(t *testing.T) {
	src := readRepoFile(t, planM6b)
	for _, task := range m6bTasks(t, src) {
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

// Step 1 must be the test itself. A description of a test leaves the implementer to
// invent the assertion, and an invented assertion is the one the implementation already
// passes.
func TestM6bPlanStepOneIsLiteralFailingTestSource(t *testing.T) {
	src := readRepoFile(t, planM6b)
	for _, task := range m6bTasks(t, src) {
		one := stepBody(task.body, "1")
		if one == "" {
			t.Errorf("Task %s has no Step 1", task.num)
			continue
		}
		blocks := fencedBlocks(one, "go")
		sawTestFunc := false
		for _, b := range blocks {
			if strings.Contains(b, "func Test") {
				sawTestFunc = true
			}
		}
		if !sawTestFunc {
			t.Errorf("Task %s step 1 has no literal `func Test...` source in a ```go block", task.num)
		}
	}
}

func TestM6bPlanStepTwoNamesCommandAndExpectedFailure(t *testing.T) {
	src := readRepoFile(t, planM6b)
	for _, task := range m6bTasks(t, src) {
		two := stepBody(task.body, "2")
		if two == "" {
			t.Errorf("Task %s has no Step 2", task.num)
			continue
		}
		if !regexp.MustCompile(`(?m)^Run: `).MatchString(two) && len(fencedBlocks(two, "bash")) == 0 {
			t.Errorf("Task %s step 2 names no command to run", task.num)
		}
		if !strings.Contains(two, "go test") {
			t.Errorf("Task %s step 2 does not name a `go test` command", task.num)
		}
		if !strings.Contains(two, "Expected:") {
			t.Errorf("Task %s step 2 does not state the expected failure (no `Expected:` line)", task.num)
		}
	}
}

// Every task says which spec section it discharges, and §4.1 and §2 are both covered.
func TestM6bPlanTasksDischargeTheSpecSections(t *testing.T) {
	src := readRepoFile(t, planM6b)
	covered := map[string]bool{}
	for _, task := range m6bTasks(t, src) {
		line := regexp.MustCompile(`(?m)^\*\*Discharges:\*\*.*$`).FindString(task.body)
		if line == "" {
			t.Errorf("Task %s has no **Discharges:** line naming its spec section", task.num)
			continue
		}
		for _, m := range specSectionM6b.FindAllStringSubmatch(line, -1) {
			covered[m[1]] = true
		}
	}
	for _, want := range []string{"4.1", "2"} {
		if !covered[want] {
			t.Errorf("no task discharges spec §%s", want)
		}
	}
}

// The byte-identical-selection requirement is a property of the whole milestone: every
// task can break it, so stating it inside one task's section would let the other nine
// read as unconstrained.
func TestM6bPlanMakesByteIdenticalSelectionAGlobalConstraint(t *testing.T) {
	src := readRepoFile(t, planM6b)

	start := strings.Index(src, "## Global Constraints")
	if start < 0 {
		t.Fatalf("%s has no `## Global Constraints` block", planM6b)
	}
	end := len(src)
	if i := taskHeadingM6b.FindStringIndex(src); i != nil {
		end = i[0]
	}
	constraints := src[start:end]
	if !strings.Contains(constraints, "byte-identical") {
		t.Errorf("%s does not state the byte-identical-selection requirement in its "+
			"`## Global Constraints` block", planM6b)
	}
	if !regexp.MustCompile(`(?i)seed`).MatchString(constraints) {
		t.Errorf("the byte-identical constraint does not say it applies to SEEDED repositories")
	}
}

// The three decisions PRD #230 leaves under-determined, each resolved with a reason and
// pinned to a task that tests it.
func TestM6bPlanResolvesTheThreeUnderDeterminedDecisions(t *testing.T) {
	src := readRepoFile(t, planM6b)

	if !regexp.MustCompile(`(?i)under-determined decisions`).MatchString(src) {
		t.Errorf("%s has no section resolving the under-determined decisions", planM6b)
	}
	for _, want := range []struct{ name, pattern string }{
		{"the TS candidate set (does path proximity admit?)", `(?i)path proximity`},
		{"what a Fidelity() == none adapter selects", `(?i)fidelity.{0,20}none`},
		{"where the TS gate reads from", `(?i)selection.{0,30}static.{0,80}Len\(\)|mapCannotAnswer`},
		{"the new Inputs fields, injected because Select is pure", `ImportDistance`},
		{"test_for resolution asks whether a file exists", `Exists`},
	} {
		if !regexp.MustCompile(want.pattern).MatchString(src) {
			t.Errorf("%s does not resolve %s (no match for %q)", planM6b, want.name, want.pattern)
		}
	}
}

// PRD #230's nine acceptance criteria are the plan's scope. A criterion no task covers is
// a criterion the sibling issues will not implement.
func TestM6bPlanCoversEveryPRDAcceptanceCriterion(t *testing.T) {
	src := readRepoFile(t, planM6b)
	for _, want := range []struct{ criterion, pattern string }{
		{"1: TierTS between T1 and T2, documented order updated", `TierT1 < TierTS < TierT2`},
		{"2: TierTS.String() is \"TS\"", `TierTS\.String\(\)`},
		{"3: byte-identical golden regression guard", `(?i)golden`},
		{"4: TS only when the coverage relation cannot answer", `(?i)unseeded|cannot answer`},
		{"5: three-level ranking", `(?i)import distance`},
		{"6: test_for templates expand {dir} and {name} in order", `\{dir\}`},
		{"7: import ranking skipped, not fatal, without importscan", `importscan`},
		{"8: the reason never says \"recorded coverage\"", `recorded coverage`},
		{"9: static adapters are not told to seed", `run rtdd seed`},
		{"10: the CI gate", `scripts/ci-local\.sh`},
	} {
		if !regexp.MustCompile(want.pattern).MatchString(src) {
			t.Errorf("%s does not cover PRD #230 criterion %s (no match for %q)",
				planM6b, want.criterion, want.pattern)
		}
	}
}

func TestM6bPlanClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planM6b)
	const heading = "## What this plan deliberately leaves undone"
	i := strings.Index(src, heading)
	if i < 0 {
		t.Fatalf("%s must close with `%s`", planM6b, heading)
	}
	tail := src[i:]
	if strings.Contains(tail, "\n## ") {
		t.Errorf("`%s` must be the final section of %s", heading, planM6b)
	}
	// The four things the sibling and later PRDs own.
	for _, want := range []struct{ name, pattern string }{
		{"the runner", `(?i)runner`},
		{"the junit-xml parser", `junit-xml`},
		{"the shipped non-Python adapters", `(?i)shipped .{0,20}adapter|adapter (yaml|set)`},
		{"the measured evidence table", `(?i)evidence`},
		{"the sibling PRDs that own them", `(?i)PRD`},
	} {
		if !regexp.MustCompile(want.pattern).MatchString(tail) {
			t.Errorf("`%s` does not name %s as owned elsewhere", heading, want.name)
		}
	}
}

// adapters/python.yaml is byte-frozen for the whole M6 chain: it is the only adapter that
// has ever produced a map, and not touching it is the cheapest way to keep spec §4.1's
// byte-identical promise.
func TestM6bPlanDoesNotTouchThePythonAdapter(t *testing.T) {
	src := readRepoFile(t, planM6b)
	for _, task := range m6bTasks(t, src) {
		files := regexp.MustCompile(`(?m)^\*\*Files:\*\*.*$`).FindString(task.body)
		if strings.Contains(files, "adapters/python.yaml") {
			t.Errorf("Task %s lists adapters/python.yaml in **Files:**, which is byte-frozen", task.num)
		}
	}
}
