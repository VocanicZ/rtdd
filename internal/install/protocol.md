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

Run `rtdd doctor` at any point to see which adapters matched and what fidelity they give.
<!-- rtdd:variant target=global-agents -->
Check for `.rtdd/map.jsonl` first. If it exists, rtdd is set up — use the commands below. If
it does not, run `rtdd init` then `rtdd seed` once, and commit the map. `rtdd init` exiting 2
is the no-adapter refusal: nothing matched a toolchain rtdd can instrument, so add the
adapter it names or re-run with `--force`.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=what title="What rtdd reports" targets=skill,agents,mdc,global,global-agents order=10 -->
`rtdd` reports which tests cover the code you just changed, and which of the lines you just
changed nothing covers. Both come from coverage recorded during real execution of this
repository's suite, not from a static call graph, so the relation includes edges reached
through dynamic dispatch, dependency injection, plugin registries, and monkeypatching, and
omits any path no test has ever taken.

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
The report classifies your changed lines into three classes, and the distinction matters:

- **covered** — an executing test touched these changed lines.
- **uncovered** — no test executed them.
- **import-time** — executed during collection and attributed to no test. Reported
  separately and never counted as uncovered. Dataclasses, enums, config modules, ORM model
  definitions, route decorators, and `__init__.py` re-exports land here routinely while
  being correctly tested.

An uncovered range is information about the suite, not a verdict on the patch.
<!-- rtdd:variant target=agents -->
The uncovered report splits changed lines into covered, uncovered, and import-time.
Import-time lines execute during collection and are attributed to no test — they are
reported separately and are not a coverage gap.
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

<!-- rtdd:section id=fidelity title="Selection fidelity" targets=skill,agents,mdc,global,global-agents order=55 -->
Every `--json` document carries `selection_fidelity`, which answers a different question
from `tier`: `tier` says how much of the suite was selected, `selection_fidelity` says what
that answer was derived from. It is never null and never absent, and it is one of three
values:

- **`execution-derived`** — tests were chosen from per-test coverage recorded by a real
  run. Everything else in this document assumes this fidelity.
- **`static`** — this toolchain records nothing, so tests were chosen from declared
  correspondence and imports.
- **`none`** — neither is available, so nothing narrower than the full suite can be
  selected.

The distinction changes how a green run should be read: a static selection is derived from
declared correspondence and imports, not from a recorded run, so it can miss a test that
execution-derived selection would have caught. A passing static selection is therefore
weaker evidence than a passing execution-derived one. Read a green `static` run as "the
tests I could name passed", not as "this change is covered".

`rtdd doctor` is the one command that reports which fidelity this repository can achieve
and why: one row per detected adapter, the fidelity it can reach here, and the clause of
its own declaration that determined it.
<!-- rtdd:variant target=agents -->
`--json` carries `selection_fidelity`: `execution-derived` (tests chosen from recorded
coverage), `static` (chosen from declared correspondence and imports, because this
toolchain records nothing), or `none` (nothing narrower than the full suite). A static
selection can miss a test an execution-derived one would have caught, so a passing static
selection is weaker evidence. `rtdd doctor` reports which fidelity this repository can
achieve, and why.
<!-- rtdd:endvariant -->
<!-- rtdd:variant target=mdc -->
`--json` carries `selection_fidelity`, which says what the selection was derived from:
`execution-derived` (tests chosen from coverage recorded by a real run), `static` (chosen
from declared correspondence and imports, because this toolchain records nothing), or
`none` (nothing narrower than the full suite is available). A static selection can miss a
test an execution-derived one would have caught, so a passing static selection is
weaker evidence — read a green `static` run as "the tests I could name passed".
`rtdd doctor` reports which fidelity this repository can achieve, and why.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=json title="JSON output" targets=skill,global order=60 -->
`--json` emits one object for programmatic consumption:

```json
{
  "tier": "T0",
  "reason": "changed files intersect 12 recorded test rows",
  "selection_fidelity": "execution-derived",
  "base": "HEAD",
  "changed": ["src/auth.py", "src/db.py"],
  "direct": ["tests/test_auth.py"],
  "tests": ["tests/test_auth.py::test_login", "tests/test_db.py::test_pool"],
  "uncovered": [{"path": "src/auth.py", "ranges": [{"start": 52, "end": 58}]}],
  "import_time": [{"path": "src/constants.py", "ranges": [{"start": 1, "end": 12}]}],
  "selected_duration_ms": 1412,
  "map_tests": 8471
}
```

`tier` is one of `empty`, `direct`, `T0`, `T1`, `T2`. `T2` means the full suite was selected
because the map is unseeded, a dependency manifest changed, the test-harness config changed,
or the drift guard was reached; `reason` says which.
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

`rtdd seed` is the only operation that may narrow a test's recorded file set. Every other
path unions, because a subset run legitimately records less coverage than a full run — an
import-time line migrates to whichever test ran first, and a failing test records only a
truncated prefix of its real path.
<!-- rtdd:endsection -->

<!-- rtdd:section id=limits title="What it cannot see" targets=skill,mdc,global order=80 -->
Stated plainly, because a selector that hides its blind spots is worse than no selector:

- Coverage only knows paths some test actually took. A branch nothing has ever exercised has
  no edge, and `rtdd which` will not find it.
- Anything executed once per process — `@lru_cache`, module singletons, DI containers,
  session-scoped fixtures — is attributed to whichever test happened to run first, so the
  most coupled file in a repo can appear as its cleanest in `rtdd doctor`.
- Selection is file-level. Changing one function in a file selects every test that touched
  any part of that file.
- A merge commit escalates, because a union-merged map cannot narrow relative to its parents
  but can still be stale relative to the merged code.
- `rtdd verify` is a convenience, not a substitute for CI.
<!-- rtdd:variant target=mdc -->
Coverage only knows paths some test actually took; a never-exercised branch has no edge.
Once-per-process execution (`@lru_cache`, singletons, session fixtures) is attributed to
whichever test ran first. Selection is file-level. `rtdd verify` does not replace CI.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=map title="The map file" targets=skill,global order=90 -->
`.rtdd/map.jsonl` is committed, sorted by test id, one line per test, file-level only:

```
{"t":"tests/test_auth.py::test_login","f":["src/auth.py","src/db.py"],"c":"a3f21e0","d":412,"s":"pass"}
```

`rtdd init` installs `.rtdd/map.jsonl merge=union` into `.gitattributes`. Two agents editing
different tests merge cleanly; two editing the same test leave two lines, which `rtdd map
compact` collapses by set-union of `f`. Committing the map is what lets a fresh worktree
inherit it and pay no seed cost.

Line-level coverage is never persisted. It is recomputed after each run and used
immediately, which is also why the uncovered report never suffers line drift.
<!-- rtdd:endsection -->
