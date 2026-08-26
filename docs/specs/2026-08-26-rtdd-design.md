# RTDD — Relational Test-Driven Development

**Status:** design v2, 2026-08-26
**Owner:** VocanicZ
**Supersedes:** v1 of the same date, rewritten after three adversarial audits.
See [`docs/audits/2026-08-26-design-audit.md`](../audits/2026-08-26-design-audit.md) for
what was measured and what it killed.

---

## 1. Problem

TDD's cycle cost grows at least linearly with suite size while the useful signal per cycle
stays constant. At 10,000 tests, an agent's red-green-refactor iteration is dominated by
process startup, I/O, fixtures, containers, and database setup for thousands of tests that
cannot observe the change. The cost is **execution time**, not tokens and not authorship.

A second problem sits underneath it. When an agent changes code that no test exercises,
a full-suite run reports green. The suite's size hides the gap rather than surfacing it.

## 2. What RTDD is

A tool that tells an agent **which tests cover the code it just changed**, and **which of
the lines it just changed nothing covers**, derived from real execution rather than a
static call graph.

RTDD is a **context provider, not a gate.** It reports; the agent decides. This is the
central correction from v1, and it is empirically motivated rather than stylistic — see §3.

### Non-goals

- **Enforcement.** RTDD does not block, gate, or fail a cycle on policy. `rtdd run` exits
  nonzero when a test fails, and for no other reason.
- Replacing CI. `rtdd verify` is a convenience, not a substitute for a full run.
- Sound program analysis. This is risk-managed test selection and says so.
- Reducing token cost. The hot path contains no model calls.
- Polyglot repos, and every language but Python, in v1. See §8.

## 3. Prior art, and the actual open question

