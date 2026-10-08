# Skill and Install (N3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ] `) syntax for tracking.

**Goal:** Make what every agent reads, and the command that installs it, describe v0.3.0. `protocol/PROTOCOL.md` — and so every generated front-end (skill, agents, mdc, global, global-agents) — states spec §10's five-step rounds process, says rtdd runs no tests, says empty Rounds 1 and 2 are "no linked test", never a pass, describes graphify as optional, and carries no v0.2 word, no language and no test framework. `rtdd init` sets up every git repository: it writes the front-ends, a `.rtdd/config.yaml` the graph loads, and the `.rtdd/graph.json` ignore line, and on a v0.2 repository it deletes that release's state and says so. README, DEVELOPMENT.md and the user-facing `docs/` pages describe v0.3.0, and the superseded specs say they are superseded.

**Architecture:** Three surfaces, each already one module. **The text** is `protocol/PROTOCOL.md`, parsed and rendered by `internal/protocol` (`targets.go` holds the five targets, their `Required` section ids, the skill descriptions and the Cursor globs); `rtdd-gen render` writes `dist/`, `rtdd-gen check`/`verify` hold it there, `internal/protocol/testdata/golden/` pins the render byte for byte, and `internal/install/protocol.md` is the copy embedded in the binary. N3 rewrites the source and regenerates the other four exactly as they are regenerated today; it adds two tests on the render (`internal/protocol`) and one guard over `dist/` (`internal/contract`). **The install** is `internal/install.Plan` + `Apply`, called by `cmd/rtdd/init.go`: `Plan` loses its `force` and adapter parameters, gains a migration step list (`PlanMigration`, new `migrate.go`), a `.gitignore` step and a config written from `graph.DefaultConfig()` (`DefaultConfig`, rewritten `config.go`), so the config `init` writes and the defaults the graph applies are one list. **The docs** are prose plus two guards in `internal/contract` and a README-example test in `cmd/rtdd`.

**Tech Stack:** Go 1.24 (module `github.com/VocanicZ/rtdd`), stdlib plus `gopkg.in/yaml.v3` — already the module's one dependency, used here by `isV02Config` to read a config's keys. Tests use stdlib `testing` and real git repositories built with `internal/gitctx/gittest`. No toolchain of any language is invoked.

**Spec:** [`docs/specs/2026-10-07-node-graph.md`](../specs/2026-10-07-node-graph.md) §8 (`init`: no refusal, the v0.2 removals), §10 (the skill and its five steps), §11 (non-goals: no test execution, no coverage, no running graphify) — PRD #411. §4.5 (the gitignored cache), §5 (graphify) and §12 (settled defaults) are read, not changed. Each task names the sibling issue it is and the PRD acceptance criteria it discharges; the map after the decisions lists all eight.

## Global Constraints

