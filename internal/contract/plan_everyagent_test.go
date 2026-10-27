package contract

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The every-agent plan document is a contract, not prose: issue #524 makes it the first
// child of PRD #511 and nothing else in that PRD starts before it merges. #510 (the
// feature × agent matrix) and one child per agent are implemented against it, so a plan
// that leaves an agent's install path, its file format or its limitation to the implementer
// hands ten lanes ten guesses about tools none of them has installed — and a guessed path
// is a file written where the agent never looks. docs/agent-capabilities.md is where each
// of those facts is cited. Both are asserted here. planTasksOf, planSection, stepBody and
// fencedBlocks live in plan_n2_test.go and plan_m6a_test.go.
const (
	planEveryAgent     = "docs/plans/2026-10-24-every-agent.md"
	agentCapabilities  = "docs/agent-capabilities.md"
	everyAgentPRDCount = 4 // PRD #511's acceptance criteria
)

// everyAgent is PRD #511's "Agents in scope", in the order the PRD names them. Cursor's
// editor and `cursor-agent` are one agent with one column: they read the same rules.
var everyAgent = []string{
	"Claude Code", "Antigravity", "Codex", "Gemini CLI", "Copilot CLI",
	"Cursor", "OpenCode", "Zcode", "Hermes", "OpenClaw",
}

// Issue #524: the header block every plan in docs/plans/ opens with, naming the PRD and
// the two issues that are its spec.
func TestEveryAgentPlanOpensWithTheHeaderBlockNamingTheSpec(t *testing.T) {
	src := readRepoFile(t, planEveryAgent)
	if !strings.HasPrefix(src, "# ") {
		t.Errorf("%s must open with a level-1 title", planEveryAgent)
	}
	head := src
	if i := strings.Index(src, "## Global Constraints"); i > 0 {
		head = src[:i]
	}
	for _, field := range []string{"**Goal:**", "**Architecture:**", "**Tech Stack:**", "**Spec:**"} {
		if !strings.Contains(head, field) {
			t.Errorf("%s must carry %s in the header block, before ## Global Constraints", planEveryAgent, field)
		}
	}
	spec := regexp.MustCompile(`(?m)^\*\*Spec:\*\*.*$`).FindString(head)
	for _, want := range []string{"#511", "#509", "#510", agentCapabilities} {
		if !strings.Contains(spec, want) {
			t.Errorf("the **Spec:** line does not name %q: %q", want, spec)
		}
	}
}

// PRD #511 and #509: rtdd writes only directories it owns, a shared file only when named
// with --append-to, and the matrix runs with no real agent CLI installed.
func TestEveryAgentPlanGlobalConstraintsCarryThePRDConstraints(t *testing.T) {
	block := planSection(t, planEveryAgent, readRepoFile(t, planEveryAgent), "## Global Constraints")
	for _, want := range []struct{ what, token string }{
		{"no shared memory file", "never a shared memory file"},
		{"the one way into a shared file", "--append-to"},
		{"the project opt-in", "--project-skill"},
		{"the fake home", "RTDD_HOME"},
		{"no real CLI", "no real agent CLI"},
		{"the single source", "protocol/PROTOCOL.md"},
		{"the gate", "scripts/ci-local.sh"},
		{"the release-artifact test", "static"},
	} {
		if !strings.Contains(block, want.token) {
			t.Errorf("`## Global Constraints` does not state %s (no %q)", want.what, want.token)
		}
	}
}

