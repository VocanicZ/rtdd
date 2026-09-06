package contract

import (
	"regexp"
	"strings"
	"testing"
)

// The M6d plan document is a contract, not prose: issue #308 makes it the first child of
// PRD #232 and nothing else in that PRD starts before it merges, so every later M6d issue
// is checked against this file. A plan whose tasks sketch their tests instead of writing
// them hands the implementer the design work the plan exists to have already done, and a
// plan that leaves the PRD's under-determined decisions open hands them a guess. Both are
// asserted here rather than reviewed by eye.
const planM6d = "docs/plans/06-m6d-shipped-adapters.md"

// m6dSpecSection matches a reference to one of the spec sections this plan discharges.
// §4.4 is polyglot detection and §8 is the shipped adapter set; those two are the
// milestone. §4.2 and §4.3 appear on the tasks that extend the contract the siblings shipped.
var m6dSpecSection = regexp.MustCompile(`§(4\.2|4\.3|4\.4|8)\b`)

// m6dTaskHeading matches a numbered task heading, e.g. "## Task 3 — `report_cmd`".
var m6dTaskHeading = regexp.MustCompile(`(?m)^#{2,3} Task (\d+) — `)

// m6dShippedAdapters is PRD #232 AC1's nine, in the order the PRD names them. python is
// the tenth adapter of the completeness table and is NOT here: it ships already, and it is
// byte-frozen.
var m6dShippedAdapters = []string{
	"vitest", "jest", "go", "cargo-nextest", "maven", "gradle", "rspec", "dotnet", "phpunit",
}

// m6dRequiresAdapters is PRD #232 AC3's five: the runners that cannot emit JUnit XML with
// nothing extra installed.
var m6dRequiresAdapters = []string{"jest", "rspec", "go", "dotnet", "cargo-nextest"}

// m6dTask is one task section: its number and the body between its heading and the next.
type m6dTask struct {
	num  string
	body string
}

// m6dTasks splits the plan into its numbered task sections.
func m6dTasks(t *testing.T, src string) []m6dTask {
	t.Helper()
	locs := m6dTaskHeading.FindAllStringSubmatchIndex(src, -1)
	if len(locs) == 0 {
		t.Fatalf("%s has no `## Task N — ` sections", planM6d)
	}
	var out []m6dTask
	for i, loc := range locs {
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, m6dTask{num: src[loc[2]:loc[3]], body: src[loc[0]:end]})
	}
	return out
}

func TestM6dPlanOpensWithTheHeaderBlock(t *testing.T) {
	src := readRepoFile(t, planM6d)

	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planM6d)
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	}
	// The header block of docs/plans/02-m1b-python-adapter.md, plus the Spec line that
	// binds this plan to the sections it discharges.
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planM6d, field)
		}
	}
}

func TestM6dPlanHasGlobalConstraints(t *testing.T) {
	src := readRepoFile(t, planM6d)
	if !regexp.MustCompile(`(?m)^## Global Constraints\s*$`).MatchString(src) {
		t.Errorf("%s must carry a `## Global Constraints` block", planM6d)
	}
}

