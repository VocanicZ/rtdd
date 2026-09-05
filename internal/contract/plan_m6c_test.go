package contract

import (
	"regexp"
	"strings"
	"testing"
)

// The M6c plan document is a contract, not prose: issue #258 makes it the first child of
// PRD #231 and nothing else in that PRD starts before it merges, so every later M6c issue
// is checked against this file. A plan whose tasks sketch their tests instead of writing
// them hands the implementer the design work the plan exists to have already done, and a
// plan that leaves the PRD's under-determined decisions open hands them a guess. Both are
// asserted here rather than reviewed by eye.
const planM6c = "docs/plans/06-m6c-junit-report.md"

// m6cSpecSection matches a reference to one of the spec sections this plan discharges.
// §4.3 is the milestone's own section; §4.2 is the contract half M6a shipped and this
// plan extends by one validation rule.
var m6cSpecSection = regexp.MustCompile(`§(4\.2|4\.3)\b`)

// m6cTaskHeading matches a numbered task heading, e.g. "## Task 3 — `id_template`".
var m6cTaskHeading = regexp.MustCompile(`(?m)^#{2,3} Task (\d+) — `)

// m6cTask is one task section: its number and the body between its heading and the next.
type m6cTask struct {
	num  string
	body string
}

// m6cTasks splits the plan into its numbered task sections.
func m6cTasks(t *testing.T, src string) []m6cTask {
	t.Helper()
	locs := m6cTaskHeading.FindAllStringSubmatchIndex(src, -1)
	if len(locs) == 0 {
		t.Fatalf("%s has no `## Task N — ` sections", planM6c)
	}
	var out []m6cTask
	for i, loc := range locs {
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, m6cTask{num: src[loc[2]:loc[3]], body: src[loc[0]:end]})
	}
	return out
}

func TestM6cPlanOpensWithTheHeaderBlock(t *testing.T) {
	src := readRepoFile(t, planM6c)

	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planM6c)
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	}
	// The header block of docs/plans/02-m1b-python-adapter.md, plus the Spec line that
	// binds this plan to the section it discharges.
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planM6c, field)
		}
	}
}

func TestM6cPlanHasGlobalConstraints(t *testing.T) {
	src := readRepoFile(t, planM6c)
	if !regexp.MustCompile(`(?m)^## Global Constraints\s*$`).MatchString(src) {
		t.Errorf("%s must carry a `## Global Constraints` block", planM6c)
	}
}

