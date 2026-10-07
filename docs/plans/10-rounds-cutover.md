# Rounds Cutover (N2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ] `) syntax for tracking.

**Goal:** Make the node graph PRD #409 built the only thing rtdd selects from. A pure `Rounds` turns the graph and the changed lines of a diff into Round 1 (the changed node's tests), Round 2 (the tests of its depth-1 neighbours) and the untested changed nodes; `rtdd which`, `rtdd explain` and `rtdd doctor` are rewritten on it; `rtdd which --json` becomes schema 3. Then the coverage pipeline — `seed`, `run`, `verify`, `status`, `map compact`, the adapters, the map and every package that served them — is deleted, so v0.3.0 has one selection path and it runs nothing.

**Architecture:** One new package, `internal/rounds`, holds spec §7 as a pure function over `graph.Graph` and a `map[file][]LineRange`: no `os`, no git, no process — every rule is tested on a graph built by hand. `cmd/rtdd` is the only place it meets the world: `which.go` takes changed line ranges from `gitctx.ChangedSet` (the one diff the engine already parses), builds the graph with `graphbuild.Build` exactly as `rtdd graph` does, calls `rounds.Rounds`, and prints text or the schema-3 document. `explain.go` and `doctor.go` read the same built graph. The commands are rewritten in place first, while the v0.2 code they used to call still compiles beside them; the v0.2 commands are then retired behind a "removed in v0.3.0" exit 2; and only then are the packages deleted, leaf first, in an order where every commit builds and `scripts/ci-local.sh` passes.

**Tech Stack:** Go 1.24 (module `github.com/VocanicZ/rtdd`), stdlib only for everything new (`cmp`, `slices`, `strings`, `encoding/json`, `go/parser` in tests); `gopkg.in/yaml.v3` stays the module's one dependency. Tests use stdlib `testing` and real git repositories built with `internal/gitctx/gittest`; no toolchain of any scanned language is invoked — a PATH tripwire proves it.

**Spec:** [`docs/specs/2026-10-07-node-graph.md`](../specs/2026-10-07-node-graph.md) §7 (rounds), §8 (commands, removals, exit codes), §9 (`--json` schema 3), §11 (non-goals) and §12 (the defaults this plan must not re-decide) — PRD #410. Each task names the sibling issue it is and the PRD acceptance criteria it discharges; the map after the decisions lists all nine.

## Global Constraints