func TestM6dPlanTasksCarryFilesAndInterfaces(t *testing.T) {
	src := readRepoFile(t, planM6d)
	for _, task := range m6dTasks(t, src) {
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

// Step 1 must be the test itself. A description of a test ("assert that Detect returns
// both adapters...") leaves the implementer to invent the assertion, and an invented
// assertion is the one the implementation already passes.
func TestM6dPlanStepOneIsLiteralFailingTestSource(t *testing.T) {
	src := readRepoFile(t, planM6d)
	for _, task := range m6dTasks(t, src) {
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
func TestM6dPlanStepTwoNamesCommandAndExpectedFailure(t *testing.T) {
	src := readRepoFile(t, planM6d)
	for _, task := range m6dTasks(t, src) {
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

// Every task says which spec section it discharges, and §4.4 and §8 — the milestone's own
// two sections — are both collectively covered.
func TestM6dPlanTasksDischargeSpecSections44And8(t *testing.T) {
	src := readRepoFile(t, planM6d)
	covered := map[string]bool{}
	for _, task := range m6dTasks(t, src) {
		line := regexp.MustCompile(`(?m)^\*\*Discharges:\*\*.*$`).FindString(task.body)
		if line == "" {
			t.Errorf("Task %s has no **Discharges:** line naming its spec section", task.num)
			continue
		}
		found := m6dSpecSection.FindAllStringSubmatch(line, -1)
		if len(found) == 0 {
			t.Errorf("Task %s **Discharges:** line names no spec section of §4.2/§4.3/§4.4/§8: %q", task.num, line)
		}
		for _, m := range found {
			covered[m[1]] = true
		}
	}
	for _, want := range []string{"4.4", "8"} {
		if !covered[want] {
			t.Errorf("no task discharges spec §%s, which is half of this milestone", want)
		}
	}
}

// Three sibling PRDs landed the machinery this plan only populates. A plan that does not
// say so invites an implementer to re-add a tier, a parser or a validation rule that
// already exists.
func TestM6dPlanNamesWhatTheSiblingPRDsAlreadyShipped(t *testing.T) {
	src := readRepoFile(t, planM6d)
	for _, want := range []string{"#229", "#230", "#231", "§4.1", "§4.2", "§4.3"} {
		if !strings.Contains(src, want) {
			t.Errorf("%s does not name %q as machinery a sibling PRD already shipped", planM6d, want)
		}
	}
	if !regexp.MustCompile(`(?i)already (shipped|landed)|out of scope here`).MatchString(src) {
		t.Errorf("%s must state which parts of §4.1/§4.2/§4.3 the siblings already shipped", planM6d)
	}
}

// The PRD leaves a set of things under-determined. Each is resolved once, with a reason,
// in the style of the M6a plan's own decisions section — otherwise every implementer
// decides them again, differently.
func TestM6dPlanResolvesTheUnderdeterminedDecisions(t *testing.T) {
	src := readRepoFile(t, planM6d)
	i := regexp.MustCompile(`(?mi)^#{3,4} The \w+ under-determined decisions`).FindStringIndex(src)
	if i == nil {
		t.Fatalf("%s must carry a `### The <n> under-determined decisions, resolved here` section", planM6d)
	}
	decisions := src[i[0]:]
	if j := strings.Index(decisions, "\n## "); j > 0 {
		decisions = decisions[:j]
	}
	for _, want := range []struct{ what, token string }{
		{"how vitest and jest avoid detecting each other's repos", "package.json"},
		{"whether the glob-only `detect` key has to grow", "detect"},
		{"what the completeness table can assert about the frozen python adapter", "byte-frozen"},
		{"how a map row carries its adapter", "map.jsonl"},
		{"what an untagged row read from a pre-PRD map means", "untagged"},
		{"what meta.json's singular adapter field becomes", "meta.json"},
		{"the exit code when one adapter fails and another passes", "exit code"},
		{"what `rtdd seed` does in a polyglot repo", "seed"},
		{"which adapters render a file-granular id", "file-granular"},
	} {
		if !strings.Contains(decisions, want.token) {
			t.Errorf("the decisions section does not resolve %s (no %q)", want.what, want.token)
		}
	}
}

// The exit-code precedence across adapters is a TABLE, not a sentence: PRD #232 AC7 says
// a failure is reported per adapter and the exit code reflects it, and the frozen table in
// 00-interfaces.md distinguishes 1, 2 and 3. A prose answer leaves the ordering to taste.
func TestM6dPlanStatesExitCodePrecedenceAsATable(t *testing.T) {
	src := readRepoFile(t, planM6d)
	rows := regexp.MustCompile(`(?m)^\|.*\|.*$`).FindAllString(src, -1)
	joined := strings.Join(rows, "\n")
	for _, code := range []string{"3", "2", "1", "0"} {
		if !strings.Contains(joined, "| "+code+" ") && !strings.Contains(joined, "| `"+code+"` ") {
			t.Errorf("%s has no table row for exit code %s in the across-adapter precedence rule", planM6d, code)
		}
	}
}

// Every one of the nine adapters is SPECIFIED here, not left to the implementer to invent:
// a literal YAML block naming the keys PRD #232 AC2 requires non-empty.
func TestM6dPlanSpecifiesEveryShippedAdapterAsLiteralYAML(t *testing.T) {
	src := readRepoFile(t, planM6d)
	blocks := fencedBlocks(src, "yaml")
	byName := map[string]string{}
	for _, b := range blocks {
		m := regexp.MustCompile(`(?m)^name:\s*(\S+)\s*$`).FindStringSubmatch(b)
		if m == nil {
			continue
		}
		byName[m[1]] = b
	}
	for _, name := range m6dShippedAdapters {
		block, ok := byName[name]
		if !ok {
			t.Errorf("%s has no ```yaml block declaring `name: %s`", planM6d, name)
			continue
		}
		for _, key := range []string{
			"detect:", "subset:", "list:", "test_globs:", "source_globs:",
			"opaque:", "full_escalate:", "report_path:", "id_template:", "test_for:",
			"selection: static", "coverage: none", "report: junit-xml",
		} {
			if !strings.Contains(block, key) {
				t.Errorf("adapter %s's YAML block declares no %q", name, key)
			}
		}
		if !strings.Contains(block, "{tests}") {
			t.Errorf("adapter %s's subset declares no {tests} placeholder", name)
		}
	}
}

// PRD #232 AC3 splits the nine five/four on `requires`. Getting it backwards ships an
// adapter that fails mid-run against a report file no runner was ever going to write.
func TestM6dPlanSplitsTheNineOnRequires(t *testing.T) {
	src := readRepoFile(t, planM6d)
	blocks := fencedBlocks(src, "yaml")
	byName := map[string]string{}
	for _, b := range blocks {
		m := regexp.MustCompile(`(?m)^name:\s*(\S+)\s*$`).FindStringSubmatch(b)
		if m != nil {
			byName[m[1]] = b
		}
	}
	needs := map[string]bool{}
	for _, n := range m6dRequiresAdapters {
		needs[n] = true
	}
	for _, name := range m6dShippedAdapters {
		block, ok := byName[name]
		if !ok {
			continue // already reported by the completeness test
		}
		has := strings.Contains(block, "requires:")
		if needs[name] && !has {
			t.Errorf("adapter %s must declare a non-empty requires (PRD #232 AC3)", name)
		}
		if !needs[name] && has {
			t.Errorf("adapter %s must declare NO requires: its runner emits JUnit XML unaided (PRD #232 AC3)", name)
		}
	}
	// cargo-nextest configures JUnit through a config file, never a CLI flag (spec §4.3).
	if !strings.Contains(src, ".config/nextest.toml") {
		t.Errorf("%s does not name .config/nextest.toml as how cargo-nextest configures JUnit output", planM6d)
	}
}

// Nine fixture repositories, one per adapter, plus the tenth polyglot one AC10 requires.
// Each is named by path, and each names the test file `rtdd which` must select.
func TestM6dPlanNamesTheTenFixtureRepositories(t *testing.T) {
	src := readRepoFile(t, planM6d)
	for _, name := range append(append([]string{}, m6dShippedAdapters...), "polyglot") {
		if !strings.Contains(src, "testdata/fixtures/"+name+"/") {
			t.Errorf("%s names no fixture repository testdata/fixtures/%s/", planM6d, name)
		}
	}
	if !regexp.MustCompile(`(?i)exactly two adapters`).MatchString(src) {
		t.Errorf("%s must state that the polyglot fixture detects exactly two adapters (PRD #232 AC10)", planM6d)
	}
}

// The plan's task set must cover every acceptance criterion of PRD #232. Naming them is
// how a reviewer checks that without re-deriving the mapping.
func TestM6dPlanCoversEveryAcceptanceCriterionOfThePRD(t *testing.T) {
	src := readRepoFile(t, planM6d)
	for _, ac := range []string{"AC1", "AC2", "AC3", "AC4", "AC5", "AC6", "AC7", "AC8", "AC9", "AC10", "AC11"} {
		if !regexp.MustCompile(ac + `\b`).MatchString(src) {
			t.Errorf("%s never names PRD #232 %s", planM6d, ac)
		}
	}
}

// adapters/python.yaml is byte-frozen and its digest is not to be edited. A task that
// lists it under **Files:** is the one way this plan could break the regression promise
// spec §4.1 makes.
func TestM6dPlanKeepsThePythonAdapterFrozen(t *testing.T) {
	src := readRepoFile(t, planM6d)
	constraints := src
	if i := strings.Index(src, "## Global Constraints"); i >= 0 {
		constraints = src[i:]
		if j := strings.Index(constraints, "\n## Task"); j > 0 {
			constraints = constraints[:j]
		}
	}
	if !strings.Contains(constraints, "byte-frozen") {
		t.Errorf("%s must state as a GLOBAL CONSTRAINT that adapters/python.yaml is byte-frozen", planM6d)
	}
	if !strings.Contains(constraints, "adapter_freeze_test.go") {
		t.Errorf("%s must name internal/contract/adapter_freeze_test.go's digest as not to be edited", planM6d)
	}
	for _, task := range m6dTasks(t, src) {
		files := regexp.MustCompile(`(?m)^\*\*Files:\*\*.*$`).FindString(task.body)
		if strings.Contains(files, "adapters/python.yaml") {
			t.Errorf("Task %s lists adapters/python.yaml in **Files:**, which is byte-frozen", task.num)
		}
	}
}

func TestM6dPlanClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planM6d)
	const heading = "## What this plan deliberately leaves undone"
	i := strings.Index(src, heading)
	if i < 0 {
		t.Fatalf("%s must close with `%s`", planM6d, heading)
	}
	tail := src[i:]
	if strings.Contains(tail, "\n## ") {
		t.Errorf("`%s` must be the final section of %s", heading, planM6d)
	}
	// #233 owns the measured evidence and the front-end fidelity statements; §11 and
	// audit A5 keep per-test coverage for these ecosystems deferred.
	for _, want := range []string{"#233", "§11", "A5", "§7"} {
		if !strings.Contains(tail, want) {
			t.Errorf("`%s` does not name %q as owned elsewhere", heading, want)
		}
	}
}
