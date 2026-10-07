# RTDD agent protocol

This file is the single source for every generated agent front-end. Edit it, then run
`rtdd-gen render`. Do not edit anything under `dist/`.

<!-- rtdd:meta version=1 -->

<!-- rtdd:section id=setup title="Setting up a repository" targets=global,global-agents order=5 -->
Check for `.rtdd/map.jsonl` before anything else. It decides which of two things you are doing.

**The repository has one.** rtdd is set up. Use `rtdd which` and `rtdd run` as described
below, and do not re-run `rtdd init`.

**The repository has none.** rtdd cannot select anything yet — there is no recorded coverage
to select from, so every answer would be "run the full suite". Set it up once:

```
rtdd init      # writes the front-ends, the merge driver and .rtdd/config.yaml
rtdd seed      # one full instrumented run that builds .rtdd/map.jsonl
```

Then commit `.rtdd/map.jsonl` along with the `.gitattributes` line `rtdd init` added. The map
is the expensive artifact: seeding costs one full suite run, and committing it is what stops
every clone of the repository from paying that cost again.

**`rtdd init` exited 2.** That is the no-adapter refusal, not a failure to install. It means
nothing in the repository matched a toolchain rtdd knows how to instrument, so it declined to
install instructions promising a selection it could not make. Its message names what it found.
Detection keys on a config file, not on a language: a JavaScript repository configuring Jest
inside `package.json` rather than a `jest.config.*` file matches nothing, and so does a Vitest
project configured inside `vite.config.ts`. Two ways forward:

- Add the config file the adapter detects, or write `.rtdd/adapters/<language>.yaml` for the
  toolchain this repository actually uses, then re-run `rtdd init`.
- `rtdd init --force` installs anyway. The front-ends it writes then carry a caveat saying
  selection is unavailable, because until an adapter matches, it is.

Run `rtdd doctor` at any point to see which adapters matched and what they need installed.
<!-- rtdd:variant target=global-agents -->
Check for `.rtdd/map.jsonl` first. If it exists, rtdd is set up — use the commands below. If
it does not, run `rtdd init` then `rtdd seed` once, and commit the map. `rtdd init` exiting 2
is the no-adapter refusal: nothing matched a toolchain rtdd can instrument, so add the
adapter it names or re-run with `--force`.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=seed title="Before the first selection" targets=skill,agents,mdc order=7 -->
rtdd selects from `.rtdd/map.jsonl`. If that file does not exist, every selection is the full
suite (tier T2, reason "map is unseeded"), which is no faster than not using rtdd. Run
`rtdd seed` once — one instrumented run of the whole suite — and commit `.rtdd/map.jsonl`.
After that, `rtdd run` keeps the rows of the tests it runs fresh, so seed again only when a
T2 reason asks for it.
<!-- rtdd:variant target=agents -->
If `.rtdd/map.jsonl` does not exist, every selection is the full suite. Run `rtdd seed` once
and commit the map; `rtdd run` keeps it fresh after that.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=what title="What rtdd reports" targets=skill,agents,mdc,global,global-agents order=10 -->
`rtdd` reports which tests cover the code you just changed, and which of the lines you just
changed nothing covers. In every language rtdd supports, both come from coverage recorded by
running each test file in its own process under the language's stock coverage tool, not from
a static call graph, so the relation includes edges reached through dynamic dispatch,
dependency injection, plugin registries, and monkeypatching, and omits any path no test has
ever taken.

It reports. It does not gate, block, or fail anything on policy.
<!-- rtdd:variant target=agents -->
`rtdd` reports which tests cover code you changed, and which changed lines nothing covers,
from recorded coverage rather than a static graph. It reports; it never gates.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=which title="rtdd which" targets=skill,agents,mdc,global,global-agents order=20 -->
```
rtdd which [--base <ref>] [--json]
```

Prints the ranked selection for the current changed set and the uncovered report, and runs
nothing. It costs one map lookup and one `git diff`, so it is cheap to consult often.

The changed set is the union of `git diff --name-only <base>` and
`git status --porcelain -uall`, so untracked files count — a file you just wrote is in the
changed set before you commit it.

Ranking is by descending share of each test's recorded file set that your change touches,
then last-failed first, then smallest file set, then fastest.
<!-- rtdd:variant target=agents -->
`rtdd which` prints the ranked tests covering your current changes plus the uncovered
report, and runs nothing. Untracked files count, so a file you just wrote is included.
`rtdd which --json` is the machine-readable form.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=run title="rtdd run" targets=skill,agents,mdc,global,global-agents order=30 -->
```
rtdd run [--base <ref>] [--fail-fast] [--json]
```

Runs the selection, refreshes the map rows for the tests it executed, and prints the
uncovered report computed from fresh post-run coverage. It exits non-zero only when a test
fails. An empty selection and a non-empty uncovered report are both exit 0.

`--fail-fast` is opt-in and never implied.
<!-- rtdd:variant target=agents -->
`rtdd run` runs the selection and prints the uncovered report. Exit non-zero means a test
failed, and nothing else — an empty selection and an uncovered report are both exit 0.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=uncovered title="The uncovered report" targets=skill,agents,mdc,global,global-agents order=40 -->
The report classifies your changed lines into two classes:

- **covered** — a test file's own run executed these changed lines this cycle.
- **uncovered** — no test file's run executed them.