- **rtdd runs no test and no toolchain.** Nothing in `which`, `explain`, `doctor` or `graph` executes a test runner, a compiler, a coverage tool or graphify; the only child process is git, through `internal/gitctx`. Tasks 2 and 8 prove it with a PATH holding only `git` and a tripwire for every v0.2 toolchain.
- **No per-language code path outside the scanner's single regex table**, `internal/scan/patterns.go` (spec §4.2, PRD #409). `internal/rounds` and the commands know nodes, edges and line numbers — never a language, an extension or a framework.
- **Only `internal/gitctx` shells out to git** — the existing contract test `TestOnlyGitctxShellsOutToGit` (`internal/contract/nomock_test.go`) stays green unmodified. The one git change this plan needs is an exported sentinel in `internal/gitctx` (`ErrUnknownBase`, Task 2).
- **No new runtime dependency, no cgo.** `go.mod` keeps exactly `gopkg.in/yaml.v3` (`TestGoModRequiresExactlyYAML`); the release binary stays `CGO_ENABLED=0` and statically linked.
- **Exit codes are 0 / 2 / 3 only** (spec §8): 0 success, empty rounds included; 2 usage (unknown flag, unknown `--base` ref, an `explain` argument that names nothing, a removed command, a malformed `.rtdd/config.yaml`); 3 environment (not a git repository, git unavailable, a graph that cannot be built). **No exit code reflects a test result** — rtdd runs none, and exit code 1 leaves the usage text with `run`.
- **Out of scope:** the protocol and skill text (`protocol/PROTOCOL.md`, `dist/`, `internal/protocol`), `rtdd init`'s v0.2 migration and the config it writes, README, DEVELOPMENT.md and user-facing `docs/` — PRD **#411**. rtdd-bench, the bench's rtdd strategy and its schema-3 consumer — PRD **#412** (this plan only fences the bench tests that drive a v0.2 binary; see "The bench replay harness").
- **Spec §12 is settled, not open:** depth-1 neighbours in **both** directions (#414); all same-named definitions linked, `doctor` lists names defined ≥ 8 times (#417); `.rtdd/graph.json` a gitignored cache; graphify never run. No task revisits these.
- `scripts/ci-local.sh` exits 0 at the end of every task, run with **this branch's** `rtdd` first on PATH, as `ci.yml`'s prereg job does:

  ```bash
  bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
  ```

  The bench replay gate drives whatever `rtdd` is on PATH; a stale installed binary fails it for reasons no branch change can fix.

## Review Focus

- A **changed line inside two nested nodes** — it belongs to the innermost one only; the class around a changed method is not a changed node. Test in Task 1 (`TestInnermostNodeOwnsAChangedLine`).
- A **top-level change in a test file** (an import, a fixture) — its tests are Round 1 and its `<module>` node is never "untested". Without this rule an import edit in a test file tells the agent to write a test for the test file. Test in Task 1.
- A **neighbour reached by a caller edge** — spec §12 says both directions; a callee-only implementation passes half the Round 2 tests. Test in Task 1 (caller *and* callee in one case) and Task 8 (end to end).
- **Empty Rounds 1 and 2** — the output says "no linked test" and nothing that reads as a pass, and exits 0. Tests in Tasks 2 and 8.
- **`--base` with an unknown ref** — exit 2, not 3: it is the user's typo, not the environment. Needs `gitctx.ErrUnknownBase`, not string matching. Test in Task 2.
- **A deleted file** — no line of it survives to own a node, so it selects nothing; `warnings` says so instead of staying silent. Pinned by the Task 3 golden.
- **The deletion order** — `internal/contract/contract_test.go` holds `repoRoot` and `readRepoFile` for every contract test, and `cmd/rtdd/which_test.go`, `seed_test.go` and `run_test.go` hold helpers other test files call; deleting any of them wholesale breaks unrelated packages. See "Deletion order" and Task 7.

## `internal/rounds`

Spec §7 lives in one package that imports `internal/graph` and the standard library's `cmp`, `slices` and `strings` — nothing that can open a file, run git or start a process (`TestRoundsPackageImportsNoIO`, Task 1). Its whole API:

```go
type LineRange struct{ Start, End int }       // 1-based, inclusive, new side of the diff

const ModuleName = "<module>"                  // the synthetic node's name
const KindModule graph.Kind = "module"         // its kind; no graph source produces it

type Result struct {
	ChangedNodes []graph.Node
	Round1       []graph.Node
	Round2       []graph.Node
	Untested     []graph.Node
}

func Rounds(g graph.Graph, changed map[string][]LineRange) Result   // Task 1
func Owner(g graph.Graph, file string, line int) (graph.Node, bool) // Task 1
func LinksOf(g graph.Graph, id string) Links                        // Task 4
```

- `changed` is keyed by repo-relative, slash-separated path — the spelling of `graph.Node.File` and of `gitctx.Change.Path`. A nil or empty map, or a file with no ranges, is an empty `Result`, never an error.
- Every slice of `Result` is **non-nil** and ordered by `File`, then `Start`, then `ID` — spec §7's "by file then line", made total. `Untested` carries nodes, not IDs; the JSON layer prints their IDs.
- Edges are traversed only over `graph.Relations` (`calls|method|inherits|implements|references`). That is the whole closed set of §3 today; the filter states §7 in code, so a relation added to the model later is not traversed until §7 says so.
- **Innermost ownership.** A file's nodes are painted onto its lines outer first (by `Start`, then `End` descending), so a nested node overwrites its parent's lines and each line ends with its innermost owner. `Owner` answers the same question for one line (used by `explain file:line`).
- **"Tested"** (for `Untested`) is decided per node: a changed non-test node is tested when a test has an edge to it, or a test has an edge to one of its own neighbours. A node whose only test is reached through *another* changed node's neighbour is still untested — the agent is told about the node it changed, not about a test that happens to run.
- No depth beyond 1, no weighting, no pruning of over-linked names (spec §11, §12).

## Changed line ranges: what `internal/gitctx` gives

`rtdd which` takes changed ranges from **`gitctx.ChangedSet(root, base)`**, through `cmd/rtdd/changed.go`'s `changedSet` (which drops `.rtdd/`). No new git function is needed:

| `--base <ref>` | What `ChangedSet` already returns | Ranges handed to `Rounds` |
|---|---|---|
| `HEAD` (default) | `git diff -M <base>` name-status ∪ `git status --porcelain -uall`; `Lines` from `git diff --unified=0 <base>` hunks (new side) | each `Change.Lines`, verbatim |
| any commit | the same, so committed-since-`<ref>`, staged, unstaged and untracked are one set | the same |
| untracked file | `Status: Added`, `Lines` = the whole file | the whole file: every node in it is changed |
| deleted file | `Status: Deleted`, `Lines` nil | none — and one entry in `warnings` (Task 2) |
| pure deletion inside a file | a one-line range at the surviving line after the cut (`parseHunkHeader`) | that line: its owner is the changed node |
| renamed file | `Status: Renamed`, `Lines` from its hunks (none for a 100 % rename) | its hunks |

`git diff <base>` compares `<base>` to the **working tree**, so every range is in working-tree line numbers — the numbering of the graph `graphbuild.Build` returns, because every file in the working-tree changed set is re-scanned from the working tree (spec §4.5, §5). The two always agree; nothing re-maps lines.

The one gitctx change (Task 2): `ChangedSet` wraps its unknown-base error around a new exported sentinel, `var ErrUnknownBase = errors.New("unknown base")`, keeping the message text `gitctx: unknown base "<ref>"`. `which` maps `errors.Is(err, gitctx.ErrUnknownBase)` to exit 2 and every other `ChangedSet` error to 3. `gitctx.RawDiff` is no longer called by anything after Task 7 and stays (it is gitctx's, and harmless).

## The synthetic `<module>` node

A changed line that no node contains — an import, a constant, a decorator, a `main` block — maps to one synthetic node per file:

| field | value |
|---|---|
| `ID` | `<file>::<module>` — e.g. `src/report.py::<module>`; IDs are `file::name` (PRD #409's `Node.ID` rule) and `<module>` is never a scanned name, so it cannot collide |
| `Name` | `rounds.ModuleName` = `"<module>"` |
| `Kind` | `rounds.KindModule` = `"module"` — declared in `internal/rounds`, not in `internal/graph`: no graph source produces it and §3's model stays closed |
| `File` | the file |
| `Start`, `End` | the first and last changed top-level line of that file (so `changed_nodes` says where) |
| `IsTest` | true when the file holds any test node |

- **Its callers** (spec §7) are synthetic `calls` edges `A → <file>::<module>` for every graph `calls` edge `A → B` where `B.File` is the file and `A.File` is **another** file. A caller in the same file is not a caller of the module — that would make the whole file its own neighbour. Tests among those callers are Round 1; non-tests are its neighbours, and their tests Round 2. It has no callees: the scanner attributes no call to top-level code.
- **Only a file the graph holds nodes for** gets a `<module>` node. A changed `README.md`, `go.mod` or YAML file has no node and contributes no changed node at all — otherwise every documentation edit would be "untested".
- **A test file's `<module>`** (`IsTest` true) puts every test node of that file in Round 1 and is never in `Untested`. A changed fixture or import in a test file changes those tests; a "write a test for `tests/test_x.py::<module>`" instruction would be noise.

## Commands: shared code

Three commands open the graph the same way, and Tasks 2, 4 and 5 may land in any order. Each of them creates **`cmd/rtdd/graphenv.go` with exactly this content** if it is not already on the base it rebased onto (identical text, so a concurrent add rebases cleanly):

```go
package main

import (
	"fmt"
	"os"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
)

// graphRoot is the repository the caller stands in. Its error is an environment error:
// the caller exits 3 on it (spec §8).
func graphRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		return "", fmt.Errorf("not inside a git work tree, or git is unavailable: %w", err)
	}
	return root, nil
}

// buildGraph loads .rtdd/config.yaml and builds the node graph, as `rtdd graph` does.
// code is the exit code for a non-nil err: 2 for a malformed config (the user's to fix),
// 3 for a graph that cannot be built or read.
func buildGraph(root string) (cfg graph.Config, res *graphbuild.Result, code int, err error) {
	if cfg, err = graph.LoadConfig(root); err != nil {
		return cfg, nil, 2, err
	}
	if res, err = graphbuild.Build(root, cfg, graphbuild.Options{}); err != nil {
		return cfg, nil, 3, err
	}
	return cfg, res, 0, nil
}
```

`rtdd graph` (`cmd/rtdd/graph.go`) keeps its own inline copy of these steps; Task 3 only moves its `graphObject` construction into `graphObjectOf` so `which --json` and `graph --json` share one `graph` object (PRD #409 decision 12). `plural` moves from `explain.go` to `cmd/rtdd/plural.go` in Task 4, unchanged, because `which`, `doctor` and the v0.2 renderers all call it. `ignoredWhy` stays in `graph.go`.

## `rtdd which`: text

Exactly this shape (Task 2's test pins it byte for byte):

```
graph: scanner, built at 1d64301, 0 stale files
changed nodes:
  src/calc.py::total  (lines 5-6)
  src/calc.py::unused  (lines 9-10)
Round 1 — run these first:
  tests/test_calc.py::test_total
Round 2 — then these:
  tests/test_calc.py::test_add
  tests/test_report.py::test_report
Round 3 — the full suite, once, at the end
untested:
  src/calc.py::unused
```

- The `graph:` line is source, `built_at_commit` and the stale-file count; when graphify was ignored it appends `(graphify ignored — <ignoredWhy>; run `graphify --update` to use it again)`.
- An empty round prints `  no linked test` — the constant `noLinkedTest`. Nothing on stdout says pass, green, OK or "nothing to run": an empty round means no test links to the change, and Round 3 still runs. An empty `changed nodes` or `untested` prints `  none`.
- Each test line is the runnable id (spec §3: `file::name`), so an agent can paste it.
- `warnings` (Task 2: one per deleted file) go to stderr as `rtdd which: warning: <text>` in text mode and into the document in `--json` mode.

## `rtdd which --json`: schema 3

The Go types (Task 3, `cmd/rtdd/which.go`). Encoding uses `json.Encoder` with `SetEscapeHTML(false)` — `<module>` stays readable — and two-space indent:

```go
type whichJSON struct {
	Schema       int           `json:"schema"`        // 3
	Command      string        `json:"command"`       // "which"
	Base         string        `json:"base"`          // --base as given; "HEAD" by default
	Graph        graphObject   `json:"graph"`         // shared with `rtdd graph --json`
	Changed      []changedFile `json:"changed"`
	ChangedNodes []nodeJSON    `json:"changed_nodes"`
	Rounds       []any         `json:"rounds"`        // testRound{1}, testRound{2}, fullSuiteRound{3}
	Untested     []string      `json:"untested"`      // node IDs
	Warnings     []string      `json:"warnings"`
}

type changedFile struct {
	Path  string      `json:"path"`
	Lines []lineRange `json:"lines"` // [] for a deleted file
}

type lineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type nodeJSON struct {
	ID    string `json:"id"`
	File  string `json:"file"`
	Name  string `json:"name"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type testJSON struct {
	ID   string `json:"id"`
	File string `json:"file"`
	Name string `json:"name"`
}

type testRound struct {
	Round int        `json:"round"`
	Tests []testJSON `json:"tests"`
	Files []string   `json:"files"` // the round's test files, de-duplicated, in test order
}

type fullSuiteRound struct {
	Round     int  `json:"round"`
	FullSuite bool `json:"full_suite"`
}
```

- Exactly spec §9's nine top-level keys, in §9's order. Every array is `[]`, never `null`. Round 3 is exactly `{"round": 3, "full_suite": true}`.
- `graph` is `graphObject` from `graph.go` — `source`, `built_at_commit`, `stale_files` (a count), `graphify_ignored` (only when ignored), `nodes`, `edges`, `tests` — a superset of §9's example, as PRD #409 decision 12 settled.
- `changed` lists every changed path `changedSet` returns (sorted by path), deleted ones included with `lines: []`; `changed_nodes` includes `<module>` nodes.
- **Golden fixture repository:** `cmd/rtdd/testdata/which-golden/repo/` (committed tree: Python `src/` + `tests/`, a Go package `money/`, and `src/legacy.py`), `cmd/rtdd/testdata/which-golden/edit/` (files copied over it after the commit) — the test also deletes `src/legacy.py` — and `cmd/rtdd/testdata/which-golden/want.json`, the whole document with HEAD's short sha written `<HEAD>`. Regenerating it is the explicit `RTDD_UPDATE_GOLDEN=1 go test ./cmd/rtdd/ -run '^TestWhichJSONGolden$'`, the repo's existing golden idiom, and the diff is read before it is committed. Files under `testdata/` are never test files (`test_exclude`), so the fixture does not leak into this repository's own rounds.
- Schema 2 is no longer emitted by anything once Task 3 lands. Consumers reject any other schema (spec §9); the bench's consumer moves in PRD #412.

## `rtdd explain` and `rtdd doctor`

**`rtdd explain <file[:line]|name>`** (Task 4) resolves its one argument in this order, and prints every node it resolves to, separated by a blank line:

1. a node **ID** (`src/calc.py::total`) — that node;
2. **`file:line`** (`src/calc.py:6`) — the innermost node owning that line (`rounds.Owner`); a line in no node resolves to nothing;
3. a **file** (`src/calc.py`) — every node of that file, in line order;
4. a **name** (`total`) — every node so named, in file/line order: all same-named definitions (§12, #417).

Paths are relative to the caller's directory (normalised with `paths.Normalize`, as v0.2's `explain` did). An argument that resolves to nothing exits **2** with `rtdd explain: no node matches "<arg>" (give a file, file:line, node id or name)` on stderr; no argument exits 2 with the usage line; outside a repository, 3. Per node:

```
src/calc.py::total  (func, lines 5-6)
  tests:
    tests/test_calc.py::test_total  (tests/test_calc.py:8, calls)
  callers:
    src/report.py::report  (src/report.py:4, calls)
  callees:
    src/calc.py::add  (src/calc.py:1, calls)
```

`tests` are the test nodes with an edge **to** the node, `callers` the non-test nodes with an edge to it, `callees` the nodes it has an edge to — over every relation, each line naming its relation, from `rounds.LinksOf`.

**`rtdd doctor`** (Task 5) takes no flag (`--limit` goes with the fan-out table) and prints exactly:

```
graph source:  scanner
built at:      1d64301
graphify:      not present (graphify-out/graph.json)
nodes:         16
edges:         8
test files:    2 matched by test_files, 1 with a test node
test nodes:    1
names defined 8 or more times (a call to one links to every definition):
  run  8
```

- `graphify:` is `not present (<graphify_path>)`, or `used, built at <sha> — <n> stale files, <k> of its <m> code files (max_stale_ratio 0.50)`, or `ignored — <ignoredWhy>; run `graphify --update` to use it again`.
- `test files` counts the listed, `scan_exclude`-filtered files (`gitctx.ListFiles` → `scan.Filter`) that `graph.IsTestFile` accepts, and how many of them hold a test node — the gap is the diagnostic.
- The over-link list is every **non-test** node name defined ≥ `overLinkDefinitions = 8` times, most-defined first, then by name; `  none` when empty. Nothing mentions an adapter, a coverage map or fan-out.

## Removed commands

`seed`, `run`, `verify`, `status` and `map` (with or without `compact`) dispatch to one function in `main.go` (Task 6):

```
rtdd seed: removed in v0.3.0 — rtdd no longer runs tests or keeps a coverage map.
Run `rtdd which` and run its rounds with the project's own test command.
```

on stderr, nothing on stdout, exit **2**, and nothing written. `verify` and `map compact` never shipped as top-level commands in this tree; they are answered the same way because the spec and older docs name them. `rtdd --help` lists none of them, and its exit-code block loses `1  a test failed`. `rtdd init`'s closing line stops pointing at `rtdd seed` (Task 6): `Next: edit code, then run `rtdd which` for the tests to run, in rounds.`

## The bench replay harness

`bench/replay` (the Axis 2 replay harness) drives the **real** `rtdd` binary on PATH: `rtddio.seed` runs `rtdd seed`, `rtddio.which` demands `which --json` schema 2, `rtddio.run` runs `rtdd run`. Moving it to schema 3 and to rounds is PRD #412's AC1, which is blocked by this PRD. Until then, once Task 6 (no `seed`) and later Task 3 (schema 3) are on the branch, the three bench tests that execute a real binary — at the time of writing `tests/test_rtddio.py::test_real_binary_honours_the_consumed_schema`, `tests/test_strategy_rtdd.py::test_verified_against_the_synthetic_repo` and `tests/test_replay.py::test_a_base_tree_rtdd_refuses_to_seed_is_skipped_not_fatal` — cannot pass against this branch's binary, and `scripts/ci-local.sh` runs them.

**Task 6 adds this file** (it lands before Task 3, and removing `seed` is what first breaks these tests) and marks those tests with it:

```python
# bench/tests/v02binary.py
"""The bench tests that drive a real `rtdd` need a v0.2 one.

The replay harness's rtdd strategy seeds a coverage map (`rtdd seed`) and reads
`rtdd which --json` schema 2. rtdd v0.3.0 removes both (PRD #410); PRD #412 moves the
strategy to `rtdd graph` and schema 3 and deletes this file. Until then a test that
executes the binary runs only when the `rtdd` on PATH still has `rtdd seed`.
"""
from __future__ import annotations

import shutil
import subprocess

import pytest


def rtdd_on_path_is_v02() -> bool:
    exe = shutil.which("rtdd")
    if exe is None:
        return False
    out = subprocess.run([exe, "--help"], capture_output=True, text=True, check=False)
    return "rtdd seed" in out.stdout


requires_v02_rtdd = pytest.mark.skipif(
    not rtdd_on_path_is_v02(),
    reason="needs a v0.2 rtdd on PATH (seed, which --json schema 2); PRD #412 moves the bench to schema 3",
)
```

Each of the three tests gains `@requires_v02_rtdd` (imported `from tests.v02binary import requires_v02_rtdd`, the way the suite already imports `tests.synthrepo`). Find any other with `PATH="$bin:$PATH" uv run pytest -q` in `bench/` against the branch binary; fence it the same way, and say which in the PR. Task 6 comments on #412 naming the fenced tests, so the PRD that rewrites the strategy deletes the fence with it. No bench test is deleted or weakened, and nothing else under `bench/` changes.

## Old tests: who deletes what

A test that pins **v0.2 behaviour of a command** is deleted by the task that changes that command, in the same commit as the change, and the commit message lists the deleted test names. A test that pins behaviour that survives is never deleted to make a task pass — if it fails, the code is wrong. Tests found with `grep -l '"which"\|"explain"\|…' cmd/rtdd/*_test.go` at the time of writing:

| Task | Deletes (functions in `cmd/rtdd/`) |
|---|---|
| 2 (`which` text) | `main_test.go`: `TestWhichSelectsAJustWrittenUntrackedTestFile`, `TestWhichRanksT0`, `TestWhichEmptySelectionIsExitZeroAndSaysSo`, `TestWhichEscalatesOnAFullEscalateFile`, `TestWhichEscalatesWhenTheSeedCommitIsUnreachable`, `TestWhichReportsDeletionsAndRespectsBase`, `TestWhichReportsARename`, `TestWhichReportsAMissingAdapter`, `TestAMalformedGlobInTheAdapterIsAConfigurationError`; `whichgolden_test.go` (whole file) and `testdata/which-python-golden.txt`; `polyglotcli_test.go`: `TestWhichNamesEachAdapterInAPolyglotRepository`; `detect_test.go`: `TestWhichStillWarnsWhenDetectionFindsNoAdapter`, `TestExplicitAdapterPathThatDoesNotExistIsAConfigError` — exactly the thirteen that fail once `which` is on the graph (checked) |
| 3 (`which --json`) | `main_test.go`: `TestWhichJSON`, `TestWhichJSONReportsChangedLineRanges`, `TestWhichJSONEmptySelectionEmitsArraysNotNull`, `TestWhichJSONCarriesTheMissingAdapterWarningInTheDocument`, `TestWhichJSONReportsACompleteSelectionWithNoWarnings`; `which_test.go`: `TestWhichJSONNeverClaimsAFreshUncoveredReport`, `TestWhichJSONReportsNothingExecuted`, `TestWhichJSONUnmappedFilesIsFileGranularAndNeverNull`, `TestWhichJSONUnmappedFilesIsAnEmptyArrayWhenEveryFileIsMapped`; `polyglotcli_test.go`: `TestWhichJSONCarriesOneSelectionBlockPerAdapter`, `TestWhichJSONOmitsSelectionsForASingleAdapter`; `detect_test.go`: `TestWhichDetectsTheAdapterOnAStockPostInitRepo`; `hostadapter_test.go`: `TestDetectionResolvesAHostAuthoredAdapter`, `TestWhichWarnsAboutAnUnloadableHostAdapterAndCarriesOn`; `mapversion_test.go`: `TestAV1MapIsTreatedAsUnseeded` — exactly the fifteen that fail once `--json` is schema 3, with Task 6 landed (checked) |
| 4 (`explain`) | `explain_test.go` (whole file: it pins `RenderExplain` over the map) |
| 5 (`doctor`) | `doctor_test.go`: the seven `TestRenderDoctor*` renderer tests, `TestDoctorCommandRanksFilesByFanOut`, `TestDoctorCommandHonoursLimit`, `TestDoctorOnAnUnseededRepoSaysToSeedAndStillCaveats` (it keeps `TestDoctorWithAPositionalArgumentIsAUsageError`, `TestDoctorWithAnUnknownFlagIsAUsageError`, `TestUsageDocumentsDoctor`, which still hold); `doctor_adapters_test.go` (whole file); `hostadapter_test.go`: `TestDoctorReportsAHostOverrideByName`, `TestDoctorNamesAnUnloadableHostAdapterAndStillExitsZero` |
| 6 (removals) | `seed_test.go`, `seedoutput_test.go`, `run_test.go`, `exit_test.go`, `acceptance_test.go`, `pipeline_*_test.go` (every `TestPipeline*`, with `pipeline_helpers_test.go`); `main_test.go`: `TestStatus*` (4); `detect_test.go`: `TestSeedExitsTwoWhenDetectionFindsNoAdapter`, `TestStatusDetectsTheAdapterWhenNoAdapterFileExists`, `TestWhichAndRunAgreeOnTheDetectedAdapter`; `stale_test.go`: `TestRunOnAnUnseededRepoLeavesACurrentMap`, `TestDeletedTestFileIsNeverSelectedAndItsRowIsPruned`; `mapversion_test.go`: `TestAV1MapIsTreatedAsUnseededByRun`; `changed_test.go`: `TestCmdRunTierIsStableAcrossConsecutiveRuns`, `TestCmdRunStillEscalatesOnAUserAuthoredJSON`; `polyglot_run_test.go`: the four `TestRun…`/`TestCmdRun…` tests; `polyglotcli_test.go`: `TestRunNamesEachAdapterInAPolyglotRepository`; `init_next_test.go`: `TestInitInACoverageRepoKeepsTheSeedNextStep` (replaced by a test of the new closing line) |
| 7 (packages) | every remaining test of a deleted package or file — see Task 7 — including `doctor_requires_test.go` and `init_requires_test.go` (the prerequisite report goes with `requires.go`) and `init_gate_test.go`'s refusal and adapter-record tests (`TestInitRefusesARepoWithNoAdapter`, `TestInitWritesNoSkillFileIntoATypeScriptRepo`, `TestInitDryRunAlsoRefusesARepoWithNoAdapter`, `TestInitForceInstallsTheNoAdapterCaveat`, `TestInitRefusalNamesTheMalformedHostAdapter`, `TestInitProceedsAndRecordsTheDetectedAdapter`) |

A test that exercises two retired commands goes with whichever task first makes it fail. **Helpers are not tests**: `which_test.go`'s `rawObject`, `decodeOutput`, `anyWarningContains`; `seed_test.go`'s `chdir`, `realRepo`, `readMapJSONL`, `containsStr`, `appendLine`; `run_test.go`'s `touchLogic`, `makeSuiteGreen`; `seedoutput_test.go`'s `fakeRepo`; `exit_test.go`'s `uncoveredReports` are called from other test files. When a task deletes the file that defines one still called elsewhere, it moves it verbatim to **`cmd/rtdd/v02_test.go`** (created by whichever task needs it first); Task 7 deletes `v02_test.go` with the last caller. Production helpers still called by v0.2 code go to **`cmd/rtdd/v02.go`** the same way (Task 2: `whichNotes`, `nonNilStrings`, and the old JSON path as `cmdWhichV02`).

## Deletion order

Every commit builds, vets and passes `scripts/ci-local.sh`. The order is by importer: nothing is deleted while something still imports it.

1. **Commands rewritten in place** (Tasks 2–5; 2 and 4 after Task 1, 3 after 2 and 6). `which`, `explain`, `doctor` stop calling `loadEnv`, `internal/mapstore`, `internal/selector`, `internal/uncovered`, `internal/adapter`, `internal/doctor`. Their v0.2 helpers that other v0.2 code still calls move to `v02.go` / `v02_test.go`. Until Task 3, `which --json` still answers through `cmdWhichV02` (schema 2) so the JSON tests, the `TestPipeline*` gate and the bench keep passing between the two tasks.
2. **Commands retired** (Task 6, any time after this plan, and before Task 3). `main.go` dispatches `seed`, `run`, `verify`, `status`, `map` to `removedCommand`. Deleted: `cmd/rtdd/seed.go`, `run.go`, `status.go`, `exit.go` (`ExitCodeFor` — the exit-1 path) and the tests in the table; the `TestPipeline*` gate leaves both CI gates (see "CI gates"). `polyglot.go`, `rows.go`, `meta.go`, `jsonout.go` and the packages still compile, now reachable only from the v0.2 `which --json` path (until Task 3) or from nothing.
3. **Packages deleted** (Task 7, after Tasks 2–6), in five commits:
   1. `cmd/rtdd`: delete `polyglot.go`, `rows.go`, `meta.go`, `escalate.go`, `distance.go`, `requires.go`, `uncoveredtext.go`, `jsonout.go`, `whichtext.go`, `v02.go`, `v02_test.go` and every test file of them; delete `env`, `loadEnv`, `adapterSource`, `noAdapterReason` from `main.go`; in `init.go` delete adapter detection, the no-adapter refusal (`writeNoAdapterRefusal`), the prerequisite report and `adapterRecords` (passing `nil` records to `install.Plan`), and move `findRepoRoot` (still used by `init` and `uninstall`) from `rows.go` into `init.go`. `changed.go`'s comment and `changed_test.go`'s example paths stop naming `meta.json`/`map.jsonl` (use `.rtdd/config.yaml`, `.rtdd/graph.json`). Nothing in `cmd/` imports a pipeline package any more.
   2. `internal/install`: delete `NoAdapterCaveat`, `WithNoAdapterCaveat` and `caveat_test.go`; delete the `.gitattributes` step from `install.Plan` (init writes no merge driver for a file nothing writes) and move `gitattributesLine` to `uninstall.go`, which still removes it from a v0.2 repository; delete the init tests that assert the line is written. `rtdd init` now succeeds in any git repository with no adapter output. Its config contents and v0.2 migration stay PRD #411's.
   3. `internal/contract`: **prune, never delete, `contract_test.go`** — it defines `repoRoot` and `readRepoFile` for the whole package. Drop its `internal/adapter` and `internal/doctor` imports and the tests that read deleted code or pin deleted behaviour: `TestInterfaceContractRecordsTheAdapterLoaders`, `TestInterfaceContractRecordsTheHostAdapterLoaders`, `TestInterfaceContractRecordsExpandTests`, `TestInterfaceContractSignaturesDoNotContradictTheImplementation`, `TestInterfaceContractDropsSrcFromExpand`, `TestInterfaceContractStructFieldsDoNotContradictTheImplementation`, `TestInterfaceContractRecordsDoctorCaveat`, `TestDoctorCaveatNamesTheOncePerProcessMechanisms`, `TestJSONSchemaV2KeySetMatchesTheInterfaceContract`, `TestDocumentedResolutionOrderMatchesTheSelector`, `TestGitAttributesDeclaresUnionMergeForMap` (and this repository's `.gitattributes` line with it), and the two shipped-adapter entries of `localCIChecks` (the `TestPipeline` ones went in Task 6). Delete `shipped_adapters_test.go`. In `nomock_test.go` drop `internal/pytestfixture` from `TestTestOnlyHelpersDoNotCompileInTesting`'s list. `plan_m6*_test.go` read only plan documents and stay — they also hold `stepBody` and `fencedBlocks`, which every plan contract test uses. `docs/plans/00-interfaces.md` is not edited.
   4. Delete the packages: `internal/adapter`, `internal/covfmt`, `internal/runner`, `internal/mapstore`, `internal/selector`, `internal/uncovered`, the **`adapters/`** directory (a Go package embedding the YAML), and the three their deletion leaves with **no importer**: **`internal/coverage`** (imported only by `cmd/rtdd`, `runner`, `uncovered`), **`internal/doctor`** (imported only by `cmd/rtdd/doctor.go` and `contract_test.go`; its fan-out ranking reads the map) and **`internal/pytestfixture`** (imported only by the pipeline tests). `internal/contract` is **not** deleted: it is a test-only package of repository contracts with many live tests. `go mod tidy` must leave `go.mod` and `go.sum` unchanged.
   5. CI and shipped paths (see "CI gates"), then `TestNoGoCodeReferencesTheCoveragePipeline` and `TestCoveragePipelinePackagesAreGone` — written first as Task 7's failing test, **committed last**, with the commit that turns them green, so no commit carries a red test.

Checked before this plan was written: steps 2 and 3, applied to a scratch copy of the tree with Tasks 1–5's code in place, build and vet, and the `cmd/rtdd` and `internal/contract` tests left failing were exactly the ones the table above and Task 7 delete (the copy had no `.git`, so tests that read git history were not part of that check).

## CI gates and shipped-path lists

| Where | Change | Task |
|---|---|---|
| `scripts/ci-local.sh` "one-pipeline adapter tests" | delete the step (`go test -list '^TestPipeline' ./cmd/rtdd/` guard and its `-run`) | 6 |
| `.github/workflows/ci.yml` "pipeline tests" | delete the step, and the toolchain provisioning only it used (`setup-python` + `pip install pytest pytest-cov`, `rust-toolchain` + `cargo-llvm-cov`, `setup-php`, `setup-ruby`) with their comment | 6 |
| `internal/contract/contract_test.go` `localCIChecks` | drop `"go test -list '^TestPipeline' ./cmd/rtdd/"` and `"-run '^TestPipeline' ./cmd/rtdd/"` | 6 |
| `internal/contract/ci_workflow_test.go` `TestCIPipelineStepRunsUnderPipefail` | delete (its step is gone) | 6 |
| `scripts/ci-local.sh` "shipped-adapter completeness gate (#310)" | delete the step (`-list '^TestEveryShippedAdapterIsFullySpecified$'` guard and its `-run`) | 7 |
| `.github/workflows/ci.yml` "shipped-adapter completeness gate" | delete the step | 7 |
| `internal/contract/contract_test.go` `localCIChecks` | drop the two `TestEveryShippedAdapterIsFullySpecified` entries and their comment | 7 |
| `internal/installtest/release_snapshot_test.go` `trackedDirs` | drop `"adapters"` | 7 |
| `.goreleaser.yaml`, `internal/installtest` archive contents | nothing names `adapters/` today (the YAML was embedded); assert it with `grep -rn adapters .goreleaser.yaml internal/installtest` and leave both alone | 7 |

No new `-list` guard is needed: every new test of this plan runs inside `go test ./...`, as PRD #409's perf tests do. Every remaining `-list` guard in both gates must name a test that exists — `TestBothGatesRunTheReleaseSnapshotAndItsTests` and `TestLocalCIEntrypointIsExecutableAndRunsTheSameChecks` keep the two gates in step.

## Decisions this plan settles

The spec leaves these open; each is settled here so nine lanes do not settle it nine ways, and each is pinned by a test in the task named.

1. **`Rounds` lives in `internal/rounds`** with the signature `func Rounds(g graph.Graph, changed map[string][]LineRange) Result` and `type Result struct { ChangedNodes, Round1, Round2, Untested []graph.Node }`; non-nil slices, ordered by file, line, ID. Task 1.
2. **Changed ranges come from `gitctx.ChangedSet`**'s `Lines`, through `changedSet` (drops `.rtdd/`), for `--base <ref>` (default `HEAD`) and the working tree in one call; untracked files are whole-file ranges, deleted files contribute none and one warning. Tasks 2, 3.
3. **`gitctx.ErrUnknownBase`** separates exit 2 (bad ref) from exit 3 (git failing). Task 2.
4. **The `<module>` node**: ID `<file>::<module>`, kind `rounds.KindModule`, lines = the changed top-level lines, callers = other files' `calls` into the file; only for files with nodes; a test file's `<module>` selects its tests and is never untested. Task 1.
5. **"Untested"** is per changed node: no test links to it or to one of its own neighbours. Task 1.
6. **Text output** is the block in "`rtdd which`: text"; empty rounds say `no linked test`. Task 2.
7. **Schema 3** is `whichJSON` above, encoded unescaped with two-space indent; `graph` is `graphObject`; `files` de-duplicated in test order. Task 3.
8. **Golden**: `cmd/rtdd/testdata/which-golden/{repo,edit,want.json}`, HEAD's sha normalised to `<HEAD>`, regenerated only by `RTDD_UPDATE_GOLDEN=1`. Task 3.
9. **`explain`** resolves ID → `file:line` → file → name; no match exits 2; it lists tests, callers and callees over every relation. Task 4.
10. **`doctor`** has no flags; over-linked = non-test names with ≥ 8 definitions. Task 5.
11. **Removed commands** exit 2 naming v0.3.0 and `rtdd which`; `--help` lists none; exit code 1 is gone from the usage text. Task 6.
12. **The packages deleted** are the six of the spec, `adapters/`, and `internal/coverage`, `internal/doctor`, `internal/pytestfixture`; `internal/contract` stays. Task 7.
13. **The no-reference guard** scans every `.go` file (tests included): real imports of a deleted package are an error anywhere; the strings `.rtdd/map.jsonl`, `.rtdd/meta.json`, `.rtdd/adapters/` are an error outside an allowlist of four files owned by PRD #411's text and v0.2-state removal (plus the guard itself) — and an allowlisted file that stops naming them is an error too, so the list only shrinks. Task 7.
14. **The bench fence** (`bench/tests/v02binary.py`) is added by Task 6; PRD #412 removes it. Task 6.

## Acceptance-criterion map

| PRD #410 AC | What | Task(s) | Issue(s) |
|---|---|---|---|
| AC1 | `Rounds(graph, changedRanges)` exactly as §7 | 1 | #446 |
| AC2 | `rtdd which [--base] [--json]` prints changed nodes, Rounds 1–3, untested, graph source; runs nothing; "no linked test" | 2 | #447 |
| AC3 | `--json` schema 3, exactly §9's keys, golden-pinned | 3 | #448 |
| AC4 | `explain` (tests, callers, callees); `doctor` (source, staleness, test files, names ≥ 8) | 4, 5 | #449, #450 |
| AC5 | commands, packages, `adapters/` and v0.2 state removed; removed command exits 2 naming v0.3.0 | 6 (commands), 7 (packages) | #451, #452 |
| AC6 | exit codes 0 / 2 / 3; none reflects a test result | 2, 3, 6 | #447, #448, #451 |
| AC7 | end to end on a Python + Go temp repository | 8 | #453 |
| AC8 | `rtdd which` on this repository, warm cache, < 1 s | 9 | #454 |
| AC9 | `scripts/ci-local.sh` exits 0; `-list` guards updated | every task; guards in 6 and 7 | #451, #452 |

Order: Task 1 first. Tasks 5 and 6 need only this plan. Tasks 2 and 4 after Task 1. **Task 3 after Tasks 2 and 6** — the `TestPipeline*` gate Task 6 retires reads `rtdd which --json` as schema 2, so schema 3 cannot land while it is still a gate (#448 is blocked by #451 for this reason). Task 9 after Task 2; Task 8 after Task 3. Task 7 after Tasks 2–6. Tasks 8 and 9 may land before or after Task 7.

## File Structure

| Path | Responsibility | Task |
|---|---|---|
| `internal/rounds/rounds.go` | `LineRange`, `ModuleName`, `KindModule`, `Result`, `Rounds`, `Owner`, the index | 1 |
| `internal/rounds/links.go` | `Link`, `Links`, `LinksOf` | 4 |
| `internal/gitctx/changedset.go` | `ErrUnknownBase` | 2 |
| `cmd/rtdd/graphenv.go` | `graphRoot`, `buildGraph` | first of 2, 4, 5 |
| `cmd/rtdd/which.go` | `cmdWhich`, `changedRanges`, `renderWhich`, `graphLine`; schema 3 types and `buildWhichJSON` | 2, 3 |
| `cmd/rtdd/graph.go` | `graphObjectOf` extracted | 3 |
| `cmd/rtdd/explain.go` | `cmdExplain`, `explainTargets`, `renderExplain` | 4 |
| `cmd/rtdd/plural.go` | `plural`, moved from `explain.go` | 4 |
| `cmd/rtdd/doctor.go` | `cmdDoctor`, `renderDoctor`, `graphifyState`, `overLinked` | 5 |
| `cmd/rtdd/main.go` | usage text; `removedCommand` | 2, 4, 5, 6, 7 |
| `cmd/rtdd/v02.go`, `cmd/rtdd/v02_test.go` | v0.2 helpers still called by v0.2 code, until Task 7 | 2–6 create, 7 deletes |
| `cmd/rtdd/which_rounds_test.go` | `whichRepo`, `editTotalAndUnused`, `tripwirePATH`; text-mode tests | 2 |
| `cmd/rtdd/which_json_test.go` | `copyTree`, `goldenWhichRepo`; schema-3 tests | 3 |
| `cmd/rtdd/testdata/which-golden/` | `repo/`, `edit/`, `want.json` | 3 |
| `cmd/rtdd/explain_graph_test.go`, `doctor_graph_test.go`, `removed_test.go` | command tests | 4, 5, 6 |
| `cmd/rtdd/e2e_rounds_test.go` | Python + Go end to end | 8 |
| `cmd/rtdd/which_perf_test.go` | warm `which` < 1 s; `BenchmarkRoundsOnThisRepository` | 9 |
| `internal/contract/pipeline_removed_test.go` | `TestCoveragePipelinePackagesAreGone`, `TestNoGoCodeReferencesTheCoveragePipeline` | 7 |
| `bench/tests/v02binary.py` | the v0.2-binary fence | 6 |
| `internal/contract/plan_n2_test.go` | this document's own contract (issue #445) | — |

---

### Task 1: `Rounds` — changed nodes, Round 1, Round 2 and untested, exactly as spec §7

**Issue:** #446

**Discharges:** PRD #410 AC1. Spec §7, §12 (both directions).

**Files:**
- Create: `internal/rounds/rounds.go`
- Test: `internal/rounds/rounds_test.go`

**Interfaces:**
- Consumes: `graph.Graph`, `graph.Node`, `graph.Edge`, `graph.Kind`, `graph.Relations`, `graph.RelCalls`, `graph.KindFunc`, `graph.KindClass` (PRD #409).
- Produces: `type LineRange struct{ Start, End int }`; `const ModuleName = "<module>"`; `const KindModule graph.Kind = "module"`; `type Result struct { ChangedNodes, Round1, Round2, Untested []graph.Node }`; `func Rounds(g graph.Graph, changed map[string][]LineRange) Result`; `func Owner(g graph.Graph, file string, line int) (graph.Node, bool)`; unexported `newIndex`, `byFileThenLine` (Task 4's `LinksOf` reuses both).

- [ ] **Step 1: Write the failing tests** — `internal/rounds/rounds_test.go`, one test per rule of spec §7 on a graph built by hand:

```go
package rounds

import (
	"go/parser"
	"go/token"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// n is a non-test node. path is the node's names below the file, outermost first
// ("Calc::plus"); the node's Name is its last segment, as internal/graph IDs it.
func n(file, path string, start, end int) graph.Node {
	name := path
	if i := strings.LastIndex(path, "::"); i >= 0 {
		name = path[i+2:]
	}
	return graph.Node{ID: file + "::" + path, File: file, Name: name, Kind: graph.KindFunc, Start: start, End: end}
}

// tn is a test node.
func tn(file, path string, start, end int) graph.Node {
	x := n(file, path, start, end)
	x.IsTest = true
	return x
}

func calls(from, to graph.Node) graph.Edge {
	return graph.Edge{From: from.ID, To: to.ID, Relation: graph.RelCalls}
}

func ids(ns []graph.Node) []string {
	out := []string{}
	for _, x := range ns {
		out = append(out, x.ID)
	}
	return out
}

type want struct{ changed, round1, round2, untested []string }

func check(t *testing.T, got Result, w want) {
	t.Helper()
	for _, c := range []struct {
		set       string
		got, want []string
	}{
		{"changed_nodes", ids(got.ChangedNodes), w.changed},
		{"round1", ids(got.Round1), w.round1},
		{"round2", ids(got.Round2), w.round2},
		{"untested", ids(got.Untested), w.untested},
	} {
		if c.want == nil {
			c.want = []string{}
		}
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s = %q, want %q", c.set, c.got, c.want)
		}
	}
}

func lines(path string, rs ...LineRange) map[string][]LineRange {
	return map[string][]LineRange{path: rs}
}

// Spec §7, changed_nodes: the innermost node containing each changed line.
func TestInnermostNodeOwnsAChangedLine(t *testing.T) {
	k := n("a.py", "K", 1, 10)
	k.Kind = graph.KindClass
	m := n("a.py", "K::m", 2, 6)
	step := n("a.py", "K::m::step", 3, 4)
	g := graph.Graph{Nodes: []graph.Node{k, m, step}}
	got := Rounds(g, lines("a.py", LineRange{3, 3}, LineRange{6, 6}, LineRange{9, 9}))
	check(t, got, want{
		changed:  []string{"a.py::K", "a.py::K::m", "a.py::K::m::step"},
		untested: []string{"a.py::K", "a.py::K::m", "a.py::K::m::step"},
	})
}

// Owner answers the same question for one line, for `rtdd explain file:line`.
func TestOwnerIsTheInnermostNodeOfALine(t *testing.T) {
	g := graph.Graph{Nodes: []graph.Node{n("a.py", "K", 1, 10), n("a.py", "K::m", 2, 6), n("a.py", "K::m::step", 3, 4)}}
	for line, want := range map[int]string{3: "a.py::K::m::step", 6: "a.py::K::m", 9: "a.py::K"} {
		if got, ok := Owner(g, "a.py", line); !ok || got.ID != want {
			t.Errorf("Owner(a.py:%d) = %q, %v; want %q", line, got.ID, ok, want)
		}
	}
	if _, ok := Owner(g, "a.py", 11); ok {
		t.Error("a line in no node has an owner")
	}
}

// Spec §7: a changed line in no node maps to file::<module>, whose callers are the nodes
// of OTHER files that call any node in that file.
func TestTopLevelLineIsTheModuleNodeAndItsCrossFileCallers(t *testing.T) {
	f := n("src/lib.py", "f", 3, 4)
	g2 := n("src/lib.py", "g", 6, 7)
	run := n("src/app.py", "run", 1, 2)
	testF := tn("tests/test_lib.py", "test_f", 1, 2)
	testG := tn("tests/test_lib.py", "test_g", 4, 5)
	testRun := tn("tests/test_app.py", "test_run", 1, 2)
	g := graph.Graph{
		Nodes: []graph.Node{f, g2, run, testF, testG, testRun},
		Edges: []graph.Edge{calls(g2, f), calls(run, f), calls(testF, f), calls(testG, g2), calls(testRun, run)},
	}
	got := Rounds(g, lines("src/lib.py", LineRange{1, 1}))
	check(t, got, want{
		changed: []string{"src/lib.py::<module>"},
		round1:  []string{"tests/test_lib.py::test_f", "tests/test_lib.py::test_g"},
		round2:  []string{"tests/test_app.py::test_run"},
	})
	mod := got.ChangedNodes[0]
	if mod.Name != ModuleName || mod.Kind != KindModule || mod.File != "src/lib.py" || mod.Start != 1 || mod.End != 1 {
		t.Errorf("module node = %+v, want name %q kind %q file src/lib.py lines 1-1", mod, ModuleName, KindModule)
	}
}

// A changed line in a file the graph holds no node for (README.md, a config file) is no
// changed node at all: there is nothing in it a test can call.
func TestALineInAFileWithNoNodeIsNoChangedNode(t *testing.T) {
	g := graph.Graph{Nodes: []graph.Node{n("src/a.py", "f", 1, 2)}}
	check(t, Rounds(g, lines("README.md", LineRange{1, 5})), want{})
}

// Spec §7, round1: changed test nodes.
func TestAChangedTestNodeIsInRound1(t *testing.T) {
	x := tn("tests/test_a.py", "test_x", 1, 3)
	check(t, Rounds(graph.Graph{Nodes: []graph.Node{x}}, lines("tests/test_a.py", LineRange{2, 2})), want{
		changed: []string{"tests/test_a.py::test_x"},
		round1:  []string{"tests/test_a.py::test_x"},
	})
}

// A top-level line of a test file (an import, a fixture) changes every test in it: the
// module node of a file holding test nodes selects them, and is never untested.
func TestATopLevelLineOfATestFileSelectsItsTests(t *testing.T) {
	a := tn("tests/test_m.py", "test_a", 3, 4)
	b := tn("tests/test_m.py", "test_b", 6, 7)
	check(t, Rounds(graph.Graph{Nodes: []graph.Node{a, b}}, lines("tests/test_m.py", LineRange{1, 1})), want{
		changed: []string{"tests/test_m.py::<module>"},
		round1:  []string{"tests/test_m.py::test_a", "tests/test_m.py::test_b"},
	})
}

// Spec §7: round1 by an edge to a changed node; round2 by an edge to a neighbour — a
// callee AND a caller (spec §12, #414: both directions).
func TestRound1IsTestsOfTheChangedNodeAndRound2TestsOfCallersAndCallees(t *testing.T) {
	a := n("src/m.py", "a", 1, 2)
	b := n("src/m.py", "b", 4, 5)
	tt := n("src/m.py", "t", 7, 9)
	c := n("src/m.py", "c", 11, 12)
	testA := tn("tests/test_m.py", "test_a", 1, 2)
	testB := tn("tests/test_m.py", "test_b", 4, 5)
	testC := tn("tests/test_m.py", "test_c", 7, 8)
	testT := tn("tests/test_m.py", "test_t", 10, 11)
	g := graph.Graph{
		Nodes: []graph.Node{a, b, tt, c, testA, testB, testC, testT},
		Edges: []graph.Edge{calls(tt, b), calls(c, tt), calls(testA, a), calls(testB, b), calls(testC, c), calls(testT, tt)},
	}
	check(t, Rounds(g, lines("src/m.py", LineRange{8, 8})), want{
		changed: []string{"src/m.py::t"},
		round1:  []string{"tests/test_m.py::test_t"},
		round2:  []string{"tests/test_m.py::test_b", "tests/test_m.py::test_c"},
	})
}

// Spec §7: neighbours are reached over calls|method|inherits|implements|references, in
// either direction.
func TestEveryRelationIsTraversedInBothDirections(t *testing.T) {
	for _, rel := range graph.Relations {
		for _, outward := range []bool{true, false} {
			t.Run(string(rel)+"/outward="+strconv.FormatBool(outward), func(t *testing.T) {
				k := n("x.py", "k", 1, 2)
				nb := n("x.py", "nb", 4, 5)
				testNb := tn("tests/test_x.py", "test_nb", 1, 2)
				e := graph.Edge{From: k.ID, To: nb.ID, Relation: rel}
				if !outward {
					e = graph.Edge{From: nb.ID, To: k.ID, Relation: rel}
				}
				g := graph.Graph{Nodes: []graph.Node{k, nb, testNb}, Edges: []graph.Edge{e, calls(testNb, nb)}}
				check(t, Rounds(g, lines("x.py", LineRange{1, 1})), want{
					changed: []string{"x.py::k"},
					round2:  []string{"tests/test_x.py::test_nb"},
				})
			})
		}
	}
}

// Spec §7: test nodes are never neighbours, so a test that only calls another test is
// in no round.
func TestATestNodeIsNeverANeighbour(t *testing.T) {
	k := n("src/k.py", "k", 1, 2)
	testK := tn("tests/test_k.py", "test_k", 1, 2)
	testOther := tn("tests/test_k.py", "test_other", 4, 5)
	g := graph.Graph{Nodes: []graph.Node{k, testK, testOther}, Edges: []graph.Edge{calls(testK, k), calls(k, testK), calls(testOther, testK)}}
	check(t, Rounds(g, lines("src/k.py", LineRange{1, 1})), want{
		changed: []string{"src/k.py::k"},
		round1:  []string{"tests/test_k.py::test_k"},
	})
}

// Spec §7: round2 is minus round1.
func TestRound2ExcludesRound1(t *testing.T) {
	tt := n("src/m.py", "t", 1, 2)
	b := n("src/m.py", "b", 4, 5)
	both := tn("tests/test_m.py", "test_both", 1, 3)
	g := graph.Graph{Nodes: []graph.Node{tt, b, both}, Edges: []graph.Edge{calls(tt, b), calls(both, tt), calls(both, b)}}
	check(t, Rounds(g, lines("src/m.py", LineRange{1, 1})), want{
		changed: []string{"src/m.py::t"},
		round1:  []string{"tests/test_m.py::test_both"},
	})
}

// Spec §7: untested is the changed non-test nodes with no test in round1 or round2 — a
// node is tested when a test links to it or to one of its neighbours.
func TestUntestedIsTheChangedNodesNoRoundReaches(t *testing.T) {
	lone := n("src/u.py", "lone", 1, 2)
	direct := n("src/u.py", "direct", 4, 5)
	viaNeighbour := n("src/u.py", "via_neighbour", 7, 8)
	b := n("src/u.py", "b", 10, 11)
	testDirect := tn("tests/test_u.py", "test_direct", 1, 2)
	testB := tn("tests/test_u.py", "test_b", 4, 5)
	g := graph.Graph{
		Nodes: []graph.Node{lone, direct, viaNeighbour, b, testDirect, testB},
		Edges: []graph.Edge{calls(testDirect, direct), calls(viaNeighbour, b), calls(testB, b)},
	}
	check(t, Rounds(g, lines("src/u.py", LineRange{1, 2}, LineRange{4, 5}, LineRange{7, 8})), want{
		changed:  []string{"src/u.py::lone", "src/u.py::direct", "src/u.py::via_neighbour"},
		round1:   []string{"tests/test_u.py::test_direct"},
		round2:   []string{"tests/test_u.py::test_b"},
		untested: []string{"src/u.py::lone"},
	})
}

// Spec §7: within a round tests are ordered by file then line, and the answer does not
// depend on the order the graph lists nodes and edges in.
func TestRoundsAreOrderedByFileThenLineAndDeterministic(t *testing.T) {
	tt := n("src/m.py", "t", 1, 2)
	var nodes []graph.Node
	var edges []graph.Edge
	nodes = append(nodes, tt)
	for _, f := range []string{"tests/z_test.py", "tests/a_test.py", "tests/m_test.py"} {
		for _, line := range []int{30, 10, 20} {
			x := tn(f, "test_"+strconv.Itoa(line), line, line+1)
			nodes = append(nodes, x)
			edges = append(edges, calls(x, tt))
		}
	}
	var wantR1 []string
	for _, f := range []string{"tests/a_test.py", "tests/m_test.py", "tests/z_test.py"} {
		for _, line := range []int{10, 20, 30} {
			wantR1 = append(wantR1, f+"::test_"+strconv.Itoa(line))
		}
	}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		rng.Shuffle(len(nodes), func(a, b int) { nodes[a], nodes[b] = nodes[b], nodes[a] })
		rng.Shuffle(len(edges), func(a, b int) { edges[a], edges[b] = edges[b], edges[a] })
		g := graph.Graph{Nodes: append([]graph.Node(nil), nodes...), Edges: append([]graph.Edge(nil), edges...)}
		check(t, Rounds(g, lines("src/m.py", LineRange{1, 1})), want{changed: []string{"src/m.py::t"}, round1: wantR1})
	}
}

// No changed range is an empty answer, not an error, and every slice is non-nil so
// `--json` prints [] rather than null.
func TestNoChangedRangeIsEmptyRounds(t *testing.T) {
	g := graph.Graph{Nodes: []graph.Node{n("a.py", "f", 1, 2)}}
	for _, changed := range []map[string][]LineRange{nil, {}, {"a.py": nil}} {
		got := Rounds(g, changed)
		if got.ChangedNodes == nil || got.Round1 == nil || got.Round2 == nil || got.Untested == nil {
			t.Errorf("Rounds(%v) has a nil slice: %+v", changed, got)
		}
		check(t, got, want{})
	}
}

// Rounds is pure (issue #446): its package imports internal/graph and the standard
// library's pure packages, nothing that can touch a file, git or a process.
func TestRoundsPackageImportsNoIO(t *testing.T) {
	allowed := map[string]bool{"cmp": true, "slices": true, "strings": true, "github.com/VocanicZ/rtdd/internal/graph": true}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), e.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if !allowed[p] {
				t.Errorf("%s imports %q; internal/rounds may import only %v", e.Name(), p, allowed)
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/rounds/
```

Expected: FAIL — the package has no non-test Go file: `undefined: Result`, `undefined: LineRange`, `undefined: Rounds`, `undefined: Owner`, `undefined: ModuleName`, `undefined: KindModule`.

- [ ] **Step 3: Implement** — `internal/rounds/rounds.go`:

```go
// Package rounds turns a node graph and the changed lines of a diff into the rounds an
// agent runs (spec §7). It is pure: it reads no file, runs no git and starts no process —
// the caller hands it the graph and the ranges, so every rule is testable on a graph
// built by hand.
package rounds

import (
	"cmp"
	"slices"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// LineRange is a 1-based, inclusive range of changed lines on the new side of a diff.
type LineRange struct{ Start, End int }

// ModuleName names the synthetic node a changed top-level line maps to (spec §7); its ID
// is "<file>::<module>".
const ModuleName = "<module>"

// KindModule is the synthetic node's kind. No graph source ever produces it.
const KindModule graph.Kind = "module"

// Result is spec §7's four sets. Every slice is non-nil and ordered by File, then Start,
// then ID.
type Result struct {
	ChangedNodes []graph.Node // the innermost node of each changed line, or the file's <module>
	Round1       []graph.Node // changed tests, and tests with an edge to a changed node
	Round2       []graph.Node // tests with an edge to a neighbour, minus Round1
	Untested     []graph.Node // changed non-test nodes no test in Round1 or Round2 reaches
}

// traversed is the relation set neighbours are reached over (spec §7) — all of §3's.
var traversed = func() map[graph.Relation]bool {
	m := map[graph.Relation]bool{}
	for _, r := range graph.Relations {
		m[r] = true
	}
	return m
}()

// index is the graph by ID, by file, and as adjacency lists in both directions.
type index struct {
	byID   map[string]graph.Node
	byFile map[string][]graph.Node // ordered by Start, then End descending (outer first)
	out    map[string][]graph.Edge
	in     map[string][]graph.Edge
}

func newIndex(g graph.Graph) *index {
	ix := &index{byID: map[string]graph.Node{}, byFile: map[string][]graph.Node{},
		out: map[string][]graph.Edge{}, in: map[string][]graph.Edge{}}
	for _, n := range g.Nodes {
		ix.byID[n.ID] = n
		ix.byFile[n.File] = append(ix.byFile[n.File], n)
	}
	for _, ns := range ix.byFile {
		slices.SortFunc(ns, func(a, b graph.Node) int {
			if c := cmp.Compare(a.Start, b.Start); c != 0 {
				return c
			}
			if c := cmp.Compare(b.End, a.End); c != 0 {
				return c
			}
			return strings.Compare(a.ID, b.ID)
		})
	}
	for _, e := range g.Edges {
		if !traversed[e.Relation] {
			continue
		}
		ix.out[e.From] = append(ix.out[e.From], e)
		ix.in[e.To] = append(ix.in[e.To], e)
	}
	return ix
}

// owners maps each line of file up to maxLine to the innermost node containing it.
// Nodes are painted outer first, so a nested node overwrites its parent's lines.
func (ix *index) owners(file string, maxLine int) []string {
	own := make([]string, maxLine+1)
	for _, n := range ix.byFile[file] {
		for l := max(n.Start, 1); l <= min(n.End, maxLine); l++ {
			own[l] = n.ID
		}
	}
	return own
}

// Owner returns the innermost node of g containing file:line (spec §7's line-to-node
// rule), and false when the line is in no node.
func Owner(g graph.Graph, file string, line int) (graph.Node, bool) {
	var best graph.Node
	found := false
	for _, n := range g.Nodes {
		if n.File != file || line < n.Start || line > n.End {
			continue
		}
		if !found || n.Start > best.Start || (n.Start == best.Start && n.End < best.End) {
			best, found = n, true
		}
	}
	return best, found
}

// Rounds computes spec §7 for the changed line ranges of each repo-relative file.
func Rounds(g graph.Graph, changed map[string][]LineRange) Result {
	ix := newIndex(g)
	changedSet := map[string]graph.Node{}
	moduleIn := map[string][]graph.Edge{} // a <module> node's callers, as edges into it
	round1 := map[string]graph.Node{}

	for file, ranges := range changed {
		nodes := ix.byFile[file]
		if len(nodes) == 0 || len(ranges) == 0 {
			continue // a file with no node holds nothing a test can call
		}
		maxLine := 0
		for _, r := range ranges {
			maxLine = max(maxLine, r.End)
		}
		own := ix.owners(file, maxLine)
		var mod *graph.Node
		for _, r := range ranges {
			for l := max(r.Start, 1); l <= r.End; l++ {
				if id := own[l]; id != "" {
					changedSet[id] = ix.byID[id]
					continue
				}
				if mod == nil {
					mod = &graph.Node{ID: file + "::" + ModuleName, File: file, Name: ModuleName, Kind: KindModule, Start: l, End: l}
				}
				mod.Start, mod.End = min(mod.Start, l), max(mod.End, l)
			}
		}
		if mod == nil {
			continue
		}
		for _, n := range nodes {
			if n.IsTest {
				mod.IsTest = true // a test file's top level: its tests are the changed tests
				round1[n.ID] = n
			}
			for _, e := range ix.in[n.ID] {
				if e.Relation == graph.RelCalls && ix.byID[e.From].File != file {
					moduleIn[mod.ID] = append(moduleIn[mod.ID], graph.Edge{From: e.From, To: mod.ID, Relation: graph.RelCalls})
				}
			}
		}
		changedSet[mod.ID] = *mod
	}

	in := func(id string) []graph.Edge {
		if es, ok := moduleIn[id]; ok {
			return es
		}
		return ix.in[id]
	}
	testsOf := func(id string) []graph.Node {
		var out []graph.Node
		for _, e := range in(id) {
			if t, ok := ix.byID[e.From]; ok && t.IsTest {
				out = append(out, t)
			}
		}
		return out
	}

	neighbours := map[string][]string{} // changed ID -> its non-test neighbours
	for id, c := range changedSet {
		if c.IsTest && c.Kind != KindModule {
			round1[id] = c
		}
		for _, t := range testsOf(id) {
			round1[t.ID] = t
		}
		for _, e := range ix.out[id] {
			if n, ok := ix.byID[e.To]; ok && !n.IsTest {
				neighbours[id] = append(neighbours[id], n.ID)
			}
		}
		for _, e := range in(id) {
			if n, ok := ix.byID[e.From]; ok && !n.IsTest {
				neighbours[id] = append(neighbours[id], n.ID)
			}
		}
	}

	round2 := map[string]graph.Node{}
	for _, ns := range neighbours {
		for _, nb := range ns {
			for _, t := range testsOf(nb) {
				if _, in1 := round1[t.ID]; !in1 {
					round2[t.ID] = t
				}
			}
		}
	}

	untested := map[string]graph.Node{}
	for id, c := range changedSet {
		if c.IsTest || len(testsOf(id)) > 0 {
			continue
		}
		reached := false
		for _, nb := range neighbours[id] {
			if len(testsOf(nb)) > 0 {
				reached = true
				break
			}
		}
		if !reached {
			untested[id] = c
		}
	}

	return Result{
		ChangedNodes: ordered(changedSet),
		Round1:       ordered(round1),
		Round2:       ordered(round2),
		Untested:     ordered(untested),
	}
}

// ordered is a set's nodes by File, then Start, then ID — never nil.
func ordered(set map[string]graph.Node) []graph.Node {
	out := make([]graph.Node, 0, len(set))
	for _, n := range set {
		out = append(out, n)
	}
	slices.SortFunc(out, byFileThenLine)
	return out
}

func byFileThenLine(a, b graph.Node) int {
	if c := strings.Compare(a.File, b.File); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Start, b.Start); c != 0 {
		return c
	}
	return strings.Compare(a.ID, b.ID)
}
```

- [ ] **Step 4: Run to verify they pass**

```bash
go test ./internal/rounds/ -count=1 -v && go vet ./internal/rounds/ && gofmt -l internal/rounds
```

Expected: every test PASS (`TestEveryRelationIsTraversedInBothDirections` has ten subtests); no vet or gofmt output.

- [ ] **Step 5: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0. Nothing outside `internal/rounds` changed, so nothing else can have moved.

- [ ] **Step 6: Commit**

```bash
git add internal/rounds
git commit -m "feat(rounds): Rounds(graph, changedRanges) — changed nodes, Round 1, Round 2 and untested exactly as spec §7 (closes #446)"
```

---

### Task 2: `rtdd which` prints changed nodes, Rounds 1–3, untested and graph source, running nothing

**Issue:** #447

**Discharges:** PRD #410 AC2, AC6 (text mode). Spec §7, §8.

**Files:**
- Modify: `internal/gitctx/changedset.go` (`ErrUnknownBase`)
- Create: `cmd/rtdd/graphenv.go` (if absent — exact content in "Commands: shared code")
- Rename: `cmd/rtdd/which.go` → `cmd/rtdd/v02.go`, `cmdWhich` → `cmdWhichV02`
- Create: `cmd/rtdd/which.go`
- Modify: `cmd/rtdd/main.go` (usage line `rtdd which  [--base <ref>] [--json]`)
- Delete: the Task 2 row of "Old tests: who deletes what"
- Test: `cmd/rtdd/which_rounds_test.go`

**Interfaces:**
- Consumes: `rounds.Rounds`, `rounds.Result`, `rounds.LineRange` (Task 1); `graphRoot`, `buildGraph` (`graphenv.go`); `changedSet` (`changed.go`); `ignoredWhy` (`graph.go`); `plural` (`explain.go`, or `plural.go` after Task 4).
- Produces: `var gitctx.ErrUnknownBase`; `const noLinkedTest`, `const roundThree`; `func changedRanges(changes []gitctx.Change) (map[string][]rounds.LineRange, []string)`; `func renderWhich(res *graphbuild.Result, cfg graph.Config, r rounds.Result) string`; `func graphLine(res *graphbuild.Result, cfg graph.Config) string`; test helpers `whichRepo`, `editTotalAndUnused`, `tripwirePATH` (Tasks 3, 6, 8, 9 use them).

- [ ] **Step 1: Write the failing tests** — `cmd/rtdd/which_rounds_test.go`:

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// whichRepo is a committed Python project: total calls add, report calls total, and each
// of add, total and report has a test; unused has none.
func whichRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n\n\ndef unused():\n    return 0\n")
	gittest.Write(t, dir, "src/report.py", "from src.calc import total\n\n\ndef report(xs):\n    return str(total(xs))\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add, total\n\n\ndef test_add():\n    assert add(1, 2) == 3\n\n\ndef test_total():\n    assert total([1, 2]) == 3\n")
	gittest.Write(t, dir, "tests/test_report.py", "from src.report import report\n\n\ndef test_report():\n    assert report([1, 2]) == \"3\"\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// editTotalAndUnused changes the body of total (line 6) and of unused (line 10).
func editTotalAndUnused(t *testing.T, dir string) {
	t.Helper()
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1]) + 0\n\n\ndef unused():\n    return 1\n")
}

// tripwirePATH replaces PATH with a directory holding git and a tripwire for every test
// runner and toolchain rtdd v0.2 used to invoke. It returns the file a tripwire writes
// when anything runs one: `rtdd which` must leave it absent (spec §1: rtdd never
// executes a test and has no per-language code path).
func tripwirePATH(t *testing.T) string {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not on PATH")
	}
	bin := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "tripped")
	for _, tool := range []string{"python", "python3", "pytest", "go", "node", "npm", "npx", "cargo",
		"mvn", "gradle", "dotnet", "php", "phpunit", "ruby", "bundle", "rspec", "graphify"} {
		script := "#!/bin/sh\necho " + tool + " >> " + marker + "\nexit 1\n"
		if err := os.WriteFile(filepath.Join(bin, tool), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	return marker
}

// PRD #410 AC2: changed nodes, Rounds 1-3, untested and the graph source, in text.
func TestWhichPrintsChangedNodesRoundsUntestedAndGraphSource(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	code, out, errOut := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	want := "graph: scanner, built at " + gittest.HeadShort(t, dir) + ", 0 stale files\n" +
		"changed nodes:\n" +
		"  src/calc.py::total  (lines 5-6)\n" +
		"  src/calc.py::unused  (lines 9-10)\n" +
		"Round 1 — run these first:\n" +
		"  tests/test_calc.py::test_total\n" +
		"Round 2 — then these:\n" +
		"  tests/test_calc.py::test_add\n" +
		"  tests/test_report.py::test_report\n" +
		"Round 3 — the full suite, once, at the end\n" +
		"untested:\n" +
		"  src/calc.py::unused\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}
}

// PRD #410 AC2: empty Rounds 1 and 2 say "no linked test", never anything that reads as
// a pass, and exit 0 (AC6).
func TestWhichWithEmptyRoundsSaysNoLinkedTestAndExits0(t *testing.T) {
	dir := whichRepo(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n\n\ndef unused():\n    return 2\n")
	code, out, errOut := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	if got := strings.Count(out, "  no linked test\n"); got != 2 {
		t.Errorf("want Round 1 and Round 2 each to say %q, got %d in:\n%s", "no linked test", got, out)
	}
	for _, banned := range []string{"pass", "PASS", "green", "nothing to run"} {
		if strings.Contains(out, banned) {
			t.Errorf("output reads as a result (%q):\n%s", banned, out)
		}
	}
}

// PRD #410 AC2, AC6: --base selects changes since that ref; a bad ref and an unknown
// flag are usage errors (2); outside a git repository is an environment error (3).
func TestWhichBaseAndExitCodes(t *testing.T) {
	dir := whichRepo(t)
	first := gittest.HeadShort(t, dir)
	editTotalAndUnused(t, dir)
	gittest.Commit(t, dir, "edit")
	if _, out, _ := rtdd(t, dir, "which"); !strings.Contains(out, "changed nodes:\n  none\n") {
		t.Errorf("a clean tree against HEAD should change nothing:\n%s", out)
	}
	if _, out, _ := rtdd(t, dir, "which", "--base", first); !strings.Contains(out, "  src/calc.py::total  (lines 5-6)\n") {
		t.Errorf("--base %s should see the committed edit:\n%s", first, out)
	}
	for _, c := range []struct {
		name string
		dir  string
		args []string
		want int
	}{
		{"unknown ref", dir, []string{"which", "--base", "no-such-ref"}, 2},
		{"unknown flag", dir, []string{"which", "--bogus"}, 2},
		{"stray argument", dir, []string{"which", "src/calc.py"}, 2},
		{"outside a git repository", t.TempDir(), []string{"which"}, 3},
	} {
		if code, _, _ := rtdd(t, c.dir, c.args...); code != c.want {
			t.Errorf("%s: exit %d, want %d", c.name, code, c.want)
		}
	}
}

// PRD #410 AC2: `rtdd which` runs nothing but git.
func TestWhichSpawnsNoProcessButGit(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	marker := tripwirePATH(t)
	if code, _, errOut := rtdd(t, dir, "which"); code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	if b, err := os.ReadFile(marker); err == nil {
		t.Errorf("rtdd which ran %s", b)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd/ -count=1 -run 'TestWhichPrintsChangedNodesRoundsUntestedAndGraphSource|TestWhichWithEmptyRoundsSaysNoLinkedTestAndExits0|TestWhichBaseAndExitCodes|TestWhichSpawnsNoProcessButGit'
```

Expected: FAIL — `TestWhichPrintsChangedNodesRoundsUntestedAndGraphSource` gets v0.2's tier output instead of the rounds block; `TestWhichWithEmptyRoundsSaysNoLinkedTestAndExits0` finds no `no linked test` and finds `pass` ("an empty selection is not a pass"); `TestWhichBaseAndExitCodes` finds no `changed nodes:` line and gets `unknown ref: exit 3, want 2` and `stray argument: exit 0, want 2`. `TestWhichSpawnsNoProcessButGit` already passes (v0.2 `which` runs nothing either) — it is the guard that keeps it so.

- [ ] **Step 3: `gitctx.ErrUnknownBase`.** In `internal/gitctx/changedset.go`, add `"errors"` to the imports and, above `ChangedSet`:

```go
// ErrUnknownBase is ChangedSet's error for a base that names no commit, so a caller can
// tell the user's typo (a usage error) from git failing (an environment error).
var ErrUnknownBase = errors.New("unknown base")
```

and change the unknown-base return to `return nil, fmt.Errorf("gitctx: %w %q", ErrUnknownBase, base)` — the message text is unchanged.

- [ ] **Step 4: Keep v0.2's JSON answering until Task 3.** `git mv cmd/rtdd/which.go cmd/rtdd/v02.go`; rename `cmdWhich` to `cmdWhichV02` and start its comment with "cmdWhichV02 is v0.2's `rtdd which --json` (schema 2), kept until Task 3 replaces it." `whichNotes`, `emitWhichJSON` and `nonNilStrings` stay in `v02.go` with it.

- [ ] **Step 5: Implement** — create `cmd/rtdd/graphenv.go` (if absent) and `cmd/rtdd/which.go`:

```go
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// noLinkedTest is what an empty Round 1 or Round 2 says. It is never a pass: rtdd ran
// nothing, and an empty round means only that no test links to the change (spec §10).
const noLinkedTest = "no linked test"

// roundThree is Round 3, always (spec §7).
const roundThree = "Round 3 — the full suite, once, at the end"

// cmdWhich answers "which tests does this change need, in what order" from the node
// graph (spec §7, §8). It runs nothing: no test, no toolchain. Every answer exits 0.
func cmdWhich(args []string, stdout, stderr io.Writer) int {
	// Until schema 3 lands (docs/plans/10-rounds-cutover.md Task 3), --json is still the
	// v0.2 document, answered by the v0.2 path.
	for _, a := range args {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			return cmdWhichV02(args, stdout, stderr)
		}
	}
	fs := flag.NewFlagSet("which", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "HEAD", "the ref changes are measured from")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd which [--base <ref>] [--json]")
		return 2
	}
	root, err := graphRoot()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 3
	}
	changes, err := changedSet(root, *base)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		if errors.Is(err, gitctx.ErrUnknownBase) {
			return 2
		}
		return 3
	}
	cfg, res, code, err := buildGraph(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return code
	}
	ranges, warnings := changedRanges(changes)
	r := rounds.Rounds(res.Graph, ranges)
	for _, w := range warnings {
		fmt.Fprintf(stderr, "rtdd which: warning: %s\n", w)
	}
	fmt.Fprint(stdout, renderWhich(res, cfg, r))
	return 0
}

// changedRanges is the changed set as rounds' input: each file's new-side line ranges
// (gitctx.ChangedSet's Lines, verbatim). A deleted file has no lines left to own a node,
// so it selects nothing; the warning says so rather than leaving it silent.
func changedRanges(changes []gitctx.Change) (map[string][]rounds.LineRange, []string) {
	ranges := map[string][]rounds.LineRange{}
	warnings := []string{}
	for _, c := range changes {
		if c.Status == gitctx.Deleted {
			warnings = append(warnings, fmt.Sprintf("%s was deleted: tests that called it are linked to nothing now; Round 3 runs them", c.Path))
			continue
		}
		for _, l := range c.Lines {
			ranges[c.Path] = append(ranges[c.Path], rounds.LineRange{Start: l.Start, End: l.End})
		}
	}
	return ranges, warnings
}

// renderWhich is the human answer: the graph, what changed, the three rounds, untested.
func renderWhich(res *graphbuild.Result, cfg graph.Config, r rounds.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "graph: %s\n", graphLine(res, cfg))
	b.WriteString("changed nodes:\n")
	if len(r.ChangedNodes) == 0 {
		b.WriteString("  none\n")
	}
	for _, n := range r.ChangedNodes {
		fmt.Fprintf(&b, "  %s  (lines %d-%d)\n", n.ID, n.Start, n.End)
	}
	for _, rd := range []struct {
		head  string
		tests []graph.Node
	}{
		{"Round 1 — run these first:", r.Round1},
		{"Round 2 — then these:", r.Round2},
	} {
		b.WriteString(rd.head + "\n")
		if len(rd.tests) == 0 {
			b.WriteString("  " + noLinkedTest + "\n")
		}
		for _, t := range rd.tests {
			fmt.Fprintf(&b, "  %s\n", t.ID)
		}
	}
	b.WriteString(roundThree + "\n")
	b.WriteString("untested:\n")
	if len(r.Untested) == 0 {
		b.WriteString("  none\n")
	}
	for _, n := range r.Untested {
		fmt.Fprintf(&b, "  %s\n", n.ID)
	}
	return b.String()
}

// graphLine states where the graph came from: the source, the commit it describes and
// how many files graphify was not trusted for — or, when graphify was ignored, why.
func graphLine(res *graphbuild.Result, cfg graph.Config) string {
	s := fmt.Sprintf("%s, built at %s, %d stale %s", res.Source, res.BuiltAtCommit,
		len(res.StaleFiles), plural(len(res.StaleFiles), "file", "files"))
	if res.GraphifyIgnored != "" {
		s += fmt.Sprintf(" (graphify ignored — %s; run `graphify --update` to use it again)", ignoredWhy(res, cfg))
	}
	return s
}
```

In `main.go`'s usage, the `which` line becomes `  rtdd which  [--base <ref>] [--json]` (no `--adapter`).

- [ ] **Step 6: Delete the v0.2 text-mode `which` tests** in the Task 2 row of "Old tests: who deletes what"; move any helper they defined that another file still calls to `cmd/rtdd/v02_test.go`.

- [ ] **Step 7: Run to verify**

```bash
go test ./internal/gitctx/ ./cmd/rtdd/ -count=1 2>&1 | grep -E '^(--- FAIL|ok|FAIL)'
```

Expected: `ok` for both packages; the four new tests pass, and the schema-2 `TestWhichJSON*` tests still pass through `cmdWhichV02`.

- [ ] **Step 8: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0.

- [ ] **Step 9: Commit** (list the deleted tests in the body)

```bash
git add -A internal/gitctx cmd/rtdd
git commit -m "feat(which): rtdd which prints changed nodes, Rounds 1-3, untested and graph source from the node graph, running nothing (closes #447)"
```

---

### Task 3: `rtdd which --json` emits schema 3, pinned by a golden test

**Issue:** #448

**Discharges:** PRD #410 AC3, AC6 (JSON mode). Spec §9.

**Files:**
- Modify: `cmd/rtdd/which.go` (schema-3 types, `buildWhichJSON`, `graphObjectOf`, the `--json` branch; the `cmdWhichV02` hand-off goes)
- Modify: `cmd/rtdd/graph.go` (`obj := graphObjectOf(res)`)
- Modify: `cmd/rtdd/v02.go` (delete `cmdWhichV02`, `emitWhichJSON`; keep what `run`-era code still calls, or delete the file if nothing does)
- Create: `cmd/rtdd/testdata/which-golden/repo/…`, `cmd/rtdd/testdata/which-golden/edit/…`, `cmd/rtdd/testdata/which-golden/want.json`
- Delete: the Task 3 row of "Old tests: who deletes what"
- Test: `cmd/rtdd/which_json_test.go`

**Interfaces:**
- Consumes: `whichRepo`, `editTotalAndUnused` (Task 2); `rounds.Result`; `graphObject` (`graph.go`).
- Produces: `type whichJSON`, `changedFile`, `lineRange`, `nodeJSON`, `testJSON`, `testRound`, `fullSuiteRound` (exactly as in "`rtdd which --json`: schema 3"); `func buildWhichJSON(base string, res *graphbuild.Result, changes []gitctx.Change, r rounds.Result, warnings []string) whichJSON`; `func graphObjectOf(res *graphbuild.Result) graphObject`; test helpers `copyTree`, `goldenWhichRepo`.

Lands after Task 6 (see the order under the acceptance-criterion map).

- [ ] **Step 1: Write the failing tests and the fixture.** `cmd/rtdd/which_json_test.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// copyTree copies every file under src into dst, keeping relative paths.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		gittest.Write(t, dst, filepath.ToSlash(rel), string(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// goldenWhichRepo commits testdata/which-golden/repo, then applies the known edit:
// testdata/which-golden/edit over it (total and unused in src/calc.py, the import line of
// src/report.py, Parse in money/money.go) and src/legacy.py deleted.
func goldenWhichRepo(t *testing.T) string {
	t.Helper()
	fixture, err := filepath.Abs(filepath.Join("testdata", "which-golden"))
	if err != nil {
		t.Fatal(err)
	}
	dir := gittest.Init(t)
	copyTree(t, filepath.Join(fixture, "repo"), dir)
	gittest.Commit(t, dir, "fixture")
	copyTree(t, filepath.Join(fixture, "edit"), dir)
	if err := os.Remove(filepath.Join(dir, "src", "legacy.py")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// PRD #410 AC3: the whole schema-3 document for a fixture repository, byte for byte.
// HEAD's sha is the one moving part; it is written as <HEAD>. Regenerate, after
// reading the diff, with:
//
//	RTDD_UPDATE_GOLDEN=1 go test ./cmd/rtdd/ -run '^TestWhichJSONGolden$'
func TestWhichJSONGolden(t *testing.T) {
	golden, err := filepath.Abs(filepath.Join("testdata", "which-golden", "want.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := goldenWhichRepo(t)
	code, out, errOut := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("rtdd which --json = %d, stderr %q", code, errOut)
	}
	got := strings.ReplaceAll(out, `"`+gittest.HeadShort(t, dir)+`"`, `"<HEAD>"`)
	if os.Getenv("RTDD_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("rtdd which --json moved from %s:\n got: %s\nwant: %s", golden, got, want)
	}
}

// PRD #410 AC3: exactly spec §9's nine top-level keys, schema 3, and [] — never null —
// for every empty array.
func TestWhichJSONHasExactlyTheSchema3KeysAndNoNulls(t *testing.T) {
	dir := whichRepo(t) // nothing changed: every array is empty
	code, out, errOut := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("rtdd which --json = %d, stderr %q", code, errOut)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	var keys []string
	for k := range doc {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := []string{"base", "changed", "changed_nodes", "command", "graph", "rounds", "schema", "untested", "warnings"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("top-level keys = %v, want exactly %v", keys, want)
	}
	if string(doc["schema"]) != "3" || string(doc["command"]) != `"which"` || string(doc["base"]) != `"HEAD"` {
		t.Errorf("schema/command/base = %s/%s/%s, want 3/\"which\"/\"HEAD\"", doc["schema"], doc["command"], doc["base"])
	}
	for _, k := range []string{"changed", "changed_nodes", "untested", "warnings"} {
		if string(doc[k]) != "[]" {
			t.Errorf("%s = %s, want []", k, doc[k])
		}
	}
	var rounds []map[string]json.RawMessage
	if err := json.Unmarshal(doc["rounds"], &rounds); err != nil || len(rounds) != 3 {
		t.Fatalf("rounds = %s, want three round objects", doc["rounds"])
	}
	for i, r := range rounds[:2] {
		if string(r["tests"]) != "[]" || string(r["files"]) != "[]" {
			t.Errorf("round %d tests/files = %s/%s, want []/[]", i+1, r["tests"], r["files"])
		}
	}
	if string(rounds[2]["round"]) != "3" || string(rounds[2]["full_suite"]) != "true" || len(rounds[2]) != 2 {
		t.Errorf("round 3 = %v, want exactly {\"round\": 3, \"full_suite\": true}", rounds[2])
	}
}

// PRD #410 AC3: a round's files are its tests' files, de-duplicated, in test order.
func TestWhichJSONRoundFilesAreTheDeduplicatedTestFiles(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	_, out, _ := rtdd(t, dir, "which", "--json")
	var doc struct {
		Rounds []struct {
			Round int `json:"round"`
			Tests []struct {
				ID string `json:"id"`
			} `json:"tests"`
			Files []string `json:"files"`
		} `json:"rounds"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	r2 := doc.Rounds[1]
	if len(r2.Tests) != 2 {
		t.Fatalf("round 2 tests = %v, want test_add and test_report", r2.Tests)
	}
	if want := []string{"tests/test_calc.py", "tests/test_report.py"}; !reflect.DeepEqual(r2.Files, want) {
		t.Errorf("round 2 files = %v, want %v", r2.Files, want)
	}
	if want := []string{"tests/test_calc.py"}; !reflect.DeepEqual(doc.Rounds[0].Files, want) {
		t.Errorf("round 1 files = %v, want %v", doc.Rounds[0].Files, want)
	}
}

// PRD #410 AC6: --json changes the output, never the exit code.
func TestWhichJSONExitCodesMatchTextMode(t *testing.T) {
	dir := whichRepo(t)
	for _, c := range []struct {
		dir  string
		args []string
		want int
	}{
		{dir, []string{"which", "--json"}, 0},
		{dir, []string{"which", "--json", "--base", "no-such-ref"}, 2},
		{t.TempDir(), []string{"which", "--json"}, 3},
	} {
		if code, _, _ := rtdd(t, c.dir, c.args...); code != c.want {
			t.Errorf("rtdd %v: exit %d, want %d", c.args, code, c.want)
		}
	}
}
```

The fixture repository, committed as files (every `.go` file under `testdata/` is ignored by `go build` and must still be gofmt-clean, since `scripts/ci-local.sh` runs `gofmt -l .`):

`cmd/rtdd/testdata/which-golden/repo/src/calc.py`:

```python
def add(a, b):
    return a + b


def total(xs):
    return add(xs[0], xs[1])


def unused():
    return 0
```

`cmd/rtdd/testdata/which-golden/repo/src/report.py`:

```python
from src.calc import total


def report(xs):
    return str(total(xs))
```

`cmd/rtdd/testdata/which-golden/repo/src/legacy.py`:

```python
def legacy():
    return 0
```

`cmd/rtdd/testdata/which-golden/repo/tests/test_calc.py`:

```python
from src.calc import add, total


def test_add():
    assert add(1, 2) == 3


def test_total():
    assert total([1, 2]) == 3
```

`cmd/rtdd/testdata/which-golden/repo/tests/test_report.py`:

```python
from src.report import report


def test_report():
    assert report([1, 2]) == "3"
```

`cmd/rtdd/testdata/which-golden/repo/money/money.go`:

```go
package money

import "strings"

func Normalize(s string) string {
	return strings.TrimSpace(s)
}

func Parse(s string) string {
	return Normalize(s)
}
```

`cmd/rtdd/testdata/which-golden/repo/money/money_test.go`:

```go
package money

import "testing"

func TestParse(t *testing.T) {
	if Parse(" 1 ") != "1" {
		t.Fatal("Parse")
	}
}

func TestNormalize(t *testing.T) {
	if Normalize(" 1") != "1" {
		t.Fatal("Normalize")
	}
}
```

The known edit — `cmd/rtdd/testdata/which-golden/edit/src/calc.py` (bodies of `total` and `unused`):

```python
def add(a, b):
    return a + b


def total(xs):
    return add(xs[0], xs[1]) + 0


def unused():
    return 1
```

`cmd/rtdd/testdata/which-golden/edit/src/report.py` (its top-level import line):

```python
from src.calc import total  # the one import


def report(xs):
    return str(total(xs))
```

`cmd/rtdd/testdata/which-golden/edit/money/money.go` (the body of `Parse`):

```go
package money

import "strings"

func Normalize(s string) string {
	return strings.TrimSpace(s)
}

func Parse(s string) string {
	return Normalize(strings.ToLower(s))
}
```

and `cmd/rtdd/testdata/which-golden/want.json` — this document was produced by the implementation in Step 3 on the fixture above; every line of it follows from spec §7 and §9 (Round 1: `TestParse` and `test_total` link to the edited `Parse` and `total`, `test_report` calls into `src/report.py`, whose import line changed; Round 2: `TestNormalize` and `test_add` link to the callees `Normalize` and `add`; `test_report` is not repeated; `unused` has no test; the deleted `src/legacy.py` is a warning):

```json
{
  "schema": 3,
  "command": "which",
  "base": "HEAD",
  "graph": {
    "source": "scanner",
    "built_at_commit": "<HEAD>",
    "stale_files": 0,
    "nodes": 11,
    "edges": 8,
    "tests": 5
  },
  "changed": [
    {
      "path": "money/money.go",
      "lines": [
        {
          "start": 10,
          "end": 10
        }
      ]
    },
    {
      "path": "src/calc.py",
      "lines": [
        {
          "start": 6,
          "end": 6
        },
        {
          "start": 10,
          "end": 10
        }
      ]
    },
    {
      "path": "src/legacy.py",
      "lines": []
    },
    {
      "path": "src/report.py",
      "lines": [
        {
          "start": 1,
          "end": 1
        }
      ]
    }
  ],
  "changed_nodes": [
    {
      "id": "money/money.go::Parse",
      "file": "money/money.go",
      "name": "Parse",
      "start": 9,
      "end": 11
    },
    {
      "id": "src/calc.py::total",
      "file": "src/calc.py",
      "name": "total",
      "start": 5,
      "end": 6
    },
    {
      "id": "src/calc.py::unused",
      "file": "src/calc.py",
      "name": "unused",
      "start": 9,
      "end": 10
    },
    {
      "id": "src/report.py::<module>",
      "file": "src/report.py",
      "name": "<module>",
      "start": 1,
      "end": 1
    }
  ],
  "rounds": [
    {
      "round": 1,
      "tests": [
        {
          "id": "money/money_test.go::TestParse",
          "file": "money/money_test.go",
          "name": "TestParse"
        },
        {
          "id": "tests/test_calc.py::test_total",
          "file": "tests/test_calc.py",
          "name": "test_total"
        },
        {
          "id": "tests/test_report.py::test_report",
          "file": "tests/test_report.py",
          "name": "test_report"
        }
      ],
      "files": [
        "money/money_test.go",
        "tests/test_calc.py",
        "tests/test_report.py"
      ]
    },
    {
      "round": 2,
      "tests": [
        {
          "id": "money/money_test.go::TestNormalize",
          "file": "money/money_test.go",
          "name": "TestNormalize"
        },
        {
          "id": "tests/test_calc.py::test_add",
          "file": "tests/test_calc.py",
          "name": "test_add"
        }
      ],
      "files": [
        "money/money_test.go",
        "tests/test_calc.py"
      ]
    },
    {
      "round": 3,
      "full_suite": true
    }
  ],
  "untested": [
    "src/calc.py::unused"
  ],
  "warnings": [
    "src/legacy.py was deleted: tests that called it are linked to nothing now; Round 3 runs them"
  ]
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd/ -count=1 -run 'TestWhichJSONGolden$|TestWhichJSONHasExactlyTheSchema3KeysAndNoNulls|TestWhichJSONRoundFilesAreTheDeduplicatedTestFiles|TestWhichJSONExitCodesMatchTextMode'
```

Expected: FAIL — `--json` still answers schema 2 through `cmdWhichV02`: the golden differs from its first line of content (`"schema": 2` … `"tier"`), the key set is v0.2's (`adapter`, `selection`, `tier`, `uncovered` …, not the nine of §9), and `rounds` is absent. `TestWhichJSONExitCodesMatchTextMode` fails on `--base no-such-ref` (v0.2 exits 3).

- [ ] **Step 3: Implement.** In `cmd/rtdd/which.go`: delete the `--json` hand-off loop; declare `asJSON := fs.Bool("json", false, "emit machine-readable JSON (schema 3)")`; after `rounds.Rounds`, branch before printing warnings:

```go
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false) // "<module>" stays readable
		enc.SetIndent("", "  ")
		if err := enc.Encode(buildWhichJSON(*base, res, changes, r, warnings)); err != nil {
			fmt.Fprintf(stderr, "rtdd which: %v\n", err)
			return 3
		}
		return 0
	}
```

and add the types of "`rtdd which --json`: schema 3" plus:

```go
// buildWhichJSON is the schema-3 document (spec §9) for one answer.
func buildWhichJSON(base string, res *graphbuild.Result, changes []gitctx.Change, r rounds.Result, warnings []string) whichJSON {
	doc := whichJSON{Schema: 3, Command: "which", Base: base, Graph: graphObjectOf(res),
		Changed: []changedFile{}, ChangedNodes: []nodeJSON{}, Untested: []string{}, Warnings: warnings}
	for _, c := range changes {
		f := changedFile{Path: c.Path, Lines: []lineRange{}}
		for _, l := range c.Lines {
			f.Lines = append(f.Lines, lineRange{Start: l.Start, End: l.End})
		}
		doc.Changed = append(doc.Changed, f)
	}
	for _, n := range r.ChangedNodes {
		doc.ChangedNodes = append(doc.ChangedNodes, nodeJSON{ID: n.ID, File: n.File, Name: n.Name, Start: n.Start, End: n.End})
	}
	for i, tests := range [][]graph.Node{r.Round1, r.Round2} {
		rd := testRound{Round: i + 1, Tests: []testJSON{}, Files: []string{}}
		seen := map[string]bool{}
		for _, t := range tests {
			rd.Tests = append(rd.Tests, testJSON{ID: t.ID, File: t.File, Name: t.Name})
			if !seen[t.File] {
				seen[t.File] = true
				rd.Files = append(rd.Files, t.File)
			}
		}
		doc.Rounds = append(doc.Rounds, rd)
	}
	doc.Rounds = append(doc.Rounds, fullSuiteRound{Round: 3, FullSuite: true})
	for _, n := range r.Untested {
		doc.Untested = append(doc.Untested, n.ID)
	}
	return doc
}

// graphObjectOf is the `graph` object `rtdd graph --json` and `rtdd which --json` share.
func graphObjectOf(res *graphbuild.Result) graphObject {
	obj := graphObject{Source: res.Source, BuiltAtCommit: res.BuiltAtCommit, StaleFiles: len(res.StaleFiles),
		GraphifyIgnored: res.GraphifyIgnored, Nodes: len(res.Graph.Nodes), Edges: len(res.Graph.Edges)}
	for _, n := range res.Graph.Nodes {
		if n.IsTest {
			obj.Tests++
		}
	}
	return obj
}
```

In `cmd/rtdd/graph.go`, replace the inline `graphObject{…}` construction and its test-counting loop with `obj := graphObjectOf(res)`. In `v02.go`, delete `cmdWhichV02` and `emitWhichJSON`, and whatever of `whichNotes`/`nonNilStrings` no longer has a caller (`grep -n 'whichNotes(\|nonNilStrings(' cmd/rtdd/*.go`); delete `v02.go` if it is then empty.

- [ ] **Step 4: Delete the v0.2 `which --json` tests** in the Task 3 row of "Old tests: who deletes what" (helpers still called elsewhere move to `v02_test.go`).

- [ ] **Step 5: Run to verify**

```bash
go test ./cmd/rtdd/ -count=1 2>&1 | grep -E '^(--- FAIL|ok|FAIL)'
```

Expected: `ok`. The golden passes without `RTDD_UPDATE_GOLDEN`; `TestGraphCommandJSONShape` still passes (the `graph` object did not change).

- [ ] **Step 6: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0. The bench tests that read schema 2 are already fenced (Task 6); if another bench test now fails on the schema, fence it the same way and name it in the PR.

- [ ] **Step 7: Commit**

```bash
git add -A cmd/rtdd
git commit -m "feat(which): rtdd which --json emits schema 3 with exactly the §9 top-level keys, pinned by a golden test (closes #448)"
```

---

### Task 4: `rtdd explain <file[:line]|name>` prints a node's tests, callers and callees

**Issue:** #449

**Discharges:** PRD #410 AC4 (`explain`), AC6. Spec §8, §12 (all same-named definitions).

**Files:**
- Create: `internal/rounds/links.go`
- Create: `cmd/rtdd/graphenv.go` (if absent), `cmd/rtdd/plural.go` (`plural`, moved verbatim from `explain.go`)
- Rewrite: `cmd/rtdd/explain.go`
- Modify: `cmd/rtdd/main.go` (usage line `rtdd explain <file[:line]|name>`)
- Delete: `cmd/rtdd/explain_test.go`
- Test: `cmd/rtdd/explain_graph_test.go`

**Interfaces:**
- Consumes: `rounds.Owner`, `newIndex`, `byFileThenLine` (Task 1); `graphRoot`, `buildGraph`; `paths.Normalize`.
- Produces: `type rounds.Link struct { Node graph.Node; Relation graph.Relation }`; `type rounds.Links struct { Tests, Callers, Callees []Link }`; `func rounds.LinksOf(g graph.Graph, id string) Links`; `func explainTargets(root string, g graph.Graph, arg string) []graph.Node`; `func renderExplain(n graph.Node, l rounds.Links) string`.

- [ ] **Step 1: Write the failing tests** — `cmd/rtdd/explain_graph_test.go` (its fixture, `explainRepo`, is Task 2's `whichRepo` under its own name, so this task does not wait for Task 2):

```go
package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// explainRepo is a committed Python project: total calls add, report calls total, and
// add, total and report each have a test.
func explainRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n\n\ndef unused():\n    return 0\n")
	gittest.Write(t, dir, "src/report.py", "from src.calc import total\n\n\ndef report(xs):\n    return str(total(xs))\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add, total\n\n\ndef test_add():\n    assert add(1, 2) == 3\n\n\ndef test_total():\n    assert total([1, 2]) == 3\n")
	gittest.Write(t, dir, "tests/test_report.py", "from src.report import report\n\n\ndef test_report():\n    assert report([1, 2]) == \"3\"\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// totalExplained is `rtdd explain` on src/calc.py::total in explainRepo.
const totalExplained = "src/calc.py::total  (func, lines 5-6)\n" +
	"  tests:\n" +
	"    tests/test_calc.py::test_total  (tests/test_calc.py:8, calls)\n" +
	"  callers:\n" +
	"    src/report.py::report  (src/report.py:4, calls)\n" +
	"  callees:\n" +
	"    src/calc.py::add  (src/calc.py:1, calls)\n"

// PRD #410 AC4: explain prints a node's tests, callers and callees, for each of the
// three argument forms.
func TestExplainPrintsTestsCallersAndCallees(t *testing.T) {
	dir := explainRepo(t)
	for _, c := range []struct {
		arg, want string
	}{
		{"src/calc.py:6", totalExplained},
		{"total", totalExplained},
		{"src/calc.py::total", totalExplained},
		{"src/report.py", "src/report.py::report  (func, lines 4-5)\n" +
			"  tests:\n" +
			"    tests/test_report.py::test_report  (tests/test_report.py:4, calls)\n" +
			"  callers:\n" +
			"    none\n" +
			"  callees:\n" +
			"    src/calc.py::total  (src/calc.py:5, calls)\n"},
	} {
		code, out, errOut := rtdd(t, dir, "explain", c.arg)
		if code != 0 {
			t.Errorf("rtdd explain %s = %d, stderr %q", c.arg, code, errOut)
			continue
		}
		if out != c.want {
			t.Errorf("rtdd explain %s:\n%s\nwant:\n%s", c.arg, out, c.want)
		}
	}
}

// A file argument is relative to the caller's directory, like any path on a command line.
func TestExplainResolvesAFileRelativeToTheWorkingDirectory(t *testing.T) {
	dir := explainRepo(t)
	code, out, errOut := rtdd(t, dir+"/src", "explain", "calc.py:6")
	if code != 0 || out != totalExplained {
		t.Errorf("rtdd explain calc.py:6 from src/ = %d %q\n%s", code, errOut, out)
	}
}

// Spec §12 (#417): a name defined in several files lists every definition.
func TestExplainANameListsEveryDefinition(t *testing.T) {
	dir := explainRepo(t)
	gittest.Write(t, dir, "src/other.py", "def total(xs):\n    return 0\n")
	gittest.Commit(t, dir, "second total")
	code, out, _ := rtdd(t, dir, "explain", "total")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"src/calc.py::total  (func", "src/other.py::total  (func"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain total does not list %q:\n%s", want, out)
		}
	}
}

// An argument naming nothing, or no argument, is a usage error (2); outside a git
// repository is an environment error (3).
func TestExplainExitCodes(t *testing.T) {
	dir := explainRepo(t)
	for _, c := range []struct {
		dir  string
		args []string
		want int
		msg  string
	}{
		{dir, []string{"explain", "no_such_name"}, 2, `no node matches "no_such_name"`},
		{dir, []string{"explain", "src/calc.py:3"}, 2, `no node matches "src/calc.py:3"`},
		{dir, []string{"explain"}, 2, "usage: rtdd explain <file[:line]|name>"},
		{t.TempDir(), []string{"explain", "total"}, 3, "not inside a git work tree"},
	} {
		code, _, errOut := rtdd(t, c.dir, c.args...)
		if code != c.want || !strings.Contains(errOut, c.msg) {
			t.Errorf("rtdd %v = %d, stderr %q; want %d and %q", c.args, code, errOut, c.want, c.msg)
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd/ -count=1 -run 'TestExplainPrintsTestsCallersAndCallees|TestExplainResolvesAFileRelativeToTheWorkingDirectory|TestExplainANameListsEveryDefinition|TestExplainExitCodes'
```

Expected: FAIL — v0.2 `explain` reads the coverage map: for every argument it prints `<arg> is covered by 0 tests.` / `The map is empty. Run `rtdd seed` first.`, it exits 0 for `no_such_name` and `src/calc.py:3`, and its usage line is `usage: rtdd explain <file>`.

- [ ] **Step 3: Implement.** `internal/rounds/links.go`:

```go
package rounds

import (
	"slices"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// Link is a node one edge away, and the relation of that edge.
type Link struct {
	Node     graph.Node
	Relation graph.Relation
}

// Links is one node's depth-1 surroundings, as `rtdd explain` prints them: the tests with
// an edge to it, the non-test nodes with an edge to it, and the nodes it has an edge to.
// Each list is ordered by File, then Start, then ID, then Relation, and is non-nil.
type Links struct {
	Tests   []Link
	Callers []Link
	Callees []Link
}

// LinksOf returns the links of the node with the given ID; an unknown ID has none.
func LinksOf(g graph.Graph, id string) Links {
	ix := newIndex(g)
	l := Links{Tests: []Link{}, Callers: []Link{}, Callees: []Link{}}
	for _, e := range ix.in[id] {
		if n, ok := ix.byID[e.From]; ok {
			if n.IsTest {
				l.Tests = append(l.Tests, Link{n, e.Relation})
			} else {
				l.Callers = append(l.Callers, Link{n, e.Relation})
			}
		}
	}
	for _, e := range ix.out[id] {
		if n, ok := ix.byID[e.To]; ok {
			l.Callees = append(l.Callees, Link{n, e.Relation})
		}
	}
	for _, ls := range [][]Link{l.Tests, l.Callers, l.Callees} {
		slices.SortFunc(ls, func(a, b Link) int {
			if c := byFileThenLine(a.Node, b.Node); c != 0 {
				return c
			}
			return strings.Compare(string(a.Relation), string(b.Relation))
		})
	}
	return l
}
```

Move `plural` from `explain.go` to a new `cmd/rtdd/plural.go` unchanged (`which`, `doctor` and the v0.2 renderers call it). Then replace `cmd/rtdd/explain.go` with:

```go
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/paths"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// fileLine is an explain argument naming a line: "src/calc.py:6".
var fileLine = regexp.MustCompile(`^(.+):([0-9]+)$`)

// cmdExplain implements `rtdd explain <file[:line]|name>` (spec §8): for each node the
// argument names, its tests, callers and callees, from the graph. It runs nothing.
func cmdExplain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: rtdd explain <file[:line]|name>")
		return 2
	}
	root, err := graphRoot()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd explain: %v\n", err)
		return 3
	}
	_, res, code, err := buildGraph(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd explain: %v\n", err)
		return code
	}
	nodes := explainTargets(root, res.Graph, fs.Arg(0))
	if len(nodes) == 0 {
		fmt.Fprintf(stderr, "rtdd explain: no node matches %q (give a file, file:line, node id or name)\n", fs.Arg(0))
		return 2
	}
	for i, n := range nodes {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprint(stdout, renderExplain(n, rounds.LinksOf(res.Graph, n.ID)))
	}
	return 0
}

// explainTargets resolves the argument, in order: a node ID; file:line (the innermost
// node owning that line); a file (all its nodes); a name (every node so named — all
// same-named definitions, spec §12). A path is relative to the caller's directory.
func explainTargets(root string, g graph.Graph, arg string) []graph.Node {
	var out []graph.Node
	for _, n := range g.Nodes {
		if n.ID == arg {
			return []graph.Node{n}
		}
	}
	rel := func(p string) string {
		abs := p
		if !filepath.IsAbs(abs) {
			wd, _ := os.Getwd()
			abs = filepath.Join(wd, p)
		}
		r, _ := paths.Normalize(root, abs)
		return r
	}
	if m := fileLine.FindStringSubmatch(arg); m != nil {
		line, _ := strconv.Atoi(m[2])
		if n, ok := rounds.Owner(g, rel(m[1]), line); ok {
			return []graph.Node{n}
		}
		return nil
	}
	file := rel(arg)
	for _, n := range g.Nodes {
		if n.File == file {
			out = append(out, n)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, n := range g.Nodes {
		if n.Name == arg {
			out = append(out, n)
		}
	}
	return out
}

func renderExplain(n graph.Node, l rounds.Links) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  (%s, lines %d-%d)\n", n.ID, n.Kind, n.Start, n.End)
	for _, sec := range []struct {
		head  string
		links []rounds.Link
	}{{"tests", l.Tests}, {"callers", l.Callers}, {"callees", l.Callees}} {
		fmt.Fprintf(&b, "  %s:\n", sec.head)
		if len(sec.links) == 0 {
			b.WriteString("    none\n")
		}
		for _, x := range sec.links {
			fmt.Fprintf(&b, "    %s  (%s:%d, %s)\n", x.Node.ID, x.Node.File, x.Node.Start, x.Relation)
		}
	}
	return b.String()
}
```

Usage line: `  rtdd explain <file[:line]|name>`.

- [ ] **Step 4: Delete `cmd/rtdd/explain_test.go`** (it pins `RenderExplain` over the coverage map).

- [ ] **Step 5: Run to verify**

```bash
go test ./internal/rounds/ ./cmd/rtdd/ -count=1 2>&1 | grep -E '^(--- FAIL|ok|FAIL)'
```

Expected: `ok` for both.

- [ ] **Step 6: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0. `go list -deps ./cmd/rtdd` still lists the pipeline packages (other v0.2 code imports them); `explain.go` itself imports none of `internal/mapstore`, `internal/selector`, `internal/uncovered`, `internal/adapter`.

- [ ] **Step 7: Commit**

```bash
git add -A internal/rounds cmd/rtdd
git commit -m "feat(explain): rtdd explain <file[:line]|name> prints a node's tests, callers and callees from the graph (closes #449)"
```

---

### Task 5: `rtdd doctor` prints graph source, graphify staleness, test-file counts and names defined ≥ 8 times

**Issue:** #450

**Discharges:** PRD #410 AC4 (`doctor`), AC6. Spec §8, §12 (names defined ≥ 8 times, #417).

**Files:**
- Create: `cmd/rtdd/graphenv.go` (if absent)
- Rewrite: `cmd/rtdd/doctor.go`
- Modify: `cmd/rtdd/main.go` (usage line `rtdd doctor`)
- Delete: the Task 5 row of "Old tests: who deletes what"
- Test: `cmd/rtdd/doctor_graph_test.go`

**Interfaces:**
- Consumes: `graphRoot`, `buildGraph`; `ignoredWhy` (`graph.go`); `plural`; `gitctx.ListFiles`, `scan.Filter`, `graph.IsTestFile`; `graphRepo`, `writeGraphifyFor` (`cmd/rtdd/graph_test.go`, PRD #409).
- Produces: `const overLinkDefinitions = 8`; `func renderDoctor(res *graphbuild.Result, cfg graph.Config, files []string) string`; `func graphifyState(res *graphbuild.Result, cfg graph.Config) string`; `func overLinked(g graph.Graph, atLeast int) []nameCount`.

- [ ] **Step 1: Write the failing tests** — `cmd/rtdd/doctor_graph_test.go`:

```go
package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// PRD #410 AC4: doctor prints graph source, graphify staleness, test-file counts and the
// names defined at least 8 times — and nothing about adapters or a coverage map.
func TestDoctorReportsTheGraphTestFilesAndOverLinkedNames(t *testing.T) {
	dir := gittest.Init(t)
	for i := 0; i < 8; i++ {
		src := "def run():\n    return 0\n"
		if i < 7 {
			src += "\n\ndef step():\n    return 1\n"
		}
		gittest.Write(t, dir, fmt.Sprintf("src/m%d.py", i), src)
	}
	gittest.Write(t, dir, "tests/test_run.py", "def test_run():\n    assert run() == 0\n")
	gittest.Write(t, dir, "tests/conftest_data.txt", "fixture data, not code\n")
	gittest.Commit(t, dir, "init")

	code, out, errOut := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("rtdd doctor = %d, stderr %q", code, errOut)
	}
	want := "graph source:  scanner\n" +
		"built at:      " + gittest.HeadShort(t, dir) + "\n" +
		"graphify:      not present (graphify-out/graph.json)\n" +
		"nodes:         16\n" +
		"edges:         8\n" +
		"test files:    2 matched by test_files, 1 with a test node\n" +
		"test nodes:    1\n" +
		"names defined 8 or more times (a call to one links to every definition):\n" +
		"  run  8\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}
	for _, banned := range []string{"adapter", "map.jsonl", "coverage", "fan-out"} {
		if strings.Contains(strings.ToLower(out), banned) {
			t.Errorf("doctor still mentions %q:\n%s", banned, out)
		}
	}
}

// Graphify present: used (with its staleness) when fresh, ignored with the reason when
// more than max_stale_ratio of its code files are stale.
func TestDoctorReportsGraphifyFreshAndTooStale(t *testing.T) {
	dir := graphRepo(t)
	writeGraphifyFor(t, dir, gittest.HeadShort(t, dir))
	gittest.Commit(t, dir, "graphify")
	_, out, _ := rtdd(t, dir, "doctor")
	if !strings.Contains(out, "graph source:  graphify+scanner\n") || !strings.Contains(out, "graphify:      used, built at ") {
		t.Errorf("fresh graphify not reported as used:\n%s", out)
	}

	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return b + a\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add\n\n\ndef test_add():\n    assert add(2, 1) == 3\n")
	_, out, _ = rtdd(t, dir, "doctor")
	if !strings.Contains(out, "graph source:  scanner\n") || !strings.Contains(out, "graphify:      ignored — ") ||
		!strings.Contains(out, "graphify --update") {
		t.Errorf("too-stale graphify not reported as ignored with its reason:\n%s", out)
	}
}

// AC6: environment errors exit 3, usage errors 2.
func TestDoctorExitCodes(t *testing.T) {
	if code, _, _ := rtdd(t, t.TempDir(), "doctor"); code != 3 {
		t.Errorf("outside a git repository: exit %d, want 3", code)
	}
	if code, _, _ := rtdd(t, graphRepo(t), "doctor", "--limit", "5"); code != 2 {
		t.Errorf("the removed --limit flag: exit %d, want 2", code)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd/ -count=1 -run 'TestDoctorReportsTheGraphTestFilesAndOverLinkedNames|TestDoctorReportsGraphifyFreshAndTooStale|TestDoctorExitCodes'
```

Expected: FAIL — v0.2 `doctor` prints the adapter rows and the fan-out ranking (`doctor still mentions "adapter"`, `… "fan-out"`, and the exact block differs), has no `graph source:` or `graphify:` line (`fresh graphify not reported as used`, `too-stale graphify not reported as ignored`), and accepts `--limit 5` (`exit 0, want 2`).

- [ ] **Step 3: Implement** — replace `cmd/rtdd/doctor.go` with:

```go
package main

import (
	"cmp"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
	"github.com/VocanicZ/rtdd/internal/scan"
)

// overLinkDefinitions is how many definitions of one name make it an over-link hub:
// every call to the name links to all of them (spec §4.3, §12, #417).
const overLinkDefinitions = 8

// cmdDoctor implements `rtdd doctor` (spec §8): where the graph came from, how stale
// graphify is, what the test_files globs found, and the names over-linked enough to
// widen every round that touches them. It runs nothing; it changes only the graph cache.
func cmdDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd doctor")
		return 2
	}
	root, err := graphRoot()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", err)
		return 3
	}
	cfg, res, code, err := buildGraph(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", err)
		return code
	}
	listed, err := gitctx.ListFiles(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd doctor: %v\n", err)
		return 3
	}
	fmt.Fprint(stdout, renderDoctor(res, cfg, scan.Filter(root, listed, cfg.ScanExclude)))
	return 0
}

func renderDoctor(res *graphbuild.Result, cfg graph.Config, files []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "graph source:  %s\n", res.Source)
	fmt.Fprintf(&b, "built at:      %s\n", res.BuiltAtCommit)
	fmt.Fprintf(&b, "graphify:      %s\n", graphifyState(res, cfg))
	fmt.Fprintf(&b, "nodes:         %d\n", len(res.Graph.Nodes))
	fmt.Fprintf(&b, "edges:         %d\n", len(res.Graph.Edges))

	testFiles, withTests, testNodes := 0, map[string]bool{}, 0
	for _, f := range files {
		if graph.IsTestFile(f, cfg) {
			testFiles++
		}
	}
	for _, n := range res.Graph.Nodes {
		if n.IsTest {
			testNodes++
			withTests[n.File] = true
		}
	}
	fmt.Fprintf(&b, "test files:    %d matched by test_files, %d with a test node\n", testFiles, len(withTests))
	fmt.Fprintf(&b, "test nodes:    %d\n", testNodes)

	fmt.Fprintf(&b, "names defined %d or more times (a call to one links to every definition):\n", overLinkDefinitions)
	hubs := overLinked(res.Graph, overLinkDefinitions)
	if len(hubs) == 0 {
		b.WriteString("  none\n")
	}
	for _, h := range hubs {
		fmt.Fprintf(&b, "  %s  %d\n", h.name, h.count)
	}
	return b.String()
}

// graphifyState is doctor's one line on graphify: absent, used (and how stale), or
// ignored (and why).
func graphifyState(res *graphbuild.Result, cfg graph.Config) string {
	switch {
	case res.GraphifyIgnored != "":
		return fmt.Sprintf("ignored — %s; run `graphify --update` to use it again", ignoredWhy(res, cfg))
	case res.Source == graphbuild.SourceGraphifyScanner:
		return fmt.Sprintf("used, built at %s — %d stale %s, %d of its %d code files (max_stale_ratio %.2f)",
			res.GraphifyCommit, len(res.StaleFiles), plural(len(res.StaleFiles), "file", "files"),
			res.StaleCodeFiles, res.GraphifyFiles, cfg.MaxStaleRatio)
	}
	return "not present (" + cfg.GraphifyPath + ")"
}

type nameCount struct {
	name  string
	count int
}

// overLinked is every non-test name defined at least atLeast times, most-defined first.
func overLinked(g graph.Graph, atLeast int) []nameCount {
	counts := map[string]int{}
	for _, n := range g.Nodes {
		if !n.IsTest {
			counts[n.Name]++
		}
	}
	var out []nameCount
	for name, c := range counts {
		if c >= atLeast {
			out = append(out, nameCount{name, c})
		}
	}
	slices.SortFunc(out, func(a, b nameCount) int {
		if c := cmp.Compare(b.count, a.count); c != 0 {
			return c
		}
		return strings.Compare(a.name, b.name)
	})
	return out
}
```

Usage line: `  rtdd doctor`.

- [ ] **Step 4: Delete the v0.2 `doctor` tests** in the Task 5 row of "Old tests: who deletes what"; `builtinsExcept` (in `doctor_test.go`) moves to `v02_test.go` if `hostadapter_test.go` still calls it.

- [ ] **Step 5: Run to verify**

```bash
go test ./cmd/rtdd/ -count=1 2>&1 | grep -E '^(--- FAIL|ok|FAIL)'
```

Expected: `ok`.

- [ ] **Step 6: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0. `doctor.go` imports neither `internal/mapstore` nor `internal/adapter` nor `internal/doctor`; `internal/contract`'s doctor-caveat tests still pass (they read `internal/doctor`, deleted in Task 7).

- [ ] **Step 7: Commit**

```bash
git add -A cmd/rtdd
git commit -m "feat(doctor): rtdd doctor prints graph source, graphify staleness, test-file counts and names defined >= 8 times (closes #450)"
```

---

### Task 6: `seed`, `run`, `verify`, `status` and `map compact` are removed; invoking one exits 2 naming v0.3.0

**Issue:** #451

**Discharges:** PRD #410 AC5 (commands), AC6 (no exit code reflects a test result), AC9 (the `^TestPipeline` guard). Spec §8.

**Files:**
- Modify: `cmd/rtdd/main.go` (dispatch, usage, `removedCommand`)
- Modify: `cmd/rtdd/init.go` (`RenderNextStep`)
- Delete: `cmd/rtdd/seed.go`, `run.go`, `status.go`, `exit.go`, and the Task 6 row of "Old tests: who deletes what"
- Modify: `scripts/ci-local.sh`, `.github/workflows/ci.yml`, `internal/contract/contract_test.go` (`localCIChecks`), `internal/contract/ci_workflow_test.go` (per "CI gates", Task 6 rows)
- Create: `bench/tests/v02binary.py`; modify the three bench tests it fences
- Test: `cmd/rtdd/removed_test.go`

**Interfaces:**
- Consumes: `graphRepo` (`cmd/rtdd/graph_test.go`, PRD #409).
- Produces: `func removedCommand(args []string, stderr io.Writer) int`; `RenderNextStep()` returns `"\nNext: edit code, then run `rtdd which` for the tests to run, in rounds.\n"`; `bench/tests/v02binary.py`'s `requires_v02_rtdd`.

- [ ] **Step 1: Write the failing tests** — `cmd/rtdd/removed_test.go`:

```go
package main

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// removedInV030 are the coverage-pipeline commands v0.3.0 removes (spec §8).
var removedInV030 = [][]string{{"seed"}, {"run"}, {"verify"}, {"status"}, {"map", "compact"}}

// PRD #410 AC5, AC6: invoking a removed command is a usage error (2) that names the
// release that removed it and what replaces it, and changes nothing.
func TestRemovedCommandsExit2NamingV030(t *testing.T) {
	dir := graphRepo(t)
	for _, args := range removedInV030 {
		code, out, errOut := rtdd(t, dir, args...)
		name := strings.Join(args, " ")
		if code != 2 {
			t.Errorf("rtdd %s: exit %d, want 2", name, code)
		}
		if out != "" {
			t.Errorf("rtdd %s wrote to stdout: %q", name, out)
		}
		for _, want := range []string{"rtdd " + name, "removed in v0.3.0", "rtdd which"} {
			if !strings.Contains(errOut, want) {
				t.Errorf("rtdd %s: stderr %q does not contain %q", name, errOut, want)
			}
		}
	}
	if _, err := os.Stat(dir + "/.rtdd"); err == nil {
		t.Errorf("a removed command created .rtdd/")
	}
}

// PRD #410 AC5: --help lists none of them, and no exit code is a test result.
func TestHelpListsNoRemovedCommand(t *testing.T) {
	_, help, _ := rtdd(t, graphRepo(t), "--help")
	for _, gone := range []string{"rtdd seed", "rtdd run", "rtdd verify", "rtdd status", "rtdd map", "a test failed"} {
		if strings.Contains(help, gone) {
			t.Errorf("--help still mentions %q:\n%s", gone, help)
		}
	}
}

// PRD #410 AC5: no source file in this package implements a removed command.
func TestNoSourceImplementsARemovedCommand(t *testing.T) {
	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for name, f := range pkg.Files {
			for _, d := range f.Scope.Objects {
				switch d.Name {
				case "cmdSeed", "cmdRun", "cmdVerify", "cmdStatus", "cmdMap", "cmdMapCompact":
					t.Errorf("%s still declares %s", name, d.Name)
				}
			}
		}
	}
}

// rtdd init's closing line no longer sends anyone to a removed command.
func TestInitClosesByPointingAtWhichNotSeed(t *testing.T) {
	code, out, errOut := rtdd(t, graphRepo(t), "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "Next: edit code, then run `rtdd which` for the tests to run, in rounds.") || strings.Contains(out, "rtdd seed") {
		t.Errorf("rtdd init's closing line still points at a removed command, or not at rtdd which:\n%s", out)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./cmd/rtdd/ -count=1 -run 'TestRemovedCommandsExit2NamingV030|TestHelpListsNoRemovedCommand|TestNoSourceImplementsARemovedCommand|TestInitClosesByPointingAtWhichNotSeed'
```

Expected: FAIL — `rtdd status: exit 0, want 2` and none of the five messages names `removed in v0.3.0` (`seed` and `run` really run, against the fixture's Python tests, before failing — this is the last time they do); `--help still mentions "rtdd seed"`, `"rtdd run"`, `"rtdd status"`, `"a test failed"`; `run.go still declares cmdRun`, `seed.go still declares cmdSeed`, `status.go still declares cmdStatus`; `rtdd init`'s closing line still names `rtdd seed`.

- [ ] **Step 3: Implement.** In `main.go`'s `run` switch, replace the `seed`, `run` and `status` cases with one:

```go
	case "seed", "run", "verify", "status", "map":
		return removedCommand(args, stderr)
```

and add:

```go
// removedCommand answers a v0.2 command v0.3.0 removed (spec §8): a usage error naming
// the release and what replaces it. It writes nothing.
func removedCommand(args []string, stderr io.Writer) int {
	name := args[0]
	if name == "map" && len(args) > 1 {
		name += " " + args[1]
	}
	fmt.Fprintf(stderr, "rtdd %s: removed in v0.3.0 — rtdd no longer runs tests or keeps a coverage map.\n"+
		"Run `rtdd which` and run its rounds with the project's own test command.\n", name)
	return 2
}
```

In `usage`, delete the `rtdd seed`, `rtdd run …` and `rtdd status …` lines and the exit-code line `  1  a test failed`. In `init.go`, `RenderNextStep` returns `"\nNext: edit code, then run `rtdd which` for the tests to run, in rounds.\n"` (its comment: v0.3.0 has no seed step; the graph is built on first use).

- [ ] **Step 4: Delete** `seed.go`, `run.go`, `status.go`, `exit.go` (`ExitCodeFor` was the exit-1 path) and the tests in the Task 6 row — `pipeline_*_test.go` and `acceptance_test.go` included. Helpers still called by surviving test files (`chdir`, `realRepo`, `readMapJSONL`, `containsStr`, `appendLine`, `touchLogic`, `makeSuiteGreen`, `fakeRepo`, `uncoveredReports` — check each with `grep -n 'name(' cmd/rtdd/*_test.go`) move verbatim to `cmd/rtdd/v02_test.go`. `go vet ./cmd/rtdd/` must be clean: an "undefined" there is a surviving test that called a removed command — it belongs to this task's deletions if it pins v0.2 behaviour, and is a bug otherwise.

- [ ] **Step 5: The `TestPipeline*` gate leaves both CI gates** — the Task 6 rows of "CI gates and shipped-path lists": the "one-pipeline adapter tests" step of `scripts/ci-local.sh`, the "pipeline tests" step of `ci.yml` and the toolchain provisioning only it used, the two `^TestPipeline` entries of `localCIChecks`, and `TestCIPipelineStepRunsUnderPipefail`.

- [ ] **Step 6: Fence the bench tests that drive a v0.2 binary** — create `bench/tests/v02binary.py` exactly as in "The bench replay harness", mark the three tests with `@requires_v02_rtdd`, and run the bench suite against this branch's binary:

```bash
bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && (cd bench && PATH="$bin:$PATH" uv run pytest -q)
```

Expected: the three are reported SKIPPED with the #412 reason, everything else passes. Comment on #412 naming the fenced tests.

- [ ] **Step 7: Run to verify**

```bash
go build ./... && go vet ./... && go test ./cmd/rtdd/ ./internal/contract/ -count=1 2>&1 | grep -E '^(--- FAIL|ok|FAIL)'
```

Expected: `ok` for both packages.

- [ ] **Step 8: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0, and no `-list` guard in it names a test that no longer exists.

- [ ] **Step 9: Commit** (list the deleted tests in the body)

```bash
git add -A cmd/rtdd internal/contract scripts .github bench/tests
git commit -m "feat(cli): seed, run, verify, status and map compact are removed; invoking one exits 2 naming v0.3.0 (closes #451)"
```

---

### Task 7: The coverage pipeline is deleted

**Issue:** #452

**Discharges:** PRD #410 AC5 (packages, `adapters/`, v0.2 state), AC9 (the shipped-adapter guard, `trackedDirs`). Spec §8 ("Removed").

**Files:**
- Delete: `internal/adapter/`, `internal/covfmt/`, `internal/runner/`, `internal/mapstore/`, `internal/selector/`, `internal/uncovered/`, `adapters/`, `internal/coverage/`, `internal/doctor/`, `internal/pytestfixture/`
- Delete (`cmd/rtdd`): `polyglot.go`, `rows.go`, `meta.go`, `escalate.go`, `distance.go`, `requires.go`, `uncoveredtext.go`, `jsonout.go`, `whichtext.go`, `v02.go`, `v02_test.go`, their tests, and `testdata/{map.jsonl,adapter.yaml,fixtures/}` once nothing reads them
- Modify: `cmd/rtdd/main.go` (`env`, `loadEnv` and its methods go), `cmd/rtdd/init.go` (no detection, no refusal, `findRepoRoot` moved in), `cmd/rtdd/changed.go` (comment), `cmd/rtdd/changed_test.go`, `cmd/rtdd/init_test.go`, `cmd/rtdd/init_gate_test.go`
- Modify: `internal/install/install.go`, `internal/install/uninstall.go`; delete `internal/install/caveat_test.go`
- Modify: `internal/contract/contract_test.go` (prune), `internal/contract/nomock_test.go`; delete `internal/contract/shipped_adapters_test.go`
- Modify: `scripts/ci-local.sh`, `.github/workflows/ci.yml`, `internal/installtest/release_snapshot_test.go`, `.gitattributes`
- Test: `internal/contract/pipeline_removed_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `TestCoveragePipelinePackagesAreGone`, `TestNoGoCodeReferencesTheCoveragePipeline` (`removedByN2`, `v02State`, `v02StateAllowed`, `goFiles`); `install.Plan` writes no `.gitattributes` step; `rtdd init` succeeds in any git repository and prints no adapter line.

- [ ] **Step 1: Write the failing guard** — `internal/contract/pipeline_removed_test.go`:

```go
package contract

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// removedByN2 is every package PRD #410 deletes (docs/plans/10-rounds-cutover.md,
// "Deletion order"): the six the spec names, adapters/, and the three their deletion
// leaves with no importer.
var removedByN2 = []string{
	"internal/adapter", "internal/covfmt", "internal/runner", "internal/mapstore",
	"internal/selector", "internal/uncovered",
	"adapters",
	"internal/coverage", "internal/doctor", "internal/pytestfixture",
}

// v02State are the paths of the v0.2 coverage pipeline's state in a host repository.
var v02State = []string{".rtdd/map.jsonl", ".rtdd/meta.json", ".rtdd/adapters/"}

// v02StateAllowed are the files that may still spell a v0.2 state path, each because
// removing v0.2 state from a host repository or rewriting the front-ends is PRD #411's.
// An entry that no longer spells one is an error: the list only shrinks.
var v02StateAllowed = map[string]string{
	"internal/protocol/targets.go":               "front-end descriptions; PRD #411 rewrites them",
	"internal/protocol/global_test.go":           "pins those descriptions; PRD #411 rewrites them",
	"internal/install/uninstall.go":              "removes the v0.2 merge-driver line from a host repository",
	"internal/install/uninstall_test.go":         "proves uninstall leaves v0.2 state it does not own",
	"internal/contract/pipeline_removed_test.go": "this guard",
}

// goFiles is every .go file in the module, repo-relative and slash-separated.
func goFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "build", "work", ".harness":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// PRD #410 AC5: the packages are gone.
func TestCoveragePipelinePackagesAreGone(t *testing.T) {
	root := repoRoot(t)
	for _, p := range removedByN2 {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s still exists", p)
		}
	}
}

// PRD #410 AC5: no Go file — test files included — imports a removed package, and none
// outside v02StateAllowed reads, writes or names .rtdd/map.jsonl, .rtdd/meta.json or
// .rtdd/adapters/.
func TestNoGoCodeReferencesTheCoveragePipeline(t *testing.T) {
	root := repoRoot(t)
	spelled := map[string]bool{}
	for _, rel := range goFiles(t) {
		src, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(token.NewFileSet(), rel, src, parser.ImportsOnly)
		if err != nil {
			continue // testdata fixtures need not parse
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, gone := range removedByN2 {
				if p == "github.com/VocanicZ/rtdd/"+gone || strings.HasPrefix(p, "github.com/VocanicZ/rtdd/"+gone+"/") {
					t.Errorf("%s imports the removed package %s", rel, p)
				}
			}
		}
		for _, s := range v02State {
			if !strings.Contains(string(src), s) {
				continue
			}
			spelled[rel] = true
			if _, ok := v02StateAllowed[rel]; !ok {
				t.Errorf("%s still names %s", rel, s)
			}
		}
	}
	for rel, why := range v02StateAllowed {
		if !spelled[rel] {
			t.Errorf("%s is allowed to name v0.2 state (%s) but no longer does: remove it from v02StateAllowed", rel, why)
		}
	}
}
```

and, in `cmd/rtdd/init_gate_test.go`, the test that replaces the refusal tests (it uses the file's existing `newUnsupportedRepo`, a repository no v0.2 adapter matches):

```go
// PRD #410 AC5 / #452: init no longer detects adapters and no longer refuses: every git
// repository is supported (spec §8).
func TestInitSucceedsInARepositoryNoAdapterWouldHaveMatched(t *testing.T) {
	dir := newUnsupportedRepo(t)
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, stderr %q", code, errOut)
	}
	for _, banned := range []string{"adapter", "--force"} {
		if strings.Contains(out+errOut, banned) {
			t.Errorf("rtdd init still talks about %q:\n%s%s", banned, out, errOut)
		}
	}
	if strings.Contains(out, ".gitattributes") {
		t.Errorf("rtdd init still writes a merge driver for a map nothing writes:\n%s", out)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/contract/ ./cmd/rtdd/ -count=1 -run 'TestCoveragePipelinePackagesAreGone|TestNoGoCodeReferencesTheCoveragePipeline|TestInitSucceedsInARepositoryNoAdapterWouldHaveMatched'
```

Expected: FAIL — `internal/adapter still exists` … `internal/pytestfixture still exists` (ten lines); `… imports the removed package github.com/VocanicZ/rtdd/internal/…` and `… still names .rtdd/map.jsonl` for every file of "Deletion order" step 3; `rtdd init` exits 2 with `no adapter detected`.

- [ ] **Step 3: `cmd/rtdd`** — "Deletion order" step 3.1. Commit when `go build ./... && go vet ./...` pass and `go test ./cmd/rtdd/` is `ok`, with neither Step 1 test in the commit yet (`git add` everything but `internal/contract/pipeline_removed_test.go` and the new init test, which go in with Steps 7 and 4):

```bash
git commit -m "refactor(cli): no command imports the coverage pipeline any more (#452)"
```

- [ ] **Step 4: `internal/install`** — step 3.2. `init_test.go`'s assertions that the merge-driver line is written are inverted (it is **not** written); the refusal and adapter-record tests of `init_gate_test.go` (Task 7 row) go, and `TestInitSucceedsInARepositoryNoAdapterWouldHaveMatched` — added in this commit — now passes. Commit:

```bash
git commit -m "refactor(init): rtdd init detects no adapter and refuses no repository (#452)"
```

- [ ] **Step 5: `internal/contract`** — step 3.3 (prune `contract_test.go`; never delete it). Commit:

```bash
git commit -m "test(contract): drop the contracts of the deleted coverage pipeline (#452)"
```

- [ ] **Step 6: the packages** — step 3.4. `go build ./... && go vet ./... && go mod tidy && git diff --exit-code go.mod go.sum`. Commit:

```bash
git commit -m "refactor: delete adapter, covfmt, runner, mapstore, selector, uncovered, coverage, doctor, pytestfixture and adapters/ (#452)"
```

- [ ] **Step 7: CI and shipped paths** — the Task 7 rows of "CI gates and shipped-path lists"; `.gitattributes` loses `.rtdd/map.jsonl merge=union`. Add `internal/contract/pipeline_removed_test.go` to this commit.

- [ ] **Step 8: Run to verify**

```bash
go build ./... && go vet ./... && go test ./... -count=1 2>&1 | grep -E '^(--- FAIL|FAIL)'
```

Expected: no output. `go list -deps ./cmd/rtdd | grep -E 'internal/(adapter|covfmt|runner|mapstore|selector|uncovered|coverage|doctor)'` prints nothing.

- [ ] **Step 9: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0; every `-list` guard it runs names a test that exists; the release snapshot gates pass with no `adapters` in `trackedDirs`.

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat: the coverage pipeline is deleted: adapter, covfmt, runner, mapstore, selector, uncovered, adapters/ and the map/meta files (closes #452)"
```

---

### Task 8: End to end — editing one function in a Python + Go repository selects its tests, then its neighbours' tests

**Issue:** #453

**Discharges:** PRD #410 AC7. Spec §7, §12 (both directions).

**Files:**
- Test: `cmd/rtdd/e2e_rounds_test.go`

**Interfaces:**
- Consumes: `tripwirePATH` (Task 2); `rtdd which --json` schema 3 (Task 3); `rtdd which` text (Task 2).
- Produces: `e2eFiles`, `e2eRepo`, `edit`, `whichJSONDoc` (test-only).

This task adds proof, not behaviour: on a correct Tasks 1–3 the test passes as soon as it compiles. Step 2 therefore proves it can fail, by running it against a deliberately wrong `Rounds`.

- [ ] **Step 1: Write the test** — `cmd/rtdd/e2e_rounds_test.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// e2eFiles is a Python project and a Go module side by side. In each: parse is the
// function the cases edit; normalize is its callee and charge its caller, each with its
// own test; unrelated has a test that must appear in neither round; orphan has no test
// and no caller. Names differ between the languages so no call crosses them.
var e2eFiles = map[string]string{
	"pyapp/money.py": "def normalize(s):\n    return s.strip()\n\n\n" +
		"def parse_amount(s):\n    return int(normalize(s))\n\n\n" +
		"def unrelated_py():\n    return 1\n\n\n" +
		"def orphan_py():\n    return 2\n",
	"pyapp/billing.py": "from pyapp.money import parse_amount\n\n\ndef charge(s):\n    return parse_amount(s) * 2\n",
	"tests/test_money.py": "from pyapp.money import normalize, parse_amount, unrelated_py\n\n\n" +
		"def test_normalize():\n    assert normalize(\" 1 \") == \"1\"\n\n\n" +
		"def test_parse_amount():\n    assert parse_amount(\"3\") == 3\n\n\n" +
		"def test_unrelated_py():\n    assert unrelated_py() == 1\n",
	"tests/test_billing.py": "from pyapp.billing import charge\n\n\ndef test_charge():\n    assert charge(\"2\") == 4\n",
	"go.mod":                "module example.com/e2e\n\ngo 1.24\n",
	"goapp/money.go": "package goapp\n\nimport \"strings\"\n\n" +
		"func Normalize(s string) string {\n\treturn strings.TrimSpace(s)\n}\n\n" +
		"func ParseAmount(s string) string {\n\treturn Normalize(s)\n}\n\n" +
		"func Unrelated() int {\n\treturn 1\n}\n\n" +
		"func Orphan() int {\n\treturn 2\n}\n",
	"goapp/billing.go": "package goapp\n\nfunc Charge(s string) string {\n\treturn ParseAmount(s) + ParseAmount(s)\n}\n",
	"goapp/money_test.go": "package goapp\n\nimport \"testing\"\n\n" +
		"func TestNormalize(t *testing.T) {\n\tif Normalize(\" 1\") != \"1\" {\n\t\tt.Fatal(\"Normalize\")\n\t}\n}\n\n" +
		"func TestParseAmount(t *testing.T) {\n\tif ParseAmount(\"1\") != \"1\" {\n\t\tt.Fatal(\"ParseAmount\")\n\t}\n}\n\n" +
		"func TestUnrelated(t *testing.T) {\n\tif Unrelated() != 1 {\n\t\tt.Fatal(\"Unrelated\")\n\t}\n}\n",
	"goapp/billing_test.go": "package goapp\n\nimport \"testing\"\n\n" +
		"func TestCharge(t *testing.T) {\n\tif Charge(\"1\") != \"11\" {\n\t\tt.Fatal(\"Charge\")\n\t}\n}\n",
}

func e2eRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	for p, src := range e2eFiles {
		gittest.Write(t, dir, p, src)
	}
	gittest.Commit(t, dir, "python and go")
	return dir
}

// edit rewrites one line of a committed file, by exact text.
func edit(t *testing.T, dir, path, old, new string) {
	t.Helper()
	src := e2eFiles[path]
	if !strings.Contains(src, old) {
		t.Fatalf("%s has no %q", path, old)
	}
	gittest.Write(t, dir, path, strings.Replace(src, old, new, 1))
}

type e2eRound struct {
	Round int `json:"round"`
	Tests []struct {
		ID string `json:"id"`
	} `json:"tests"`
}

type e2eDoc struct {
	ChangedNodes []struct {
		ID string `json:"id"`
	} `json:"changed_nodes"`
	Rounds   []e2eRound `json:"rounds"`
	Untested []string   `json:"untested"`
}

func whichJSONDoc(t *testing.T, dir string) e2eDoc {
	t.Helper()
	code, out, errOut := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("rtdd which --json = %d, stderr %q", code, errOut)
	}
	var doc e2eDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return doc
}

func roundIDs(r e2eRound) []string {
	out := []string{}
	for _, x := range r.Tests {
		out = append(out, x.ID)
	}
	return out
}

// PRD #410 AC7: on a real git repository holding Python and Go — and with no toolchain
// run — editing one function selects exactly its tests in Round 1 and exactly the tests
// of its caller and callee in Round 2, in file-then-line order.
func TestEditingOneFunctionSelectsItsTestsThenItsNeighboursTests(t *testing.T) {
	for _, c := range []struct {
		lang, path, old, new string
		changed              []string
		round1, round2       []string
	}{
		{"python", "pyapp/money.py", "return int(normalize(s))", "return int(normalize(s)) + 0",
			[]string{"pyapp/money.py::parse_amount"},
			[]string{"tests/test_money.py::test_parse_amount"},
			[]string{"tests/test_billing.py::test_charge", "tests/test_money.py::test_normalize"}},
		{"go", "goapp/money.go", "return Normalize(s)\n", "return Normalize(s + \"\")\n",
			[]string{"goapp/money.go::ParseAmount"},
			[]string{"goapp/money_test.go::TestParseAmount"},
			[]string{"goapp/billing_test.go::TestCharge", "goapp/money_test.go::TestNormalize"}},
	} {
		t.Run(c.lang, func(t *testing.T) {
			dir := e2eRepo(t)
			marker := tripwirePATH(t)
			edit(t, dir, c.path, c.old, c.new)
			doc := whichJSONDoc(t, dir)
			var changed []string
			for _, n := range doc.ChangedNodes {
				changed = append(changed, n.ID)
			}
			if !reflect.DeepEqual(changed, c.changed) {
				t.Errorf("changed_nodes = %v, want %v", changed, c.changed)
			}
			if got := roundIDs(doc.Rounds[0]); !reflect.DeepEqual(got, c.round1) {
				t.Errorf("round 1 = %v, want exactly %v", got, c.round1)
			}
			if got := roundIDs(doc.Rounds[1]); !reflect.DeepEqual(got, c.round2) {
				t.Errorf("round 2 = %v, want exactly %v", got, c.round2)
			}
			if len(doc.Untested) != 0 {
				t.Errorf("untested = %v, want none", doc.Untested)
			}
			if b, err := os.ReadFile(marker); err == nil {
				t.Errorf("a toolchain ran: %s", b)
			}
		})
	}
}

// PRD #410 AC7, second case: a function nothing links to is untested, and the text says
// "no linked test" for both rounds.
func TestEditingAnUnlinkedFunctionIsUntestedAndHasNoLinkedTest(t *testing.T) {
	dir := e2eRepo(t)
	marker := tripwirePATH(t)
	edit(t, dir, "pyapp/money.py", "def orphan_py():\n    return 2\n", "def orphan_py():\n    return 3\n")
	edit(t, dir, "goapp/money.go", "func Orphan() int {\n\treturn 2\n}\n", "func Orphan() int {\n\treturn 3\n}\n")
	doc := whichJSONDoc(t, dir)
	if want := []string{"goapp/money.go::Orphan", "pyapp/money.py::orphan_py"}; !reflect.DeepEqual(doc.Untested, want) {
		t.Errorf("untested = %v, want %v", doc.Untested, want)
	}
	_, out, _ := rtdd(t, dir, "which")
	if strings.Count(out, "  no linked test\n") != 2 {
		t.Errorf("text output does not say \"no linked test\" for Rounds 1 and 2:\n%s", out)
	}
	if b, err := os.ReadFile(marker); err == nil {
		t.Errorf("a toolchain ran: %s", b)
	}
}
```

- [ ] **Step 2: Prove it fails on a callee-only `Rounds`.** Temporarily delete, in `internal/rounds/rounds.go`'s neighbour loop, the `for _, e := range in(id)` block (the caller direction), then:

```bash
go test ./cmd/rtdd/ -count=1 -run 'TestEditingOneFunctionSelectsItsTestsThenItsNeighboursTests'
```

Expected: FAIL — `python: round 2 = [tests/test_money.py::test_normalize], want exactly [tests/test_billing.py::test_charge tests/test_money.py::test_normalize]` and `go: round 2 = [goapp/money_test.go::TestNormalize], want exactly [goapp/billing_test.go::TestCharge goapp/money_test.go::TestNormalize]`. Restore `rounds.go` (`git checkout internal/rounds/rounds.go`).

- [ ] **Step 3: Run to verify it passes**

```bash
go test ./cmd/rtdd/ -count=1 -v -run 'TestEditingOneFunctionSelectsItsTestsThenItsNeighboursTests|TestEditingAnUnlinkedFunctionIsUntestedAndHasNoLinkedTest'
```

Expected: PASS, both subtests of the first. If it fails on a correct-looking `Rounds`, the defect is in Tasks 1–3's code: fix it there, in this branch, with this test as the failing test — never by loosening an exact-set assertion.

- [ ] **Step 4: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0.

- [ ] **Step 5: Commit**

```bash
git add cmd/rtdd/e2e_rounds_test.go
git commit -m "test(which): end to end, editing one function in a Python+Go temp repo selects exactly its callers' tests in Round 1 and neighbours' tests in Round 2 (closes #453)"
```

---

### Task 9: `rtdd which` on this repository with a warm graph cache finishes in under 1 s

**Issue:** #454

**Discharges:** PRD #410 AC8. Spec §1 (success, measured).

**Files:**
- Test: `cmd/rtdd/which_perf_test.go`
- Modify (only if the budget is exceeded): whatever the profile names — `internal/rounds/rounds.go`, `cmd/rtdd/which.go`, `internal/graphbuild`

**Interfaces:**
- Consumes: `rtdd which` (Task 2); `rounds.Rounds`; `graphbuild.Build`, `graph.DefaultConfig`; `gittest.Run`.
- Produces: `thisRepoWorktree` (test-only), `TestWhichOnThisRepositoryWithAWarmCacheIsUnder1s`, `BenchmarkRoundsOnThisRepository`.

Measured when this plan was written, on the prototype of Tasks 1–3: warm `rtdd which` on this repository **444 ms**; `Rounds` on its graph (5 510 nodes, 62 161 edges) **54 ms/op**; a cold `rtdd graph` **2.1 s**. The test follows PRD #409's warm-graph test (`internal/graphbuild/perf_test.go`): it runs inside `go test ./...`, so no `-list` guard is needed. It measures in a temporary `git worktree` of HEAD with one edit, so the checkout is never written and the changed set is real.

- [ ] **Step 1: Write the test** — `cmd/rtdd/which_perf_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// thisRepoWorktree checks this repository's HEAD out into a temporary worktree with one
// small edit, so the measurement has a real changed set without touching the checkout.
func thisRepoWorktree(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	gittest.Run(t, root, "worktree", "add", "--detach", "--quiet", wt, "HEAD")
	t.Cleanup(func() { gittest.Run(t, root, "worktree", "remove", "--force", wt) })
	p := filepath.Join(wt, "internal", "graph", "graph.go")
	src, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(src, "\n// a small working-tree edit\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	return wt
}

// PRD #410 AC8: `rtdd which` on this repository, warm cache, small edit: under 1 s.
func TestWhichOnThisRepositoryWithAWarmCacheIsUnder1s(t *testing.T) {
	wt := thisRepoWorktree(t)
	if code, _, errOut := rtdd(t, wt, "which"); code != 0 { // cold: fills .rtdd/graph.json
		t.Fatalf("cold rtdd which = %d, stderr %q", code, errOut)
	}
	start := time.Now()
	code, out, errOut := rtdd(t, wt, "which")
	took := time.Since(start)
	if code != 0 {
		t.Fatalf("warm rtdd which = %d, stderr %q", code, errOut)
	}
	t.Logf("warm rtdd which: %v\n%s", took, out)
	if took >= time.Second {
		t.Errorf("warm rtdd which took %v, want < 1s", took)
	}
}

// PRD #410 AC8: what Rounds alone costs on this repository's graph. Run with
//
//	go test ./cmd/rtdd/ -run '^$' -bench '^BenchmarkRoundsOnThisRepository$'
func BenchmarkRoundsOnThisRepository(b *testing.B) {
	wd, _ := os.Getwd()
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		b.Skipf("not in a git checkout: %v", err)
	}
	res, err := graphbuild.Build(root, graph.DefaultConfig(), graphbuild.Options{CachePath: filepath.Join(b.TempDir(), "graph.json")})
	if err != nil {
		b.Fatal(err)
	}
	changed := map[string][]rounds.LineRange{"internal/graph/graph.go": {{Start: 1, End: 200}}}
	b.Logf("graph: %d nodes, %d edges", len(res.Graph.Nodes), len(res.Graph.Edges))
	for b.Loop() {
		rounds.Rounds(res.Graph, changed)
	}
}
```

- [ ] **Step 2: Prove it fails without the warm cache.** Temporarily make `buildGraph` (in `cmd/rtdd/graphenv.go`) pass `graphbuild.Options{CachePath: filepath.Join(os.TempDir(), fmt.Sprintf("rtdd-%d.json", time.Now().UnixNano()))}`, so every `which` is cold, then:

```bash
go test ./cmd/rtdd/ -count=1 -v -run '^TestWhichOnThisRepositoryWithAWarmCacheIsUnder1s$'
```

Expected: FAIL — `warm rtdd which took 2.…s, want < 1s` (a cold build of this repository). Restore `graphenv.go`.

- [ ] **Step 3: Run to verify it passes, and record `Rounds`' cost**

```bash
go test ./cmd/rtdd/ -count=1 -v -run '^TestWhichOnThisRepositoryWithAWarmCacheIsUnder1s$'
go test ./cmd/rtdd/ -run '^$' -bench '^BenchmarkRoundsOnThisRepository$'
```

Expected: PASS with the warm time logged; the benchmark's ns/op and the graph size go into the PR description. Over budget: profile (`-cpuprofile`), fix the cost the profile names without weakening a selection rule, and re-run — no selection test may change to make this pass.

- [ ] **Step 4: The gate** — `bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh` exits 0.

- [ ] **Step 5: Commit**

```bash
git add cmd/rtdd/which_perf_test.go
git commit -m "test(which): rtdd which on this repository with a warm graph cache finishes in under 1 s (closes #454)"
```

---

## What this plan deliberately leaves undone

Everything below is real work; none of it belongs to PRD #410, and no task above may start it.

- **The skill and the front-ends.** `protocol/PROTOCOL.md`, `dist/`, `internal/protocol` (including the skill descriptions that still say ".rtdd/map.jsonl", allowlisted by Task 7's guard), the five-step process of spec §10 — PRD #411.
- **`rtdd init` for v0.3.0.** The `.rtdd/config.yaml` it writes (`test_files`, `scan_exclude`, `graphify_path`, `max_stale_ratio`), the `.gitignore` line for `.rtdd/graph.json`, removing or no-op'ing `--force`, and migrating a v0.2 repository (deleting `map.jsonl`, `meta.json`, `.rtdd/adapters/` and the merge-driver line, saying so) — PRD #411. Task 7 only stops init detecting adapters, refusing, and writing a new merge-driver line; `uninstall` still removes an old one.
- **README, DEVELOPMENT.md, `docs/LIMITATIONS.md`, `docs/outcomes/`, superseded specs** — PRD #411. `docs/plans/00-interfaces.md` and the older plans are history and are not edited; the contract tests that compared them with deleted code go with the code.
- **The bench.** `bench/replay`'s rtdd strategy, its schema-3 consumer, the Round 1 / Rounds 1+2 arms and the published results — PRD #412. This plan only fences the three bench tests that drive a v0.2 binary, and #412 removes the fence.
- **Depth beyond 1, weights, pruning over-linked names, or any per-language rule in `Rounds`** (spec §11, §12). `doctor` reports hubs; nothing acts on them.
- **Selecting callers of a deleted file.** A deleted file has no lines left to own a node, so its callers are not in Round 1 or 2; the warning says so and Round 3 covers them. Re-pointing a deleted file's callers would need the graph *before* the change — a graph of `<base>`, which rtdd does not build.
- **Running anything.** No round is executed, no outcome collected, no pass/fail reported, no coverage read (spec §11).
- **Spec §12's defaults.** Both directions, link-all, the gitignored cache, never running graphify. A lane that finds one wrong files an issue against the spec; it does not change it here.
- **Tagging and releasing v0.3.0** — a human action (#418).