func TestM6cPlanTasksCarryFilesAndInterfaces(t *testing.T) {
	src := readRepoFile(t, planM6c)
	for _, task := range m6cTasks(t, src) {
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

// Step 1 must be the test itself. A description of a test ("assert that the parser
// rejects...") leaves the implementer to invent the assertion, and an invented assertion
// is the one the implementation already passes.
func TestM6cPlanStepOneIsLiteralFailingTestSource(t *testing.T) {
	src := readRepoFile(t, planM6c)
	for _, task := range m6cTasks(t, src) {
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
func TestM6cPlanStepTwoNamesCommandAndExpectedFailure(t *testing.T) {
	src := readRepoFile(t, planM6c)
	for _, task := range m6cTasks(t, src) {
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

// Every task says which spec section it discharges, and §4.3 — the milestone's own
// section — is collectively covered.
func TestM6cPlanTasksDischargeSpecSection43(t *testing.T) {
	src := readRepoFile(t, planM6c)
	covered := map[string]bool{}
	for _, task := range m6cTasks(t, src) {
		line := regexp.MustCompile(`(?m)^\*\*Discharges:\*\*.*$`).FindString(task.body)
		if line == "" {
			t.Errorf("Task %s has no **Discharges:** line naming its spec section", task.num)
			continue
		}
		found := m6cSpecSection.FindAllStringSubmatch(line, -1)
		if len(found) == 0 {
			t.Errorf("Task %s **Discharges:** line names no spec section of §4.2/§4.3: %q", task.num, line)
		}
		for _, m := range found {
			covered[m[1]] = true
		}
	}
	if !covered["4.3"] {
		t.Errorf("no task discharges spec §4.3, which is the whole of this milestone")
	}
}

// M6a shipped the declarative half of §4.3 (#237, #238, #250). A plan that does not say
// so invites an implementer to re-add validation that already exists, or to edit the
// frozen Python adapter to "wire up" a contract that is already wired.
func TestM6cPlanNamesWhatM6aAlreadyShippedOfSpec43(t *testing.T) {
	src := readRepoFile(t, planM6c)
	for _, want := range []string{"M6a", "report_path", "id_template", "requires"} {
		if !strings.Contains(src, want) {
			t.Errorf("%s does not name %q as already shipped by the contract half of §4.3", planM6c, want)
		}
	}
	if !regexp.MustCompile(`(?i)already (shipped|landed)|out of scope here`).MatchString(src) {
		t.Errorf("%s must state which parts of §4.3 M6a already shipped and are out of scope here", planM6c)
	}
}

// The PRD leaves four things under-determined. Each is resolved once, with a reason, in
// the style of the M6a plan's own decisions section — otherwise every implementer decides
// them again, differently.
func TestM6cPlanResolvesTheFourUnderdeterminedDecisions(t *testing.T) {
	src := readRepoFile(t, planM6c)
	i := regexp.MustCompile(`(?mi)^#{3,4} The four under-determined decisions`).FindStringIndex(src)
	if i == nil {
		t.Fatalf("%s must carry a `### The four under-determined decisions, resolved here` section", planM6c)
	}
	decisions := src[i[0]:]
	if j := strings.Index(decisions, "\n## "); j > 0 {
		decisions = decisions[:j]
	}
	for _, want := range []struct{ what, token string }{
		{"the placeholder spelling", "{classname}"},
		{"the PRD's own spelling, named as the one that loses", "{class}"},
		{"what {file} expands to when the runner omits it", "{file}"},
		{"whether report_path names one file or a set", "report_path"},
		{"how chunked runs merge", "chunk"},
	} {
		if !strings.Contains(decisions, want.token) {
			t.Errorf("the decisions section does not resolve %s (no %q)", want.what, want.token)
		}
	}
}

// Fixtures are the load-bearing evidence of this milestone: hand-written XML proves only
// that the parser reads XML someone wrote to make it pass. The plan must name the six
// runners and record how each capture was produced, so a later reader can regenerate it.
func TestM6cPlanNamesTheSixFixtureRunnersAndTheirCommands(t *testing.T) {
	src := readRepoFile(t, planM6c)
	for _, runner := range []string{"Vitest", "Jest", "go-junit-report", "Surefire", "RSpec", "PHPUnit"} {
		if !strings.Contains(src, runner) {
			t.Errorf("%s does not name the %s fixture", planM6c, runner)
		}
	}
	// The flag or artefact path that produces each capture, per spec §4.3's table.
	for _, cmd := range []string{
		"--reporter=junit",        // Vitest
		"jest-junit",              // Jest
		"go-junit-report",         // Go
		"target/surefire-reports", // Maven Surefire
		"RspecJunitFormatter",     // RSpec
		"--log-junit",             // PHPUnit
	} {
		if !strings.Contains(src, cmd) {
			t.Errorf("%s records no command producing the fixture that needs %q", planM6c, cmd)
		}
	}
}

// The plan's task set must cover every acceptance criterion of PRD #231. Naming them is
// how a reviewer checks that without re-deriving the mapping.
func TestM6cPlanCoversEveryAcceptanceCriterionOfThePRD(t *testing.T) {
	src := readRepoFile(t, planM6c)
	for i := 1; i <= 9; i++ {
		want := regexp.MustCompile(`AC` + itoa(i) + `\b`)
		if !want.MatchString(src) {
			t.Errorf("%s never names PRD #231 AC%d", planM6c, i)
		}
	}
}

// itoa keeps the AC loop readable without pulling strconv in for one call.
func itoa(i int) string { return string(rune('0' + i)) }

// The existing pytest path is not a refactoring opportunity. The two parsers sit side by
// side and are chosen by the adapter's report: field; adapters/python.yaml stays
// byte-frozen, so internal/contract's digest test needs no edit.
func TestM6cPlanLeavesThePytestReportlogPathUntouched(t *testing.T) {
	src := readRepoFile(t, planM6c)
	if !strings.Contains(src, "pytest-reportlog") {
		t.Errorf("%s must state that internal/report's pytest-reportlog path is untouched", planM6c)
	}
	if !strings.Contains(src, "byte-frozen") {
		t.Errorf("%s must state that adapters/python.yaml stays byte-frozen", planM6c)
	}
	for _, task := range m6cTasks(t, src) {
		files := regexp.MustCompile(`(?m)^\*\*Files:\*\*.*$`).FindString(task.body)
		if strings.Contains(files, "adapters/python.yaml") {
			t.Errorf("Task %s lists adapters/python.yaml in **Files:**, which is byte-frozen", task.num)
		}
		if strings.Contains(files, "internal/report/reportlog.go") {
			t.Errorf("Task %s lists internal/report/reportlog.go in **Files:**, which this PRD does not touch", task.num)
		}
	}
}

func TestM6cPlanClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planM6c)
	const heading = "## What this plan deliberately leaves undone"
	i := strings.Index(src, heading)
	if i < 0 {
		t.Fatalf("%s must close with `%s`", planM6c, heading)
	}
	tail := src[i:]
	if strings.Contains(tail, "\n## ") {
		t.Errorf("`%s` must be the final section of %s", heading, planM6c)
	}
	// The three siblings that own what this plan may not absorb, named by number so the
	// boundary is checkable rather than rhetorical.
	for _, want := range []string{"#230", "#232", "§11", "A5"} {
		if !strings.Contains(tail, want) {
			t.Errorf("`%s` does not name %q as owned elsewhere", heading, want)
		}
	}
	if !strings.Contains(tail, "TS") {
		t.Errorf("`%s` does not name the TS tier's selection logic as a sibling PRD's", heading)
	}
}
