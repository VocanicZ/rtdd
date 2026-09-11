# rtdd

**rtdd tells an agent which tests cover the code it just changed, and which of the lines it
just changed nothing covers** — derived from real execution rather than a static call graph.

It is a context provider, not a gate. `rtdd run` exits non-zero when a test fails, and for
no other reason.

```
$ rtdd which
base:     HEAD
changed:  1 files
  modified  src/calc.py
  tier: T0  (2 tests selected, ranked)
  reason: tests whose recorded coverage intersects the changed set
    tests/test_calc.py::test_sub
    tests/test_calc.py::test_add

$ rtdd run
tier T0: 2 selected (tests whose recorded coverage intersects the changed set)
2 ran, 0 failed, 2 rows in the map

  UNCOVERED: src/calc.py:3-4  (2 changed lines, no executing test)
  import-time: src/calc.py:1  (executed during collection, not attributed)
  import-time: src/calc.py:5  (executed during collection, not attributed)
```

The run above is *execution-derived* selection, and it is Python only today. Every other
language gets *static* selection instead — declared file correspondence and imports, with
nothing instrumented. It never watched a test run, so it can miss a test the Python tier
would have caught. Passing it is weaker evidence, and every surface tells you which tier
you are reading.

That static tier was pre-registered against a naive path heuristic and did not beat it, so
it does not carry its weight as a distinct tier. It ships because a repository RTDD cannot
instrument is otherwise offered nothing — not because it is measured to be better. The
numbers are in [`bench/results/flask/summary.json`](bench/results/flask/summary.json) and
[`bench/results/httpie/summary.json`](bench/results/httpie/summary.json).

## Install

There are two routes, and which one works depends on whether a release exists: **no release
is published yet**, so build from source today; the one-line installer below starts working
the moment a release is published, and is the shorter route once it does.

Build from source — Go 1.24 or newer:

```
git clone https://github.com/VocanicZ/rtdd && cd rtdd
go build ./cmd/rtdd      # writes ./rtdd — put it somewhere on your PATH
```

Or, once a release exists:

```
curl -fsSL https://raw.githubusercontent.com/VocanicZ/rtdd/main/install.sh | sh
```

Either way, the loop is the same:

```
cd your-python-repo
rtdd init      # front-ends, .gitattributes merge=union, config
rtdd seed      # one full instrumented run to build the map
rtdd which     # what covers your current changes
```

`rtdd init` installs a Claude Code skill at `.claude/skills/rtdd/SKILL.md`, a Cursor rule at
`.cursor/rules/rtdd.mdc`, and a short marker-delimited block in `AGENTS.md` (and `CLAUDE.md`
if you have one). It never rewrites a byte outside its own markers.

It first checks that an adapter matches the repository. If none does, it writes **nothing**
and exits 2: agent instructions promising a selection RTDD cannot make are worse than no
instructions at all. The way out is an adapter of your own in `.rtdd/adapters/<language>.yaml`
— or `rtdd init --force`, which installs anyway and states the caveat in the first paragraph
of the skill it writes.

## Does it work?

RTDD is built for one loop: an agent edits, runs tests, edits again, dozens of times in a
single task. Without it the agent runs the whole suite every time. So there are two
questions — does RTDD save time, and does it miss anything the full suite would catch.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/results/figures/rtdd-vs-full-dark.svg">
  <img alt="RTDD against running the whole suite. Ten edits to one module: the full suite costs 28390 ms, rtdd run 19962 ms (1.42x faster), rtdd run --record=auto 10127 ms (2.80x faster). On five flask commits that break something, rtdd and the full suite both catch 5 of 5, and both catch 4 of 4 where exactly one test fails, with rtdd running 0.757 of the suite's test time against the full suite's 1.000." src="docs/results/figures/rtdd-vs-full-light.svg">
</picture>

### It saves time

Ten edits to one module in a real `flask` clone, tests run after each edit. Same machine,
same edits, three copies of the same repository:

| after every change | 10 edits | |
|---|---|---|
| run the whole suite | 28390 ms | |
| `rtdd run` | 19962 ms | **1.42× faster** |
| `rtdd run --record=auto` | 10127 ms | **2.80× faster** |

486 tests in the suite, 16 selected. Reproduce with
[`scripts/agent-session-bench.sh`](scripts/agent-session-bench.sh) against a seeded clone.

`--record=auto` skips re-recording coverage on cycles where the map has nothing new to
learn. You give up that cycle's uncovered report, and it tells you when it does.

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

### It leaves your tests alone

RTDD never writes, edits, reorders or deletes a test. Whatever your suite caught before, it
still catches. The only thing RTDD changes is which of them run.

### Should you use it instead of running everything?

In an agent loop, yes — 1.42× faster as shipped, 2.80× with `--record=auto`, and on every
commit that could be measured it missed nothing.

In CI, or anywhere a missed regression is expensive, no. Run everything. Five commits on
one repository is where this evidence starts, not where it ends.

## What it does not do

- **It does not enforce anything.** No gate, no policy exit code, no expected-phase flag.
- **It does not replace CI.** Running `rtdd` is a convenience for the inner loop, not a
  substitute for a full CI run.
- **It is not sound program analysis.** It is risk-managed test selection: it can be
  wrong, and the tier and uncovered report are there so you can see when.
- **It does not reduce token cost.** There are no model calls in the hot path.
- **Only the Python tier is measured coverage.** Per-test attribution does not exist in the
  JavaScript or Go ecosystems — Istanbul and v8 carry aggregate counters with no test
  dimension ([vitest#6735](https://github.com/vitest-dev/vitest/issues/6735), open since
  October 2024), and Go's `-coverprofile` has none either. Those languages get static
  selection, which was measured against a path heuristic and did not beat it
  ([the comparison](docs/results/axis2-selection-baselines.md)).
- **Coverage is blind in its own way.** It only knows paths some test actually took, and it
  attributes nothing to code executed at import time — which is why the uncovered report has
  a separate import-time class instead of calling dataclasses and enums untested.
- **`COVERAGE_CORE=ctrace` is forced**, so instrumented runs pay roughly 2× tracing overhead.
  With coverage.py's `sysmon` core — the default on Python 3.14+ — dynamic contexts are
  silently dropped with a warning and a zero exit, producing a mostly-empty map; RTDD treats
  that warning as fatal.

## Documentation

- [Prior art](docs/PRIOR-ART.md) — pytest-testmon, TDAD, and the rest of the field RTDD
  builds on, with what each of them already does better.
- [Baseline comparison](docs/results/axis2-selection-baselines.md) — RTDD against
  pytest-testmon, a path heuristic and four other selectors, with wall-clock distributions
  per repository. RTDD does not win this one.
- [Design](docs/specs/2026-08-26-rtdd-design.md) — the full specification, including the
  measurements that killed three earlier design decisions.
- [Design audit](docs/audits/2026-08-26-design-audit.md) — what was measured, and what it
  killed.
- [Agent protocol](protocol/PROTOCOL.md) — the single source for every generated front-end
  under `dist/`. Edit it, run `rtdd-gen render`; CI fails if `dist/` is stale or if any
  front-end is wrong for its target.

## License

MIT. See [LICENSE](LICENSE).