// The task shape every plan in docs/plans/ carries: files, interfaces, checkboxed steps,
// step 1 literal failing Go test source, step 2 the command and the failure it produces.
func TestEveryAgentPlanTasksCarryLiteralFailingTestsAndTheCommandThatFailsThem(t *testing.T) {
	for _, task := range planTasksOf(t, planEveryAgent, readRepoFile(t, planEveryAgent)) {
		for _, want := range []string{"**Files:**", "**Interfaces:**", "- [ ] **Step 1:", "- [ ] **Step 2:"} {
			if !strings.Contains(task.body, want) {
				t.Errorf("Task %d has no %s", task.num, want)
			}
		}
		sawTest := false
		for _, b := range fencedBlocks(stepBody(task.body, "1"), "go") {
			if strings.Contains(b, "func Test") {
				sawTest = true
			}
		}
		if !sawTest {
			t.Errorf("Task %d step 1 has no ```go block holding a `func Test`", task.num)
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

// PRD #511 "Task 1": #510 is the matrix child, then one child per agent. Every PRD
// acceptance criterion is discharged by a task that says so.
func TestEveryAgentPlanTasksAreTheMatrixThenOnePerAgent(t *testing.T) {
	src := readRepoFile(t, planEveryAgent)
	tasks := planTasksOf(t, planEveryAgent, src)
	if want := 1 + len(everyAgent); len(tasks) != want {
		t.Fatalf("%d tasks, want %d — the matrix (#510) and one per agent", len(tasks), want)
	}
	if !regexp.MustCompile(`(?m)^\*\*Issue:\*\* #510\b`).MatchString(tasks[0].body) {
		t.Errorf("Task 1 must be the matrix child, `**Issue:** #510`")
	}
	for i, name := range everyAgent {
		task := tasks[i+1]
		heading := task.body[:strings.Index(task.body, "\n")]
		if !strings.Contains(heading, name) {
			t.Errorf("Task %d heading %q does not name agent %q (PRD #511's order)", task.num, heading, name)
		}
	}
	acRef := regexp.MustCompile(`\bAC(\d+)\b`)
	claimed := map[string]bool{}
	for _, task := range tasks {
		if !strings.Contains(task.body, "**Issue:**") {
			t.Errorf("Task %d has no **Issue:** line", task.num)
		}
		line := regexp.MustCompile(`(?m)^\*\*Discharges:\*\*.*$`).FindString(task.body)
		if line == "" {
			t.Errorf("Task %d has no **Discharges:** line", task.num)
			continue
		}
		for _, ac := range acRef.FindAllStringSubmatch(line, -1) {
			claimed[ac[1]] = true
		}
	}
	for ac := 1; ac <= everyAgentPRDCount; ac++ {
		if !claimed[strconv.Itoa(ac)] {
			t.Errorf("PRD #511 AC%d is discharged by no task", ac)
		}
	}
	planSection(t, planEveryAgent, src, "## Acceptance-criterion map")
}

// Issue #524: an agent that can only read a shared file gets the explicit --append-to
// opt-in and nothing else, the plan states the limitation, and the README says so.
func TestEveryAgentPlanStatesTheSharedFileLimitation(t *testing.T) {
	block := planSection(t, planEveryAgent, readRepoFile(t, planEveryAgent), "## Agents that read only a shared file")
	for _, want := range []string{"--append-to", "README", "AGENTS.md", "never"} {
		if !strings.Contains(block, want) {
			t.Errorf("`## Agents that read only a shared file` does not state %q", want)
		}
	}
	readme := readRepoFile(t, "README.md")
	if !strings.Contains(readme, "reads only a shared file") || !strings.Contains(readme, "--append-to") {
		t.Errorf("README.md must say that an agent which reads only a shared file gets only `--append-to`")
	}
}

// Issue #524: docs/agent-capabilities.md has a row per agent, each with where it loads
// instructions at user and project level, the format, discovery, whether a self-contained
// directory works and the version checked — and every cell cites its source.
func TestAgentCapabilitiesCitesEveryCellForEveryAgent(t *testing.T) {
	src := readRepoFile(t, agentCapabilities)
	header := regexp.MustCompile(`(?m)^\| agent \|.*$`).FindString(src)
	if header == "" {
		t.Fatalf("%s has no table whose header starts `| agent |`", agentCapabilities)
	}
	cols := splitRow(header)
	for _, want := range []string{"user level", "project level", "format", "discovery", "self-contained", "version checked"} {
		found := false
		for _, c := range cols {
			if strings.Contains(strings.ToLower(c), want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s table has no %q column: %q", agentCapabilities, want, header)
		}
	}
	rows := map[string][]string{}
	for _, line := range strings.Split(src, "\n") {
		if !strings.HasPrefix(line, "| ") || line == header || strings.HasPrefix(line, "| ---") {
			continue
		}
		cells := splitRow(line)
		if len(cells) == len(cols) {
			rows[cells[0]] = cells
		}
	}
	// A citation is an inline link to a URL, or a shortcut reference `[label]` whose
	// definition `[label]: <source>` the file carries.
	cite := regexp.MustCompile(`\]\(https?://[^)]+\)|\[([a-z0-9][a-z0-9-]*)\]`)
	codeSpan := regexp.MustCompile("`[^`]*`")
	for _, name := range everyAgent {
		var row []string
		for k, v := range rows {
			if strings.Contains(k, name) {
				row = v
			}
		}
		if row == nil {
			t.Errorf("%s has no row for %s", agentCapabilities, name)
			continue
		}
		for i, cell := range row[1:] {
			if strings.TrimSpace(cell) == "" {
				t.Errorf("%s: %s's %q cell is empty", agentCapabilities, name, cols[i+1])
				continue
			}
			// Brackets inside a code span are code, such as a regular expression's class.
			refs := cite.FindAllStringSubmatch(codeSpan.ReplaceAllString(cell, ""), -1)
			if len(refs) == 0 {
				t.Errorf("%s: %s's %q cell cites no source: %q", agentCapabilities, name, cols[i+1], cell)
			}
			for _, ref := range refs {
				if ref[1] == "" {
					continue
				}
				def := regexp.MustCompile(`(?m)^\[` + regexp.QuoteMeta(ref[1]) + `\]: \S`)
				if !def.MatchString(src) {
					t.Errorf("%s: %s cites [%s] but the file defines no `[%s]: <source>`", agentCapabilities, name, ref[1], ref[1])
				}
			}
		}
	}
}

// PRD #511's non-goals: Harness's adapters are a sibling PRD, and selection quality is not
// this PRD's. The plan ends by saying so.
func TestEveryAgentPlanClosesWithWhatItLeavesUndone(t *testing.T) {
	src := readRepoFile(t, planEveryAgent)
	heads := regexp.MustCompile(`(?m)^## .*$`).FindAllString(src, -1)
	if len(heads) == 0 || strings.TrimSpace(heads[len(heads)-1]) != "## What this plan deliberately leaves undone" {
		t.Errorf("%s must close with `## What this plan deliberately leaves undone` as its last section", planEveryAgent)
	}
	undone := planSection(t, planEveryAgent, src, "## What this plan deliberately leaves undone")
	for _, want := range []string{"Harness", "selection"} {
		if !strings.Contains(undone, want) {
			t.Errorf("`## What this plan deliberately leaves undone` does not name %q", want)
		}
	}
}

// splitRow returns a markdown table row's cells, trimmed, without the outer pipes.
func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(strings.TrimSuffix(line, "|"), "|")
	// A `|` inside a code span is not a column separator.
	var cells []string
	cur := ""
	inCode := false
	for _, r := range line {
		switch {
		case r == '`':
			inCode = !inCode
			cur += string(r)
		case r == '|' && !inCode:
			cells = append(cells, strings.TrimSpace(cur))
			cur = ""
		default:
			cur += string(r)
		}
	}
	return append(cells, strings.TrimSpace(cur))
}