Test Impact Analysis is mature — Bazel, Google TAP, Microsoft's Azure DevOps TIA,
[pytest-testmon](https://www.testmon.org/blog/determining-affected-tests/),
`jest --findRelatedTests`, [Ekstazi](https://users.ece.utexas.edu/~gligoric/papers/GligoricETAL15Ekstazi.pdf),
NCrunch, [Infinitest](https://github.com/infinitest/infinitest) (2007, explicitly marketed
for "tight TDD cycles"), and Meta's predictive test selection. Agent-facing versions exist
too: [Wallaby.js ships an MCP server](https://wallabyjs.com/docs/features/ai/) exposing
`wallaby_coveredLinesForTest` and `wallaby_allTestsForFileAndLine` to Claude Code and Cursor
today. "Uncovered change is a failure" is SonarQube's new-code coverage gate, Codecov's
patch status, and `diff-cover`.

Most directly: **[TDAD (arXiv:2603.17973)](https://arxiv.org/abs/2603.17973)**, March 2026,
builds a source↔test dependency map so an agent knows which tests to verify before
committing, and ships it as an agent skill file. On SWE-bench Verified it reduced
regressions from **6.08% to 1.82%**. A [TypeScript port](https://github.com/fmguerreiro/tdad-ts)
exists.

**RTDD does not claim to have invented any of this, and the README will say so.**

TDAD's second result is why this design looks the way it does. Adding TDD *procedural*
instructions without targeted test context raised regressions to **9.94% — worse than no
intervention at all.** Their conclusion: surfacing contextual information outperforms
prescribing procedural workflows. v1 of this spec was overwhelmingly procedural. v2 is not.

### The question RTDD exists to answer

TDAD's map is a **static dependency graph**. RTDD's is **dynamic coverage**.

Static graphs are blind to dynamic dispatch, dependency injection, reflection, plugin
registries, and monkeypatching — the places a call graph reports zero callers while real
execution reaches the code. Coverage sees exactly what ran. It is also blind in its own
way: it only knows paths some test actually took, and it attributes nothing to code
executed at import time (§6).

**Does a dynamic coverage map beat a static graph at reducing agent regressions?** That is
an open, testable question with a published baseline and a published methodology. It is the
contribution. Everything else in this document is the apparatus for answering it.

## 4. The map

### What is stored

`.rtdd/map.jsonl` — committed, sorted by test id, one line per test, **file-level only**.

```
{"t":"tests/test_auth.py::test_login","f":["src/auth.py","src/db.py"],"c":"a3f21e0","d":412,"s":"pass"}
```

| Field | Meaning | Source |
|---|---|---|
| `t` | Test id, in the form the runner accepts as a selector | test report |
| `f` | Repo-relative source files this test executed, sorted | coverage |
| `c` | Short SHA of `HEAD` when recorded | git |
| `d` | Last duration in ms — ranking tiebreak | test report |
| `s` | Last outcome — ranking tiebreak | test report |

`s` and `d` come from the **test report** (`--report-log`), not the coverage report, which
carries neither. Metadata lives in `.rtdd/meta.json`, kept out of the JSONL because the
JSONL is union-merged.

**Line-level coverage is never persisted.** Measured: `coverage json --show-contexts` is
12× larger than plain and extrapolates to ~0.5–1.2 GB at 10k tests, while file-level rows
are ~200 B each — about 2 MB for the same suite. Line data is computed fresh after each run
and used immediately (§6), which also removes the staleness problem entirely: line numbers
recorded at an old commit are worthless, and post-run line numbers are current by
construction.

### Where it comes from

Read `.coverage` (SQLite) directly — it *is* the bipartite relation, at a fraction of the
size of any exported format:

```sql
SELECT DISTINCT f.path, c.context FROM line_bits lb
  JOIN file f ON f.id = lb.file_id JOIN context c ON c.id = lb.context_id
```

`coverage lcov` and `coverage xml` have no `--show-contexts` option at all; gocover and
JaCoCo XML cannot express a test identifier. Only the SQLite store and
`coverage json --show-contexts` carry contexts, and SQLite is the cheaper of the two.

**`COVERAGE_CORE=ctrace` is forced.** With `sysmon` — the default on Python 3.14+ where
supported — dynamic contexts are silently dropped with a *warning*, not an error, producing
a 90%-empty map on a run that exits 0. RTDD treats the `no-sysmon-context` warning as fatal.
The consequence is that the fast core is unavailable and Python pays ~2× tracing overhead
during instrumented runs.

### Concurrency

```
.gitattributes:  .rtdd/map.jsonl merge=union
```

Two agents editing different tests merge cleanly. Two agents editing the same test leave
two lines; RTDD resolves by **set-union of `f`**, which can only widen.

The honest scope of that guarantee: union merge cannot narrow **relative to its two parent
maps**. It can and does leave the merged map stale **relative to the merged code** — branch
B may have created an edge that neither parent recorded. Merge commits therefore escalate
(§5).

Committing the map means a fresh worktree inherits the branch point's map — fresh or stale —
and pays no seed cost. That is what makes RTDD usable inside a parallel fleet.

### Lifecycle

Seeded once by `rtdd seed`. Each run refreshes the rows of the tests it executed.

**`f` is unioned, never replaced**, outside a full re-seed. A subset run legitimately
records *less* coverage than the seed run — import-time and first-caller-wins lines migrate
to whichever test runs first in that subset, and a failing test records a truncated prefix
of its real path. Replacing on those runs silently narrowed rows in v1, on a single branch,
with no merge involved. Only `rtdd seed` may shrink a row.

## 5. Selection

### The changed set

Defined explicitly, because v1 left it implicit and both natural definitions were wrong:

```
rtdd --base <ref>     default: HEAD
```

The changed set is the union of `git diff --name-only <base>` and
`git status --porcelain -uall` (untracked files included — a just-written file is the most
common input in TDD and `git diff` does not list it), with deletions retained (a deleted
path still selects the tests whose `f` contains it).

Known behaviour to measure, not hide: with `--base HEAD` the changed set grows monotonically
across a long uncommitted session, so selection ratio degrades toward 1.0 the longer an
agent runs without committing. §10 measures this as its own axis.

### Tiers

| Tier | Contents | Triggered by |
|---|---|---|
| **direct** | Changed and newly-added test files, run as-is | Always, ahead of everything else |
| **T0** | Tests whose `f` intersects the changed set | Default |
| **T1** | T0 ∪ tests whose test module transitively imports an import-time-only changed file (§6) ∪ tests covering files in a changed opaque file's directory | Import-time-only change; opaque file changed; row staler than `stale_commits`; merge commit |
| **T2** | Full suite | Map unseeded or schema-mismatched; dependency manifest changed; test-harness config changed; `drift_guard` reached; `rtdd verify` |
| **empty** | Nothing selected — reported explicitly, distinct from "all passed" | Every under-selection path terminates here, so it is never silently green |

The **direct** tier exists because in v1 a newly written test had no map row and was
therefore in no tier — meaning step 3 of v1's own agent loop never executed the test the
agent had just written.

Defaults in `.rtdd/config.yaml`: `stale_commits: 50`, `drift_guard: 100`,
`hub_threshold: 0.40`.

### Ranking

T0 runs in relevance order: descending `|f ∩ changed| / |f|`, then last-failed first, then
ascending `|f|`, then ascending `d`.

Fail-fast is **opt-in** (`--fail-fast`), never implied. In v1 it was on by default in the
red phase, which combined with last-failed-first ordering meant a quarantined failing test
halted the run before the agent's own test executed.

## 6. The uncovered-change signal

RTDD's second output, and the one with no equivalent in a static-graph tool.

Computed **after** the selected tests run, from fresh coverage, against the changed line
ranges from `git diff --unified=0`. Both sides are current, so there is no line-drift
problem.

```
$ rtdd run
  changed: src/auth.py:40-58, src/constants.py:1-12
  12 tests selected, ranked

  ..........✓✓                                    12 passed  1.4s

  UNCOVERED: src/auth.py:52-58  (7 changed lines, no executing test)
  import-time: src/constants.py:1-12  (executed during collection, not attributed)
```

Three classes, and the distinction is the whole point:

- **Covered** — an executing test touched these changed lines.
- **Uncovered** — no test executed them. This is the real signal, and it is **line-granular**.
  v1's file-granular version detected only new *files*; adding a function to an
  already-covered file left T0 non-empty and reported green on uncovered code, which was the
  exact pathology §1 opens with.
- **Import-time** — executed, but attributed to no test. Reported separately and **never
  counted as uncovered**.

That third class is not a technicality. Measured on this machine: a `constants.py`
containing a dataclass and a module constant, imported and asserted on by two passing tests,
is attributed to **zero** test contexts, because pytest imports every test module during
collection before any dynamic context is set. Under v1's rule that file hard-REDed while
being correctly tested. The affected class is enormous — dataclasses, enums, config modules,
Pydantic and Django models, route decorators, `__init__.py` re-exports.

**Selecting for import-time-only files** is the one place RTDD uses static analysis: a file
that appears only in the empty context cannot be reached through the coverage relation, so
RTDD falls back to a Python AST import scan and selects tests whose module transitively
imports it. Bounded, single-purpose, and applied only where dynamic coverage provably cannot
answer — which is a better argument for a hybrid than the one v1 rejected on speculation.

`rtdd run` exits nonzero **only** when a test fails. An uncovered report is information, not
a verdict.

## 7. Commands

```
rtdd status                  # adapter, map freshness, seed state
rtdd seed                    # one full instrumented run; the only op that may shrink a row
rtdd which                   # print ranked selection + uncovered report; run nothing
rtdd run [--fail-fast]       # run the selection, refresh rows, print the uncovered report
rtdd verify                  # full suite
rtdd doctor                  # fan-out / coupling report
rtdd explain <file>          # which tests cover this file
rtdd map compact             # collapse duplicate rows after a union merge
rtdd init                    # install .gitattributes, config, and agent front-ends
```

`rtdd which` is the primary integration point for an agent — it is the "surface the context"
operation TDAD's result argues for, and it costs one map lookup and one git diff.

There is no `--expect red` / `--expect green`. RTDD reports what happened; the TDD discipline
stays with the agent and its skill prompt, where it can be exercised without a tool that
mistakes a characterization test for a broken one.

## 8. The Python adapter

**v1 is Python-only.** v1-of-this-spec shipped Go and TypeScript as first-class adapters on
a capability that does not exist in either ecosystem:

- **JS:** Istanbul/v8 coverage carries aggregate counters with no test dimension.
  [vitest#6735](https://github.com/vitest-dev/vitest/issues/6735) requests per-test
  attribution; open since October 2024, no maintainer response. Per-test-file isolation
  measured at **27.5×**. Batched attribution instead produces a ratchet that drives
  selection ratio to 1.0 within a few cycles.
- **Go:** default `-coverprofile` records count 0 for every package but the one under test;
  `-coverpkg=./...` is mandatory and still gives no test dimension. Per-test requires a
  prebuilt `-c` binary driven once per test (7 ms floor on a *trivial* package, re-running
  `TestMain` each time), or injecting a `TestMain` wrapper into the user's repo. Subtests —
  the dominant Go idiom — are invisible to `go test -list` and their names are not
  round-trippable.

Adding a language is therefore **engine work**, not a data file. v1's "adapters as data"
claim is withdrawn. The YAML declares what genuinely is declarative; execution and parsing
are implemented per language.

```yaml
name: python
detect: ["pytest.ini", "pyproject.toml", "setup.cfg"]
env:    { COVERAGE_CORE: ctrace }
seed:   "pytest --cov={src} --cov-context=test --report-log={log}"
subset: "pytest {tests} --cov={src} --cov-context=test --report-log={log}"
coverage: sqlite            # read .coverage directly
report:   pytest-reportlog  # source of `s` and `d`
test_globs: ["tests/**/*.py", "**/test_*.py"]
exit_codes: { 4: bad-selector, 5: no-tests-collected }
opaque: ["**/*.yaml", "**/*.yml", "**/*.sql", "**/*.html", "**/*.j2", "**/fixtures/**"]
full_escalate: ["requirements.txt", "pyproject.toml", "**/conftest.py"]
```

Engine responsibilities the YAML cannot express, and which v1 omitted: stripping the
`|run`/`|setup`/`|teardown` phase suffix from context ids; round-tripping parametrised ids
containing `::`, `[`, `]`; normalising coverage paths against git paths; respecting the host
repo's own `.coveragerc` source/omit settings so seed and subset agree on scope; chunking
test ids (8,000 ids ≈ 613 KB — fits Linux `ARG_MAX`, is 74× over the Windows `CMD` limit,
and pytest has no argfile option); and mapping exit codes 4 and 5, which otherwise look like
test failures.

## 9. `rtdd doctor`, and what it cannot see

Ranks source files by fan-out — how many tests cover them. That number is a coupling metric
computed in milliseconds from data already on disk, and it surfaces the hub files where
selection ratio collapses.

**Documented limitation, stated in the tool's own output:** anything executed once per
process gets a fan-out of 1. `@lru_cache`, module singletons, DI containers, session-scoped
fixtures, `sync.Once` — the body runs during whichever test happened to go first, so the
repo's most coupled file can appear as its cleanest. `doctor` prints this caveat alongside
the table rather than letting the number mislead. It is also why fan-out is reported as a
diagnostic and never used as an automatic escalation trigger.

## 10. Benchmark

Two axes. The first answers the research question; the second establishes that the selector
is competitive.

### Axis 1 — agent regression rate (primary)

**SWE-bench Verified**, matching TDAD's methodology so the numbers are directly comparable
to published ones. Sample size, seed, and model class are pre-registered before the run.

TDAD's published run: **n=100** SWE-bench Verified instances, Qwen3-Coder 30B (Q4_K_M).
Reference implementation at [github.com/pepealonso95/TDAD](https://github.com/pepealonso95/TDAD)
(Python, MIT, ships its own harness under `claudecode_n_codex_swebench/`), so every arm is
**run locally on one harness** rather than cited across harnesses.

| arm | regressions | resolution | source |
|---|---|---|---|
| vanilla | 6.08% | 31% | TDAD, reproduced locally |
| TDD procedural prose | 9.94% | 31% | TDAD, reproduced locally |
| TDAD GraphRAG **+ TDD prose** | 1.82% | 29% | TDAD, reproduced locally |
| **RTDD context only** | **?** | **?** | this work — the non-procedural claim |
| **RTDD context + TDD prose** | **?** | **?** | this work — apples-to-apples with TDAD's arm |

**Five arms, not four.** TDAD's winning configuration is graph *plus* procedural prose, so
comparing it against a prose-free RTDD would be a confounded comparison in RTDD's own
favour-losing direction. RTDD runs both: context-only, which is the design's actual claim
(§3), and context-plus-prose, which is the only arm directly comparable to TDAD's.

Note that TDAD's regression win came with a small resolution-rate cost (29% vs 31%).
Resolution rate is therefore published alongside regression rate for every arm; a tool that
prevents regressions by solving fewer problems has not won.

The RTDD arms must be **structurally** non-procedural, not merely intended to be: the arm
builder asserts `build("rtdd") == build("vanilla") + context_block` byte-for-byte, with an
imperative-phrase lint on the block, both enforced in CI. TDAD measured procedural prose at
worse-than-baseline, and an arm that smuggles prose in through the context block would
silently reproduce that confound.

### Axis 2 — selection quality on real commits

Replay the last N real commits of each corpus repo; ground truth is what the suite actually
did. Real commits add files, rename, move code, and change fixtures — the classes mutation
testing structurally cannot produce, and precisely where file-level TIA is weakest.

**Baselines, all of them, or the comparison means nothing:** pytest-testmon (method-level
checksums — finer-grained than RTDD v1, and the first thing a reviewer will ask about), a
naive `tests/test_<module>.py` path heuristic (if RTDD does not clearly beat this, the map
is unjustified), `pytest --lf`, a static import graph, `pytest -n auto` (the intervention a
real team actually reaches for), and random selection at equal selection ratio.

### Metrics

- **Change-level recall** with `F_rtdd ∩ F_full ≠ ∅` required — an unrelated flaky failure
  must not score as a detection.
- **Test-level recall** `|F_rtdd ∩ F_full| / |F_full|`. Both are published; v1 published only
  the flattering one. Meta publishes both.
- **Stratified by `|F_full|`, with the `|F_full| == 1` stratum broken out.** Single-killer
  changes are the only place selection safety is genuinely under test; pooled recall is
  mostly a measurement of hub coverage.
- **Selected-duration fraction**, not just test count. Durations are heavy-tailed; selecting
  2% of tests that happen to be the integration tests is not a 98% saving. Ekstazi selects a
  small fraction and still reports only 32% average end-to-end reduction — that gap is the
  finding.
- **End-to-end wall-clock**, three columns: full uninstrumented / subset instrumented /
  subset uninstrumented, so the ~2× instrumentation tax RTDD *adds* per cycle is visible.
- **False-signal rate** — how often the uncovered report fires on a change that is in fact
  adequately tested. Given §6's import-time class, this is the number that decides whether
  anyone leaves the feature on, and v1 never measured it.

### Rules

- **Pre-register** the corpus: selection criteria and the frozen repo list published before
  results, with an "attempted and excluded, with reason" table. Exclusions are where
  cherry-picking hides.
- **Never pool across repos.** Per-repo tables, or duration-weighted aggregates.
- Shipped defaults only; the full config printed in the results table.
- Every cycle counted, including escalations; escalation rate published as its own number.
- Wall-clock only from disclosed hardware, never from CI runners.
- **A pre-registered kill criterion**, chosen before Axis 1 runs: a stratified recall floor
  below which the tool is not published. Without it, "recall below 100% is printed" is
  unfalsifiable.

## 11. Deferred

- **Method-level checksums.** testmon has done this since ~2016; matching it is a v2 goal,
  not a v1 claim.
- **JS and Go adapters.** JS needs a published Vitest coverage provider built on
  `Profiler.takePreciseCoverage()` (verified to reset counters per call, ~1.2 ms on a small
  process); Go needs either a prebuilt-binary driver or `runtime/coverage.ClearCounters()`
  with an injected `TestMain`. Both are real packages, not YAML.
- **Polyglot repos.** `detect` resolves to one adapter; rows would need an adapter tag.
- **Time-budgeted selection.**

## 12. Milestones

| # | Slice | Done when |
|---|---|---|
| **M1a** | mapstore + selector + `status`/`which`, driven by a hand-written fixture map | Tier logic, ranking, union-merge resolution, compaction, and changed-set computation all provable with no subprocess and no coverage |
| **M1b** | Python adapter, `seed`, `run` | Full loop works on a real Python repo; SQLite reader, report-log parser, `ctrace` forcing, path normalisation, id round-tripping, argv chunking |
| **M2** | Uncovered-change signal, `doctor`, `explain`, `init` | Line-level post-run report with the three classes; import-time fallback selection |
| **M3** | Axis 2 — real-commit replay + all baselines | Per-repo tables against testmon, path heuristic, `--lf`, import graph, xdist, random |
| **M4** | Axis 1 — SWE-bench Verified | The four-arm table, with the vanilla arm reproduced locally |
| **M5** | Front-ends, release | `PROTOCOL.md` → `SKILL.md`/`AGENTS.md`/`.mdc`, drift check in CI, GoReleaser, README |

M1a is deliberately first and adapter-free: it is where all the set, rank, and merge logic
gets tested cheaply, before it collides with the ugly realities in M1b. v1 put all of this
in one slice, which was the entire engine.

**M3 and M4 gate publication.** No recall or regression claim ships unmeasured, and the kill
criterion is set before M4 runs.

## 13. Repository layout

```
rtdd/
  cmd/rtdd/
  internal/
    mapstore/     JSONL I/O, union resolution, compaction
    selector/     tiers, ranking, changed-set computation
    adapter/      detection, templating, exit-code mapping, path normalisation
    coverage/     .coverage SQLite reader, report-log parser
    uncovered/    hunk parsing, line-class computation
    doctor/
  adapters/python.yaml
  protocol/PROTOCOL.md
  dist/           GENERATED — SKILL.md, AGENTS.md, cursor/rules/rtdd.mdc
  bench/
    swebench/     Axis 1
    replay/       Axis 2
    corpus.yaml   pre-registered, frozen
    results/
  docs/specs/  docs/audits/
```

`dist/` regenerates from `protocol/PROTOCOL.md`; CI fails if stale. `rtdd init` installs
them into a host repo — including a merge strategy for an existing `AGENTS.md`/`CLAUDE.md`,
which v1 had no answer for.

## 14. Decisions of record

Decisions carried from v1, unchanged:

| # | Decision | Rationale |
|---|---|---|
| D1 | Coverage-derived edges, not a static graph | Sees dynamic dispatch, DI, reflection. Zero LLM tokens in the hot path. Now framed as the testable hypothesis, not an assumed win. |
| D4 | Go static binary | Zero runtime deps forced into the host repo. (Startup time was the wrong rationale — the runner subprocess dominates. Distribution is the real one.) |
| D5 | Committed sorted JSONL + union merge | Worktrees inherit the map; conflicts widen relative to their parents. Viable now that only file-level rows are stored (~2 MB at 10k tests). |

Decisions reversed or replaced:

| # | v1 said | v2 says | Why |
|---|---|---|---|
| D2 | File-level granularity | File-level for **selection**; line-level, ephemeral, post-run for the **uncovered signal** | Over-selection is safe for selection and unsafe for the RED rule; v1 transferred the argument illegitimately |
| D3 | Hard-RED on empty T0 | An uncovered **report**, line-granular, exit 0 | Import-time attribution made the gate fire on correctly-tested files (measured); TDAD measured procedural gating as net-harmful |
| D6 | Fail-fast on by default in the red phase | Opt-in only | Combined with last-failed-first ranking, it halted runs before the agent's own test executed |
| D7 | Mutation benchmark, recall + speed | SWE-bench regression rate + real-commit replay, with baselines | The mutation harness was degenerate (≈100% recall by construction), infeasible (455 CPU-h, ~$218/run), and blind to the uncovered path |
| D8 | Adapters as declarative YAML | YAML declares; the engine implements per language | Three of five parse formats cannot carry a test id; JS and Go need real packages |
| D9 | Three adapters at v1 | One — Python | Two of the three rested on a capability that does not exist |
| D11 | *(new)* | `f` unions, never replaces, outside `seed` | Subset runs and failing tests silently narrowed rows |
| D12 | *(new)* | Changed set defined explicitly, untracked and deleted files included | `git diff HEAD` omits untracked files, so the flagship case was invisible |
| D13 | *(new)* | `COVERAGE_CORE=ctrace` forced; `no-sysmon-context` fatal | Default on Python 3.14+ silently drops ~90% of contexts and exits 0 |
| D14 | *(new)* | Static import scan for import-time-only files | The one case dynamic coverage provably cannot reach |

## 15. Open questions

- **Does dynamic coverage actually beat a static graph for agent regressions?** The project's
  reason to exist. Unknown until M4. If TDAD's static graph matches or beats RTDD on
  SWE-bench Verified, the honest outcome is to publish that result and contribute to TDAD
  rather than ship a competitor.
- **The kill criterion.** The stratified-recall floor must be chosen and published before M4
  runs, not after the number is known.
- **Seed cost at real scale.** ~2× tracing overhead is measured on a small pure-Python
  suite; I/O-bound suites should be lower, but this is unmeasured on a 10k-test repo.
- **Session-scoped fixture attribution.** First-caller-wins gives shared setup a fan-out of
  1 (§9). Whether RTDD should special-case fixture-executed lines the way it special-cases
  import-time lines is unresolved, and it is the largest remaining correctness gap.
- **Flaky tests.** A flaky failure in the selection is indistinguishable from a real one.
  Whether to track flakiness in `s` is unresolved.