- **rtdd runs no test.** Nothing N3 adds executes a test runner, a compiler, a coverage tool or graphify; `rtdd init` runs no child process but git through `internal/gitctx` (it calls `gitctx.FindRepoRoot` only). The front-ends say so in those words: "rtdd runs no tests".
- **The front-ends name no language and no test framework.** The agent runs each round with "the project's own test command"; rtdd never names one. The `dist/` guard (Task 1) fails on a language name, a test framework or toolchain name, and every v0.2 word. The Cursor rule attaches to every file (`MdcGlobs = "**/*"`): a list of extensions is a list of languages.
- **rtdd never runs graphify.** The front-ends describe graphify as optional — used when `graphify-out/graph.json` exists, never trusted for changed files, never run by rtdd — and never tell the agent rtdd runs or updates it (spec §5, §11). N3 never runs graphify in a test either.
- **Only `internal/gitctx` shells out to git** — `TestOnlyGitctxShellsOutToGit` (`internal/contract/nomock_test.go`) stays green unmodified. N3 adds no git call.
- **No new runtime dependency, no cgo.** `go.mod` keeps exactly `gopkg.in/yaml.v3` (`TestGoModRequiresExactlyYAML`); the release binary stays `CGO_ENABLED=0` and statically linked.
- **Out of scope:** graph building, the scanner and graphify loading — PRD **#409**; `Rounds`, `which`, `explain`, `doctor`, `graph` behaviour, exit codes and `--json` schema 3 — PRD **#410** (N3 describes them; it does not change them). Pushing a `v*` tag or creating a release — a human action, **#418**. rtdd-bench, the bench's rtdd strategy and its published results — PRD **#412**.
- **Spec §12 is settled, not open:** depth-1 neighbours in both directions; all same-named definitions linked, `doctor` lists names defined ≥ 8 times; `.rtdd/graph.json` a gitignored cache (#415's default — Task 3 writes the ignore line); graphify ignored past 50 % stale and never run; v0.3.0 tagged by a human. No task revisits these.
- `scripts/ci-local.sh` exits 0 at the end of every task, run with **this branch's** `rtdd` first on PATH, as `ci.yml`'s prereg job does:

  ```bash
  bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
  ```

## Review Focus

- **A phrase wrapped across two source lines.** PROTOCOL.md is hard-wrapped, so "never trusted for changed files" can end one line and start the next. Every phrase test compares whitespace-collapsed text (`flat`, Task 1); a raw `strings.Contains` would fail on a wrap and pass on nothing useful.
- **The `dist/` guard tripping on ordinary English.** The five steps need "Run", "return", "go"; the guard forbids `run` only as `rtdd run` or a `` `run` `` code span, and language names only in their proper capitalisation. `TestV02VocabularyCatchesTheFormsAndSparesOrdinaryEnglish` (Task 1) pins both directions, so the guard can be neither vacuous nor a tax on prose.
- **The agents byte budget** (1800 bytes, `agentsMaxBytes`). The five steps land in AGENTS.md verbatim; the other agents bodies are short variants. Rendered with the text in "The protocol text", `dist/AGENTS.md` is 940 bytes after Task 1 and 1108 after Task 2, `dist/GLOBAL-AGENTS.md` 1091 and 1259 — checked by rendering it before this plan was written.
- **`--force` is removed, and nothing is lost by it.** v0.2's `--force` overrode two things: the no-adapter refusal (gone) and a differing whole-file front-end. The second is now decided by content: a file carrying `protocol.Generated` is an earlier rtdd render and is replaced; anything else is someone's own file and is a conflict no flag overrides. A v0.2 repository's skill carries the header, so migration replaces it without a flag. Tests in Tasks 3 and 4.
- **A v0.2 config versus a user's v0.3.0 config.** `init` replaces `.rtdd/config.yaml` only when it sets a v0.2 key and no v0.3.0 key; a file setting any v0.3.0 key is kept byte for byte. Unparseable YAML is kept, and `graph.LoadConfig` names it. Test in Task 4.
- **`.gitattributes` holding other lines.** Migration removes only the merge-driver line and deletes the file only when that line was all it held (uninstall's existing rule). A `.gitattributes` without the line is never planned — `TestPlanLeavesGitattributesAlone` stays green unmodified.
- **The two lanes editing PROTOCOL.md.** Task 2 adds one section and one `Required` id per target to what Task 1 wrote, and nothing else; it is blocked by Task 1 (#479 by #478) so it never rebases across a rewrite.

## The protocol text

`protocol/PROTOCOL.md` after Task 2 is exactly the file below. **Task 1 writes it without the `graphify` section** (the block from `<!-- rtdd:section id=graphify` through its `<!-- rtdd:endsection -->` and the blank line after it); **Task 2 adds that section** verbatim. Section ids, titles, targets and orders are part of the contract: the tests and `Required` lists name them.

````markdown
# RTDD agent protocol

This file is the single source for every generated agent front-end. Edit it, then run
`rtdd-gen render`. Do not edit anything under `dist/`.

<!-- rtdd:meta version=1 -->

<!-- rtdd:section id=setup title="Setting up a repository" targets=global,global-agents order=5 -->
Check for `.rtdd/config.yaml` first. It says whether rtdd is set up in this repository.

**It exists.** rtdd is set up. Follow the process below, and do not re-run `rtdd init`.

**It does not.** Run `rtdd init` once. It sets up every git repository, whatever the code is
written in: it writes these instructions for the agents the repository uses, a
`.rtdd/config.yaml` of defaults, and a `.gitignore` line for `.rtdd/graph.json`, rtdd's
rebuildable graph cache. In a repository an older rtdd set up, it also deletes the files
that release kept and prints one line for each file it removed. Commit what it wrote. There
is no other setup step: the graph is built the first time a command needs it.
<!-- rtdd:variant target=global-agents -->
If `.rtdd/config.yaml` is missing, run `rtdd init` once — it sets up any git repository —
and commit what it writes. Then follow the steps below.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=what title="What rtdd does" targets=skill,agents,mdc,global,global-agents order=10 -->
rtdd answers one question fast: which tests does this change need, and in what order. It
builds a graph of the repository's functions, methods and classes, links every test to the
code it calls, and reads the lines you changed against it:

- **Round 1** — the tests linked to the code you changed, and every test you changed.
- **Round 2** — the tests linked to that code's direct neighbours, its callers and its
  callees, minus Round 1.
- **Round 3** — the full suite, once, at the end of the task.

rtdd runs no tests. You run each round with the project's own test command, the one you
would use without rtdd. Nothing rtdd prints is a pass or a fail.
<!-- rtdd:variant target=agents -->
rtdd names the tests a change needs, in rounds: Round 1 tests the code you changed, Round 2
its direct neighbours, Round 3 is the full suite once at the end. rtdd runs no tests: you
run each round with the project's own test command.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=process title="The process" targets=skill,agents,mdc,global,global-agents order=20 -->
1. Edit code (test first, per TDD).
2. Run `rtdd which`. If `untested` names a node you changed, write its test first.
3. Run **Round 1** with the project's own test command. Fix until green.
4. Run **Round 2**. Fix until green; return to step 2 after any further edit.
5. When the task is done — before committing or handing off — run the **full suite once**.
<!-- rtdd:endsection -->

<!-- rtdd:section id=which title="rtdd which" targets=skill,agents,mdc,global,global-agents order=30 -->
```
rtdd which [--base <ref>] [--json]
```

Prints the changed nodes — the innermost function, method or class around each changed
line — then Round 1, Round 2, Round 3, the `untested` list and where the graph came from. It
runs nothing and answers in about a second, so ask it again after every edit.

The changed set is everything that differs from `--base` (default `HEAD`): committed since
it, staged, unstaged and untracked, so a file you just wrote counts before you commit it. A
changed line outside every function, such as an import, belongs to its file's `<module>`
node.

`untested` lists the changed nodes no test in Round 1 or Round 2 reaches. Write a test for
each one you changed before you run the rounds.
<!-- rtdd:variant target=agents -->
`rtdd which` prints the changed nodes, Rounds 1–3 and `untested`, and runs nothing.
Untracked files count. `rtdd which --json` is the machine-readable form.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=empty title="Empty rounds are not a pass" targets=skill,agents,mdc,global,global-agents order=40 -->
When Rounds 1 and 2 are empty, `rtdd which` prints `no linked test`. Read it as exactly
that — no linked test, never a pass: nothing in the graph links a test to the code you
changed. Write the test the change needs. Round 3 still runs the full suite once at the end.
<!-- rtdd:variant target=agents -->
Empty Rounds 1 and 2 print `no linked test`: no linked test, never a pass.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=graphify title="graphify is optional" targets=skill,agents,mdc,global,global-agents order=50 -->
rtdd's own scanner builds the graph, and needs nothing installed. graphify is optional: its
graph is used when `graphify-out/graph.json` exists (`graphify_path` in `.rtdd/config.yaml`
moves it). It is never trusted for changed files — rtdd rescans every file that changed
since graphify built its graph, and ignores the graph entirely, saying so, when more than
half of it is stale. graphify is never run by rtdd: if you want its graph, run or update
graphify yourself.
<!-- rtdd:variant target=agents -->
graphify is optional: used when `graphify-out/graph.json` exists, never trusted for changed
files, and never run by rtdd — run or update it yourself if you want it.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=json title="JSON output" targets=skill,global order=60 -->
`rtdd which --json` emits one object, schema 3:

```json
{
  "schema": 3,
  "command": "which",
  "base": "HEAD",
  "graph": {"source": "graphify+scanner", "built_at_commit": "fab6c1a", "stale_files": 4},
  "changed": [{"path": "src/auth.py", "lines": [{"start": 52, "end": 58}]}],
  "changed_nodes": [{"id": "src/auth.py::login", "file": "src/auth.py", "name": "login", "start": 48, "end": 70}],
  "rounds": [
    {"round": 1, "tests": [{"id": "tests/test_auth.py::test_login", "file": "tests/test_auth.py", "name": "test_login"}], "files": ["tests/test_auth.py"]},
    {"round": 2, "tests": [], "files": []},
    {"round": 3, "full_suite": true}
  ],
  "untested": ["src/auth.py::login_hint"],
  "warnings": []
}
```

Reject any `schema` other than `3`. `rounds` always holds three entries, and Round 3 is
always the full suite. `files` is a round's test files without duplicates, for a test
command that cannot select a single test. `graph.source` is `graphify+scanner` or `scanner`.
<!-- rtdd:endsection -->

<!-- rtdd:section id=commands title="The rest of the commands" targets=skill,global order=70 -->
```
rtdd graph [--json]                the graph's source, node, edge and test counts, staleness
rtdd explain <file[:line]|name>    a node's tests, callers and callees
rtdd doctor                        graph source, graphify staleness, test files found, names defined 8+ times
rtdd init [--dry-run]              set up this repository; any git repository
rtdd uninstall [--state]           remove what init wrote
rtdd update                        replace the binary with the latest release
rtdd --version                     print the version
```

Exit codes: 0 success, empty rounds included; 2 usage; 3 environment (not a git repository,
a graph that cannot be built). No exit code is a test result, because rtdd runs none.
<!-- rtdd:endsection -->

<!-- rtdd:section id=limits title="What it cannot see" targets=skill,mdc,global order=80 -->
Stated plainly, because a tool that hides its blind spots is worse than none:

- Links are by name. A call to `load(` links to every definition named `load` in files of
  the same kind, so a common name over-links and Round 2 can hold tests the change does not
  need. `rtdd doctor` lists the names defined eight or more times.
- A call the text does not show — through reflection, a string, a registry or a framework
  hook — has no edge, and its tests are in neither round.
- Depth is one. A caller's caller is in no round; Round 3 is the safety net.
- A deleted file has no lines left to own a node, so the tests that called it are in no
  round. `rtdd which` warns, and Round 3 runs them.
- The scanner reads text, not syntax: an unusual layout can give a node the wrong span.
<!-- rtdd:variant target=mdc -->
Links are by name, so a common name over-links; a call through reflection, a string or a
framework hook has no edge; depth is one, and Round 3 is the safety net.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->
````

`internal/protocol/targets.go`: each target's `Required` list becomes

| target | `Required` after Task 1 | Task 2 adds |
|---|---|---|
| `skill` | `what`, `process`, `which`, `empty`, `json`, `commands`, `limits` | `graphify` after `empty` |
| `agents` | `what`, `process`, `which`, `empty` | `graphify` |
| `mdc` | `what`, `process`, `which`, `empty`, `limits` | `graphify` after `empty` |
| `global` | `setup`, `what`, `process`, `which`, `empty`, `json`, `commands`, `limits` | `graphify` after `empty` |
| `global-agents` | `setup`, `what`, `process`, `which`, `empty` | `graphify` |

and the description block becomes (Task 1) — the two skill descriptions still differ, for the reason `TestGlobalSkillDescriptionDoesNotRequireASetUpRepository` states:

```go
const (
	SkillDescription = "Find the tests a code change needs, in rounds: `rtdd which` names the " +
		"changed functions, Round 1 (their tests) and Round 2 (their neighbours' tests), and " +
		"runs nothing. Use in a repository that has a .rtdd/config.yaml, after editing code " +
		"and before running its tests."
	// GlobalSkillDescription is deliberately NOT SkillDescription. The project skill is
	// installed by `rtdd init` into a repository that has already been set up, so it can
	// scope itself to one. The machine-wide skill is read in every repository on the
	// machine, most of which rtdd has never touched — reusing the project trigger would
	// tell the agent to stand down in exactly the repositories this skill exists to set up.
	GlobalSkillDescription = "Find the tests a code change needs, in rounds, on any codebase: " +
		"`rtdd which` names Round 1 (the changed code's tests) and Round 2 (its neighbours' " +
		"tests) and runs nothing. Use in any git repository after editing code: if it has a " +
		".rtdd/config.yaml, run `rtdd which`; if it does not, run `rtdd init` once to set " +
		"rtdd up for that repository first."
	MdcDescription = "Which tests a code change needs, in rounds; rtdd runs no tests."
	// MdcGlobs attaches the rule to every file: rtdd serves any codebase, and a list of
	// extensions would be a list of languages.
	MdcGlobs = "**/*"
)

```

Byte sizes rendered from this text: after Task 1 — `SKILL.md` 5066 (budget 20000), `AGENTS.md` 940 (1800), `rtdd.mdc` 2498 (4000), `GLOBAL-SKILL.md` 5871, `GLOBAL-AGENTS.md` 1091; after Task 2 — 5593, 1108, 3039, 6355, 1259.

### Regenerating everything the text feeds

Every task that edits `protocol/PROTOCOL.md` or `targets.go` runs, in this order, and commits all of it:

```bash
go run ./cmd/rtdd-gen render                            # rewrites dist/ (all five targets)
cp protocol/PROTOCOL.md internal/install/protocol.md    # the copy embedded in the binary
go test ./internal/protocol -run TestGolden -update     # rewrites internal/protocol/testdata/golden/
go run ./cmd/rtdd-gen check && go run ./cmd/rtdd-gen verify
diff -u protocol/PROTOCOL.md internal/install/protocol.md
```

`TestEmbeddedProtocolMatchesTheSource` (`internal/install`) and `scripts/ci-local.sh`'s `diff -u` step hold the embedded copy; `rtdd-gen check` holds `dist/`; `TestGolden` holds the goldens. None of them is edited — each fails until the regeneration above is committed.

## The `dist/` vocabulary guard

`internal/contract/frontend_vocabulary_test.go` (Task 1) reads **every file under `dist/`** — at least the five front-ends, more if a target is added — and fails on any match of `v02Vocabulary`:

| what | pattern | why this shape |
|---|---|---|
| `seed` | `(?i)\bseed(s\|ed\|ing)?\b` | no v0.3.0 sentence needs the word |
| `run` | ``(?i)\brtdd\s+run\b`` or the code span `` `run` `` | "run" is the verb of steps 3–5; only the command is v0.2 |
| `verify` | `(?i)\bverif(y\|ies\|ied\|ying)\b` | the command; the protocol says "check" where it means checking |
| `status`, `map compact` | `(?i)\brtdd\s+status\b`, `(?i)\brtdd\s+map\b` | the commands; the words alone are English |
| `map.jsonl`, `meta.json` | literal | the v0.2 state files |
| adapters | `(?i)\badapters?\b` | |
| coverage | `(?i)\b(coverage\|uncovered)\b` | the v0.2 concept and its report |
| T0/T1/T2 | `\bT[0-2]\b` (case-sensitive) | the tiers; `t1` in a path is not one |
| `selection_fidelity` | literal | |
| `--fail-fast` | literal | v0.2's run flag |
| a language | `\b(Python\|Go\|JavaScript\|TypeScript\|Java\|Rust\|Ruby\|Lua\|Luau\|Bash\|Kotlin\|PHP\|Swift\|Dart\|Perl\|Nim\|Scala\|Elixir\|Node\.js)\b`, `C#`, `C++` — **case-sensitive** | "go back", "rust", "swift" are English; a language is capitalised. The spec §1 eight plus every language v0.2 shipped an adapter for |
| a test framework or toolchain | `(?i)\b(pytest\|unittest\|jest\|vitest\|mocha\|jasmine\|junit\|testng\|nunit\|xunit\|mstest\|rspec\|minitest\|phpunit\|cargo\|maven\|mvn\|gradle\|dotnet\|npm\|npx\|yarn\|pnpm\|go test)\b` | none is an English word |

**The allowlist is `vocabularyAllowed map[string]string`** — exact phrases removed before matching, each with its reason. It **starts empty**: the patterns above are shaped so the protocol's prose needs no exception, and an allowlist entry whose phrase no `dist/` file still holds is itself an error, so the list can only shrink. A lane that wants an entry adds it with a reason a reviewer can reject; it does not loosen a pattern.

The spec §9 JSON example in the `json` section keeps its paths (`src/auth.py`, `tests/test_auth.py`): an example path is not an instruction to use a language, and the guard does not read extensions. The frontmatter is in the file, so the descriptions and `MdcGlobs` are guarded with the body.

`TestV02VocabularyCatchesTheFormsAndSparesOrdinaryEnglish` pins the guard itself: seventeen v0.2/language/framework strings must hit, seven sentences of the new text must not.

## graphify, as the front-ends state it

Section `graphify`, order 50, every target (Task 2). The long body and the agents variant both state the three facts of PRD #411 AC3 in these words, which Task 2's test matches (whitespace-collapsed):

| fact | phrase |
|---|---|
| optional | "graphify is optional" |
| used when present | "used when `graphify-out/graph.json` exists" |
| never trusted for changed files | "never trusted for changed files" |
| never run by rtdd | "never run by rtdd" |

The long body adds that `graphify_path` in `.rtdd/config.yaml` moves the file, that rtdd rescans every file changed since graphify built its graph, that it ignores the graph entirely — and says so — when more than half of it is stale, and that the agent runs or updates graphify itself if it wants the graph. No front-end says "graphify is required", "requires graphify", "install graphify", or that rtdd runs, updates, or will run or update graphify (`TestNoFrontEndRequiresGraphifyOrHasRtddRunIt`). The suggestion `graphify --update` stays where spec §5 puts it — in rtdd's own output when graphify is ignored — and out of the front-ends, so no front-end reads as an instruction to keep graphify current.

## `.rtdd/config.yaml`

`ConfigWithAdapters`, `AdapterRecord` and the three-key `defaultConfig` (`stale_commits`, `drift_guard`, `hub_threshold`) are deleted. **`func DefaultConfig() string`** in `internal/install/config.go` renders the file from `graph.DefaultConfig()` — the defaults the graph applies when the file is absent — so the two cannot drift: `TestDefaultConfigLoadsAsTheGraphDefaults` loads it back with `graph.LoadConfig` and requires `reflect.DeepEqual` with `graph.DefaultConfig()`. It writes every key the graph reads, globs double-quoted (a YAML scalar may not start with `*`):

```go
package install

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// DefaultConfig renders .rtdd/config.yaml as `rtdd init` writes it: graph.DefaultConfig()
// with every key spelled out, so a user edits a value rather than learning a key name.
// It is generated from graph.DefaultConfig, never typed twice, so the file init writes and
// the defaults the graph applies cannot drift (TestDefaultConfigLoadsAsTheGraphDefaults).
func DefaultConfig() string {
	d := graph.DefaultConfig()
	var b strings.Builder
	b.WriteString("# .rtdd/config.yaml — written by `rtdd init`. Every list is globs over\n")
	b.WriteString("# repo-relative paths; a key you set replaces its default wholesale.\n")
	b.WriteString("# See docs/specs/2026-10-07-node-graph.md §4.1, §5 and §6.\n")
	list := func(comment, key string, globs []string) {
		fmt.Fprintf(&b, "\n# %s\n%s:\n", comment, key)
		for _, g := range globs {
			fmt.Fprintf(&b, "  - %q\n", g)
		}
	}
	list("Files the scanner never reads.", "scan_exclude", d.ScanExclude)
	list("A function, method or test in a matching file is a test.", "test_files", d.TestFiles)
	list("A matching file is never a test file, whatever test_files says.", "test_exclude", d.TestExclude)
	fmt.Fprintf(&b, "\n# graphify's graph, used when this file exists. rtdd never runs graphify.\ngraphify_path: %q\n", d.GraphifyPath)
	fmt.Fprintf(&b, "\n# graphify is ignored when more than this share of its code files is stale.\nmax_stale_ratio: %v\n", d.MaxStaleRatio)
	return b.String()
}
```

which `rtdd init` writes as:

```yaml
# .rtdd/config.yaml — written by `rtdd init`. Every list is globs over
# repo-relative paths; a key you set replaces its default wholesale.
# See docs/specs/2026-10-07-node-graph.md §4.1, §5 and §6.

# Files the scanner never reads.
scan_exclude:
  - "vendor/**"
  - "node_modules/**"
  - "third_party/**"
  - "**/*.min.js"
  - "dist/**"
  - "build/**"
  - ".rtdd/**"
  - "graphify-out/**"
  - "**/*.md"
  - "**/*.mdc"
  - "**/*.markdown"
  - "**/*.rst"
  - "**/*.txt"
  - "**/*.adoc"

# A function, method or test in a matching file is a test.
test_files:
  - "**/test_*"
  - "**/*_test.*"
  - "**/*.test.*"
  - "**/*.spec.*"
  - "**/*Test.*"
  - "**/*Tests.*"
  - "**/tests/**"
  - "**/test/**"
  - "**/spec/**"
  - "**/__tests__/**"

# A matching file is never a test file, whatever test_files says.
test_exclude:
  - "**/testdata/**"
  - "**/fixtures/**"

# graphify's graph, used when this file exists. rtdd never runs graphify.
graphify_path: "graphify-out/graph.json"

# graphify is ignored when more than this share of its code files is stale.
max_stale_ratio: 0.5
```

`test_exclude` is written too: it is a real key of `graph.Config` (spec §6's "never test files" list), and leaving it out would make the one key a user cannot discover by reading the file. No key names an adapter, a language or a framework.

## `rtdd init`

`install.Plan(root string, files map[string]string) ([]Step, error)` — the `force bool` and `detected []AdapterRecord` parameters are gone. It plans, in order:

0. **The v0.2 migration** — `PlanMigration(root)`'s steps (Task 4; none on a repository v0.2 never touched).
1. **Whole-file front-ends** (`.claude/skills/rtdd/SKILL.md`, `.cursor/rules/rtdd.mdc`): missing → `Create`; identical → `Skip` "already current"; differs and contains `protocol.Generated` → **`Replace`** "an earlier rtdd render"; differs without it → `Conflict` "exists and was not written by rtdd; move it aside and re-run".
2. **Marker targets** (`AGENTS.md`, `CLAUDE.md` when present) — `MergeBlock`, unchanged.
3. **`.rtdd/config.yaml`**: missing → `Create` `DefaultConfig()`; a v0.2 config (`isV02Config`, Task 4) → `Replace` `DefaultConfig()`; otherwise `Skip` "keeping your config".
4. **`.gitignore`**: missing → `Create` `".rtdd/graph.json\n"`; a line (trimmed) equal to `.rtdd/graph.json` or `/.rtdd/graph.json` → `Skip` "already ignores .rtdd/graph.json"; otherwise **`AppendLine`**: the file, a newline if it lacked a final one, then `.rtdd/graph.json\n`. Running `init` twice leaves the line there exactly once. A broader ignore (`.rtdd/`) does not count — appending the exact line to it is harmless, and matching gitignore semantics would need git.

`Action` gains `Replace` ("replace") and `AppendLine` ("append-line"), appended after `StripBlock` so no existing value moves; `Apply` writes both like `Create`, and handles `Delete` (`os.RemoveAll`) and `StripBlock` (write `Content`) for the migration steps. `RenderInit` is unchanged: one `%-14s %s  — note` line per step, so every removal is its own line.

**`--force` is removed.** `cmd/rtdd/init.go` no longer defines it; `rtdd init --force` is a usage error (exit 2, Go's `flag provided but not defined: -force`); `rtdd init --help` lists only `-dry-run`; the usage line is `rtdd init   [--dry-run]` and the conflict message is `rtdd init: N conflict(s); nothing written; move the named file(s) aside and re-run`. A no-op flag would be a promise in `--help` that nothing keeps, and the one thing it still did — overwrite a file rtdd did not write — is a human's call to make by moving that file. `rtdd skill install --force` (the machine-wide front-ends) is a different command and is not touched.

`init` on an empty repository, a prose-only one, one in a language the scanner does not know and a polyglot one each exits 0 and writes the same five paths (Task 3's test); its output names no adapter and no detection.

## The v0.2 migration

**`func PlanMigration(root string) ([]Step, error)`** in new `internal/install/migrate.go` (Task 4):

| present in the repository | step | note `init` prints |
|---|---|---|
| `.rtdd/map.jsonl` | `Delete` | `the v0.2 coverage map; v0.3.0 builds a node graph instead` |
| `.rtdd/meta.json` | `Delete` | `the v0.2 map's metadata` |
| `.rtdd/adapters/` (any contents) | `Delete` (`os.RemoveAll`) | `v0.2 adapter definitions; v0.3.0 has no adapters` |
| the `.rtdd/map.jsonl merge=union` line, with other lines | `StripBlock`, `Content` = the file without that line (uninstall's `removeLine`) | `removed the v0.2 line .rtdd/map.jsonl merge=union` |
| that line and nothing else | `Delete` `.gitattributes` | `held nothing but the v0.2 line .rtdd/map.jsonl merge=union` |

Absent artefacts plan no step, so a fresh repository's output has no `delete`/`strip-block` line and a second `init` on a migrated repository reports nothing. The `gitattributesLine` constant and `removeLine` stay in `uninstall.go` (uninstall still removes the line) and `migrate.go` uses them. A v0.2 config is replaced in `Plan`'s config step, printed as `replace .rtdd/config.yaml — the v0.2 config; v0.3.0 reads none of its keys`; `isV02Config` is true when the file's top-level keys include one of `stale_commits`, `drift_guard`, `hub_threshold`, `adapters` and none of `scan_exclude`, `test_files`, `test_exclude`, `graphify_path`, `max_stale_ratio`. A v0.2 skill or Cursor rule carries `protocol.Generated`, so step 1 replaces it.

On a v0.2 repository `rtdd init` prints (checked against a built binary before this plan was written; the two whole-file front-end lines can swap places, because `Plan` ranges over a map — the order was already unstable before N3, and no test depends on it):

```
delete         .rtdd/map.jsonl  — the v0.2 coverage map; v0.3.0 builds a node graph instead
delete         .rtdd/meta.json  — the v0.2 map's metadata
delete         .rtdd/adapters/  — v0.2 adapter definitions; v0.3.0 has no adapters
strip-block    .gitattributes  — removed the v0.2 line .rtdd/map.jsonl merge=union
create         .cursor/rules/rtdd.mdc
replace        .claude/skills/rtdd/SKILL.md  — an earlier rtdd render
create         AGENTS.md
replace        .rtdd/config.yaml  — the v0.2 config; v0.3.0 reads none of its keys
create         .gitignore

Next: edit code, then run `rtdd which` for the tests to run, in rounds.
```

The **test fixture** is built in the test, not committed: `v02Layout` (`internal/install/migrate_test.go`) and `v02Repository` (`cmd/rtdd/init_migrate_test.go`) write a map row, a meta file, `.rtdd/adapters/python.yaml`, the adapter-era config (`stale_commits: 50`, `drift_guard: 100`, `hub_threshold: 0.40`, `adapters: [{name: python}]`), a `.gitattributes` of `*.png binary` plus the merge-driver line, and — in the command test — a v0.2-rendered skill.

**The no-reference guard.** `TestNoGoCodeReferencesTheCoveragePipeline` (`internal/contract/pipeline_removed_test.go`) fails on a Go file outside `v02StateAllowed` that spells `.rtdd/map.jsonl`, `.rtdd/meta.json` or `.rtdd/adapters/`, and on an allowed file that no longer does. Task 1 removes `internal/protocol/targets.go` and `internal/protocol/global_test.go` from `v02StateAllowed` (both stop naming v0.2 state). Task 4 adds `internal/install/migrate.go` ("rtdd init deletes v0.2 state from a host repository (PRD #411 AC6)"), `internal/install/migrate_test.go` and `cmd/rtdd/init_migrate_test.go`. `internal/install/uninstall.go` and `uninstall_test.go` stay.

## The docs

Task 5 changes these pages and no others:

| page | change |
|---|---|
| `README.md` **and** `docs/outcomes/README.positive.md` | rewritten for v0.3.0 and kept **byte-identical** (`TestRootREADMEMatchesSelectedOutcome`): what rtdd is (a skill plus a binary that answers which tests a change needs, in rounds), the five-step process, `rtdd init` on any repository and the v0.2 migration, `rtdd which` with a real example, `rtdd graph` / `explain` / `doctor`, graphify as optional, what it cannot see. The **Install** section keeps every sentence `internal/installtest/readme_install_test.go` pins; the "Does it work?" measurements stay, pinned by `outcomes_test.go`, inside a v0.2 record (below) that opens with one sentence: they were measured on v0.2.0's coverage selector, and v0.3.0's rounds are measured by rtdd-bench (PRD #412). The "how it picks" figure (`docs/results/figures/how-it-picks-*.svg`, a seed map) is no longer referenced; the files stay. |
| `DEVELOPMENT.md` | "Dependencies" says `gopkg.in/yaml.v3` reads `.rtdd/config.yaml` (not adapter definitions); "One-pipeline adapter tests" is deleted (the tests were deleted by PRD #410); a new "Breaking changes in v0.3.0" section (seed/run/verify/status/map compact removed; `which --json` schema 3; `rtdd init` migrates v0.2 state); the existing "Breaking changes in the one-pipeline release" and the benchmark-harness sections that still describe the v0.2 pipeline (PRD #412 re-points them) go inside v0.2 records. It names the five-step process and the commands by name. |
| `docs/LIMITATIONS.md` | rewritten as v0.3.0's limits, matching the protocol's `limits` section: name-based linking over-links, calls the text does not show have no edge, depth one, deleted files, a text scanner's spans, graphify staleness. |
| `docs/specs/2026-08-26-rtdd-design.md`, `2026-09-05-multi-language.md`, `2026-09-29-one-pipeline.md` | first line is the banner below, then a blank line, then the file unchanged. **Not deleted.** |

The superseded banner, verbatim, as the first line of each superseded spec:

```
> **Superseded** by [`2026-10-07-node-graph.md`](2026-10-07-node-graph.md) (v0.3.0). Kept as the record of the design it describes; do not implement from it.
```

**v0.2 records.** A v0.2 measurement or changelog kept on a v0.3.0 page sits between `<!-- rtdd:v0.2-record -->` and `<!-- /rtdd:v0.2-record -->` on their own lines. `TestUserDocsDescribeV030NotV02` reads README.md, README.positive.md, DEVELOPMENT.md and LIMITATIONS.md with those regions removed and fails on a removed command (`rtdd seed|run|verify|status|map`), `map.jsonl`, adapters, coverage, `T0`–`T2` or `selection_fidelity` — "no coverage" and "no adapters" say what v0.3.0 does not do and are exempt — and requires README and DEVELOPMENT.md to mention `rtdd init`, `rtdd which`, `rtdd graph`, `rtdd explain`, `rtdd doctor`, "Round 1", "Round 2", "full suite once" and graphify. An unbalanced marker is an error.

**Not changed:** `docs/plans/` (history — this plan included), `docs/results/`, `docs/audits/`, `docs/bench/` (dated measurement records), `docs/RELEASING.md`, `docs/PRIOR-ART.md`, `docs/outcomes/README.md` (the swap mechanism) and `docs/outcomes/README.negative.md` (the unselected v0.2 outcome branch, re-decided with the bench in PRD #412).

**README commands run as shown.** `TestREADMEWhichExampleIsRealOutput` (`cmd/rtdd`) builds a repository — `src/calc.py` with `add` and `total` (which calls `add`), `tests/test_calc.py` with `test_add` and `test_total` — edits `add`, runs `rtdd which`, and requires README's block that opens `$ rtdd which` to be its output byte for byte, the commit after `built at` normalised. Its output today, which the README pastes:

```
$ rtdd which
graph: scanner, built at aeb07e0, 0 stale files
changed nodes:
  src/calc.py::add  (lines 1-2)
Round 1 — run these first:
  tests/test_calc.py::test_add
Round 2 — then these:
  tests/test_calc.py::test_total
Round 3 — the full suite, once, at the end
untested:
  none
```

`TestREADMEShowsOnlyCommandsRtddHas` requires every `rtdd <command>` line in a README code block (outside v0.2 records) to name a command in `rtdd --help`.

## Old tests: who changes what

A test that pins **v0.2 behaviour** is deleted or rewritten by the task that changes that behaviour, in the same commit, and the commit body names it. A test that pins behaviour that survives is never weakened to make a task pass. Found by applying each task to a scratch copy of the tree before this plan was written:

| Task | Tests changed |
|---|---|
| 1 | `internal/protocol/render_test.go`: `renderSample`'s `id=run title="rtdd run"` section becomes `id=process title="The process"` with body `process body` (the sample must carry every `Required` id; fourteen render/validate tests fail on it otherwise). `validate_test.go` `TestValidateSkillFailsWhenRequiredSectionHeadingMissing`: strips `"## The process\n\n"` instead of `"## The map file\n\n"`. `global_test.go`: `TestGlobalSkillDescriptionDoesNotRequireAMap` → `TestGlobalSkillDescriptionDoesNotRequireASetUpRepository` (same assertions), and both comments stop naming the map file. `internal/contract/pipeline_removed_test.go`: the two `v02StateAllowed` entries above. |
| 2 | `render_test.go`: `renderSample`'s `id=fidelity` section becomes `id=graphify title="graphify is optional"` with body `graphify body`. |
| 3 | `internal/install/config_test.go`: all six `ConfigWithAdapters`/record tests deleted, the file replaced. `install_test.go`: `TestPlanConflictsOnADifferingWholeFileWithoutForce` and `TestPlanForceOverwritesADifferingWholeFile` deleted (replaced by `TestPlanReplacesAnEarlierRenderAndConflictsOnAForeignFile`); every `Plan(root, …, false, nil)` call becomes `Plan(root, …)`. `cmd/rtdd/init_test.go`: `TestInitConflictsOnAHandEditedSkillFileAndForceOverridesIt` → `TestInitConflictsOnASkillFileRtddDidNotWrite` (the conflict half; the `--force` half is gone). `cmd/rtdd/init_gate_test.go`: `TestInitForceRecordsNoAdaptersInTheConfig` deleted. |
| 4 | `install_test.go` `TestPlanConfigIsCreatedOnceThenNeverOverwritten` and `init_gate_test.go` `TestInitNeverRewritesAnExistingConfigToAddTheRecord`: their "tuned" config becomes `max_stale_ratio: 0.3\n` (was `stale_commits: 999\n`, which is now a v0.2 config and is replaced). Both still pin "a user's config is never overwritten". |
| 5 | none in Go; `internal/installtest`'s README tests keep passing on the rewritten README. |

## Decisions this plan settles

The spec and PRD leave these open; each is settled here so five lanes do not settle it five ways, and each is pinned by a test in the task named.

1. **The protocol text** is "The protocol text" above, verbatim: sections `setup`, `what`, `process`, `which`, `empty`, `graphify`, `json`, `commands`, `limits`; the five steps as one numbered list in section `process`, every target, no variant. Tasks 1, 2.
2. **The five steps' wording** is spec §10's with "Run `rtdd which`" for step 2 and "with the project's own test command" for step 3; "rtdd runs no tests" and "no linked test, never a pass" appear in every target. Task 1.
3. **Descriptions and globs**: the three descriptions in "The protocol text"; `MdcGlobs = "**/*"`. Task 1.
4. **Regeneration** is the five commands under "Regenerating everything the text feeds"; nothing under `dist/`, the goldens or `internal/install/protocol.md` is edited by hand. Tasks 1, 2.
5. **The `dist/` guard**: `TestFrontEndsUseNoV02Vocabulary` over every file under `dist/`, the `v02Vocabulary` table above, `vocabularyAllowed` empty and shrink-only. Task 1.
6. **graphify wording**: the four phrases in "graphify, as the front-ends state it"; the forbidden phrases with them. Task 2.
7. **The config**: `func DefaultConfig() string` generated from `graph.DefaultConfig()`, five keys, quoted globs; `ConfigWithAdapters` deleted. Task 3.
8. **`--force` is removed**; whole-file front-ends are replaced when they carry `protocol.Generated`, a conflict otherwise. Task 3.
9. **`.gitignore`**: `.rtdd/graph.json` appended once (exact line, either anchoring, counts as present); `.gitignore` created when missing. Task 3.
10. **The migration**: `func PlanMigration(root string) ([]Step, error)`, the table above, one printed line per removal, a `.gitattributes` deleted only when the merge=union line was all it held; a v0.2-only config replaced, any other config kept. Task 4.
11. **`v02StateAllowed`**: minus `targets.go` and `global_test.go` (Task 1), plus `migrate.go`, `migrate_test.go`, `init_migrate_test.go` (Task 4).
12. **The docs**: the pages in the table above; superseded specs get the banner, verbatim, as line 1; v0.2 measurements kept in `<!-- rtdd:v0.2-record -->` regions; README's `rtdd which` example is real output. Task 5.

## Acceptance-criterion map

| PRD #411 AC | What | Task(s) | Issue(s) |
|---|---|---|---|
| AC1 | PROTOCOL.md states the five steps for every target; rtdd runs no tests; the project's own test command; empty rounds are "no linked test", never a pass | 1 | #478 |
| AC2 | no front-end says a v0.2 word; a test greps every `dist/` file; no language, no test framework | 1 (guard), 2 (keeps it green) | #478, #479 |
| AC3 | graphify optional: used when present, never trusted for changed files, never run by rtdd | 2 | #479 |
| AC4 | `rtdd-gen render` output committed; `check`/`verify` pass; `internal/install/protocol.md` matches; goldens updated | 1, 2 | #478, #479 |
| AC5 | `init` succeeds on every git repository; `--force` removed; config with the four keys; `.rtdd/graph.json` in `.gitignore` | 3 | #480 |
| AC6 | `init` on v0.2 deletes the four artefacts and prints each; a test builds a v0.2 repository | 4 | #481 |
| AC7 | README, DEVELOPMENT.md, `docs/` describe v0.3.0; superseded specs marked, not deleted | 5 | #482 |
| AC8 | `scripts/ci-local.sh` exits 0 | every task; the PRD-wide run in 5 | #478–#482 |

Order: Task 1 first. **Task 2 after Task 1** (same file, #479 blocked by #478). **Task 3 needs only this plan**; **Task 4 after Task 3** (it extends `Plan`'s config step and `Apply`). **Task 5 last**, after 1–4: it documents what they ship and runs the PRD-wide gate.

## File Structure

| Path | Responsibility | Task |
|---|---|---|
| `protocol/PROTOCOL.md` | the text every front-end is rendered from | 1, 2 |
| `internal/protocol/targets.go` | `Required` per target, descriptions, `MdcGlobs` | 1, 2 |
| `dist/**`, `internal/protocol/testdata/golden/*`, `internal/install/protocol.md` | regenerated, never hand-edited | 1, 2 |
| `internal/protocol/process_test.go` | `flat`, `fiveSteps`; the five steps and the three statements, per target | 1 |
| `internal/protocol/graphify_test.go` | the graphify facts, per target | 2 |
| `internal/contract/frontend_vocabulary_test.go` | `v02Vocabulary`, `vocabularyAllowed`, the `dist/` guard and its self-test | 1 |
| `internal/install/config.go` | `DefaultConfig` | 3 |
| `internal/install/install.go` | `Plan` (no `force`, no records; replace / gitignore / migration steps), `planGitignore`, `Apply` | 3, 4 |
| `internal/install/merge.go` | `Replace`, `AppendLine` actions | 3 |
| `internal/install/migrate.go` | `v02Files`, `PlanMigration`, `isV02Config` | 4 |
| `internal/install/config_test.go`, `plan_v030_test.go`, `migrate_test.go` | install tests | 3, 3, 4 |
| `cmd/rtdd/init.go`, `cmd/rtdd/main.go` | no `--force`; usage line | 3 |
| `cmd/rtdd/init_shapes_test.go`, `init_migrate_test.go` | command tests | 3, 4 |
| `internal/install/uninstall.go`, `cmd/rtdd/uninstall.go` | `--state` notes say "config and graph cache" | 4 |
| `README.md`, `docs/outcomes/README.positive.md`, `DEVELOPMENT.md`, `docs/LIMITATIONS.md`, three specs | v0.3.0 docs, banners | 5 |
| `internal/contract/docs_v030_test.go`, `cmd/rtdd/readme_example_test.go` | docs guards | 5 |
| `internal/contract/plan_n3_test.go` | this document's own contract (issue #477) | — |

---

### Task 1: PROTOCOL.md and every front-end state the five-step rounds process, with no v0.2 vocabulary

**Issue:** #478

**Discharges:** PRD #411 AC1, AC2 (the guard), AC4, AC8. Spec §10, §11.

**Files:**
- Modify: `protocol/PROTOCOL.md` (the text in "The protocol text", without the `graphify` section)
- Modify: `internal/protocol/targets.go` (`Required`, descriptions, `MdcGlobs`)
- Regenerate: `dist/**`, `internal/protocol/testdata/golden/*`, `internal/install/protocol.md`
- Modify: `internal/protocol/render_test.go`, `validate_test.go`, `global_test.go`, `internal/contract/pipeline_removed_test.go` (Task 1 row of "Old tests")
- Test: `internal/protocol/process_test.go`, `internal/contract/frontend_vocabulary_test.go`

**Interfaces:**
- Consumes: `renderReal` (`internal/protocol/global_test.go`), `Targets`; `repoRoot` (`internal/contract/contract_test.go`).
- Produces: `func flat(s string) string` and `var fiveSteps []string` (package `protocol`, test-only; Task 2 uses `flat`); `var v02Vocabulary`, `var vocabularyAllowed map[string]string`, `func vocabularyHits(text string) []string` (package `contract`, test-only; Task 2's guard run reuses them unchanged). Section ids `setup`, `what`, `process`, `which`, `empty`, `json`, `commands`, `limits`.

- [ ] **Step 1: Write the failing tests** — `internal/protocol/process_test.go`:

```go
package protocol

import (
	"strings"
	"testing"
)

// flat collapses every run of whitespace to one space, so a phrase the source wraps across
// two lines still matches.
func flat(s string) string { return strings.Join(strings.Fields(s), " ") }

// fiveSteps is spec §10's process as PROTOCOL.md states it, one literal line per step, in
// the order every front-end must carry them (PRD #411 AC1).
var fiveSteps = []string{
	"1. Edit code (test first, per TDD).",
	"2. Run `rtdd which`. If `untested` names a node you changed, write its test first.",
	"3. Run **Round 1** with the project's own test command. Fix until green.",
	"4. Run **Round 2**. Fix until green; return to step 2 after any further edit.",
	"5. When the task is done — before committing or handing off — run the **full suite once**.",
}

// PRD #411 AC1: every target states the five steps of spec §10, in order.
func TestEveryFrontEndStatesTheFiveStepsInOrder(t *testing.T) {
	out := renderReal(t)
	for _, tgt := range Targets {
		t.Run(tgt.Name, func(t *testing.T) {
			body := flat(out[tgt.OutPath])
			at := -1
			for i, step := range fiveSteps {
				j := strings.Index(body, step)
				switch {
				case j < 0:
					t.Errorf("%s does not state step %d: %q", tgt.OutPath, i+1, step)
				case j < at:
					t.Errorf("%s states step %d before step %d", tgt.OutPath, i+1, i)
				default:
					at = j
				}
			}
		})
	}
}

// PRD #411 AC1: every target says rtdd runs no tests, that the agent runs each round with
// the project's own test command, and that empty Rounds 1 and 2 are no linked test, never
// a pass.
func TestEveryFrontEndSaysRtddRunsNoTestsAndEmptyRoundsAreNotAPass(t *testing.T) {
	out := renderReal(t)
	for _, tgt := range Targets {
		t.Run(tgt.Name, func(t *testing.T) {
			body := flat(out[tgt.OutPath])
			for _, want := range []string{
				"rtdd runs no tests",
				"run each round with the project's own test command",
				"no linked test, never a pass",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s does not say %q", tgt.OutPath, want)
				}
			}
		})
	}
}
```

and `internal/contract/frontend_vocabulary_test.go`:

```go
package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// v02Vocabulary is what no generated front-end may say (PRD #411 AC2): the v0.2 commands,
// state files and concepts, every language and every test framework. Each entry is shaped
// so ordinary English does not trip it — `run` is forbidden only as an rtdd command or a
// code span, never as the verb the five steps need; language names are matched in their
// proper capitalisation, so "go back" and "rust" are prose while "Go" and "Rust" are names.
var v02Vocabulary = []struct {
	what string
	re   *regexp.Regexp
}{
	{"the seed command", regexp.MustCompile(`(?i)\bseed(s|ed|ing)?\b`)},
	{"the run command", regexp.MustCompile("(?i)\\brtdd\\s+run\\b|`run`")},
	{"the verify command", regexp.MustCompile(`(?i)\bverif(y|ies|ied|ying)\b`)},
	{"the status command", regexp.MustCompile(`(?i)\brtdd\s+status\b`)},
	{"the map command", regexp.MustCompile(`(?i)\brtdd\s+map\b`)},
	{"the v0.2 map file", regexp.MustCompile(`map\.jsonl`)},
	{"the v0.2 meta file", regexp.MustCompile(`meta\.json`)},
	{"adapters", regexp.MustCompile(`(?i)\badapters?\b`)},
	{"coverage", regexp.MustCompile(`(?i)\b(coverage|uncovered)\b`)},
	{"the T0/T1/T2 tiers", regexp.MustCompile(`\bT[0-2]\b`)},
	{"selection_fidelity", regexp.MustCompile(`selection_fidelity`)},
	{"a v0.2 run flag", regexp.MustCompile(`--fail-fast`)},
	{"a language", regexp.MustCompile(`\b(Python|Go|JavaScript|TypeScript|Java|Rust|Ruby|Lua|Luau|Bash|Kotlin|PHP|Swift|Dart|Perl|Nim|Scala|Elixir|Node\.js)\b|C#|C\+\+`)},
	{"a test framework or toolchain", regexp.MustCompile(`(?i)\b(pytest|unittest|jest|vitest|mocha|jasmine|junit|testng|nunit|xunit|mstest|rspec|minitest|phpunit|cargo|maven|mvn|gradle|dotnet|npm|npx|yarn|pnpm|go test)\b`)},
}

// vocabularyAllowed are exact phrases under dist/ that may contain a forbidden match, each
// with the reason. It starts empty — the entries above are shaped so the protocol's own
// prose needs none — and an entry whose phrase no file under dist/ still holds is an error:
// the list only shrinks.
var vocabularyAllowed = map[string]string{}

// vocabularyHits returns what of v02Vocabulary text says, after removing allowed phrases.
func vocabularyHits(text string) []string {
	for phrase := range vocabularyAllowed {
		text = strings.ReplaceAll(text, phrase, "")
	}
	var hits []string
	for _, v := range v02Vocabulary {
		if m := v.re.FindString(text); m != "" {
			hits = append(hits, v.what+" ("+m+")")
		}
	}
	return hits
}

// PRD #411 AC2: no file under dist/ says any of v02Vocabulary.
func TestFrontEndsUseNoV02Vocabulary(t *testing.T) {
	root := repoRoot(t)
	var all strings.Builder
	files := 0
	err := filepath.WalkDir(filepath.Join(root, "dist"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files++
		all.Write(b)
		rel, _ := filepath.Rel(root, p)
		for _, hit := range vocabularyHits(string(b)) {
			t.Errorf("%s says %s", filepath.ToSlash(rel), hit)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 5 {
		t.Fatalf("read %d files under dist/, want the 5 generated front-ends", files)
	}
	for phrase, why := range vocabularyAllowed {
		if !strings.Contains(all.String(), phrase) {
			t.Errorf("vocabularyAllowed holds %q (%s) but no front-end says it: remove it", phrase, why)
		}
	}
}

// The guard is only as good as its patterns: each forbidden form trips it, and the prose
// the five steps need does not.
func TestV02VocabularyCatchesTheFormsAndSparesOrdinaryEnglish(t *testing.T) {
	for _, bad := range []string{"run `rtdd seed` once", "rtdd run --fail-fast", "rtdd verify", "rtdd status",
		"rtdd map compact", "the map.jsonl file", "meta.json", "the python adapter", "recorded coverage",
		"tier T2", "selection_fidelity", "a Go repository", "TypeScript", "C#", "run pytest", "Jest", "go test ./..."} {
		if len(vocabularyHits(bad)) == 0 {
			t.Errorf("vocabularyHits(%q) found nothing", bad)
		}
	}
	for _, ok := range []string{"Run **Round 1** with the project's own test command.", "rtdd runs no tests",
		"return to step 2", "go back to step 2", "never a pass", "the full suite once", "rtdd which --json"} {
		if hits := vocabularyHits(ok); len(hits) != 0 {
			t.Errorf("vocabularyHits(%q) = %v, want nothing: ordinary prose", ok, hits)
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/protocol/ ./internal/contract/ -count=1 -run 'TestEveryFrontEndStatesTheFiveStepsInOrder|TestEveryFrontEndSaysRtddRunsNoTestsAndEmptyRoundsAreNotAPass|TestFrontEndsUseNoV02Vocabulary|TestV02VocabularyCatchesTheFormsAndSparesOrdinaryEnglish'
```

Expected: FAIL — for each of the five targets `dist/SKILL.md does not state step 1: "1. Edit code (test first, per TDD)."` (and steps 2–5), `does not say "rtdd runs no tests"`, `"run each round with the project's own test command"`, `"no linked test, never a pass"`; `dist/AGENTS.md says the seed command (seed)`, `the run command (rtdd run)`, `the v0.2 map file (map.jsonl)`, `coverage (coverage)` and the same for the other four files. `TestV02VocabularyCatchesTheFormsAndSparesOrdinaryEnglish` already PASSES: it tests the guard, not the text.

- [ ] **Step 3: Write the text.** Replace `protocol/PROTOCOL.md` with the file in "The protocol text" minus its `graphify` section. In `targets.go`, set each `Required` to the "after Task 1" column and replace the description block with the one shown there.

- [ ] **Step 4: Regenerate** — the five commands under "Regenerating everything the text feeds". `rtdd-gen verify` must print `dist/ passes every target's validity assertions`.

- [ ] **Step 5: Update the tests that pin the old text** — the Task 1 row of "Old tests: who changes what": `renderSample`'s `run` section → `process`; `TestValidateSkillFailsWhenRequiredSectionHeadingMissing` strips `"## The process\n\n"`; `global_test.go`'s rename and two comments; `v02StateAllowed` loses `internal/protocol/targets.go` and `internal/protocol/global_test.go` (`TestNoGoCodeReferencesTheCoveragePipeline` fails until it does — the list only shrinks).

- [ ] **Step 6: Run to verify**

```bash
go vet ./... && go test ./internal/protocol/ ./internal/contract/ ./internal/install/ ./cmd/rtdd/ ./cmd/rtdd-gen/ -count=1
```

Expected: `ok` for all five packages.

- [ ] **Step 7: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0.

- [ ] **Step 8: Commit**

```bash
git add protocol dist internal/protocol internal/install/protocol.md internal/contract
git commit -m "feat(protocol): every front-end states the five-step rounds process, with no v0.2 vocabulary (closes #478)"
```

---

### Task 2: The front-ends describe graphify as optional: used when present, never trusted for changed files, never run by rtdd

**Issue:** #479

**Discharges:** PRD #411 AC3, AC2 (the Task 1 guard stays green over the new text), AC4, AC8. Spec §5, §11.

**Files:**
- Modify: `protocol/PROTOCOL.md` (add the `graphify` section, verbatim from "The protocol text")
- Modify: `internal/protocol/targets.go` (`graphify` in every `Required`)
- Regenerate: `dist/**`, `internal/protocol/testdata/golden/*`, `internal/install/protocol.md`
- Modify: `internal/protocol/render_test.go` (`renderSample`'s `fidelity` → `graphify`)
- Test: `internal/protocol/graphify_test.go`

**Interfaces:**
- Consumes: `renderReal`, `flat` (Task 1), `Targets`; Task 1's `TestFrontEndsUseNoV02Vocabulary`, unchanged.
- Produces: section id `graphify`, order 50, every target.

- [ ] **Step 1: Write the failing tests** — `internal/protocol/graphify_test.go`:

```go
package protocol

import (
	"strings"
	"testing"
)

// PRD #411 AC3: every target states the three facts about graphify — used when its graph
// exists, never trusted for changed files, never run by rtdd.
func TestEveryFrontEndDescribesGraphifyAsOptional(t *testing.T) {
	out := renderReal(t)
	for _, tgt := range Targets {
		t.Run(tgt.Name, func(t *testing.T) {
			body := flat(out[tgt.OutPath])
			for _, want := range []string{
				"graphify is optional",
				"used when `graphify-out/graph.json` exists",
				"never trusted for changed files",
				"never run by rtdd",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s does not say %q", tgt.OutPath, want)
				}
			}
		})
	}
}

// PRD #411 AC3: no target tells the agent graphify is required, or that rtdd runs or
// updates it.
func TestNoFrontEndRequiresGraphifyOrHasRtddRunIt(t *testing.T) {
	out := renderReal(t)
	for _, tgt := range Targets {
		body := strings.ToLower(flat(out[tgt.OutPath]))
		for _, bad := range []string{"graphify is required", "requires graphify", "install graphify",
			"rtdd runs graphify", "rtdd updates graphify", "rtdd will run graphify", "rtdd will update graphify"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s says %q", tgt.OutPath, bad)
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/protocol/ -count=1 -run 'TestEveryFrontEndDescribesGraphifyAsOptional|TestNoFrontEndRequiresGraphifyOrHasRtddRunIt'
```

Expected: FAIL — for each of the five targets `dist/SKILL.md does not say "graphify is optional"`, `"used when `graphify-out/graph.json` exists"`, `"never trusted for changed files"`, `"never run by rtdd"`. `TestNoFrontEndRequiresGraphifyOrHasRtddRunIt` already PASSES (no front-end mentions graphify yet); it guards the new text.

- [ ] **Step 3: Add the section and the ids.** Insert the `graphify` section of "The protocol text" after the `empty` section; add `"graphify"` to every target's `Required` as the table's last column says. Change nothing else in either file.

- [ ] **Step 4: Regenerate** — the five commands under "Regenerating everything the text feeds".

- [ ] **Step 5: Update `renderSample`** — its `id=fidelity title="Selection fidelity"` section becomes `id=graphify title="graphify is optional"` with body `graphify body`, so the sample carries every `Required` id.

- [ ] **Step 6: Run to verify**

```bash
go vet ./... && go test ./internal/protocol/ ./internal/contract/ ./internal/install/ ./cmd/rtdd/ ./cmd/rtdd-gen/ -count=1
```

Expected: `ok` for all five packages — `TestFrontEndsUseNoV02Vocabulary` included.

- [ ] **Step 7: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0.

- [ ] **Step 8: Commit**

```bash
git add protocol dist internal/protocol internal/install/protocol.md
git commit -m "feat(protocol): the front-ends describe graphify as optional, never trusted for changed files, never run by rtdd (closes #479)"
```

---

### Task 3: `rtdd init` succeeds on every git repository and writes `config.yaml` and the `graph.json` gitignore

**Issue:** #480

**Discharges:** PRD #411 AC5, AC8. Spec §4.5, §8 (`init`), §12.

**Files:**
- Modify: `internal/install/config.go` (replaced: `DefaultConfig`), `internal/install/install.go` (`Plan`, `planGitignore`, `Apply`), `internal/install/merge.go` (`Replace`, `AppendLine`)
- Modify: `cmd/rtdd/init.go` (no `--force`), `cmd/rtdd/main.go` (usage line)
- Modify: `internal/install/install_test.go`, `cmd/rtdd/init_test.go`, `cmd/rtdd/init_gate_test.go` (Task 3 row of "Old tests")
- Test: `internal/install/config_test.go` (replaced), `internal/install/plan_v030_test.go`, `cmd/rtdd/init_shapes_test.go`

**Interfaces:**
- Consumes: `graph.DefaultConfig`, `graph.LoadConfig` (PRD #409); `protocol.Generated`; `fakeFiles`, `stepFor` (`internal/install/install_test.go`); `rtdd`, `gitRun`, `graphRepo`, `readRepoFileForTest` (`cmd/rtdd`).
- Produces: `func DefaultConfig() string`; `func Plan(root string, files map[string]string) ([]Step, error)`; `Replace`, `AppendLine` actions; `writeAt(t, root, rel, body)` (test helper, `plan_v030_test.go`; Task 4 uses it). Deletes `ConfigWithAdapters`, `AdapterRecord`, `defaultConfig`.

- [ ] **Step 1: Write the failing tests** — `internal/install/config_test.go` (replace the file):

```go
package install

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// PRD #411 AC5: the config `rtdd init` writes is the graph's own defaults written out, so
// the graph code loads it back to exactly graph.DefaultConfig() — one list, two spellings
// that cannot drift.
func TestDefaultConfigLoadsAsTheGraphDefaults(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rtdd", "config.yaml"), []byte(DefaultConfig()), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := graph.LoadConfig(root)
	if err != nil {
		t.Fatalf("graph.LoadConfig over DefaultConfig(): %v\n%s", err, DefaultConfig())
	}
	if want := graph.DefaultConfig(); !reflect.DeepEqual(got, want) {
		t.Errorf("DefaultConfig() loads as\n%+v\nwant graph.DefaultConfig()\n%+v", got, want)
	}
}

// PRD #411 AC5: every key is written out, so a user edits a value rather than learning a
// key name; no v0.2 key survives.
func TestDefaultConfigNamesEveryKeyAndNoV02Key(t *testing.T) {
	cfg := DefaultConfig()
	for _, key := range []string{"scan_exclude:", "test_files:", "test_exclude:", "graphify_path:", "max_stale_ratio:"} {
		if !strings.Contains(cfg, "\n"+key) {
			t.Errorf("DefaultConfig() has no %s line:\n%s", key, cfg)
		}
	}
	for _, gone := range []string{"adapters", "stale_commits", "drift_guard", "hub_threshold"} {
		if strings.Contains(cfg, gone) {
			t.Errorf("DefaultConfig() still says %q:\n%s", gone, cfg)
		}
	}
}
```

`internal/install/plan_v030_test.go`:

```go
package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

func writeAt(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// PRD #411 AC5: a new repository gets the v0.3.0 config and a .gitignore naming the cache.
func TestPlanWritesTheConfigAndIgnoresTheGraphCache(t *testing.T) {
	root := t.TempDir()
	steps, err := Plan(root, fakeFiles())
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".rtdd/config.yaml"); s.Action != Create || s.Content != DefaultConfig() {
		t.Errorf("config step = %v %q, want Create DefaultConfig()", s.Action, s.Content)
	}
	if s := stepFor(t, steps, ".gitignore"); s.Action != Create || s.Content != ".rtdd/graph.json\n" {
		t.Errorf(".gitignore step = %v %q, want Create %q", s.Action, s.Content, ".rtdd/graph.json\n")
	}
}

// PRD #411 AC5: a host .gitignore gains the line once, after its own lines, and a
// repository that already ignores the cache — either spelling — is left alone.
func TestPlanAppendsTheGraphCacheLineOnlyWhenItIsMissing(t *testing.T) {
	for _, c := range []struct {
		existing string
		action   Action
		content  string
	}{
		{"node_modules/\n", AppendLine, "node_modules/\n.rtdd/graph.json\n"},
		{"node_modules/", AppendLine, "node_modules/\n.rtdd/graph.json\n"},
		{"node_modules/\n.rtdd/graph.json\n", Skip, ""},
		{"/.rtdd/graph.json\n", Skip, ""},
	} {
		root := t.TempDir()
		writeAt(t, root, ".gitignore", c.existing)
		steps, err := Plan(root, fakeFiles())
		if err != nil {
			t.Fatal(err)
		}
		s := stepFor(t, steps, ".gitignore")
		if s.Action != c.action || (c.action != Skip && s.Content != c.content) {
			t.Errorf(".gitignore %q: step = %v %q, want %v %q", c.existing, s.Action, s.Content, c.action, c.content)
		}
	}
}

// PRD #411 AC5: `--force` is removed. A whole-file front-end an earlier rtdd rendered —
// it carries protocol.Generated — is replaced; one rtdd did not write is a conflict.
func TestPlanReplacesAnEarlierRenderAndConflictsOnAForeignFile(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, ".claude/skills/rtdd/SKILL.md", protocol.Generated+"\n\nan earlier release's text\n")
	writeAt(t, root, ".cursor/rules/rtdd.mdc", "a rule the host wrote itself\n")
	steps, err := Plan(root, fakeFiles())
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".claude/skills/rtdd/SKILL.md"); s.Action != Replace || s.Content != fakeFiles()["dist/SKILL.md"] {
		t.Errorf("skill step = %v %q, want Replace with the new render", s.Action, s.Content)
	}
	if s := stepFor(t, steps, ".cursor/rules/rtdd.mdc"); s.Action != Conflict {
		t.Errorf("mdc step = %v, want Conflict: rtdd did not write that file", s.Action)
	}
}
```

and `cmd/rtdd/init_shapes_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

// PRD #411 AC5: `rtdd init` sets up every git repository — the shapes v0.2 refused or
// never met — writes the front-ends and a config the graph loads, and names no adapter.
func TestInitSucceedsOnEveryRepositoryShape(t *testing.T) {
	for _, shape := range []struct {
		name  string
		files map[string]string
	}{
		{"empty", nil},
		{"prose only", map[string]string{"README.md": "# notes\n", "docs/guide.rst": "Guide\n=====\n"}},
		{"a language the scanner does not know", map[string]string{
			"src/main.zig": "pub fn main() void {}\n", "build.zig": "const std = @import(\"std\");\n"}},
		{"polyglot", map[string]string{
			"src/calc.py":        "def add(a, b):\n    return a + b\n",
			"tests/test_calc.py": "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n",
			"pkg/add.go":         "package pkg\n\nfunc Add(a, b int) int { return a + b }\n",
			"pkg/add_test.go":    "package pkg\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { Add(1, 2) }\n",
			"web/app.ts":         "export function greet(n: string) { return n }\n",
			"web/app.test.ts":    "it('greets', () => { greet('a') })\n",
		}},
	} {
		t.Run(shape.name, func(t *testing.T) {
			dir := gittest.Init(t)
			for rel, body := range shape.files {
				gittest.Write(t, dir, rel, body)
			}
			if len(shape.files) > 0 {
				gittest.Commit(t, dir, "init")
			}
			code, out, errOut := rtdd(t, dir, "init")
			if code != 0 {
				t.Fatalf("rtdd init = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
			}
			for _, bad := range []string{"adapter", "detect", "refus", "--force"} {
				if strings.Contains(strings.ToLower(out+errOut), bad) {
					t.Errorf("rtdd init output says %q:\n%s%s", bad, out, errOut)
				}
			}
			for _, rel := range []string{".claude/skills/rtdd/SKILL.md", ".cursor/rules/rtdd.mdc", "AGENTS.md", ".rtdd/config.yaml", ".gitignore"} {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
					t.Errorf("rtdd init did not write %s: %v", rel, err)
				}
			}
			cfg, err := graph.LoadConfig(dir)
			if err != nil {
				t.Fatalf("the graph cannot load the config init wrote: %v", err)
			}
			if !reflect.DeepEqual(cfg, graph.DefaultConfig()) {
				t.Errorf("the config init wrote loads as %+v, want graph.DefaultConfig()", cfg)
			}
			raw := readRepoFileForTest(t, dir, ".rtdd/config.yaml")
			for _, key := range []string{"test_files:", "scan_exclude:", "graphify_path:", "max_stale_ratio:"} {
				if !strings.Contains(raw, key) {
					t.Errorf(".rtdd/config.yaml has no %s\n%s", key, raw)
				}
			}
			if strings.Contains(raw, "adapters") {
				t.Errorf(".rtdd/config.yaml still has an adapters key:\n%s", raw)
			}
		})
	}
}

// PRD #411 AC5: two runs leave `.rtdd/graph.json` in .gitignore exactly once, and the
// second run changes nothing.
func TestInitTwiceIgnoresTheGraphCacheExactlyOnce(t *testing.T) {
	dir := graphRepo(t)
	gittest.Write(t, dir, ".gitignore", "node_modules/\n")
	gittest.Commit(t, dir, "ignore")
	if code, _, errOut := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("first rtdd init = %d, stderr %q", code, errOut)
	}
	first := gitRun(t, dir, "status", "--porcelain", "-uall")
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("second rtdd init = %d, stderr %q", code, errOut)
	}
	if n := strings.Count(readRepoFileForTest(t, dir, ".gitignore"), ".rtdd/graph.json\n"); n != 1 {
		t.Errorf(".gitignore names .rtdd/graph.json %d times, want 1", n)
	}
	if again := gitRun(t, dir, "status", "--porcelain", "-uall"); again != first {
		t.Errorf("the second init changed the tree:\nafter first:\n%s\nafter second:\n%s", first, again)
	}
	for _, line := range strings.Split(strings.TrimSpace(strings.SplitN(out, "\n\n", 2)[0]), "\n") {
		if !strings.HasPrefix(line, "skip") {
			t.Errorf("the second init planned %q, want every step skipped", line)
		}
	}
}

// PRD #411 AC5: `--force` is removed — init --help does not offer it and passing it is a
// usage error.
func TestInitHasNoForceFlag(t *testing.T) {
	dir := graphRepo(t)
	code, _, errOut := rtdd(t, dir, "init", "--force")
	if code != 2 || !strings.Contains(errOut, "flag provided but not defined: -force") {
		t.Errorf("rtdd init --force = %d, stderr %q; want 2 naming the undefined flag", code, errOut)
	}
	_, _, help := rtdd(t, dir, "init", "--help")
	if strings.Contains(help, "force") {
		t.Errorf("rtdd init --help still offers --force:\n%s", help)
	}
	_, usage, _ := rtdd(t, dir, "--help")
	if !regexp.MustCompile(`(?m)^\s*rtdd init\s+\[--dry-run\]$`).MatchString(usage) {
		t.Errorf("rtdd --help does not document `rtdd init [--dry-run]`:\n%s", usage)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/install/ -count=1
go test ./cmd/rtdd/ -count=1 -run 'TestInitSucceedsOnEveryRepositoryShape|TestInitTwiceIgnoresTheGraphCacheExactlyOnce|TestInitHasNoForceFlag'
```

Expected: `internal/install` FAILS TO BUILD — `undefined: DefaultConfig`, `not enough arguments in call to Plan`, `undefined: Replace`, `undefined: AppendLine`. In `cmd/rtdd`: every shape `rtdd init did not write .gitignore` and `.rtdd/config.yaml has no test_files:` (it holds `stale_commits: 50 …`), likewise `scan_exclude:`, `graphify_path:`, `max_stale_ratio:`; `.gitignore names .rtdd/graph.json 0 times, want 1`; `rtdd init --force = 0, stderr ""; want 2 naming the undefined flag`, `rtdd init --help still offers --force`, `rtdd --help does not document `rtdd init [--dry-run]``.

- [ ] **Step 3: The config** — replace `internal/install/config.go` with the file in "`.rtdd/config.yaml`". Delete `defaultConfig` from `install.go`.

- [ ] **Step 4: `Plan`** — in `merge.go` append `Replace` and `AppendLine` after `StripBlock` with `String()` values `"replace"` and `"append-line"`. In `install.go`, as "`rtdd init`" specifies:

```go
// graphCacheLine is the .gitignore line for the graph cache (spec §4.5, §12).
const graphCacheLine = ".rtdd/graph.json"

// Plan computes what `rtdd init` would do without touching the filesystem.
//
// files maps the generator's output paths to their content, i.e. exactly what
// protocol.RenderAll returns. Whole-file targets (the Claude Code skill, the Cursor rule)
// are created, or replaced when the file on disk is an earlier rtdd render — it carries
// protocol.Generated. A file there that rtdd did not write is a conflict: init never
// overwrites someone else's file, and there is no flag that makes it. Marker-delimited
// targets (AGENTS.md, CLAUDE.md) are merged, which is always safe.
func Plan(root string, files map[string]string) ([]Step, error) {
```

The whole-file switch replaces `case force:` with

```go
		case strings.Contains(string(existing), protocol.Generated):
			steps = append(steps, Step{Path: rel, Action: Replace, Content: content, Note: "an earlier rtdd render"})
		default:
			steps = append(steps, Step{
				Path: rel, Action: Conflict,
				Note: "exists and was not written by rtdd; move it aside and re-run",
			})
```

the config step creates `DefaultConfig()`, and `Plan` ends with

```go
	// 4. .gitignore — the graph cache is rebuilt in seconds and conflicts on every branch
	// when committed (spec §4.5), so init ignores it, once.
	step, err := planGitignore(root)
	if err != nil {
		return nil, err
	}
	return append(steps, step), nil
}

// planGitignore adds graphCacheLine to the host's .gitignore unless a line already names
// it, spelled root-anchored or not.
func planGitignore(root string) (Step, error) {
	b, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	switch {
	case os.IsNotExist(err):
		return Step{Path: ".gitignore", Action: Create, Content: graphCacheLine + "\n"}, nil
	case err != nil:
		return Step{}, err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l == graphCacheLine || l == "/"+graphCacheLine {
			return Step{Path: ".gitignore", Action: Skip, Note: "already ignores " + graphCacheLine}, nil
		}
	}
	body := string(b)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return Step{Path: ".gitignore", Action: AppendLine, Content: body + graphCacheLine + "\n"}, nil
}
```

- [ ] **Step 5: The command** — in `cmd/rtdd/init.go` delete the `force` flag; the usage error is `usage: rtdd init [--dry-run]`; call `install.Plan(root, files)`; the conflict message ends `nothing written; move the named file(s) aside and re-run`; `cmdInit`'s comment says a whole-file front-end is replaced only when an earlier rtdd wrote it. In `main.go`'s `usage`, `  rtdd init` becomes `  rtdd init   [--dry-run]`.

- [ ] **Step 6: Update the tests that pinned `--force` and the adapter record** — the Task 3 row of "Old tests: who changes what". `TestInitConflictsOnASkillFileRtddDidNotWrite`:

```go
// A whole-file target rtdd did not write is a conflict: init writes nothing and exits 2,
// and no flag overrides it (PRD #411 AC5 removes --force).
func TestInitConflictsOnASkillFileRtddDidNotWrite(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, ".claude/skills/rtdd/SKILL.md", "hand written, not ours\n")

	code, _, stderr := rtdd(t, dir, "init")
	if code != 2 || !strings.Contains(stderr, "move the named file(s) aside") {
		t.Fatalf("exit code = %d, stderr %q; want 2 telling the user to move the file aside", code, stderr)
	}
	if got := readRepoFileForTest(t, dir, ".claude/skills/rtdd/SKILL.md"); got != "hand written, not ours\n" {
		t.Errorf("init overwrote a file rtdd did not write: %q", got)
	}
}
```

- [ ] **Step 7: Run to verify**

```bash
go vet ./... && go test ./internal/install/ ./cmd/rtdd/ ./internal/contract/ -count=1
```

Expected: `ok` for all three packages.

- [ ] **Step 8: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0.

- [ ] **Step 9: Commit** (name the deleted tests in the body)

```bash
git add internal/install cmd/rtdd
git commit -m "feat(init): rtdd init sets up every git repository, writes the v0.3.0 config and ignores .rtdd/graph.json; --force is removed (closes #480)"
```

---

### Task 4: `rtdd init` on a v0.2 repository deletes the map, meta, adapters and the merge=union line, printing each removal

**Issue:** #481

**Discharges:** PRD #411 AC6, AC8. Spec §8 ("Removed": `init` on a v0.2 repository deletes and says so).

**Files:**
- Create: `internal/install/migrate.go`
- Modify: `internal/install/install.go` (`Plan` step 0 and the config step; `Apply` handles `Delete`)
- Modify: `internal/install/uninstall.go`, `cmd/rtdd/uninstall.go` (`--state` notes: "config and graph cache", not "recorded map")
- Modify: `internal/contract/pipeline_removed_test.go` (`v02StateAllowed` + 3), `internal/install/install_test.go`, `cmd/rtdd/init_gate_test.go` (Task 4 row of "Old tests")
- Test: `internal/install/migrate_test.go`, `cmd/rtdd/init_migrate_test.go`

**Interfaces:**
- Consumes: `Plan`, `DefaultConfig`, `writeAt` (Task 3); `gitattributesLine`, `removeLine` (`uninstall.go`); `yaml.Unmarshal`.
- Produces: `func PlanMigration(root string) ([]Step, error)`; `func isV02Config(b []byte) bool`; `var v02Files`.

- [ ] **Step 1: Write the failing tests** — `internal/install/migrate_test.go`:

```go
package install

import (
	"os"
	"path/filepath"
	"testing"
)

// v02Layout lays out what a v0.2 `rtdd init` and `rtdd seed` left in a repository.
func v02Layout(t *testing.T, gitattributes string) string {
	t.Helper()
	root := t.TempDir()
	writeAt(t, root, ".rtdd/map.jsonl", `{"t":"tests/test_a.py","f":["src/a.py"],"c":"a3f21e0","d":12,"s":"pass","a":"python"}`+"\n")
	writeAt(t, root, ".rtdd/meta.json", `{"v":2,"seeded_at":"a3f21e0"}`+"\n")
	writeAt(t, root, ".rtdd/adapters/python.yaml", "name: python\n")
	writeAt(t, root, ".rtdd/config.yaml", "stale_commits: 50\ndrift_guard: 100\nhub_threshold: 0.40\nadapters:\n  - name: python\n")
	if gitattributes != "" {
		writeAt(t, root, ".gitattributes", gitattributes)
	}
	return root
}

// PRD #411 AC6: each of the four v0.2 artefacts is planned for removal, and a
// .gitattributes keeps every line but rtdd's.
func TestPlanMigrationRemovesEveryV02Artefact(t *testing.T) {
	root := v02Layout(t, "*.png binary\n.rtdd/map.jsonl merge=union\n")
	steps, err := PlanMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".rtdd/map.jsonl", ".rtdd/meta.json", ".rtdd/adapters/"} {
		if s := stepFor(t, steps, rel); s.Action != Delete || s.Note == "" {
			t.Errorf("%s: step = %v %q, want Delete with a note saying what it was", rel, s.Action, s.Note)
		}
	}
	if s := stepFor(t, steps, ".gitattributes"); s.Action != StripBlock || s.Content != "*.png binary\n" {
		t.Errorf(".gitattributes: step = %v %q, want StripBlock leaving %q", s.Action, s.Content, "*.png binary\n")
	}
	if err := Apply(root, steps); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".rtdd/map.jsonl", ".rtdd/meta.json", ".rtdd/adapters"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s survived Apply: %v", rel, err)
		}
	}
}

// PRD #411 AC6: a .gitattributes that held nothing but the v0.2 line goes with it, as
// uninstall already does.
func TestPlanMigrationDeletesAGitattributesHoldingOnlyTheV02Line(t *testing.T) {
	root := v02Layout(t, ".rtdd/map.jsonl merge=union\n")
	steps, err := PlanMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".gitattributes"); s.Action != Delete {
		t.Errorf(".gitattributes: step = %v, want Delete", s.Action)
	}
}

// PRD #411 AC6: a repository v0.2 never touched plans no migration step at all.
func TestPlanMigrationOnAFreshRepositoryPlansNothing(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, ".gitattributes", "*.png binary\n")
	writeAt(t, root, ".rtdd/config.yaml", DefaultConfig())
	steps, err := PlanMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 0 {
		t.Errorf("PlanMigration on a fresh repository = %+v, want no step", steps)
	}
}

// PRD #411 AC6: a config holding only v0.2 keys is replaced by the v0.3.0 defaults; a
// config that sets any v0.3.0 key is the user's and is kept.
func TestPlanReplacesAV02OnlyConfigAndKeepsAV030One(t *testing.T) {
	root := v02Layout(t, "")
	steps, err := Plan(root, fakeFiles())
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".rtdd/config.yaml"); s.Action != Replace || s.Content != DefaultConfig() {
		t.Errorf("v0.2 config: step = %v %q, want Replace with DefaultConfig()", s.Action, s.Content)
	}
	root = t.TempDir()
	writeAt(t, root, ".rtdd/config.yaml", "max_stale_ratio: 0.3\nstale_commits: 50\n")
	steps, err = Plan(root, fakeFiles())
	if err != nil {
		t.Fatal(err)
	}
	if s := stepFor(t, steps, ".rtdd/config.yaml"); s.Action != Skip {
		t.Errorf("a config setting max_stale_ratio: step = %v, want Skip", s.Action)
	}
}
```

and `cmd/rtdd/init_migrate_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
)

// v02Repository is a repository as v0.2 left it: seeded map, its meta file, a host adapter,
// the adapter-era config, the merge driver beside an unrelated .gitattributes line, and the
// skill v0.2 rendered.
func v02Repository(t *testing.T) string {
	t.Helper()
	dir := graphRepo(t)
	writeFile(t, dir, ".rtdd/map.jsonl", `{"t":"tests/test_calc.py","f":["src/calc.py"],"c":"a3f21e0","d":12,"s":"pass","a":"python"}`+"\n")
	writeFile(t, dir, ".rtdd/meta.json", `{"v":2,"seeded_at":"a3f21e0"}`+"\n")
	writeFile(t, dir, ".rtdd/adapters/python.yaml", "name: python\n")
	writeFile(t, dir, ".rtdd/config.yaml", "stale_commits: 50\ndrift_guard: 100\nhub_threshold: 0.40\nadapters:\n  - name: python\n")
	writeFile(t, dir, ".gitattributes", "*.png binary\n.rtdd/map.jsonl merge=union\n")
	writeFile(t, dir, ".claude/skills/rtdd/SKILL.md", "---\nname: rtdd\n---\n\n"+protocol.Generated+"\n\nRun `rtdd seed` once.\n")
	gittest.Commit(t, dir, "v0.2 setup")
	return dir
}

// removals are the lines of init's output that remove something.
func removals(out string) []string {
	return regexp.MustCompile(`(?m)^(delete|strip-block)\s+\S+.*$`).FindAllString(out, -1)
}

// PRD #411 AC6: init on a v0.2 repository removes all four artefacts, says so once each,
// keeps the unrelated .gitattributes line, and leaves the repository set up for v0.3.0.
func TestInitMigratesAV02Repository(t *testing.T) {
	dir := v02Repository(t)
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	for _, rel := range []string{".rtdd/map.jsonl", ".rtdd/meta.json", ".rtdd/adapters"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s survived init: %v", rel, err)
		}
	}
	if got := readRepoFileForTest(t, dir, ".gitattributes"); got != "*.png binary\n" {
		t.Errorf(".gitattributes = %q, want only the unrelated line left", got)
	}
	for _, re := range []string{`(?m)^delete\s+\.rtdd/map\.jsonl\b`, `(?m)^delete\s+\.rtdd/meta\.json\b`,
		`(?m)^delete\s+\.rtdd/adapters/`, `(?m)^strip-block\s+\.gitattributes\b`} {
		if !regexp.MustCompile(re).MatchString(out) {
			t.Errorf("init output does not report the removal %s:\n%s", re, out)
		}
	}
	if n := len(removals(out)); n != 4 {
		t.Errorf("init reported %d removals, want 4:\n%s", n, out)
	}
	cfg, err := graph.LoadConfig(dir)
	if err != nil || !reflect.DeepEqual(cfg, graph.DefaultConfig()) {
		t.Errorf("after migration the config loads as %+v (%v), want graph.DefaultConfig()", cfg, err)
	}
	if raw := readRepoFileForTest(t, dir, ".rtdd/config.yaml"); strings.Contains(raw, "stale_commits") || strings.Contains(raw, "adapters") {
		t.Errorf("the v0.2 config survived:\n%s", raw)
	}
	if !strings.Contains(readRepoFileForTest(t, dir, ".gitignore"), ".rtdd/graph.json\n") {
		t.Error("the migrated repository does not ignore .rtdd/graph.json")
	}
	files, err := install.Files()
	if err != nil {
		t.Fatal(err)
	}
	if got := readRepoFileForTest(t, dir, ".claude/skills/rtdd/SKILL.md"); got != files["dist/SKILL.md"] {
		t.Errorf("the v0.2 skill was not replaced by the v0.3.0 render:\n%s", got)
	}
}

// PRD #411 AC6: a second init on the migrated repository removes and reports nothing.
func TestInitTwiceOnAMigratedRepositoryRemovesNothing(t *testing.T) {
	dir := v02Repository(t)
	if code, _, errOut := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("first rtdd init = %d, stderr %q", code, errOut)
	}
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("second rtdd init = %d, stderr %q", code, errOut)
	}
	if r := removals(out); len(r) != 0 {
		t.Errorf("the second init reported removals %q", r)
	}
}

// PRD #411 AC6: a repository v0.2 never touched gets no removal line.
func TestInitOnAFreshRepositoryPrintsNoRemoval(t *testing.T) {
	dir := graphRepo(t)
	writeFile(t, dir, ".gitattributes", "*.png binary\n")
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, stderr %q", code, errOut)
	}
	if r := removals(out); len(r) != 0 {
		t.Errorf("init on a fresh repository reported removals %q", r)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/install/ -count=1
go test ./cmd/rtdd/ -count=1 -run 'TestInitMigratesAV02Repository|TestInitTwiceOnAMigratedRepositoryRemovesNothing|TestInitOnAFreshRepositoryPrintsNoRemoval'
```

Expected: `internal/install` FAILS TO BUILD — `undefined: PlanMigration`. In `cmd/rtdd`: `.rtdd/map.jsonl survived init`, `.rtdd/meta.json survived init`, `.rtdd/adapters survived init`, `.gitattributes = "*.png binary\n.rtdd/map.jsonl merge=union\n", want only the unrelated line left`, `init output does not report the removal (?m)^delete\s+\.rtdd/map\.jsonl\b` (and the other three), `init reported 0 removals, want 4`, `the v0.2 config survived`. `TestInitOnAFreshRepositoryPrintsNoRemoval` and `TestInitTwiceOnAMigratedRepositoryRemovesNothing` already PASS — nothing removes anything yet — and guard the implementation.

- [ ] **Step 3: The migration** — create `internal/install/migrate.go`:

```go
package install

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// v02Files are the state files a v0.2 repository holds and v0.3.0 reads none of (spec §8,
// "Removed"), with the note `rtdd init` prints as it deletes each one.
var v02Files = []struct{ path, note string }{
	{".rtdd/map.jsonl", "the v0.2 coverage map; v0.3.0 builds a node graph instead"},
	{".rtdd/meta.json", "the v0.2 map's metadata"},
	{".rtdd/adapters/", "v0.2 adapter definitions; v0.3.0 has no adapters"},
}

// PlanMigration computes what `rtdd init` removes from a repository a v0.2 init set up:
// the three state files above, and the merge-driver line in .gitattributes — the file
// itself only when that line was all it held, as uninstall does. A repository v0.2 never
// touched plans no step at all, so its init output carries no removal line.
func PlanMigration(root string) ([]Step, error) {
	steps := []Step{}
	for _, f := range v02Files {
		switch _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f.path))); {
		case os.IsNotExist(err):
		case err != nil:
			return nil, err
		default:
			steps = append(steps, Step{Path: f.path, Action: Delete, Note: f.note})
		}
	}
	b, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if !strings.Contains(string(b), gitattributesLine) {
		return steps, nil
	}
	note := "removed the v0.2 line " + gitattributesLine
	if remaining := removeLine(string(b), gitattributesLine); strings.TrimSpace(remaining) != "" {
		return append(steps, Step{Path: ".gitattributes", Action: StripBlock, Content: remaining, Note: note}), nil
	}
	return append(steps, Step{Path: ".gitattributes", Action: Delete, Note: "held nothing but the v0.2 line " + gitattributesLine}), nil
}

// isV02Config reports whether a .rtdd/config.yaml is the one v0.2 wrote: it sets at least
// one key only v0.2 read and none v0.3.0 reads. Such a file is replaced by DefaultConfig;
// a file setting any v0.3.0 key is the user's and is kept. Unparseable YAML is kept too —
// graph.LoadConfig reports it, naming the file.
func isV02Config(b []byte) bool {
	var keys map[string]any
	if yaml.Unmarshal(b, &keys) != nil {
		return false
	}
	v02 := false
	for k := range keys {
		switch k {
		case "scan_exclude", "test_files", "test_exclude", "graphify_path", "max_stale_ratio":
			return false
		case "stale_commits", "drift_guard", "hub_threshold", "adapters":
			v02 = true
		}
	}
	return v02
}
```

- [ ] **Step 4: Wire it into `Plan` and `Apply`** — `Plan` starts with

```go
	// 0. What a v0.2 init left behind goes first, each removal its own printed step.
	steps, err := PlanMigration(root)
	if err != nil {
		return nil, err
	}
```

its config step becomes

```go
	// 3. .rtdd/config.yaml — created; replaced only when it is v0.2's (isV02Config).
	cfg, err := os.ReadFile(filepath.Join(root, ".rtdd", "config.yaml"))
	switch {
	case os.IsNotExist(err):
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Create, Content: DefaultConfig()})
	case err != nil:
		return nil, err
	case isV02Config(cfg):
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Replace, Content: DefaultConfig(), Note: "the v0.2 config; v0.3.0 reads none of its keys"})
	default:
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Skip, Note: "keeping your config"})
	}
```

and `Apply`, before it creates directories for a write, removes a `Delete` step's path:

```go
		if s.Action == Delete {
			if err := os.RemoveAll(abs); err != nil {
				return err
			}
			continue
		}
```

- [ ] **Step 5: The guards and the pinned configs** — add to `v02StateAllowed`:

```go
	"internal/install/migrate.go":                "rtdd init deletes v0.2 state from a host repository (PRD #411 AC6)",
	"internal/install/migrate_test.go":           "builds the v0.2 state PlanMigration deletes",
	"cmd/rtdd/init_migrate_test.go":             "builds the v0.2 repository rtdd init migrates",
```

(`gofmt -w` realigns the map). Change the "tuned" config of `TestPlanConfigIsCreatedOnceThenNeverOverwritten` and `TestInitNeverRewritesAnExistingConfigToAddTheRecord` to `max_stale_ratio: 0.3\n`. In `uninstall.go` and `cmd/rtdd/uninstall.go`, the `.rtdd/` notes, the `--state` flag text and the `State` comment say "the config and the graph cache" instead of "the config and the recorded map".

- [ ] **Step 6: Run to verify**

```bash
go vet ./... && go test ./internal/install/ ./cmd/rtdd/ ./internal/contract/ -count=1
```

Expected: `ok` for all three packages.

- [ ] **Step 7: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0.

- [ ] **Step 8: Commit**

```bash
git add internal/install internal/contract cmd/rtdd
git commit -m "feat(init): rtdd init on a v0.2 repository deletes map.jsonl, meta.json, adapters/ and the merge=union line, printing each removal (closes #481)"
```

---

### Task 5: README, DEVELOPMENT.md and `docs/` describe v0.3.0; superseded specs are marked superseded, not deleted

**Issue:** #482

**Discharges:** PRD #411 AC7, AC8 (the PRD-wide gate, on a branch holding Tasks 1–4).

**Files:**
- Modify: `README.md`, `docs/outcomes/README.positive.md` (byte-identical), `DEVELOPMENT.md`, `docs/LIMITATIONS.md`
- Modify: `docs/specs/2026-08-26-rtdd-design.md`, `docs/specs/2026-09-05-multi-language.md`, `docs/specs/2026-09-29-one-pipeline.md` (banner, line 1)
- Test: `internal/contract/docs_v030_test.go`, `cmd/rtdd/readme_example_test.go`

**Interfaces:**
- Consumes: `readRepoFile` (`internal/contract`); `rtdd`, `usage` (`cmd/rtdd`); `gittest`.
- Produces: the `<!-- rtdd:v0.2-record -->` / `<!-- /rtdd:v0.2-record -->` markers and `supersededBanner`, for any later page that keeps a v0.2 record.

- [ ] **Step 1: Write the failing tests** — `internal/contract/docs_v030_test.go`:

```go
package contract

import (
	"regexp"
	"strings"
	"testing"
)

// v030Docs are the pages that describe rtdd as it is now (PRD #411 AC7). README.md and
// README.positive.md are one file twice (internal/installtest/outcomes_test.go).
var v030Docs = []string{"README.md", "docs/outcomes/README.positive.md", "DEVELOPMENT.md", "docs/LIMITATIONS.md"}

// A v0.2 measurement or changelog kept on a v0.3.0 page sits between these markers, and
// the v0.2 vocabulary guard does not read it: it is a dated record, not a description.
const (
	v02RecordOpen  = "<!-- rtdd:v0.2-record -->"
	v02RecordClose = "<!-- /rtdd:v0.2-record -->"
)

// outsideV02Records drops every marked record from src; an unbalanced marker is an error.
func outsideV02Records(t *testing.T, rel, src string) string {
	t.Helper()
	var b strings.Builder
	for {
		i := strings.Index(src, v02RecordOpen)
		if i < 0 {
			break
		}
		j := strings.Index(src[i:], v02RecordClose)
		if j < 0 {
			t.Errorf("%s opens a v0.2 record it never closes", rel)
			return src
		}
		b.WriteString(src[:i])
		src = src[i+j+len(v02RecordClose):]
	}
	b.WriteString(src)
	if strings.Contains(b.String(), v02RecordClose) {
		t.Errorf("%s closes a v0.2 record it never opened", rel)
	}
	return b.String()
}

// v02AsCurrent is v0.2 behaviour a v0.3.0 page may not present as current. "no coverage"
// and "no adapters" say what v0.3.0 does not do, and are removed before matching.
var v02AsCurrent = []struct {
	what string
	re   *regexp.Regexp
}{
	{"a removed command", regexp.MustCompile(`\brtdd\s+(seed|run|verify|status|map)\b`)},
	{"the v0.2 map file", regexp.MustCompile(`map\.jsonl`)},
	{"adapters", regexp.MustCompile(`(?i)\badapters?\b`)},
	{"coverage", regexp.MustCompile(`(?i)\bcoverage\b`)},
	{"the T0/T1/T2 tiers", regexp.MustCompile(`\bT[0-2]\b`)},
	{"selection_fidelity", regexp.MustCompile(`selection_fidelity`)},
}

var saysWhatV030DoesNot = regexp.MustCompile(`(?i)\bno\s+(coverage|adapters?)\b`)

// PRD #411 AC7: README, DEVELOPMENT.md and LIMITATIONS.md present no v0.2 behaviour as
// current, and README and DEVELOPMENT.md describe the v0.3.0 commands and process.
func TestUserDocsDescribeV030NotV02(t *testing.T) {
	for _, rel := range v030Docs {
		body := saysWhatV030DoesNot.ReplaceAllString(outsideV02Records(t, rel, readRepoFile(t, rel)), "")
		for _, v := range v02AsCurrent {
			for _, m := range v.re.FindAllString(body, 3) {
				t.Errorf("%s presents %s as current (%q) outside a v0.2 record", rel, v.what, m)
			}
		}
		if rel == "docs/LIMITATIONS.md" {
			continue
		}
		for _, want := range []string{"rtdd init", "rtdd which", "rtdd graph", "rtdd explain", "rtdd doctor",
			"Round 1", "Round 2", "full suite once", "graphify"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not mention %q", rel, want)
			}
		}
	}
}

// supersededSpecs are the designs docs/specs/2026-10-07-node-graph.md replaces. They stay
// as the record of what was built and why; each opens with supersededBanner.
var supersededSpecs = []string{
	"docs/specs/2026-08-26-rtdd-design.md",
	"docs/specs/2026-09-05-multi-language.md",
	"docs/specs/2026-09-29-one-pipeline.md",
}

const supersededBanner = "> **Superseded** by [`2026-10-07-node-graph.md`](2026-10-07-node-graph.md) (v0.3.0)."

// PRD #411 AC7: every superseded spec still exists and opens with the banner; the current
// spec does not carry it.
func TestSupersededSpecsAreMarkedNotDeleted(t *testing.T) {
	for _, rel := range supersededSpecs {
		if src := readRepoFile(t, rel); !strings.HasPrefix(src, supersededBanner+"\n") {
			t.Errorf("%s does not open with the banner %q", rel, supersededBanner)
		}
	}
	if strings.Contains(readRepoFile(t, "docs/specs/2026-10-07-node-graph.md"), "**Superseded**") {
		t.Error("the current spec carries a superseded banner")
	}
}
```

and `cmd/rtdd/readme_example_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// readmeBlock returns the body of README.md's fenced block that opens with first, up to
// its closing fence.
func readmeBlock(t *testing.T, first string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	i := strings.Index(src, "```\n"+first+"\n")
	if i < 0 {
		t.Fatalf("README.md has no fenced block opening with %q", first)
	}
	body := src[i+len("```\n"+first+"\n"):]
	return body[:strings.Index(body, "```\n")]
}

// PRD #411 AC7: README's `rtdd which` example is real output — this repository, this
// edit, this binary — so the README cannot describe output rtdd no longer prints.
func TestREADMEWhichExampleIsRealOutput(t *testing.T) {
	want := readmeBlock(t, "$ rtdd which")
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add, total\n\n\ndef test_add():\n    assert add(1, 2) == 3\n\n\ndef test_total():\n    assert total([1, 2]) == 3\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return b + a\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n")
	code, got, errOut := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	// The commit is the one thing that differs between runs: README shows one, the test
	// makes another.
	sha := regexp.MustCompile(`built at [0-9a-f]+,`)
	got, want = sha.ReplaceAllString(got, "built at <HEAD>,"), sha.ReplaceAllString(want, "built at <HEAD>,")
	if got != want {
		t.Errorf("README.md's `$ rtdd which` example is not what rtdd prints\n--- README ---\n%s--- rtdd which ---\n%s", want, got)
	}
}

// PRD #411 AC7: every `rtdd <command>` README shows in a code block is a command rtdd has.
func TestREADMEShowsOnlyCommandsRtddHas(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if i, j := strings.Index(src, "<!-- rtdd:v0.2-record -->"), strings.Index(src, "<!-- /rtdd:v0.2-record -->"); i >= 0 && j > i {
		src = src[:i] + src[j:]
	}
	fence := regexp.MustCompile("(?s)```[a-z]*\n(.*?)```")
	cmd := regexp.MustCompile(`(?m)^\$?\s*rtdd\s+([a-z-]+)`)
	for _, block := range fence.FindAllStringSubmatch(src, -1) {
		for _, m := range cmd.FindAllStringSubmatch(block[1], -1) {
			if !strings.Contains(usage, "rtdd "+m[1]) {
				t.Errorf("README.md shows `rtdd %s`, which `rtdd --help` does not list", m[1])
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/contract/ ./cmd/rtdd/ -count=1 -run 'TestUserDocsDescribeV030NotV02|TestSupersededSpecsAreMarkedNotDeleted|TestREADMEWhichExampleIsRealOutput|TestREADMEShowsOnlyCommandsRtddHas'
```

Expected: FAIL — `README.md presents a removed command as current ("rtdd seed") outside a v0.2 record`, `… adapters as current ("adapter")`, `… coverage as current ("coverage")`, `… the T0/T1/T2 tiers as current ("T0")`, `README.md does not mention "Round 1"`, `"full suite once"`, `"graphify"`, `"rtdd explain"` (the same for README.positive.md); `DEVELOPMENT.md presents selection_fidelity as current`, `DEVELOPMENT.md does not mention "rtdd init"`; `docs/specs/2026-08-26-rtdd-design.md does not open with the banner` (and the other two); `README.md's `$ rtdd which` example is not what rtdd prints` (it shows `tier: T0`); `README.md shows `rtdd seed`, which `rtdd --help` does not list`, likewise `rtdd run`.

- [ ] **Step 3: The banners** — prepend the banner line from "The docs", then a blank line, to each of the three superseded specs. Delete nothing.

- [ ] **Step 4: README** — rewrite as "The docs" says; paste the output of `TestREADMEWhichExampleIsRealOutput`'s repository as the `$ rtdd which` block (run the test and copy its `--- rtdd which ---` half). Put the "Does it work?" measurements inside a v0.2 record. Copy the result to `docs/outcomes/README.positive.md` (`cp README.md docs/outcomes/README.positive.md`).

- [ ] **Step 5: DEVELOPMENT.md and LIMITATIONS.md** — as the table in "The docs" says.

- [ ] **Step 6: Run to verify**

```bash
go test ./internal/contract/ ./cmd/rtdd/ ./internal/installtest/ -count=1
```

Expected: `ok` for all three packages — `internal/installtest`'s `TestRootREADMEMatchesSelectedOutcome`, the README install tests and the time/safety-axis tests included.

- [ ] **Step 7: The PRD-wide gate** — on a branch holding Tasks 1–4, `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0 (PRD #411 AC8).

- [ ] **Step 8: Commit**

```bash
git add README.md DEVELOPMENT.md docs internal/contract cmd/rtdd
git commit -m "docs: README, DEVELOPMENT.md and docs/ describe v0.3.0; superseded specs are marked, not deleted (closes #482)"
```

---

## What this plan deliberately leaves undone

Everything below is real work; none of it belongs to PRD #411, and no task above may start it.

- **Graph, scanner, graphify loading** (PRD #409) and **`Rounds`, `which`, `explain`, `doctor`, `graph`, exit codes, schema 3** (PRD #410). N3 describes their behaviour in the front-ends and docs; a front-end sentence that turns out wrong about them is fixed in the text, not by changing the command here.
- **Running or updating graphify**, from rtdd or from a test (spec §11). The front-ends say the agent does it if it wants the graph.
- **The machine-wide front-ends' installer.** `rtdd skill install` keeps its own `--force`; `rtdd update` refreshing the machine-wide skill is unchanged. Both pick up the new text through the embedded protocol and need no code change.
- **rtdd-bench** (PRD #412): the bench's rtdd strategy, its schema-3 consumer, the Round 1 / Rounds 1+2 measurements, the README's v0.3.0 numbers, `docs/outcomes/README.negative.md` and the decision of which outcome README is live. Task 5 keeps the v0.2 measurements as a labelled record until #412 replaces them.
- **Historical documents.** `docs/plans/` (this plan included), `docs/results/`, `docs/audits/`, `docs/bench/` stay as written; superseded specs gain a banner and nothing else.
- **A config migration beyond replacing a v0.2-only file.** A config mixing v0.2 and v0.3.0 keys is kept; its v0.2 keys are ignored by `graph.LoadConfig` and harmless. Rewriting YAML in place while keeping a user's comments is not worth a parser.
- **Matching `.gitignore` semantics.** `init` checks for the exact line; deciding whether a broader pattern already ignores `.rtdd/graph.json` would need `git check-ignore`, and an extra line is harmless.
- **Spec §12's defaults** — the gitignored cache included. A lane that finds one wrong files an issue against the spec; it does not change it here.
- **Tagging and releasing v0.3.0** — a human action (#418).
