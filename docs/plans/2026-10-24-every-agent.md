# Every Agent (E1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ] `) syntax for tracking.

**Goal:** rtdd installs its agent instructions for every major agent CLI — Claude Code, Google Antigravity, OpenAI Codex CLI, Gemini CLI, GitHub Copilot CLI, Cursor, OpenCode, Zcode, Hermes Agent and OpenClaw — each into a directory rtdd owns, never into a shared memory file, and every rtdd feature works through each one. An agent is listed in the README's "Supported agents" table only when its column of the feature × agent matrix is green, with "verified on <tool version>, <date>" and its limitations stated in its cell. The matrix runs with a fake `HOME` and a fake repository per agent; no agent CLI is installed or run.

**Architecture:** One table. `internal/install/agents.go` (new, Task 1) holds `Agents() []Agent`, one row per agent: its id, its name, the directory whose presence says it is installed, the files `rtdd skill install` writes (its user surface), the files `rtdd init --project-skill` writes (its project surface), its limitations and the version it was verified on. `GlobalSurfaces()` and `ProjectSkillFiles` stop being hand-written lists and are derived from it, so `PlanGlobal`, `PlanGlobalUninstall`, `Plan` and `PlanLegacyFiles` keep their signatures and pick up a new agent with no code of their own. `rtdd doctor` gains an `agents:` section rendered from the same table (`install.AgentStatus`, Task 1). The text every surface carries is still rendered from `protocol/PROTOCOL.md` by `internal/protocol`; no agent gets a target of its own unless its format demands one (Gemini CLI's manifest and Cursor's `.mdc` already exist; no agent in this plan needs a new one). The matrix is one test file, `cmd/rtdd/agent_matrix_test.go`: `matrixAgents` × `matrixRows`, every row driving the real `cmdSkill`, `cmdInit`, `refreshGlobalSkill`, `cmdUninstall`, `cmdDoctor` and `cmdWhich` against `RTDD_HOME`. Each agent task adds that agent's `matrixAgents` entry first — red — then its `Agents()` row. `cmd/rtdd/readme_agents_test.go` holds the README table to `matrixAgents`.

**Tech Stack:** Go 1.24 (module `github.com/VocanicZ/rtdd`), stdlib plus `gopkg.in/yaml.v3` — already the module's one dependency. Tests use stdlib `testing`, `t.TempDir()` homes pointed at by `RTDD_HOME`, and real git repositories built with `internal/gitctx/gittest`. No agent CLI, no network, no toolchain of any language is invoked.

**Spec:** PRD #511 (agents in scope, the seven matrix rows, acceptance criteria 1–4), #509 (self-contained install: rtdd-owned directories, `--project-skill`, `--append-to`, `--legacy-files`) and #510 (the matrix rules: every claimed agent fully supported or its limitation stated; `protocol/PROTOCOL.md` the single source). Per-agent facts — paths, formats, discovery, versions — are cited in [`docs/agent-capabilities.md`](../agent-capabilities.md), written with this plan (#524); a path this plan names and that page does not cite is a bug in one of them.

## Global Constraints