Each test file runs in its own process, so a line executed while importing a module is
executed by that test file and counts as covered.

An uncovered range is information about the suite, not a verdict on the patch.
<!-- rtdd:variant target=agents -->
The uncovered report splits changed lines into covered and uncovered: a line is covered
when some test file's own run executed it this cycle.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=empty title="An empty selection is not green" targets=skill,agents,mdc,global,global-agents order=50 -->
When nothing is selected, `rtdd` says so explicitly. An empty selection is a distinct
outcome from "all selected tests passed", because every under-selection path terminates
there. Treat it as "the map has nothing to say about this change", not as a pass.
<!-- rtdd:variant target=agents -->
An empty selection is reported as its own outcome, never as a pass.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=json title="JSON output" targets=skill,global order=60 -->
`--json` emits one object for programmatic consumption:

```json
{
  "schema": 2,
  "command": "which",
  "base": "HEAD",
  "adapter": "python",
  "tier": "T0",
  "reason": "tests whose recorded coverage intersects the changed set",
  "complete": true,
  "warnings": [],
  "changed": [{"path": "src/auth.py", "status": "modified", "instrumentable": true,
               "lines": [{"start": 52, "end": 58}]}],
  "selection": {"count": 2, "direct": ["tests/test_auth.py"],
                "tests": ["tests/test_auth.py", "tests/test_db.py"]},
  "run": {"executed": false, "passed": 0, "failed": 0, "skipped": 0, "errored": 0,
          "failures": [], "duration_ms": 0},
  "uncovered": {"available": false, "reason": "requires fresh post-run coverage; run `rtdd run`",
                "summary": {"files": 0, "covered_lines": 0, "uncovered_lines": 0}},
  "unmapped_files": [],
  "exit_code": 0
}
```

Reject any `schema` other than `2`. `tier` is one of `empty`, `direct`, `T0`, `T1`, `T2`. `T2`
means the full suite was selected because the map is unseeded, a dependency manifest changed,
the test-harness config changed, or the drift guard was reached; `reason` says which. `tests`
always lists the whole run, so `complete` is `true`; an empty selection carries a `warnings`
entry instead. In a repository with several adapters a `selections` array splits the ids per
adapter: take the ids for one runner from exactly one block. `uncovered` is `available` only
after `rtdd run`.
<!-- rtdd:endsection -->

<!-- rtdd:section id=commands title="The rest of the commands" targets=skill,global order=70 -->
```
rtdd status                  adapter, map freshness, seed state
rtdd seed                    one full instrumented run; the only op that may shrink a row
rtdd verify                  full suite
rtdd doctor                  fan-out / coupling report
rtdd explain <file>          which tests cover this file
rtdd map compact             collapse duplicate rows after a union merge
rtdd init                    install .gitattributes, config, and agent front-ends
```

`rtdd seed` is the only operation that may narrow a test file's recorded file set. Every
other path unions, because a failing test file records only a truncated prefix of its real path.
<!-- rtdd:endsection -->

<!-- rtdd:section id=limits title="What it cannot see" targets=skill,mdc,global order=80 -->
Stated plainly, because a selector that hides its blind spots is worse than no selector:

- Coverage only knows paths some test actually took. A branch nothing has ever exercised has
  no edge, and `rtdd which` will not find it.
- Each test file runs in its own process, so anything executed once per process —
  `@lru_cache`, module singletons, DI containers, session-scoped fixtures — is attributed to
  every test file that runs it. That inflates fan-out in `rtdd doctor`: code reached only
  through shared setup looks coupled to every test file.
- Selection is file-level on both sides. Changing one function in a file selects every test
  file that executed any part of that file, and the smallest unit selected is a test file
  (for Go, the package filtered to that file's own `Test` functions).
- The price is one process per test file, plus the coverage tool's overhead, on `rtdd seed`
  and on every `rtdd run`. It is small for Python, Go, and Node, and large where a process
  start is slow (the JVM under Maven or Gradle, .NET).
- Test files that depend on each other — through shared state on disk, ordering, or a port —
  may fail or pass differently when run one per process than in a full suite. The map only
  records what each did alone.
- A merge commit escalates, because a union-merged map cannot narrow relative to its parents
  but can still be stale relative to the merged code.
- `rtdd verify` is a convenience, not a substitute for CI.
<!-- rtdd:variant target=mdc -->
Coverage only knows paths some test actually took; a never-exercised branch has no edge.
Each test file runs in its own process, so once-per-process code (`@lru_cache`, singletons,
session fixtures) is attributed to every test file that runs it. Selection is file-level, and
the cost is one process per test file. `rtdd verify` does not replace CI.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=map title="The map file" targets=skill,global order=90 -->
`.rtdd/map.jsonl` is committed, sorted by test file, one line per test file, file-level only:

```
{"t":"tests/test_auth.py","f":["src/auth.py","src/db.py"],"c":"a3f21e0","d":412,"s":"pass","a":"python"}
```

`rtdd init` installs `.rtdd/map.jsonl merge=union` into `.gitattributes`. Two agents editing
different tests merge cleanly; two editing the same test leave two lines, which `rtdd map
compact` collapses by set-union of `f`. Committing the map is what lets a fresh worktree
inherit it and pay no seed cost.

Line-level coverage is never persisted. It is recomputed after each run and used
immediately, which is also why the uncovered report never suffers line drift.
<!-- rtdd:endsection -->
