package contract

import (
	"regexp"
	"strings"
	"testing"
)

// The M6a plan document is a contract, not prose: issue #235 makes it the first child of
// the multi-language PRD, and every later M6a issue is checked against it. A plan whose
// tasks sketch their tests instead of writing them hands the implementer the design work
// the plan exists to have already done, so the shape is asserted here rather than
// reviewed by eye.
const planM6a = "docs/plans/06-m6a-adapter-contract-v2.md"

// specSection matches a reference to one of the four spec sections this plan discharges.
var specSection = regexp.MustCompile(`§(4\.2|4\.5|5|6)\b`)

// taskHeading matches a numbered task heading, e.g. "## Task 3 — `init` gating".
var taskHeading = regexp.MustCompile(`(?m)^## Task (\d+) — `)

// m6aTask is one task section: its number and the body between its heading and the next.
type m6aTask struct {
	num  string
	body string
}

// m6aTasks splits the plan into its numbered task sections.
func m6aTasks(t *testing.T, src string) []m6aTask {
	t.Helper()
	locs := taskHeading.FindAllStringSubmatchIndex(src, -1)
	if len(locs) == 0 {
		t.Fatalf("%s has no `## Task N — ` sections", planM6a)
	}
	var out []m6aTask
	for i, loc := range locs {
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, m6aTask{num: src[loc[2]:loc[3]], body: src[loc[0]:end]})
	}
	return out
}

// stepBody returns the slice of a task body from the "Step n:" marker to the next step
// marker (or the end of the task), which is the material that step is allowed to use.
func stepBody(body string, n string) string {
	start := strings.Index(body, "**Step "+n+":")
	if start < 0 {
		return ""
	}
	rest := body[start:]
	if next := regexp.MustCompile(`\*\*Step \d+:`).FindStringIndex(rest[1:]); next != nil {
		return rest[:next[0]+1]
	}
	return rest
}

// fencedBlocks returns the bodies of every fenced code block of the given language.
func fencedBlocks(src, lang string) []string {
	var out []string
	rest := src
	open := "```" + lang + "\n"
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			return out
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, "\n```")
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j+4:]
	}
}

func TestM6aPlanOpensWithTheHeaderBlock(t *testing.T) {
	src := readRepoFile(t, planM6a)

	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planM6a)
	}
	// The header block of docs/plans/02-m1b-python-adapter.md, plus the Spec line that
	// binds this plan to the sections it discharges.
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(src, field) {
			t.Errorf("%s is missing the %s header field", planM6a, field)
		}
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	}
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planM6a, field)
		}
	}
}

func TestM6aPlanHasGlobalConstraints(t *testing.T) {
	src := readRepoFile(t, planM6a)
	if !regexp.MustCompile(`(?m)^## Global Constraints\s*$`).MatchString(src) {
		t.Errorf("%s must carry a `## Global Constraints` block", planM6a)
	}
}

func TestM6aPlanTasksCarryFilesAndInterfaces(t *testing.T) {
	src := readRepoFile(t, planM6a)
	for _, task := range m6aTasks(t, src) {
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

// Step 1 must be the test itself. A description of a test ("assert that the loader
// rejects...") leaves the implementer to invent the assertion, and an invented assertion
// is the one the implementation already passes.
func TestM6aPlanStepOneIsLiteralFailingTestSource(t *testing.T) {
	src := readRepoFile(t, planM6a)
	for _, task := range m6aTasks(t, src) {
		one := stepBody(task.body, "1")
		if one == "" {
			t.Errorf("Task %s has no Step 1", task.num)
			continue
		}
		blocks := fencedBlocks(one, "go")
		if len(blocks) == 0 {
			t.Errorf("Task %s step 1 has no ```go block: step 1 must be literal test source", task.num)
			continue
		}
		sawTestFunc := false
		for _, b := range blocks {
			if strings.Contains(b, "func Test") && strings.Contains(b, "package ") {
				sawTestFunc = true
			}
		}
		if !sawTestFunc {
			t.Errorf("Task %s step 1 has no compilable `package ...` + `func Test...` source", task.num)
		}
	}
}

// Step 2 must name the command and the failure it produces, so "red" is a verdict the
// implementer reads rather than one they assume.
func TestM6aPlanStepTwoNamesCommandAndExpectedFailure(t *testing.T) {
	src := readRepoFile(t, planM6a)
	for _, task := range m6aTasks(t, src) {
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
		if !strings.Contains(cmds[0], "go test") {
			t.Errorf("Task %s step 2 command block does not run `go test`: %q", task.num, cmds[0])
		}
		if !strings.Contains(two, "Expected:") {
			t.Errorf("Task %s step 2 does not state the expected failure (no `Expected:` line)", task.num)
		}
	}
}

// Every task says which spec section it discharges, and the four are collectively covered.
func TestM6aPlanTasksDischargeTheFourSpecSections(t *testing.T) {
	src := readRepoFile(t, planM6a)
	covered := map[string]bool{}
	for _, task := range m6aTasks(t, src) {
		line := regexp.MustCompile(`(?m)^\*\*Discharges:\*\*.*$`).FindString(task.body)
		if line == "" {
			t.Errorf("Task %s has no **Discharges:** line naming its spec section", task.num)
			continue
		}
		found := specSection.FindAllStringSubmatch(line, -1)
		if len(found) == 0 {
			t.Errorf("Task %s **Discharges:** line names no spec section of §4.2/§4.5/§5/§6: %q", task.num, line)
		}
		for _, m := range found {
			covered[m[1]] = true
		}
	}
	for _, want := range []string{"4.2", "4.5", "5", "6"} {
		if !covered[want] {
			t.Errorf("no task discharges spec §%s", want)
		}
	}
}

func TestM6aPlanClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planM6a)
	const heading = "## What this plan deliberately leaves undone"
	i := strings.Index(src, heading)
	if i < 0 {
		t.Fatalf("%s must close with `%s`", planM6a, heading)
	}
	tail := src[i:]
	if strings.Contains(tail, "\n## ") {
		t.Errorf("`%s` must be the final section of %s", heading, planM6a)
	}
	for _, want := range []string{"TS", "junit-xml", "PRD"} {
		if !strings.Contains(tail, want) {
			t.Errorf("`%s` does not name %q as owned elsewhere", heading, want)
		}
	}
	if !regexp.MustCompile(`(?i)adapter (yaml|definitions)|yaml adapter|shipped adapter`).MatchString(tail) {
		t.Errorf("`%s` does not name the non-Python adapter YAML set as owned by another PRD", heading)
	}
}

// M6a changes the contract the Python adapter is already valid under; it does not change
// the Python adapter. A plan that edits adapters/python.yaml cannot promise spec §4.1's
// "byte-identical to today" regression requirement, so the plan must freeze the file and
// no task may list it.
func TestM6aPlanFreezesThePythonAdapter(t *testing.T) {
	src := readRepoFile(t, planM6a)

	if !strings.Contains(src, "byte-frozen") {
		t.Errorf("%s must state that adapters/python.yaml is byte-frozen for this PRD", planM6a)
	}
	for _, task := range m6aTasks(t, src) {
		files := regexp.MustCompile(`(?m)^\*\*Files:\*\*.*$`).FindString(task.body)
		if strings.Contains(files, "adapters/python.yaml") {
			t.Errorf("Task %s lists adapters/python.yaml in **Files:**, which is byte-frozen for this PRD", task.num)
		}
	}
}