- **rtdd writes only directories it owns — never a shared memory file.** Every user-level file is under `<agent dir>/…/rtdd/`, every project-level file under `.<agent>/…/rtdd/` or `.agents/skills/rtdd/` or is rtdd's own whole file (`.cursor/rules/rtdd.mdc`). `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `~/.codex/AGENTS.md`, `~/.gemini/GEMINI.md`, `.github/copilot-instructions.md` and every other file an agent shares with its user are written only when the user names one with `rtdd init --append-to <file>` (#509). No task adds an implicit append.
- **An agent directory that does not exist is skipped, never created** (`PlanGlobal`'s rule, kept). The skip prints one line per agent, so `rtdd skill install` says what it considered.
- **`--project-skill` stays opt-in.** `rtdd init` without it writes inside `.rtdd/` only (`TestInitInstallsIntoTheWorkingDirectory`, unmodified). With it, it writes every agent's project surface whose path this plan settles — whether or not that agent is installed, because the repository is shared with contributors who use other tools.
- **The fake home is `RTDD_HOME`.** Every matrix row sets it with `t.Setenv` to a `t.TempDir()`; none reads the real home directory. `cmd/rtdd`'s `rtdd()` helper already gives a test without one an empty home.
- **no real agent CLI** is installed, run or required by any test, and no test skips because one is absent. A fact the matrix cannot check without the tool — that Codex really loads `~/.agents/skills/` — is cited in `docs/agent-capabilities.md`, not tested.
- **`protocol/PROTOCOL.md` is the single source.** Every surface is a render of it (`dist/GLOBAL-SKILL.md`, `dist/GLOBAL-AGENTS.md`, `dist/SKILL.md`, `dist/cursor/rules/rtdd.mdc`); no task writes instruction text in Go. The matrix row `same-instructions` asserts every file every agent receives carries the five steps of the `process` section verbatim.
- **No new runtime dependency, no cgo.** `go.mod` keeps exactly `gopkg.in/yaml.v3` (`TestGoModRequiresExactlyYAML`); the release binary stays `CGO_ENABLED=0` and statically linked — the release-artifact static-link test (`internal/installtest/release_artifacts_test.go`) passes unmodified.
- **Only `internal/gitctx` shells out to git** — `TestOnlyGitctxShellsOutToGit` stays green unmodified. No task adds a child process of any kind.
- `scripts/ci-local.sh` exits 0 at the end of every task, run with **this branch's** `rtdd` first on PATH:

  ```bash
  bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
  ```

  It runs `go build ./... && go vet ./... && go test ./... -count=1` (PRD #511 AC4) and the generator checks.

## Review Focus

- **Two agents, one home directory.** Antigravity keeps its configuration in `~/.gemini/config/`, inside Gemini CLI's `~/.gemini/`. Gating Gemini CLI on `~/.gemini/` would install a Gemini CLI extension on every machine that has only Antigravity. Each agent's `Present` path is the narrowest one its own tool creates (the surface table); the matrix row `user-install` seeds only that path and asserts the files that appear are exactly that agent's (`assertOnlyCreated`, Task 1), and `TestSkillInstallForAntigravityAloneWritesNoGeminiExtension` (Task 3) pins the one pair that collides.
- **Two agents, one skills directory.** `~/.agents/skills/` is read by Codex, Gemini CLI, Copilot CLI, Cursor, OpenCode and OpenClaw; `.agents/skills/` in a repository by Codex, Antigravity, Gemini CLI, Copilot CLI, Cursor, OpenCode, Hermes and OpenClaw. rtdd writes `~/.agents/skills/rtdd/` once (for Codex) and `.agents/skills/rtdd/` once (for all eight), and an agent that also reads another agent's directory — OpenCode reads `~/.claude/skills/` — may see rtdd twice. Every user-level copy is the same bytes (`dist/GLOBAL-SKILL.md`), so whichever an agent loads says the same thing; the row `same-instructions` asserts it. Uninstalling one agent must not remove a directory another still reads: `PlanGlobalUninstall` removes `~/.agents/skills/rtdd/` with `rtdd skill uninstall` (all agents) only.
- **A path that moved.** v0.3.0 wrote Codex's skill to `~/.codex/skills/rtdd/`; the documentation now names `~/.agents/skills/` (`docs/agent-capabilities.md`). Task 4 moves it: `rtdd skill install` writes the new path and deletes the old directory **only** when every file in it carries `protocol.Generated` — anything else is the user's and is kept, with a note. The legacy report names the old directory until it is gone.
- **The README table drifting from the matrix.** It lists exactly the `matrixAgents` names, in order, each row with `verified on <version>, <date>` equal to the agent's `Agent.Verified`; `TestREADMESupportedAgentsAreTheMatrixColumns` fails on a missing, extra or out-of-date row. A column is added to `matrixAgents` only when every row passes for it, so "listed" and "green" cannot diverge.
- **A limitation hidden by a skip.** An agent with no file-based user level (an agent whose `User` is empty) must still produce a `skip` step naming the limitation and `rtdd skill prompt`, never silence (#510's Cursor example). The row `user-install` asserts the note for those agents.
- **The description budget.** Zcode summarises a description to 250 characters and drops a skill whose description exceeds 1024; OpenCode requires `name` to equal the directory name. `GlobalSkillDescription` is 340 characters and `name: rtdd` lives in `rtdd/`. `TestEverySkillFrontmatterFitsEveryAgent` (Task 1) pins `name == "rtdd"`, `len(description) <= 1024` and the first sentence — what Zcode shows — under 250 characters.
- **`--force` reaching the wrong file.** `PlanGlobal`'s `--force` overwrites a hand-edited file inside rtdd's own directory only; with ten agents it now reaches ten directories, still all rtdd's. No step it plans may name a path outside an `Owned` directory — `TestAgentsTableInvariants` (Task 1) checks it for every row, and `TestPlanGlobalWritesOnlyRtddOwnedDirectories` keeps every assertion it has when Tasks 3 and 4 move its paths.

## The surface table

This is `Agents()` after the last task, in PRD #511's order; each task adds its row. Paths are relative to the home directory (user) or the repository root (project). `Present` is what must exist for `rtdd skill install` to write the user surface. Citations for every path are in `docs/agent-capabilities.md`.

| id | agent | `Present` | user surface (`rtdd skill install`) | project surface (`rtdd init --project-skill`) | task |
|---|---|---|---|---|---|
| `claude` | Claude Code | `.claude/` | `.claude/skills/rtdd/SKILL.md` ← `GLOBAL-SKILL.md` | `.claude/skills/rtdd/SKILL.md` ← `SKILL.md` | 2 |
| `antigravity` | Antigravity | `.gemini/config/` | `.gemini/config/skills/rtdd/SKILL.md` ← `GLOBAL-SKILL.md` | `.agents/skills/rtdd/SKILL.md` ← `SKILL.md` | 3 |
| `codex` | Codex | `.codex/` | `.agents/skills/rtdd/SKILL.md` ← `GLOBAL-SKILL.md` | `.agents/skills/rtdd/SKILL.md` ← `SKILL.md` | 4 |
| `gemini` | Gemini CLI | `.gemini/settings.json` or `.gemini/oauth_creds.json` | `.gemini/extensions/rtdd/gemini-extension.json` (literal manifest), `.gemini/extensions/rtdd/GEMINI.md` ← `GLOBAL-AGENTS.md` | `.agents/skills/rtdd/SKILL.md` ← `SKILL.md` (read in a trusted folder) | 3 (gate), 5 |
| `copilot` | Copilot CLI | `.copilot/` | `.copilot/skills/rtdd/SKILL.md` ← `GLOBAL-SKILL.md` | `.agents/skills/rtdd/SKILL.md` ← `SKILL.md` | 6 |
| `cursor` | Cursor | `.cursor/` | `.cursor/skills/rtdd/SKILL.md` ← `GLOBAL-SKILL.md` | `.cursor/rules/rtdd.mdc` ← `cursor/rules/rtdd.mdc` | 7 |
| `opencode` | OpenCode | `.config/opencode/` | `.config/opencode/skills/rtdd/SKILL.md` ← `GLOBAL-SKILL.md` | `.agents/skills/rtdd/SKILL.md` ← `SKILL.md` | 8 |
| `zcode` | Zcode | `.zcode/` | `.zcode/skills/rtdd/SKILL.md` ← `GLOBAL-SKILL.md` | none — no documented project path | 9 |
| `hermes` | Hermes Agent | `.hermes/` | `.hermes/skills/rtdd/SKILL.md` ← `GLOBAL-SKILL.md` | `.agents/skills/rtdd/SKILL.md` ← `SKILL.md` (loaded after `hermes skills trust`) | 10 |
| `openclaw` | OpenClaw | `.openclaw/` | `.openclaw/skills/rtdd/SKILL.md` ← `GLOBAL-SKILL.md` | `.agents/skills/rtdd/SKILL.md` ← `SKILL.md` (read when the repository is the workspace) | 11 |

`← X` names the `dist/` render the file is written from — a user-level file is a byte copy; a project-level file is that render with `install.OptInMarker` on the line after `protocol.Generated`, as `--project-skill` writes it today. `.agents/skills/rtdd/SKILL.md` appears on seven rows and is one file: `ProjectSkillFiles` is a map, so the seven rows plan one step.

## Agents that read only a shared file

An agent whose only instruction source is a shared file — `AGENTS.md`, `GEMINI.md`, `.github/copilot-instructions.md`, a settings pane — has no directory rtdd can own. For such an agent rtdd **never** writes that file on its own initiative: `rtdd skill install` plans a `skip` step whose note states the limitation and names `rtdd skill prompt`, and `rtdd init --project-skill` writes nothing for it. The one way rtdd's text reaches it is the explicit opt-in `rtdd init --append-to <file>`, which merges rtdd's marker-delimited block and touches no byte outside the markers (#509). Its `Agent.Limits` carries the sentence, its README cell repeats it, and the README says, once, above the table: an agent that reads only a shared file gets only `--append-to`.

As checked on 2026-10-27 (`docs/agent-capabilities.md`), no agent in scope is shared-file-only at both levels. Cursor's User Rules and Copilot CLI's `copilot-instructions.md` are shared, and every agent reads `AGENTS.md` or a file like it, but each of the ten also reads a skills or extensions directory rtdd can own, so each gets one. The rule still applies in full: to a release of a listed tool that lacks its directory — v0.3.0's Codex gate already prints such a skip for a Codex with no `~/.codex/skills/` — and to any agent added later whose only source is a shared file. `TestAgentsTableInvariants` (Task 1) fails on a row with no user surface and no stated limitation, so such an agent cannot be added silently.

## Decisions this plan settles

1. **`Agents()` is the one table**, in `internal/install/agents.go`; `GlobalSurfaces()` and `ProjectSkillFiles` derive from it and are not edited by hand again. Task 1.
2. **`Present` is per agent and as narrow as the tool makes it** — `.gemini/config/` for Antigravity, `.gemini/settings.json` or `.gemini/oauth_creds.json` for Gemini CLI. Task 1 (the field), Task 3 (both values), Task 5 (Gemini CLI's, checked against its package).
3. **`.agents/skills/rtdd/SKILL.md` is one project file** shared by Antigravity, Codex, Gemini CLI, Copilot CLI, OpenCode, Hermes and OpenClaw, and **`~/.agents/skills/rtdd/` is Codex's user surface**. rtdd writes the vendor-neutral directory once rather than one copy per tool. Tasks 3, 4, 5, 6, 8, 10, 11.
4. **Codex moves from `~/.codex/skills/rtdd/` to `~/.agents/skills/rtdd/`**, and the old directory is removed by `rtdd skill install` only when all of it is rtdd's render. Task 4.
5. **`rtdd doctor` prints an `agents:` section**, one line per `Agents()` row: `  <name>  installed  <owned dir>` · `  <name>  not installed` · `  <name>  present, not installed — run \`rtdd skill install\`` · `  <name>  <limitation>`. Rendered by `install.AgentStatus(home string) []AgentState`. Task 1.
6. **`Agent.Verified` is the README cell**: `"verified on <tool> <version>, <YYYY-MM-DD>"`, the version and date of the `docs/agent-capabilities.md` row. Each agent task sets it.
7. **Zcode gets no project surface** until Z.ai documents one; OpenClaw's project surface is `.agents/skills/` and its cell states the workspace limitation. Tasks 9, 11.
8. **The README "Supported agents" table** replaces the prose list under "Install"; `What rtdd writes` lists every path of the surface table. Each agent task adds its row to both. Task 1 creates the table.

## Acceptance-criterion map

| PRD #511 AC | What | Task(s) | Issue(s) |
|---|---|---|---|
| AC1 | the matrix test, fake `HOME`/project per agent, green for every agent × every row | 1 (harness), 2–11 (one column each) | #510, the agent children |
| AC2 | README "Supported agents" lists exactly the green columns, "verified on <tool version>, <date>", limitations per cell | 1 (table + guard), 2–11 (rows) | #510, the agent children |
| AC3 | `rtdd skill install` with all ten tools present creates only rtdd-owned paths; every pre-existing user file byte-identical | 1 (`TestSkillInstallWithEveryAgentPresentTouchesNoUserFile`), 11 (all ten present) | #510, OpenClaw's child |
| AC4 | `go build ./... && go vet ./... && go test ./... -count=1` exits 0; the release-artifact static-link test passes | every task | all |

Order: this plan (#524) first. Then Task 1 (#510). Then Tasks 2–11, one child issue per agent, filed from this plan when it merges; they touch one shared table row each and the matrix file's `matrixAgents`, so they run **one at a time**, in the order below — two lanes appending rows to one slice literal is a merge conflict for nothing.

## File Structure

| Path | Responsibility | Task |
|---|---|---|
| `internal/install/agents.go` | `Agent`, `AgentFile`, `Agents()`, `AgentStatus`, `AgentState` | 1; one row per agent task |
| `internal/install/global.go` | `GlobalSurfaces()` derived from `Agents()`; old-directory migration (Codex) | 1, 4 |
| `internal/install/install.go` | `ProjectSkillFiles` derived from `Agents()` | 1 |
| `internal/install/agents_test.go` | the table's own invariants: unique ids, every path inside an owned directory, every source a `dist/` render | 1 |
| `cmd/rtdd/agent_matrix_test.go` | `matrixAgents`, `matrixRows`, `TestAgentMatrix` | 1; one entry per agent task |
| `cmd/rtdd/readme_agents_test.go` | `TestREADMESupportedAgentsAreTheMatrixColumns` | 1 |
| `cmd/rtdd/doctor.go` | the `agents:` section | 1 |
| `cmd/rtdd/init.go` | the `--project-skill` help names every project surface | 1 |
| `README.md` | "Supported agents", "What rtdd writes" | 1; one row per agent task |
| `docs/agent-capabilities.md` | the cited facts | this plan; a task that finds a fact changed updates its row |
| `internal/contract/plan_everyagent_test.go` | this document's own contract (#524) | — |

---

### Task 1: The feature × agent matrix and the one agent table

**Issue:** #510

**Discharges:** AC1, AC2, AC3, AC4

**Files:**
- Create: `internal/install/agents.go`, `internal/install/agents_test.go`, `cmd/rtdd/agent_matrix_test.go`, `cmd/rtdd/readme_agents_test.go`
- Modify: `internal/install/global.go` (`GlobalSurfaces()` derived; `GlobalSurface.Present`; `GlobalFile.Content`; `PlanGlobalUninstall` skips an empty `Owned`), `internal/install/global_test.go` (one new test), `internal/install/install.go` (`ProjectSkillFiles` derived), `cmd/rtdd/doctor.go` (the `agents:` section), `cmd/rtdd/doctor_graph_test.go` (its exact expected output gains the `agents:` section — the only existing assertion this task extends, because the output it pins grows), `cmd/rtdd/init.go` (`--project-skill` help), `README.md` ("Supported agents", "What rtdd writes")

**Interfaces:**
- Produces:

  ```go
  // Agent is one agent CLI rtdd installs for, and every path rtdd writes for it.
  type Agent struct {
  	ID, Name string
  	// Present: home-relative paths, any one of which existing means the agent is installed.
  	// A trailing slash is a directory.
  	Present []string
  	// Needs and NeedsNote: a further home-relative directory the user surface requires, and
  	// the skip note when it is absent (Codex before Task 4). Empty for every other agent.
  	Needs, NeedsNote string
  	// Owned: rtdd's user-level directory, home-relative with a trailing slash; "" when User
  	// is empty.
  	Owned string
  	User    []GlobalFile // `rtdd skill install`; empty: no file-based user level
  	Project []GlobalFile // `rtdd init --project-skill`; Rel is repo-relative
  	Limits  []string     // one sentence each: the skip note, doctor's line, the README cell
  	// Verified: "verified on <tool> <version>, <YYYY-MM-DD>"; "" until the agent's task.
  	Verified string
  	// Retired: home-relative directories an earlier rtdd wrote for this agent (Task 4).
  	Retired []string
  }

  func Agents() []Agent
  func (f GlobalFile) Content(files map[string]string) string // Literal, or files[Source]

  type AgentState struct {
  	Agent Agent
  	State string // "installed", "present", "absent", "limited"
  }

  func AgentStatus(home string) ([]AgentState, error)
  ```

  `GlobalSurfaces()` returns one `GlobalSurface` per `Agents()` row with a non-empty `User`, in table order; an `Agent` with an empty `User` yields a surface with no files whose skip note is its `Limits[0]` plus "; `rtdd skill prompt` prints a copy to give it". `ProjectSkillFiles` is built from every row's `Project`, so a path several agents share is one entry.
- Consumes: `install.PlanGlobal`, `PlanGlobalUninstall`, `Plan`, `PlanLegacyFiles`, `Files()`; `protocol.Generated`, `protocol.BeginMarker`; `cmd/rtdd`'s `rtdd`, `newTestRepo`, `writeFile`, `homeFile`, `worktreeFiles`, `eventLines`, `eventLogVar`.

Task 1's `Agents()` holds the four columns v0.3.0 already claims, at v0.3.0's paths and with `Verified: ""`: `claude`, `codex` (`Present: [".codex/"]`, `Needs: ".codex/skills"`, user `.codex/skills/rtdd/SKILL.md`), `gemini` (`Present: [".gemini/"]`, the extension), `cursor` (no `User`, `Limits: ["Cursor's user rules live in its settings, not in a file"]`, project `.cursor/rules/rtdd.mdc`). Nothing rtdd writes changes in this task except Cursor's new skip line and doctor's new section.

- [ ] **Step 1: Write the failing tests.** Create `cmd/rtdd/agent_matrix_test.go`:

```go
package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
)

// PRD #511's feature × agent matrix. A column is one agent; a row is one rtdd feature.
// Every cell runs the real command against a fake home (RTDD_HOME) and, where the feature
// needs one, a fresh git repository — no agent CLI is installed or run. An agent is listed
// in the README's "Supported agents" table only when its whole column is green
// (TestREADMESupportedAgentsAreTheMatrixColumns), so adding an entry here is the claim.

// matrixAgent is one column: what a fake home holds for the agent to count as installed,
// and every path rtdd may write for it. It is written out by hand, never derived from
// install.Agents(), so the table and the test are two statements that have to agree.
type matrixAgent struct {
	id      string   // install.Agent.ID
	name    string   // install.Agent.Name, and the README row
	present []string // home-relative; a trailing slash is a directory
	user    []string // home-relative files `rtdd skill install` writes; empty: no user level
	project []string // repo-relative files `rtdd init --project-skill` writes for it
	shared  []string // home-relative files the agent shares with its user; seeded, never written
	legacy  []string // the shared files an older `rtdd skill install` merged a block into
	limit   string   // a phrase the skip note and doctor's line carry; "" when none
}

var matrixAgents = []matrixAgent{
	{
		id: "claude", name: "Claude Code", present: []string{".claude/"},
		user:    []string{".claude/skills/rtdd/SKILL.md"},
		project: []string{".claude/skills/rtdd/SKILL.md"},
		shared:  []string{".claude/CLAUDE.md"},
	},
	{
		id: "codex", name: "Codex", present: []string{".codex/skills/"},
		user:   []string{".codex/skills/rtdd/SKILL.md"},
		shared: []string{".codex/AGENTS.md"}, legacy: []string{".codex/AGENTS.md"},
	},
	{
		id: "gemini", name: "Gemini CLI", present: []string{".gemini/"},
		user:   []string{".gemini/extensions/rtdd/GEMINI.md", ".gemini/extensions/rtdd/gemini-extension.json"},
		shared: []string{".gemini/GEMINI.md"}, legacy: []string{".gemini/GEMINI.md"},
	},
	{
		id: "cursor", name: "Cursor", present: []string{".cursor/"},
		project: []string{".cursor/rules/rtdd.mdc"},
		limit:   "user rules live in its settings",
	},
}

// matrixRows are PRD #511's "Features every agent must pass", one function each.
var matrixRows = []struct {
	name string
	run  func(t *testing.T, a matrixAgent)
}{
	{"user-install", rowUserInstall},
	{"project-opt-in", rowProjectOptIn},
	{"update-refreshes", rowUpdateRefreshes},
	{"uninstall-and-legacy", rowUninstallAndLegacy},
	{"doctor-reports", rowDoctorReports},
	{"same-instructions", rowSameInstructions},
	{"event-log", rowEventLog},
}

func TestAgentMatrix(t *testing.T) {
	for _, a := range matrixAgents {
		for _, row := range matrixRows {
			t.Run(a.id+"/"+row.name, func(t *testing.T) { row.run(t, a) })
		}
	}
}

// The matrix and the table name the same agents, in the same order.
func TestAgentMatrixColumnsAreTheAgentsTable(t *testing.T) {
	var got, want []string
	for _, a := range install.Agents() {
		got = append(got, a.ID+"="+a.Name)
	}
	for _, a := range matrixAgents {
		want = append(want, a.id+"="+a.name)
	}
	if !slices.Equal(got, want) {
		t.Errorf("install.Agents() = %v, matrixAgents = %v; a column is added to both or to neither", got, want)
	}
}

// PRD #511 AC3: every agent present at once, and still only rtdd-owned paths appear.
func TestSkillInstallWithEveryAgentPresentTouchesNoUserFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RTDD_HOME", home)
	var want []string
	for _, a := range matrixAgents {
		seedAgent(t, home, a)
		want = append(want, a.user...)
	}
	before := snapshot(t, home)
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	assertOnlyCreated(t, home, before, want)
}

// seedAgent makes the agent present in home and gives each of its shared files the user's
// own text.
func seedAgent(t *testing.T, home string, a matrixAgent) {
	t.Helper()
	for _, p := range a.present {
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(filepath.Join(home, filepath.FromSlash(p)), 0o755); err != nil {
				t.Fatal(err)
			}
		} else {
			writeFile(t, home, p, "{}\n")
		}
	}
	for _, s := range a.shared {
		writeFile(t, home, s, userText)
	}
}

const userText = "# my own rules\n\nkeep me byte for byte\n"

// matrixHome is a fresh fake home holding exactly one agent, and its contents before rtdd
// ran.
func matrixHome(t *testing.T, a matrixAgent) (string, map[string]string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("RTDD_HOME", home)
	seedAgent(t, home, a)
	return home, snapshot(t, home)
}

// snapshot reads every file under dir, keyed by its slash-separated relative path.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// assertOnlyCreated fails unless every file in before is byte-identical in dir now and the
// files that appeared are exactly want.
func assertOnlyCreated(t *testing.T, dir string, before map[string]string, want []string) {
	t.Helper()
	after := snapshot(t, dir)
	for p, b := range before {
		if after[p] != b {
			t.Errorf("%s changed; a file rtdd does not own must stay byte-identical", p)
		}
	}
	var created []string
	for p := range after {
		if _, ok := before[p]; !ok {
			created = append(created, p)
		}
	}
	slices.Sort(created)
	want = slices.Compact(slices.Sorted(slices.Values(want)))
	if !slices.Equal(created, want) {
		t.Errorf("created %v, want exactly %v", created, want)
	}
}

func agentRow(t *testing.T, id string) install.Agent {
	t.Helper()
	for _, a := range install.Agents() {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("install.Agents() has no %q", id)
	return install.Agent{}
}

func rowUserInstall(t *testing.T, a matrixAgent) {
	home, before := matrixHome(t, a)
	code, stdout, stderr := rtdd(t, t.TempDir(), "skill", "install")
	if code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	assertOnlyCreated(t, home, before, a.user)
	if a.limit != "" && !strings.Contains(stdout, a.limit) {
		t.Errorf("the output does not state %s's limitation %q:\n%s", a.name, a.limit, stdout)
	}
}

func rowProjectOptIn(t *testing.T, a matrixAgent) {
	matrixHome(t, a)
	dir := newTestRepo(t)
	before := map[string]string{}
	for _, f := range worktreeFiles(t, dir) {
		before[f] = readRepoFileForTest(t, dir, f)
	}
	if code, _, stderr := rtdd(t, dir, "init", "--project-skill"); code != 0 {
		t.Fatalf("rtdd init --project-skill exited %d: %s", code, stderr)
	}
	created := map[string]bool{}
	for _, f := range worktreeFiles(t, dir) {
		if _, ok := before[f]; !ok {
			created[f] = true
		}
	}
	for _, p := range a.project {
		if !created[p] {
			t.Errorf("--project-skill did not write %s for %s", p, a.name)
		}
	}
	// A path several agents share is written once whichever row asked for it, so the
	// files on disk cannot show that this agent's row names its own project surface.
	var rels []string
	for _, f := range agentRow(t, a.id).Project {
		rels = append(rels, f.Rel)
	}
	if !slices.Equal(rels, a.project) {
		t.Errorf("install.Agents() %s Project = %v, want %v", a.id, rels, a.project)
	}
	for p := range created {
		if _, ours := install.ProjectSkillFiles[p]; !ours && !strings.HasPrefix(p, ".rtdd/") {
			t.Errorf("--project-skill wrote %s, which is no agent's project surface", p)
		}
	}
}

func rowUpdateRefreshes(t *testing.T, a matrixAgent) {
	home, _ := matrixHome(t, a)
	files, err := install.Files()
	if err != nil {
		t.Fatal(err)
	}
	ag := agentRow(t, a.id)
	for _, f := range ag.User {
		writeFile(t, home, f.Rel, f.Marker+"\nan older release's text\n")
	}
	before := snapshot(t, home)
	var out, errb strings.Builder
	refreshGlobalSkill(&out, &errb)
	var want []string
	for _, f := range ag.User {
		if got := homeFile(t, home, f.Rel); got != f.Content(files) {
			t.Errorf("rtdd update left %s as an older release's render", f.Rel)
		}
		want = append(want, f.Rel)
	}
	for p, b := range before {
		if !slices.Contains(want, p) && snapshot(t, home)[p] != b {
			t.Errorf("rtdd update changed %s, which is not an rtdd file", p)
		}
	}
}

func rowUninstallAndLegacy(t *testing.T, a matrixAgent) {
	home, before := matrixHome(t, a)
	dir := t.TempDir()
	if code, _, stderr := rtdd(t, dir, "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	if code, _, stderr := rtdd(t, dir, "skill", "uninstall"); code != 0 {
		t.Fatalf("rtdd skill uninstall exited %d: %s", code, stderr)
	}
	if after := snapshot(t, home); !reflect.DeepEqual(after, before) {
		t.Errorf("install then uninstall left %v, want the home exactly as it was %v", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(before)))
	}
	for _, l := range a.legacy {
		writeFile(t, home, l, "# my own rules\n\n"+protocol.BeginMarker+"\nold block\n"+protocol.EndMarker+"\n\nkeep me\n")
	}
	repo := newTestRepo(t)
	if code, _, stderr := rtdd(t, repo, "uninstall", "--legacy-files"); code != 0 {
		t.Fatalf("rtdd uninstall --legacy-files exited %d: %s", code, stderr)
	}
	for _, l := range a.legacy {
		got := homeFile(t, home, l)
		if strings.Contains(got, protocol.BeginMarker) || !strings.Contains(got, "# my own rules") || !strings.Contains(got, "keep me") {
			t.Errorf("--legacy-files left %s as %q; want the block gone and the user's text kept", l, got)
		}
	}
	for _, s := range a.shared {
		if !slices.Contains(a.legacy, s) && homeFile(t, home, s) != userText {
			t.Errorf("--legacy-files changed %s, which no rtdd ever wrote", s)
		}
	}
}

// doctorLine is the line of doctor's agents: section that names the agent.
func doctorLine(t *testing.T, out, name string) string {
	t.Helper()
	_, section, ok := strings.Cut(out, "\nagents:\n")
	if !ok {
		t.Fatalf("rtdd doctor printed no agents: section:\n%s", out)
	}
	for _, l := range strings.Split(section, "\n") {
		if strings.HasPrefix(l, "  "+name+"  ") {
			return l
		}
	}
	t.Fatalf("rtdd doctor's agents: section has no line for %s:\n%s", name, section)
	return ""
}

func rowDoctorReports(t *testing.T, a matrixAgent) {
	matrixHome(t, a)
	dir := newTestRepo(t)
	_, out, _ := rtdd(t, dir, "doctor")
	first := doctorLine(t, out, a.name)
	if len(a.user) == 0 {
		if !strings.Contains(first, a.limit) {
			t.Errorf("doctor's line for %s does not state its limitation %q: %q", a.name, a.limit, first)
		}
		return
	}
	if !strings.Contains(first, "present, not installed") {
		t.Errorf("before skill install, doctor's line for %s = %q, want \"present, not installed\"", a.name, first)
	}
	rtdd(t, dir, "skill", "install")
	_, out, _ = rtdd(t, dir, "doctor")
	if got := doctorLine(t, out, a.name); !strings.Contains(got, "installed  ~/"+agentRow(t, a.id).Owned) {
		t.Errorf("after skill install, doctor's line for %s = %q, want \"installed  ~/%s\"", a.name, got, agentRow(t, a.id).Owned)
	}
}

// protocolSource is read before any test changes directory.
var protocolSource, protocolErr = os.ReadFile(filepath.Join("..", "..", "protocol", "PROTOCOL.md"))

// processSteps are the five numbered lines of PROTOCOL.md's process section, whitespace
// collapsed: the rounds process every agent must be told, word for word.
func processSteps(t *testing.T) []string {
	t.Helper()
	if protocolErr != nil {
		t.Fatal(protocolErr)
	}
	_, body, _ := strings.Cut(string(protocolSource), "id=process ")
	body, _, _ = strings.Cut(body, "<!-- rtdd:endsection -->")
	steps := regexp.MustCompile(`(?m)^[1-5]\. .*$`).FindAllString(body, -1)
	if len(steps) != 5 {
		t.Fatalf("PROTOCOL.md's process section has %d numbered steps, want 5", len(steps))
	}
	return steps
}

func rowSameInstructions(t *testing.T, a matrixAgent) {
	home, _ := matrixHome(t, a)
	dir := newTestRepo(t)
	rtdd(t, dir, "skill", "install")
	rtdd(t, dir, "init", "--project-skill")
	flat := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	check := func(where, body string) {
		for _, step := range processSteps(t) {
			if !strings.Contains(flat(body), flat(step)) {
				t.Errorf("%s does not carry the process step %q", where, step)
			}
		}
	}
	for _, p := range a.user {
		if strings.HasSuffix(p, ".json") {
			var m map[string]any
			if err := json.Unmarshal([]byte(homeFile(t, home, p)), &m); err != nil || m["name"] != "rtdd" {
				t.Errorf("~/%s is not rtdd's manifest", p)
			}
			continue
		}
		check("~/"+p, homeFile(t, home, p))
	}
	for _, p := range a.project {
		check(p, readRepoFileForTest(t, dir, p))
	}
}

func rowEventLog(t *testing.T, a matrixAgent) {
	matrixHome(t, a)
	dir := newTestRepo(t)
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(eventLogVar, log)
	t.Setenv("HARNESS_ISSUE", "511")
	t.Setenv("HARNESS_AGENT", a.id)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 9\n")
	if code, _, stderr := rtdd(t, dir, "which"); code != 0 {
		t.Fatalf("rtdd which exited %d: %s", code, stderr)
	}
	ev := eventLines(t, log)
	if len(ev) != 1 {
		t.Fatalf("%d events, want 1", len(ev))
	}
	// ctx holds every HARNESS_* variable, and a fleet host sets more than these two; the
	// row asserts the two it set, as event_test.go does.
	ctx, _ := ev[0]["ctx"].(map[string]any)
	if ctx["HARNESS_ISSUE"] != "511" || ctx["HARNESS_AGENT"] != a.id {
		t.Errorf("event ctx under %s = %v, want HARNESS_ISSUE=511 and HARNESS_AGENT=%s", a.name, ctx, a.id)
	}
	if ev[0]["cmd"] != "which" || ev[0]["round1"] == nil {
		t.Errorf("event under %s = %v, want a which event with round1", a.name, ev[0])
	}
}
```

Create `cmd/rtdd/readme_agents_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
)

// PRD #511 AC2: the README's "Supported agents" table lists exactly the matrix's columns,
// in its order, each with the table's Verified cell and every limitation stated.
func TestREADMESupportedAgentsAreTheMatrixColumns(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(b), "\n## Supported agents\n")
	if !ok {
		t.Fatal("README.md has no `## Supported agents` section")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	var rows [][]string
	for _, l := range strings.Split(section, "\n") {
		if !strings.HasPrefix(l, "| ") || strings.HasPrefix(l, "| agent ") || strings.HasPrefix(l, "| ---") {
			continue
		}
		var cells []string
		for _, c := range strings.Split(strings.Trim(l, "|"), " | ") {
			cells = append(cells, strings.TrimSpace(c))
		}
		rows = append(rows, cells)
	}
	if len(rows) != len(matrixAgents) {
		t.Fatalf("README lists %d agents, the matrix has %d columns", len(rows), len(matrixAgents))
	}
	for i, a := range matrixAgents {
		ag := agentRow(t, a.id)
		if rows[i][0] != a.name {
			t.Errorf("README row %d is %q, want %q (the matrix's order)", i+1, rows[i][0], a.name)
		}
		verified := ag.Verified
		if verified == "" {
			verified = "not yet verified"
		}
		if !strings.Contains(strings.Join(rows[i], " | "), verified) {
			t.Errorf("README row %s does not say %q", a.name, verified)
		}
		for _, lim := range ag.Limits {
			if !strings.Contains(strings.Join(rows[i], " | "), lim) {
				t.Errorf("README row %s does not state the limitation %q", a.name, lim)
			}
		}
	}
	if !strings.Contains(section, "reads only a shared file") || !strings.Contains(section, "--append-to") {
		t.Error("`## Supported agents` must say that an agent which reads only a shared file gets only `--append-to`")
	}
}

// Every user-level skill an agent reads fits every agent's frontmatter rules: OpenCode and
// Cursor require name to equal the directory, Zcode drops a description over 1024
// characters and shows the first 250.
func TestEverySkillFrontmatterFitsEveryAgent(t *testing.T) {
	files, err := install.Files()
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{"dist/GLOBAL-SKILL.md", "dist/SKILL.md"} {
		front, _, _ := strings.Cut(strings.TrimPrefix(files[src], "---\n"), "\n---\n")
		var name, desc string
		for _, l := range strings.Split(front, "\n") {
			if v, ok := strings.CutPrefix(l, "name: "); ok {
				name = v
			}
			if v, ok := strings.CutPrefix(l, "description: "); ok {
				desc = v
			}
		}
		if name != "rtdd" {
			t.Errorf("%s name = %q, want rtdd — the directory every agent's copy lives in", src, name)
		}
		if len(desc) > 1024 {
			t.Errorf("%s description is %d characters; Zcode drops a skill over 1024", src, len(desc))
		}
		if first, _, _ := strings.Cut(desc, ". "); len(first) > 250 {
			t.Errorf("%s description's first sentence is %d characters; Zcode shows 250", src, len(first))
		}
	}
}
```

Create `internal/install/agents_test.go`:

```go
package install

import (
	"strings"
	"testing"
)

// The agent table's own invariants. Each is a way a new row could quietly write somewhere
// rtdd does not own, or render nothing.
func TestAgentsTableInvariants(t *testing.T) {
	files, err := Files()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, a := range Agents() {
		if a.ID == "" || a.Name == "" || seen[a.ID] {
			t.Errorf("agent %+v: empty or duplicate id/name", a)
		}
		seen[a.ID] = true
		if len(a.Present) == 0 {
			t.Errorf("%s has no Present path; rtdd could not tell whether it is installed", a.ID)
		}
		if len(a.User) == 0 && len(a.Limits) == 0 {
			t.Errorf("%s has no user surface and states no limitation", a.ID)
		}
		for _, f := range a.User {
			if a.Owned == "" || !strings.HasPrefix(f.Rel, a.Owned) || !strings.HasSuffix(a.Owned, "/rtdd/") {
				t.Errorf("%s writes %s outside an rtdd-owned directory (Owned %q)", a.ID, f.Rel, a.Owned)
			}
		}
		for _, f := range append(append([]GlobalFile{}, a.User...), a.Project...) {
			if f.Source != "" && files[f.Source] == "" {
				t.Errorf("%s: %s renders from %s, which Files() does not hold", a.ID, f.Rel, f.Source)
			}
			if f.Marker == "" {
				t.Errorf("%s: %s has no Marker, so an upgrade could not tell rtdd's render from the user's file", a.ID, f.Rel)
			}
		}
		for _, f := range a.Project {
			if !strings.Contains(f.Rel, "/rtdd/") && !strings.HasSuffix(f.Rel, "/rtdd.mdc") {
				t.Errorf("%s: project file %s is not rtdd's own file", a.ID, f.Rel)
			}
		}
	}
}

// Doctor's per-agent state, from a fake home.
func TestAgentStatusReadsTheHome(t *testing.T) {
	home := homeWith(t, ".claude")
	states, err := AgentStatus(home)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range states {
		got[s.Agent.ID] = s.State
	}
	if got["claude"] != "present" || got["codex"] != "absent" || got["cursor"] != "limited" {
		t.Errorf("AgentStatus = %v, want claude present, codex absent, cursor limited", got)
	}
	steps, _ := PlanGlobal(home, globalFiles(), false)
	if err := Apply(home, steps); err != nil {
		t.Fatal(err)
	}
	states, _ = AgentStatus(home)
	for _, s := range states {
		if s.Agent.ID == "claude" && s.State != "installed" {
			t.Errorf("after install, claude is %q, want installed", s.State)
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/install ./cmd/rtdd -run 'TestAgents|TestAgentStatus|TestAgentMatrix|TestSkillInstallWithEveryAgentPresent|TestREADMESupportedAgents|TestEverySkillFrontmatter' -count=1
```

Expected: FAIL — build error `undefined: Agents`, `undefined: AgentStatus`, `f.Content undefined (type GlobalFile has no field or method Content)` in both packages.

- [ ] **Step 3: `internal/install/agents.go`.** Write `Agent`, `AgentState`, `Agents()` with the four rows above (the v0.3.0 paths and markers `GlobalSurfaces()` holds today, moved, not changed), `GlobalFile.Content`, and `AgentStatus`, checked in this order: `limited` when the agent has no `User` (whether or not it is installed, its line states the limitation), `absent` when no `Present` path exists, `installed` when every `User` file exists, otherwise `present`. The package comment says why the table is one list: three hand-kept lists (`GlobalSurfaces`, `ProjectSkillFiles`, the README) drifted the day #509 moved two of them.
- [ ] **Step 4: Derive the old lists.** `GlobalSurfaces()` becomes a loop over `Agents()`. `GlobalSurface` keeps its doc comment and fields, and gains `Present []string`, which `globalSkip` checks (any one) in place of `Dir`, then `Needs`; `Dir` is deleted once nothing reads it. `PlanGlobal` is otherwise untouched. `PlanGlobalUninstall` skips a surface whose `Owned` is empty — `filepath.Join(home, "")` is the home directory itself, and a plan that deleted it would be the worst bug this PRD could ship; `TestPlanGlobalUninstallNeverPlansTheHomeDirectory` (add it here, beside `TestPlanGlobalUninstallKeepsADirectoryHoldingAForeignFile`) pins it for an empty home and for one holding only `.cursor/`. A row with no `User` yields a `Skip` step at `Present[0]` with the note `"<Limits[0]>; `rtdd skill prompt` prints a copy to give it"`. `ProjectSkillFiles` becomes `projectSkillFiles()` built from every row's `Project` (`Rel` → `Source`), assigned to the package variable in an `init`-free `var ProjectSkillFiles = projectSkillFiles()`. `go test ./internal/install` must stay green with no existing test edited.
- [ ] **Step 5: `rtdd doctor`.** After the legacy section, `renderDoctor` prints `agents:`, then one line per `AgentStatus` row: `  <Name>  installed  ~/<Owned>`, `  <Name>  present, not installed — run ` + "`rtdd skill install`", `  <Name>  not installed`, `  <Name>  <Limits[0]>`. `cmdDoctor` passes the home from `homeDir()`; an error reading it prints `  (could not read the home directory: <err>)` and never changes the exit code.
- [ ] **Step 6: README.** Add `## Supported agents` after `## Install`: one sentence — rtdd installs its instructions into a directory each agent reads and rtdd owns; an agent that reads only a shared file gets only `--append-to`, never an implicit edit — then a table `| agent | user level | project level (`--project-skill`) | verified | limitations |` with one row per `matrixAgents` entry; `verified` reads `not yet verified` until the agent's task. Cursor's limitations cell carries its `Limits[0]` verbatim (`TestREADMESupportedAgentsAreTheMatrixColumns` matches it); the "Install" section's Cursor sentence is reworded to point at the table. `What rtdd writes` is unchanged in this task.
- [ ] **Step 7: Run to verify they pass, then the gate**

```bash
go test ./internal/install ./cmd/rtdd -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 28 subtests (4 agents × 7 rows).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(install): one agent table and the feature × agent matrix for Claude Code, Codex, Gemini CLI and Cursor (closes #510)"
```

---


### Task 2: Claude Code — the reference column, verified

**Issue:** the Claude Code child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4

Claude Code is the column every other agent is measured against: its paths have not moved since v0.3.0, and its format is the one `dist/GLOBAL-SKILL.md` and `dist/SKILL.md` are written in. This task checks it against `docs/agent-capabilities.md` and records the verification; nothing it writes changes.

**Files:**
- Modify: `internal/install/agents.go`, `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `claude` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["claude"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, the `claude` entry of `matrixAgents` is already Task 1's. Add, below `TestAgentMatrixColumnsAreTheAgentsTable`:

```go
// verifiedOn is each agent's README cell, from docs/agent-capabilities.md: an agent's task
// adds its entry when its column has been checked against that page, and its Agents() row
// must say the same.
var verifiedOn = map[string]string{
	"claude": "verified on Claude Code 2.1.311, 2026-10-27",
}

func TestEveryVerifiedColumnSaysSo(t *testing.T) {
	for id, want := range verifiedOn {
		if got := agentRow(t, id).Verified; got != want {
			t.Errorf("%s: Verified = %q, want %q", id, got, want)
		}
	}
}

// The project skill is dist/SKILL.md — the repository-scoped description, which assumes a
// set-up repository — with the opt-in marker, never the machine-wide render.
func TestClaudeCodeProjectSkillIsTheProjectRender(t *testing.T) {
	matrixHome(t, matrixAgents[0])
	dir := newTestRepo(t)
	if code, _, stderr := rtdd(t, dir, "init", "--project-skill"); code != 0 {
		t.Fatalf("rtdd init --project-skill exited %d: %s", code, stderr)
	}
	got := readRepoFileForTest(t, dir, ".claude/skills/rtdd/SKILL.md")
	if !strings.Contains(got, "description: "+protocol.SkillDescription+"\n") {
		t.Errorf("the project skill does not carry protocol.SkillDescription:\n%s", got)
	}
	if !strings.Contains(got, install.OptInMarker) {
		t.Errorf("the project skill carries no opt-in marker:\n%s", got)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestClaudeCodeProjectSkillIsTheProjectRender' -count=1
```

Expected: FAIL — `TestEveryVerifiedColumnSaysSo`: `claude: Verified = "", want "verified on Claude Code 2.1.311, 2026-10-27"`. (`TestClaudeCodeProjectSkillIsTheProjectRender` passes already: it pins behaviour this task must not move.)

- [ ] **Step 3: The table row.** Set the `claude` row's `Verified`. No path changes. The `claude` row of `Agents()` after this task:

```go
	{
		ID: "claude", Name: "Claude Code",
		Present: []string{".claude/"},
		Owned:   ".claude/skills/rtdd/",
		User:    []GlobalFile{{Rel: ".claude/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		Project: []GlobalFile{{Rel: ".claude/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
		Verified: "verified on Claude Code 2.1.311, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, Claude Code's row: user level `~/.claude/skills/rtdd/`, project level `.claude/skills/rtdd/SKILL.md`, `verified on Claude Code 2.1.311, 2026-10-27`, and no limitation. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read Claude Code's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 28 subtests (4 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): Claude Code — ~/.claude/skills/rtdd/ as a verified matrix column (closes #<the Claude Code child>)"
```

---

### Task 3: Google Antigravity — `~/.gemini/config/skills/rtdd/`, and Gemini CLI's gate narrowed

**Issue:** the Antigravity child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4

Antigravity keeps its configuration in `~/.gemini/config/`, inside the `~/.gemini/` Gemini CLI uses, so this task also narrows Gemini CLI's `Present` from `.gemini/` to `.gemini/settings.json` and `.gemini/oauth_creds.json` — the files Gemini CLI writes and Antigravity does not (Task 5 re-checks them against the 0.65.0 package). Without that, every Antigravity machine would get a Gemini CLI extension. It also adds the shared project file `.agents/skills/rtdd/SKILL.md`, which Codex, OpenCode, Hermes and OpenClaw reuse.

**Files:**
- Modify: `internal/install/agents.go`, `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `antigravity` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["antigravity"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`, and `verifiedOn` / `TestEveryVerifiedColumnSaysSo` from Task 2.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, add after the `claude` entry this `matrixAgents` entry:

```go
	{
		id: "antigravity", name: "Antigravity", present: []string{".gemini/config/"},
		user:    []string{".gemini/config/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".gemini/GEMINI.md"},
	},
```

add `"antigravity": "verified on agy 1.1.27, 2026-10-27",` to `verifiedOn`, and add:

```go
// Antigravity and Gemini CLI share ~/.gemini/. A machine with only Antigravity gets
// Antigravity's skill and nothing of Gemini CLI's.
func TestSkillInstallForAntigravityAloneWritesNoGeminiExtension(t *testing.T) {
	home := fakeHome(t, ".gemini/config")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "extensions")); !os.IsNotExist(err) {
		t.Errorf("a machine with only Antigravity got ~/.gemini/extensions/ (err %v)", err)
	}
	if got := homeFile(t, home, ".gemini/config/skills/rtdd/SKILL.md"); !strings.Contains(got, "name: rtdd") {
		t.Errorf("Antigravity's skill is not the rtdd skill:\n%s", got)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestSkillInstallForAntigravityAloneWritesNoGeminiExtension' -count=1
```

Expected: FAIL — `TestAgentMatrixColumnsAreTheAgentsTable` (no `antigravity` row), `TestAgentMatrix/antigravity/*` and `TestSkillInstallForAntigravityAloneWritesNoGeminiExtension`: `a machine with only Antigravity got ~/.gemini/extensions/`.

- [ ] **Step 3: The table row.** Add the `antigravity` row after `claude`. In the `gemini` row, `Present` becomes `[]string{".gemini/settings.json", ".gemini/oauth_creds.json"}`; in `matrixAgents`, the `gemini` entry's `present` becomes `[]string{".gemini/settings.json"}`. `ProjectSkillFiles` gains `.agents/skills/rtdd/SKILL.md` through the row; `TestInitProjectSkillWritesTheWholeFileFrontEndsOnly` stays green unmodified (it checks two files carry the marker, not that only two exist). Four existing tests make Gemini CLI present with a bare `.gemini/` directory, which is exactly the gate this task narrows: `TestPlanGlobalWritesAGeminiExtension`, `TestPlanGlobalWritesOnlyRtddOwnedDirectories`, `TestPlanGlobalUninstallRestoresTheHostsFiles` (`internal/install`) and `TestSkillInstallCreatesOnlyRtddOwnedDirectories` (`cmd/rtdd`). Each seeds `.gemini/settings.json` instead, in the same commit, and keeps every assertion it has. The `antigravity` row of `Agents()` after this task:

```go
	{
		ID: "antigravity", Name: "Antigravity",
		Present: []string{".gemini/config/"},
		Owned:   ".gemini/config/skills/rtdd/",
		User:    []GlobalFile{{Rel: ".gemini/config/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
		Limits: []string{
			"also reads `GEMINI.md` and `AGENTS.md` as rules; rtdd writes neither",
		},
		Verified: "verified on agy 1.1.27, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, Antigravity's row: user level `~/.gemini/config/skills/rtdd/`, project level `.agents/skills/rtdd/SKILL.md`, `verified on agy 1.1.27, 2026-10-27`, and its limitations: also reads `GEMINI.md` and `AGENTS.md` as rules; rtdd writes neither. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read Antigravity's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 35 subtests (5 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): Antigravity — ~/.gemini/config/skills/rtdd/ as a verified matrix column (closes #<the Antigravity child>)"
```

---

### Task 4: OpenAI Codex CLI — `~/.agents/skills/rtdd/`, the old `~/.codex/skills/rtdd/` retired

**Issue:** the Codex child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4

Codex's documentation names `~/.agents/skills/` and no longer mentions `~/.codex/skills/`, where v0.3.0 writes (`docs/agent-capabilities.md`). The skill moves, `Needs`/`NeedsNote` go (any Codex that reads skills reads `~/.agents/skills/`), and `Retired: [".codex/skills/rtdd/"]` makes `PlanGlobal` delete the old directory when every file in it carries `protocol.Generated`, and keep it — with the note `kept; it holds files rtdd did not write` — when it does not. `PlanGlobalLegacyFiles` reports a retired directory that is still there.

**Files:**
- Modify: `internal/install/agents.go`, `internal/install/global.go`, `internal/install/global_test.go`, `internal/install/legacy.go`, `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `codex` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["codex"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`, and `verifiedOn` / `TestEveryVerifiedColumnSaysSo` from Task 2.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, replace the `codex` entry with this `matrixAgents` entry:

```go
	{
		id: "codex", name: "Codex", present: []string{".codex/"},
		user:    []string{".agents/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".codex/AGENTS.md"},
		legacy:  []string{".codex/AGENTS.md"},
	},
```

add `"codex": "verified on codex-cli 0.154.0, 2026-10-27",` to `verifiedOn`, and add:

```go
// v0.3.0 wrote ~/.codex/skills/rtdd/. Installing now moves rtdd's render to
// ~/.agents/skills/rtdd/ and removes the old copy, so Codex never loads two — but a file
// the user put there is theirs, and it and its directory stay.
func TestSkillInstallRetiresTheOldCodexSkillDirectory(t *testing.T) {
	home := fakeHome(t, ".codex")
	writeFile(t, home, ".codex/skills/rtdd/SKILL.md", protocol.Generated+"\nv0.3.0's skill\n")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "rtdd")); !os.IsNotExist(err) {
		t.Errorf("the old ~/.codex/skills/rtdd/ is still there (err %v)", err)
	}
	homeFile(t, home, ".agents/skills/rtdd/SKILL.md")

	home = fakeHome(t, ".codex")
	writeFile(t, home, ".codex/skills/rtdd/SKILL.md", protocol.Generated+"\nv0.3.0's skill\n")
	writeFile(t, home, ".codex/skills/rtdd/notes.md", "mine\n")
	_, stdout, _ := rtdd(t, t.TempDir(), "skill", "install")
	if homeFile(t, home, ".codex/skills/rtdd/notes.md") != "mine\n" {
		t.Error("a file the user put in the old directory was not kept")
	}
	if !strings.Contains(stdout, "kept; it holds files rtdd did not write") {
		t.Errorf("the output does not say why the old directory stayed:\n%s", stdout)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestSkillInstallRetiresTheOldCodexSkillDirectory' -count=1
```

Expected: FAIL — `TestAgentMatrix/codex/user-install`: `created [], want exactly [.agents/skills/rtdd/SKILL.md]` (the v0.3.0 `Needs` gate skips a home with `.codex/` and no `.codex/skills/`); `TestSkillInstallRetiresTheOldCodexSkillDirectory`: `the old ~/.codex/skills/rtdd/ is still there`.

- [ ] **Step 3: The table row.** Rewrite the `codex` row as below. In `internal/install/global.go`, `PlanGlobal` appends, after a surface's writes, one step per `Retired` directory that exists: `Delete` when every file in it carries `protocol.Generated`, else a `Delete` per such file and a `Skip` "kept; it holds files rtdd did not write" (`PlanGlobalUninstall`'s rule, reused, not copied). `PlanGlobalUninstall` also removes a `Retired` directory. `TestPlanGlobalSkipsACodexWithNoSkillsDirectory` asserted the v0.3.0 gate this task removes; it is replaced, in the same commit, by `TestPlanGlobalWritesCodexToTheSharedAgentsDirectory` (a home with `.codex/` and no `.codex/skills/` gets `.agents/skills/rtdd/SKILL.md`) — the behaviour it pins is the bug. Three more existing tests hard-code `.codex/skills/rtdd/` and move with it, in the same commit, to `.agents/skills/rtdd/`: `TestPlanGlobalWritesOnlyRtddOwnedDirectories` (`internal/install`), `TestSkillInstallCreatesOnlyRtddOwnedDirectories` and `TestUninstallGlobalDryRunPrintsThePlanAndChangesNothing` (`cmd/rtdd`). Each keeps every assertion it has; only the path changes. The `codex` row of `Agents()` after this task:

```go
	{
		ID: "codex", Name: "Codex",
		Present: []string{".codex/"},
		Owned:   ".agents/skills/rtdd/",
		User:    []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
		Retired: []string{".codex/skills/rtdd/"},
		Limits: []string{
			"`~/.agents/skills/` is shared with other agents; `rtdd skill uninstall` removes rtdd's copy for all of them",
		},
		Verified: "verified on codex-cli 0.154.0, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, Codex's row: user level `~/.agents/skills/rtdd/`, project level `.agents/skills/rtdd/SKILL.md`, `verified on codex-cli 0.154.0, 2026-10-27`, and its limitations: `~/.agents/skills/` is shared with other agents; `rtdd skill uninstall` removes rtdd's copy for all of them. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read Codex's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 35 subtests (5 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): Codex — ~/.agents/skills/rtdd/ as a verified matrix column (closes #<the Codex child>)"
```

---

### Task 5: Gemini CLI — the extension kept, its gate checked against 0.65.0, the trusted-folder project skill

**Issue:** the Gemini CLI child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4

The extension `~/.gemini/extensions/rtdd/` stays: a hand-placed extension folder loads without `gemini extensions install` and is enabled by default (`docs/agent-capabilities.md`). This task confirms against the `@google/gemini-cli` 0.65.0 package which files Gemini CLI itself writes to `~/.gemini/` — the `Present` list Task 3 set — corrects the list if the package says otherwise, and gives Gemini CLI the project file `.agents/skills/rtdd/SKILL.md`, which it reads in a trusted folder.

**Files:**
- Modify: `internal/install/agents.go`, `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `gemini` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["gemini"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`, and `verifiedOn` / `TestEveryVerifiedColumnSaysSo` from Task 2.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, replace the `gemini` entry with this `matrixAgents` entry:

```go
	{
		id: "gemini", name: "Gemini CLI", present: []string{".gemini/settings.json"},
		user:    []string{".gemini/extensions/rtdd/GEMINI.md", ".gemini/extensions/rtdd/gemini-extension.json"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".gemini/GEMINI.md"},
		legacy:  []string{".gemini/GEMINI.md"},
	},
```

add `"gemini": "verified on Gemini CLI 0.65.0, 2026-10-27",` to `verifiedOn`, and add:

```go
// Gemini CLI reads the extension's context file by the manifest's contextFileName; a
// manifest naming any other file would load an extension with no instructions in it.
func TestGeminiExtensionManifestNamesItsContextFile(t *testing.T) {
	home := fakeHome(t)
	writeFile(t, home, ".gemini/settings.json", "{}\n")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	var m struct{ Name, Version, ContextFileName string }
	if err := json.Unmarshal([]byte(homeFile(t, home, ".gemini/extensions/rtdd/gemini-extension.json")), &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "rtdd" || m.ContextFileName != "GEMINI.md" || m.Version == "" {
		t.Errorf("manifest = %+v, want name rtdd, a version, contextFileName GEMINI.md", m)
	}
	homeFile(t, home, ".gemini/extensions/rtdd/"+m.ContextFileName)
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestGeminiExtensionManifestNamesItsContextFile' -count=1
```

Expected: FAIL — `TestEveryVerifiedColumnSaysSo` (`gemini: Verified = ""`) and `TestAgentMatrix/gemini/project-opt-in`: `install.Agents() gemini Project = [], want [.agents/skills/rtdd/SKILL.md]` (the file itself is already written for Antigravity, so only the row's own claim can fail).

- [ ] **Step 3: The table row.** In the `gemini` row: `Project` gains `.agents/skills/rtdd/SKILL.md`, `Limits` and `Verified` as below. Before setting `Present`, extract `@google/gemini-cli@0.65.0` with `npm pack` into an empty scratch directory (never run it) and grep `bundle/` for the files written under the user directory (`settings.json`, `oauth_creds.json`, `installation_id`, `google_accounts.json`); keep in `Present` only names Antigravity's agy 1.1.27 does not also write — `installation_id` and `google_accounts.json` are written by both on this machine — and record the finding in `docs/agent-capabilities.md`. The `gemini` row of `Agents()` after this task:

```go
	{
		ID: "gemini", Name: "Gemini CLI",
		Present: []string{".gemini/settings.json", ".gemini/oauth_creds.json"},
		Owned:   ".gemini/extensions/rtdd/",
		User: []GlobalFile{
			{Rel: ".gemini/extensions/rtdd/gemini-extension.json", Literal: geminiManifest, Marker: `"name": "rtdd"`},
			{Rel: ".gemini/extensions/rtdd/GEMINI.md", Source: protocol.GlobalAgentsPath, Marker: protocol.BeginMarker},
		},
		Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
		Limits: []string{
			"the project skill loads only in a folder Gemini CLI trusts",
		},
		Verified: "verified on Gemini CLI 0.65.0, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, Gemini CLI's row: user level `~/.gemini/extensions/rtdd/`, project level `.agents/skills/rtdd/SKILL.md`, `verified on Gemini CLI 0.65.0, 2026-10-27`, and its limitations: the project skill loads only in a folder Gemini CLI trusts. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read Gemini CLI's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 35 subtests (5 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): Gemini CLI — ~/.gemini/extensions/rtdd/ as a verified matrix column (closes #<the Gemini CLI child>)"
```

---

### Task 6: GitHub Copilot CLI — `~/.copilot/skills/rtdd/`, never `copilot-instructions.md`

**Issue:** the Copilot CLI child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4

Copilot CLI reads skills from `~/.copilot/skills/` and, in a repository, `.agents/skills/` among others (`docs/agent-capabilities.md`). Its `copilot-instructions.md` files are shared instruction files and are never written.

**Files:**
- Modify: `internal/install/agents.go`, `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `copilot` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["copilot"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`, and `verifiedOn` / `TestEveryVerifiedColumnSaysSo` from Task 2.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, add after the `gemini` entry this `matrixAgents` entry:

```go
	{
		id: "copilot", name: "Copilot CLI", present: []string{".copilot/"},
		user:    []string{".copilot/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".copilot/copilot-instructions.md"},
	},
```

add `"copilot": "verified on @github/copilot 1.0.104, 2026-10-27",` to `verifiedOn`, and add:

```go
// Copilot's instruction files are the user's; rtdd's way in is the skills directory.
func TestSkillInstallNeverWritesCopilotInstructions(t *testing.T) {
	home := fakeHome(t, ".copilot")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	for _, p := range []string{".copilot/copilot-instructions.md", ".copilot/instructions"} {
		if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(p))); !os.IsNotExist(err) {
			t.Errorf("rtdd skill install created %s", p)
		}
	}
	homeFile(t, home, ".copilot/skills/rtdd/SKILL.md")
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestSkillInstallNeverWritesCopilotInstructions' -count=1
```

Expected: FAIL — `TestAgentMatrixColumnsAreTheAgentsTable` (no `copilot` row) and `TestSkillInstallNeverWritesCopilotInstructions`: `read .copilot/skills/rtdd/SKILL.md: no such file or directory`.

- [ ] **Step 3: The table row.** Add the `copilot` row after `gemini`. The `copilot` row of `Agents()` after this task:

```go
	{
		ID: "copilot", Name: "Copilot CLI",
		Present: []string{".copilot/"},
		Owned:   ".copilot/skills/rtdd/",
		User:    []GlobalFile{{Rel: ".copilot/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
		Limits: []string{
			"a `COPILOT_HOME` other than `~/.copilot` is not followed; set `RTDD_HOME` to its parent",
		},
		Verified: "verified on @github/copilot 1.0.104, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, Copilot CLI's row: user level `~/.copilot/skills/rtdd/`, project level `.agents/skills/rtdd/SKILL.md`, `verified on @github/copilot 1.0.104, 2026-10-27`, and its limitations: a `COPILOT_HOME` other than `~/.copilot` is not followed; set `RTDD_HOME` to its parent. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read Copilot CLI's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 42 subtests (6 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): Copilot CLI — ~/.copilot/skills/rtdd/ as a verified matrix column (closes #<the Copilot CLI child>)"
```

---

### Task 7: Cursor — a user-level skill at `~/.cursor/skills/rtdd/`, the project rule kept

**Issue:** the Cursor child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4

v0.3.0 said Cursor has no file-based user level; Cursor 2.4 reads Agent Skills from `~/.cursor/skills/` (`docs/agent-capabilities.md`). Cursor gains a user surface, its `limit` in `matrixAgents` empties (it is no longer a skip), and its limitations move to the README cell. The project rule `.cursor/rules/rtdd.mdc` stays; Cursor also reads `.agents/skills/` and `.claude/skills/`, so with `--project-skill` it may see rtdd more than once, each copy the same process.

**Files:**
- Modify: `internal/install/agents.go`, `internal/install/global.go` (doc comment), `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `cursor` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["cursor"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`, and `verifiedOn` / `TestEveryVerifiedColumnSaysSo` from Task 2.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, replace the `cursor` entry with this `matrixAgents` entry:

```go
	{
		id: "cursor", name: "Cursor", present: []string{".cursor/"},
		user:    []string{".cursor/skills/rtdd/SKILL.md"},
		project: []string{".cursor/rules/rtdd.mdc"},
	},
```

add `"cursor": "verified on Cursor 2.4, 2026-10-27",` to `verifiedOn`, and add:

```go
// Cursor 2.4 reads ~/.cursor/skills/; installing writes rtdd's skill there and no rule
// file, because User Rules are not files.
func TestCursorGetsAUserLevelSkill(t *testing.T) {
	home := fakeHome(t, ".cursor")
	_, stdout, _ := rtdd(t, t.TempDir(), "skill", "install")
	if got := homeFile(t, home, ".cursor/skills/rtdd/SKILL.md"); !strings.Contains(got, "name: rtdd") {
		t.Errorf("Cursor's user skill is not the rtdd skill:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".cursor", "rules")); !os.IsNotExist(err) {
		t.Error("rtdd skill install created ~/.cursor/rules/")
	}
	if strings.Contains(stdout, "user rules live in its settings") {
		t.Errorf("the install still reports Cursor as having no user level:\n%s", stdout)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestCursorGetsAUserLevelSkill' -count=1
```

Expected: FAIL — `TestCursorGetsAUserLevelSkill`: `read .cursor/skills/rtdd/SKILL.md: no such file or directory`; `TestAgentMatrix/cursor/user-install`: `created [], want exactly [.cursor/skills/rtdd/SKILL.md]`.

- [ ] **Step 3: The table row.** Rewrite the `cursor` row as below. Remove the "Cursor is absent on purpose" paragraph from `GlobalSurfaces`' doc comment and the Cursor sentence from the README's "Install" section; `rtdd skill prompt` stays for agents outside the table. The `cursor` row of `Agents()` after this task:

```go
	{
		ID: "cursor", Name: "Cursor",
		Present: []string{".cursor/"},
		Owned:   ".cursor/skills/rtdd/",
		User:    []GlobalFile{{Rel: ".cursor/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		Project: []GlobalFile{{Rel: ".cursor/rules/rtdd.mdc", Source: "dist/cursor/rules/rtdd.mdc", Marker: protocol.Generated}},
		Limits: []string{
			"User Rules live in Cursor's settings and are not written",
			"the `agent` CLI reading `~/.cursor/skills/` is unverified",
		},
		Verified: "verified on Cursor 2.4, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, Cursor's row: user level `~/.cursor/skills/rtdd/`, project level `.cursor/rules/rtdd.mdc`, `verified on Cursor 2.4, 2026-10-27`, and its limitations: User Rules live in Cursor's settings and are not written; the `agent` CLI reading `~/.cursor/skills/` is unverified. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read Cursor's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 42 subtests (6 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): Cursor — ~/.cursor/skills/rtdd/ as a verified matrix column (closes #<the Cursor child>)"
```

---

### Task 8: OpenCode — `~/.config/opencode/skills/rtdd/`

**Issue:** the OpenCode child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4

OpenCode reads `~/.config/opencode/skills/<name>/SKILL.md` and requires `name` to equal the directory (`docs/agent-capabilities.md`). `TestEverySkillFrontmatterFitsEveryAgent` (Task 1) already pins `name: rtdd`.

**Files:**
- Modify: `internal/install/agents.go`, `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `opencode` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["opencode"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`, and `verifiedOn` / `TestEveryVerifiedColumnSaysSo` from Task 2.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, add after the `cursor` entry this `matrixAgents` entry:

```go
	{
		id: "opencode", name: "OpenCode", present: []string{".config/opencode/"},
		user:    []string{".config/opencode/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".config/opencode/AGENTS.md"},
	},
```

add `"opencode": "verified on opencode-ai 1.19.4, 2026-10-27",` to `verifiedOn`, and add:

```go
// OpenCode rejects a skill whose frontmatter name differs from its directory.
func TestOpenCodeSkillNameEqualsItsDirectory(t *testing.T) {
	home := fakeHome(t, ".config/opencode")
	rtdd(t, t.TempDir(), "skill", "install")
	rel := ".config/opencode/skills/rtdd/SKILL.md"
	dirName := filepath.Base(filepath.Dir(filepath.FromSlash(rel)))
	if got := homeFile(t, home, rel); !strings.Contains(got, "\nname: "+dirName+"\n") {
		t.Errorf("%s does not declare name: %s:\n%s", rel, dirName, got)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestOpenCodeSkillNameEqualsItsDirectory' -count=1
```

Expected: FAIL — `TestAgentMatrixColumnsAreTheAgentsTable` (no `opencode` row) and `TestOpenCodeSkillNameEqualsItsDirectory`: `read .config/opencode/skills/rtdd/SKILL.md: no such file or directory`.

- [ ] **Step 3: The table row.** Add the `opencode` row after `cursor`. The `opencode` row of `Agents()` after this task:

```go
	{
		ID: "opencode", Name: "OpenCode",
		Present: []string{".config/opencode/"},
		Owned:   ".config/opencode/skills/rtdd/",
		User:    []GlobalFile{{Rel: ".config/opencode/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
		Limits: []string{
			"also reads `~/.claude/skills/` and `~/.agents/skills/`, so it may list rtdd more than once; every copy is the same text",
		},
		Verified: "verified on opencode-ai 1.19.4, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, OpenCode's row: user level `~/.config/opencode/skills/rtdd/`, project level `.agents/skills/rtdd/SKILL.md`, `verified on opencode-ai 1.19.4, 2026-10-27`, and its limitations: also reads `~/.claude/skills/` and `~/.agents/skills/`, so it may list rtdd more than once; every copy is the same text. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read OpenCode's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 49 subtests (7 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): OpenCode — ~/.config/opencode/skills/rtdd/ as a verified matrix column (closes #<the OpenCode child>)"
```

---

### Task 9: Zcode (Z.ai ZCode) — `~/.zcode/skills/rtdd/`, no project surface

**Issue:** the Zcode child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4

PRD #511 asked Task 1 to confirm the official tool named Zcode: it is Z.ai's ZCode, a desktop application with no official CLI, reading `~/.zcode/skills/<name>/SKILL.md` (`docs/agent-capabilities.md`). It is supported at user level. Its project-level path is documented only by a third party, so Zcode has no `Project` until Z.ai documents one — `Decisions this plan settles`, 7.

**Files:**
- Modify: `internal/install/agents.go`, `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `zcode` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["zcode"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`, and `verifiedOn` / `TestEveryVerifiedColumnSaysSo` from Task 2.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, add after the `opencode` entry this `matrixAgents` entry:

```go
	{
		id: "zcode", name: "Zcode", present: []string{".zcode/"},
		user:    []string{".zcode/skills/rtdd/SKILL.md"},
	},
```

add `"zcode": "verified on ZCode 3.16.3, 2026-10-27",` to `verifiedOn`, and add:

```go
// Zcode's project path is undocumented, so --project-skill writes nothing on its behalf.
func TestZcodeHasNoProjectSurface(t *testing.T) {
	if p := agentRow(t, "zcode").Project; len(p) != 0 {
		t.Errorf("zcode has a project surface %v; no Z.ai document names one", p)
	}
	home := fakeHome(t, ".zcode")
	rtdd(t, t.TempDir(), "skill", "install")
	homeFile(t, home, ".zcode/skills/rtdd/SKILL.md")
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestZcodeHasNoProjectSurface' -count=1
```

Expected: FAIL — `TestAgentMatrixColumnsAreTheAgentsTable` (no `zcode` row) and `TestZcodeHasNoProjectSurface`: `install.Agents() has no "zcode"`.

- [ ] **Step 3: The table row.** Add the `zcode` row after `opencode`. The `zcode` row of `Agents()` after this task:

```go
	{
		ID: "zcode", Name: "Zcode",
		Present: []string{".zcode/"},
		Owned:   ".zcode/skills/rtdd/",
		User:    []GlobalFile{{Rel: ".zcode/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		Limits: []string{
			"a desktop app, not a CLI; no documented project-level path, so `--project-skill` writes nothing for it",
			"press Refresh under Settings \u2192 Skills after installing",
		},
		Verified: "verified on ZCode 3.16.3, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, Zcode's row: user level `~/.zcode/skills/rtdd/`, project level none, `verified on ZCode 3.16.3, 2026-10-27`, and its limitations: a desktop app, not a CLI; no documented project-level path, so `--project-skill` writes nothing for it; press Refresh under Settings → Skills after installing. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read Zcode's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 56 subtests (8 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): Zcode — ~/.zcode/skills/rtdd/ as a verified matrix column (closes #<the Zcode child>)"
```

---

### Task 10: Hermes Agent (Nous Research) — `~/.hermes/skills/rtdd/`

**Issue:** the Hermes Agent child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4

Hermes reads `~/.hermes/skills/` — "the primary directory and source of truth" — and `<git root>/.agents/skills/` once the user trusts the project (`docs/agent-capabilities.md`). rtdd never edits `~/.hermes/config.yaml` to add an external directory.

**Files:**
- Modify: `internal/install/agents.go`, `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `hermes` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["hermes"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`, and `verifiedOn` / `TestEveryVerifiedColumnSaysSo` from Task 2.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, add after the `zcode` entry this `matrixAgents` entry:

```go
	{
		id: "hermes", name: "Hermes Agent", present: []string{".hermes/"},
		user:    []string{".hermes/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".hermes/config.yaml"},
	},
```

add `"hermes": "verified on Hermes Agent v0.22.5, 2026-10-27",` to `verifiedOn`, and add:

```go
// ~/.hermes/config.yaml is the user's; the skills directory needs no entry in it.
func TestSkillInstallLeavesHermesConfigAlone(t *testing.T) {
	home := fakeHome(t, ".hermes")
	writeFile(t, home, ".hermes/config.yaml", "skills:\n  external_dirs: []\n")
	rtdd(t, t.TempDir(), "skill", "install")
	if got := homeFile(t, home, ".hermes/config.yaml"); got != "skills:\n  external_dirs: []\n" {
		t.Errorf("rtdd skill install changed ~/.hermes/config.yaml:\n%s", got)
	}
	homeFile(t, home, ".hermes/skills/rtdd/SKILL.md")
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestSkillInstallLeavesHermesConfigAlone' -count=1
```

Expected: FAIL — `TestAgentMatrixColumnsAreTheAgentsTable` (no `hermes` row) and `TestSkillInstallLeavesHermesConfigAlone`: `read .hermes/skills/rtdd/SKILL.md: no such file or directory`.

- [ ] **Step 3: The table row.** Add the `hermes` row after `zcode`. The `hermes` row of `Agents()` after this task:

```go
	{
		ID: "hermes", Name: "Hermes Agent",
		Present: []string{".hermes/"},
		Owned:   ".hermes/skills/rtdd/",
		User:    []GlobalFile{{Rel: ".hermes/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
		Limits: []string{
			"the project skill loads only after `hermes skills trust`, which rtdd never runs",
		},
		Verified: "verified on Hermes Agent v0.22.5, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, Hermes Agent's row: user level `~/.hermes/skills/rtdd/`, project level `.agents/skills/rtdd/SKILL.md`, `verified on Hermes Agent v0.22.5, 2026-10-27`, and its limitations: the project skill loads only after `hermes skills trust`, which rtdd never runs. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read Hermes Agent's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 63 subtests (9 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): Hermes Agent — ~/.hermes/skills/rtdd/ as a verified matrix column (closes #<the Hermes Agent child>)"
```

---

### Task 11: OpenClaw — `~/.openclaw/skills/rtdd/`, and the matrix complete

**Issue:** the OpenClaw child of #511, filed from this plan

**Discharges:** AC1, AC2, AC4, AC3

OpenClaw reads `~/.openclaw/skills/` and, in its workspace, `.agents/skills/` (`docs/agent-capabilities.md`). The workspace's `AGENTS.md` is injected every session and is the user's. This is the last column: the task ends by asserting PRD #511's ten agents are all columns and all verified, and README "What rtdd writes" lists every path in the surface table.

**Files:**
- Modify: `internal/install/agents.go`, `cmd/rtdd/agent_matrix_test.go`, `README.md`, `docs/agent-capabilities.md` (only if the tool's current release moved a fact)

**Interfaces:**
- Produces: the `openclaw` row of `install.Agents()`; its `matrixAgents` entry and `verifiedOn["openclaw"]`.
- Consumes: Task 1's `Agent`, `GlobalFile`, `matrixAgents`, `matrixRows`, `agentRow`, `fakeHome`, `homeFile`, and `verifiedOn` / `TestEveryVerifiedColumnSaysSo` from Task 2.

- [ ] **Step 1: Write the failing tests.** In `cmd/rtdd/agent_matrix_test.go`, add after the `hermes` entry this `matrixAgents` entry:

```go
	{
		id: "openclaw", name: "OpenClaw", present: []string{".openclaw/"},
		user:    []string{".openclaw/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".openclaw/workspace/AGENTS.md"},
	},
```

add `"openclaw": "verified on openclaw 2026.10.7, 2026-10-27",` to `verifiedOn`, and add:

```go
// PRD #511's ten agents, every one a column, every column verified.
func TestEveryAgentInScopeIsAVerifiedMatrixColumn(t *testing.T) {
	want := []string{"claude", "antigravity", "codex", "gemini", "copilot", "cursor", "opencode", "zcode", "hermes", "openclaw"}
	var got []string
	for _, a := range matrixAgents {
		got = append(got, a.id)
		if verifiedOn[a.id] == "" {
			t.Errorf("%s is a matrix column with no verifiedOn entry", a.id)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("matrix columns = %v, want PRD #511's %v", got, want)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd ./internal/install -run 'TestAgentMatrix|TestEveryVerifiedColumnSaysSo|TestEveryAgentInScopeIsAVerifiedMatrixColumn' -count=1
```

Expected: FAIL — `TestAgentMatrixColumnsAreTheAgentsTable` (no `openclaw` row), `TestEveryVerifiedColumnSaysSo` and every `TestAgentMatrix/openclaw/*` row: `install.Agents() has no "openclaw"`. `TestEveryAgentInScopeIsAVerifiedMatrixColumn` passes from this step on — it is the guard that stops a later change from dropping a column, not this task's red.

- [ ] **Step 3: The table row.** Add the `openclaw` row after `hermes`. In the README's "What rtdd writes", check every path in this plan's surface table is listed exactly once. The `openclaw` row of `Agents()` after this task:

```go
	{
		ID: "openclaw", Name: "OpenClaw",
		Present: []string{".openclaw/"},
		Owned:   ".openclaw/skills/rtdd/",
		User:    []GlobalFile{{Rel: ".openclaw/skills/rtdd/SKILL.md", Source: protocol.GlobalSkillPath, Marker: protocol.Generated}},
		Project: []GlobalFile{{Rel: ".agents/skills/rtdd/SKILL.md", Source: "dist/SKILL.md", Marker: protocol.Generated}},
		Limits: []string{
			"its project is a workspace (`~/.openclaw/workspace` by default), so the project skill is read only when the repository is the workspace",
		},
		Verified: "verified on openclaw 2026.10.7, 2026-10-27",
	},
```

- [ ] **Step 4: README.** In `## Supported agents`, OpenClaw's row: user level `~/.openclaw/skills/rtdd/`, project level `.agents/skills/rtdd/SKILL.md`, `verified on openclaw 2026.10.7, 2026-10-27`, and its limitations: its project is a workspace (`~/.openclaw/workspace` by default), so the project skill is read only when the repository is the workspace. In `## What rtdd writes`, one row per path above that is not already listed.

- [ ] **Step 5: Re-check the facts.** Read OpenClaw's row of `docs/agent-capabilities.md` against its cited sources. If the tool's current release moved a path or a format, change the row, its citation, the surface table in this plan and `Verified` together, and say so in the commit body.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
go test ./cmd/rtdd ./internal/install -count=1
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS; `TestAgentMatrix` runs 70 subtests (10 agents × 7 rows).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(install): OpenClaw — ~/.openclaw/skills/rtdd/ as a verified matrix column (closes #<the OpenClaw child>)"
```


## What this plan deliberately leaves undone

Everything below is real work; none of it belongs to PRD #511, and no task above may start it.

- **Harness's CLI adapters.** Teaching Harness to launch each agent CLI is the sibling PRD in VocanicZ/Harness; this plan only makes rtdd's instructions reach each agent and keeps `RTDD_EVENT_LOG` and `HARNESS_*` behaving the same under each.
- **Selection quality** — #504, #505, #507, #508 and whatever follows. What Round 1 and Round 2 contain — selection — is not changed by any task here; the matrix asserts that `rtdd which` answers the same under each agent, not that the answer is good.
- **Running an agent CLI in a test.** The matrix proves rtdd writes the right file in the right place; that the tool then reads it is cited, not executed. A live smoke test per tool would need ten installed CLIs and their accounts on the CI host.
- **An agent's shared memory file.** No task appends to `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.github/copilot-instructions.md` or any settings pane unless the user names it with `--append-to`.
- **Agents outside PRD #511's list.** `rtdd skill prompt` remains the answer for every other tool.
- **Tagging and releasing** the version that ships this — a human action, as for v0.3.0.
