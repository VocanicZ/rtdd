# rtdd

**rtdd tells a coding agent which tests a change needs, in rounds.** Running every test
after every edit is slow; running only the test named after the file you touched misses the
code that calls it. rtdd is a skill — a testing process the agent follows on any codebase —
and a small binary that answers one question fast: which tests does this change need, and
in what order.

## Install

```
npx github:VocanicZ/rtdd
```

One command, the same on Linux, macOS and Windows. It needs Node 18 or newer — `npx` is what
makes a single line work in bash, PowerShell and `cmd` alike, where no shell script can:
stock Windows has no POSIX `sh`, and Linux and macOS have no PowerShell.

It downloads the release binary for your platform, checks it against the published
`checksums.txt`, puts it on your PATH, and installs the agent skill below. `--version=<tag>`
pins a release, `--install-dir=<path>` chooses where the binary goes.

Or build from source, with Go 1.24 or newer, if you would rather not involve Node:

```
git clone https://github.com/VocanicZ/rtdd && cd rtdd
go build ./cmd/rtdd
```

The installer also installs a **machine-wide agent skill**, so your coding agent knows rtdd
exists in every repository — including ones where rtdd has not been set up yet, where the
skill tells it to run `rtdd init` first. It is written only for agents you already have: it
lands in `~/.claude/skills/rtdd/`, and as a marker-delimited block in `~/.codex/AGENTS.md`
and `~/.gemini/GEMINI.md`, and any of those directories that does not exist is skipped
rather than created. Pass `--no-skill`, or set `RTDD_NO_SKILL=1`, to install the binary
alone.

For any other agent — Cursor, whose user rules live in its settings rather than in a file,
or anything else — `rtdd skill prompt` prints a self-contained document to hand it.

```
rtdd skill install     # (re)install the machine-wide front-ends
rtdd skill prompt      # print a copy to paste into any other agent
rtdd skill uninstall   # remove them
```

`rtdd update` replaces the binary with the latest release, and only if it is newer, then
refreshes the machine-wide skill to match; `rtdd update --check` asks without installing.
`rtdd uninstall` removes what `rtdd init` wrote into a repository, leaving `.rtdd/` unless
you add `--state`, and the machine-wide front-ends unless you add `--global`.

## Usage

```
cd your-repo
rtdd init      # front-ends, .rtdd/config.yaml, .gitignore line for the graph cache
rtdd which     # the tests your current changes need, in rounds
```

`rtdd init` sets up any git repository, whatever it is written in. It writes the agent
instructions — a Claude Code skill at `.claude/skills/rtdd/SKILL.md`, a Cursor rule at
`.cursor/rules/rtdd.mdc`, and a marker-delimited block in `AGENTS.md`, and in `CLAUDE.md`
when the repository has one — a `.rtdd/config.yaml` of defaults, and a `.gitignore` line
for `.rtdd/graph.json`, rtdd's rebuildable graph cache. It never edits outside its own
markers and never overwrites a config you have set. There is no other setup step: the graph
is built the first time a command needs it. Commit what `init` wrote.

On a repository set up by v0.2, `rtdd init` also deletes the state that release kept — its
map, its metadata, its host language definitions and its `.gitattributes` merge line — and
prints one line for each file it removes. A config v0.2 wrote is replaced by v0.3.0's
defaults.

## The process

The skill `rtdd init` writes tells the agent to work in five steps:

1. Edit code (test first, per TDD).
2. Run `rtdd which`. If `untested` names a node you changed, write its test first.
3. Run **Round 1** with the project's own test command. Fix until green.
4. Run **Round 2**. Fix until green; return to step 2 after any further edit.
5. When the task is done — before committing or handing off — run the **full suite once**.

rtdd runs no tests. Each round runs under the project's own test command, the one you would
use without rtdd, and nothing rtdd prints is a pass or a fail.

## How it picks

rtdd builds a graph of the repository's functions, methods and classes, links every test to
the code it calls, and reads the lines you changed against it:

- **Round 1** — the tests linked to the code you changed, and every test you changed.
- **Round 2** — the tests linked to that code's direct neighbours, its callers and its
  callees, minus Round 1.
- **Round 3** — the full suite, once, at the end of the task.

Take `src/calc.py`, where `total` calls `add`, and `tests/test_calc.py` with `test_add` and
`test_total`. Edit `add`, and:

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

`test_add` calls the function you changed, so it is Round 1. `test_total` never names `add`,
but it calls `total`, which does — so it is Round 2. A path heuristic would stop at the first.

The changed set is everything that differs from `--base` (default `HEAD`): committed since
it, staged, unstaged and untracked, so a file you just wrote counts before you commit it. A
changed line outside every function, such as an import, belongs to its file's `<module>`
node. `untested` lists the changed nodes no test in Round 1 or Round 2 reaches.

When Rounds 1 and 2 are both empty, `rtdd which` prints `no linked test`. That is exactly
what it means — nothing links a test to the code you changed — and never a pass. Round 3
still runs the full suite once at the end.

`rtdd which --json` is the same answer as one object, schema 3, for tools that read it.

## The other commands

```
rtdd graph [--json]                the graph's source, node, edge and test counts, staleness
rtdd explain <file[:line]|name>    a node's tests, callers and callees
rtdd doctor                        graph source, graphify staleness, test files found, names defined 8+ times
rtdd uninstall [--state]           remove what init wrote
rtdd update                        replace the binary with the latest release
rtdd --version                     print the version
```

Exit codes: 0 success, empty rounds included; 2 usage; 3 environment (not a git repository,
a graph that cannot be built). No exit code is a test result, because rtdd runs none.
It is a context provider, not a gate.

## graphify is optional

rtdd's own scanner builds the graph from the text of the source, and needs nothing
installed: no language toolchain, no test framework, no coverage tool. If the repository
has a graphify graph at `graphify-out/graph.json`
(`graphify_path` in `.rtdd/config.yaml` moves it), rtdd uses it as well. It never trusts it
for changed files — rtdd rescans every file that changed since graphify built its graph,
and ignores the graph entirely, saying so, when more than half of it is stale. rtdd never
runs graphify: if you want its graph, run or update graphify yourself.

## What it cannot see

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

More in [Limitations](docs/LIMITATIONS.md).

## Does it work?

<!-- rtdd:v0.2-record -->

The measurements below were taken on v0.2.0's coverage selector, which v0.3.0 replaces;
v0.3.0's rounds are measured by [rtdd-bench](https://github.com/VocanicZ/rtdd-bench) (PRD #412).

RTDD is built for one loop: an agent edits, runs tests, edits again, dozens of times in a
single task. Without it the agent runs the whole suite every time. So there are two
questions — does RTDD save time, and does it miss anything the full suite would catch.

**Every number in this section was measured on the release before the one-pipeline change**
(per-test-case Python coverage): the time table, 4 of 4, the 0.757 fraction, the 12 uncovered
reports with 0 wrong, 486 tests with 16 selected, and 167 ms for `rtdd which`. A first
mechanical measurement on the one pipeline is in
[one-pipeline-first-measure.md](docs/results/one-pipeline-first-measure.md): Go now selects
from a recorded map, and on those small suites `rtdd run` is slower than the full suite. The
agent-session re-measurement is pending.

### It saves time

Ten edits to one module in a real `flask` clone, tests run after each edit. Same machine,
same edits, two copies of the same repository:

| after every change | 10 edits | |
|---|---|---|
| run the whole suite | 28390 ms | |
| `rtdd run` | 19962 ms | **1.42× faster** |

486 tests in the suite, 16 selected. Reproduce with
[`scripts/agent-session-bench.sh`](scripts/agent-session-bench.sh) against a seeded clone.

RTDD's own cost is inside those numbers. `rtdd which` answers in **167 ms** on this clone,
down from **9774 ms** before its staleness scan was memoised — which was slower than just
running all 486 tests.

The edits above are additive no-ops, so nothing fails in any row. This measures cost only.

### It catches what the full suite catches

Testing that needs commits that actually break something: replay a commit's tests against
its parent's source and watch them fail. Most commits can't — they change only source, or
only tests, or only docs. Picking commits that changed both is what makes the question
answerable at all:

| flask commits replayed | that detect anything |
|---|---|
| 23 most recent | 3 |
| 6 that changed tests and source together | **5** |

Each one is a real commit whose real tests really failed against its parent's real source.
Nothing is injected or mutated. From
[`bench/results/paired/flask/summary.md`](bench/results/paired/flask/summary.md):

| | caught the change | where one test fails | share of suite time |
|---|---|---|---|
| run the whole suite | 1.000 (5/5) | 1.000 (4/4) | 1.000 |
| **rtdd** | **1.000 (5/5)** | **1.000 (4/4)** | **0.757** |

The middle column is the one to read. Those are the changes where exactly one test stands
between you and a missed regression, and RTDD ran all four of them. Matching the full suite
is the best result available here — a full run catches what it catches by definition.

**How far this goes.** Five commits, one repository. They were chosen for changing tests
and source together, and RTDD's map was seeded at the commit under test, so it knew about
code an agent would not have recorded yet. Enough to measure. Not enough to generalise —
more repositories are what would change that.

RTDD also reports which of your changed lines no test covers. On the published corpus that
report fired 12 times and was wrong zero times. Running the whole suite tells you nothing
about this.

<!-- /rtdd:v0.2-record -->

## Documentation

- [Limitations](docs/LIMITATIONS.md) — where the rounds can be wrong, and why Round 3 exists.
- [Design](docs/specs/2026-10-07-node-graph.md) — the v0.3.0 specification: the graph, the
  scanner, graphify loading, the rounds and the commands. The earlier specs it supersedes
  are kept under [`docs/specs/`](docs/specs/) as the record of what was built before.
- [Prior art](docs/PRIOR-ART.md) — pytest-testmon, TDAD, and the rest of the field RTDD
  builds on, with what each of them already does better.
- [Baseline comparison](docs/results/axis2-selection-baselines.md) — v0.2's selector
  against pytest-testmon, a path heuristic and four other selectors, with wall-clock
  distributions per repository. RTDD does not win this one.
- [Design audit](docs/audits/2026-08-26-design-audit.md) — what was measured, and what it
  killed.
- [Agent protocol](protocol/PROTOCOL.md) — the single source for every generated front-end
  under `dist/`. Edit it, run `rtdd-gen render`; CI fails if `dist/` is stale or if any
  front-end is wrong for its target.
- [Full benchmark result](https://github.com/VocanicZ/rtdd-bench) comparion tdd vs rtdd on real project using go/python

## License

MIT. See [LICENSE](LICENSE).

## Star History

<a href="https://www.star-history.com/?repos=vocanicz%2Frtdd&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=vocanicz/rtdd&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=vocanicz/rtdd&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=vocanicz/rtdd&type=date&legend=top-left" />
 </picture>
</a>
