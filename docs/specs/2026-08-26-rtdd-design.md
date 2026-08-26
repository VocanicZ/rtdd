# RTDD — Relational Test-Driven Development

**Status:** design, approved 2026-08-26
**Owner:** VocanicZ
**Destination:** a published, benchmarked, agent-agnostic tool at `github.com/VocanicZ/rtdd`

---

## 1. Problem

TDD degrades superlinearly with suite size. At 1,000 tests the implementation is a
minority of the cycle; at 10,000 tests a single red-green-refactor iteration can take
hours. The cost is **execution time** — process startup, I/O, fixtures, containers,
database setup — not tokens and not authorship.

The waste is structural: a change to one function re-executes the entire suite, when
only a small subset of tests can possibly observe that change.

A second, quieter failure compounds it. A full-suite run reports **green** on a change
that no test covers. Ten thousand passing tests, none of which touch the new function,
and TDD's own feedback signal says the work is done. The suite's size actively hides
the gap.

## 2. What RTDD is

A single static binary plus a protocol document. It maintains a relation between tests
and the code they execute, uses that relation to run only the tests that can observe a
change, and treats an uncovered change as a **failing state** rather than a silent pass.

RTDD is not a Claude Code skill with a CLI bolted on. It is a **CLI with generated agent
front-ends**, so the same protocol reaches Claude Code, Codex, Cursor, and Copilot
without drifting between them.

### Non-goals

- Replacing CI. `rtdd verify` is a safety net, not a substitute for a full run.
- Sound program analysis. RTDD is risk-managed test selection, and says so.
- A general knowledge-graph tool. The map is a bipartite test↔file relation, nothing more.
- Reducing token cost. The hot path contains no model calls; tokens are incidental.

## 3. Prior art, and what is actually new

Test Impact Analysis is mature: Bazel's target graph, Google TAP, Microsoft's TIA in
Azure DevOps, `pytest-testmon`, `jest --findRelatedTests`, `cargo-nextest`, NCrunch,
Meta's predictive test selection. RTDD does not claim to have invented test selection
and the README must say so plainly.

Every one of those tools optimizes **CI**. None of them define how an **agent** should
drive a TDD inner loop. That is the gap RTDD fills, and the contributions are:

1. **Hard-RED on empty coverage.** Uncovered change is a failure, not a warning. RTDD is
   *stricter* than full-suite TDD, not merely faster.
2. **TDD-phase-aware selection.** `--expect red` and `--expect green` are different
   execution strategies, and `--expect red` catches a new test that passes before
   implementation exists.
3. **Conflict-safe shared map.** Designed for N parallel agents on N branches, where a
   merge conflict can only widen selection, never narrow it.
4. **Adapters as data.** A new language is a YAML file, not a change to the engine.
5. **A published safety number.** Detection recall alongside speed, in the same table.

## 4. The map

### Storage

`.rtdd/map.jsonl` — committed to the repository, sorted by test id, one line per test.

```
{"t":"tests/test_auth.py::test_login","f":["src/auth.py","src/db.py"],"c":"a3f21e0","d":412,"s":"pass"}
{"t":"tests/test_cart.py::test_add","f":["src/cart.py","src/db.py"],"c":"a3f21e0","d":88,"s":"pass"}
```

| Field | Meaning |
|---|---|
| `t` | Test identifier, in the form the adapter's runner accepts as a selector |
| `f` | Repo-relative source files this test executed, sorted |
| `c` | Short SHA of `HEAD` when the row was recorded — drives staleness escalation |
| `d` | Last observed duration in ms — drives fastest-first tiebreaking |
| `s` | Last observed outcome — drives last-failed-first ordering |

Metadata lives in a **separate** `.rtdd/meta.json` (schema version, adapter name, seed
commit, cycle counter). Keeping it out of the JSONL is deliberate: the JSONL is
union-merged, and a header line inside it would survive merges in duplicate.

Granularity is **file-level**, not line-level. Line numbers shift on every edit and
invalidate the map; file paths are stable under refactor. The failure mode of file-level
granularity is over-selection, which is safe. Symbol-level tracking with content
checksums is deferred to v2 (§10).

### Concurrency

```
.gitattributes:  .rtdd/map.jsonl merge=union
```

Two agents editing different tests merge cleanly. Two agents editing the same test leave
two lines for that test id; RTDD resolves the duplicate by taking the **set-union of `f`**.

This resolution can only widen the selection. A merge conflict may make RTDD slower. It
can never make RTDD miss a test. `rtdd map compact` collapses duplicates on demand.

Committing the map means a fresh Harness worktree inherits it from the branch point and
pays **zero seed cost** — the property that makes RTDD viable inside a parallel fleet.

### Lifecycle

Seeded once by a full instrumented run (`rtdd seed`). Every subsequent cycle rewrites
only the rows of the tests it just executed. The map self-heals as a byproduct of the
loop it serves; there is no separate maintenance step and no scheduled rebuild.

## 5. Selection

### Tiers

| Tier | Contents | Triggered by |
|---|---|---|
| **RED** | *nothing runs; exit 1* | A changed instrumentable file whose T0 is empty — an uncovered change |
| **skip** | *nothing runs; exit 0* | Change touches only documentation / markdown |
| **T0** | Tests whose `f` intersects the changed file set | Default path |
| **T1** | T0 ∪ tests covering any file in a changed file's directory | An opaque file changed; or a selected row's `c` is staler than `stale_commits` |
| **T2** | Full suite | Map unseeded or schema-mismatched; dependency manifest changed; test-harness config changed; cycle counter hit `drift_guard`; explicit `rtdd verify` |

Tier thresholds are configurable in `.rtdd/config.yaml`, with defaults:

| Knob | Default | Meaning |
|---|---|---|
| `stale_commits` | 50 | A selected row whose `c` is more than this many commits behind `HEAD` escalates to T1 |
| `drift_guard` | 100 | Every N successful cycles, one T2 run executes regardless of tier |
| `hub_threshold` | 0.40 | Fan-out fraction above which `doctor` reports a file as a hub |

**Opaque files** are those coverage cannot attribute — fixtures, SQL, templates, YAML,
JSON data. Coverage records that a test read `users.json` nowhere. The adapter declares
them by glob, and a change to one escalates to T1 within its directory.

**Full-escalation files** are those that can invalidate the whole map at once: dependency
manifests (`go.mod`, `package.json`, `requirements.txt`, `Cargo.toml`, `pom.xml`) and
test-harness configuration (`conftest.py`, `vitest.config.ts`, `TestMain`). Declared by
the adapter, they go straight to T2.

### Ranking

T0 executes in relevance order, not map order:

1. **Specificity** — descending `|f ∩ changed| / |f|`. A test covering only the changed
   file outranks a test that covers it among five hundred others.
2. **Last outcome** — `s == "fail"` first.
3. **Focus** — ascending `|f|`.
4. **Duration** — ascending `d`.

Ranking exists because of hub files. When `db.py` is touched and T0 is 8,000 tests,
ordering is the only thing standing between the agent and a full-suite wait.

### Hard-RED semantics

A changed instrumentable file with an empty T0 is an **uncovered change**. RTDD exits 1
and refuses to emit green:

```
$ rtdd cycle
  changed: src/auth/refresh.py  (new file)
  T0 → {}   no test covers this file

  RED: uncovered change
  Write a test whose execution reaches src/auth/refresh.py, then re-run.
```

This is not a special case grafted onto TDD — it *is* red-green-refactor, with the map
acting as a faster oracle for "is there a red?". It is also the claim that makes RTDD
stricter than the thing it replaces: a full suite run on the same change prints
`10000 passed` and tells the agent to move on.

## 6. The cycle

```
rtdd status                  # is the map seeded? stale? which adapter?
rtdd seed                    # one full instrumented run, builds the map
rtdd cycle --expect red      # after writing a test: T0 ranked, fail-fast, assert a red exists
rtdd cycle --expect green    # after implementing: full T0, assert all green, rewrite rows
rtdd verify                  # commit boundary: T2 full run
rtdd doctor                  # fan-out report — coupling hotspots
rtdd map compact             # collapse duplicate rows after a union merge
rtdd explain <file>          # which tests cover this file, and why it was selected
```

`--expect red` is a correctness check, not only a speed strategy. If the newly written
test **passes** before any implementation exists, the test is broken — asserting nothing,
or asserting something already true. RTDD fails the cycle on it. Standard TDD relies on
the developer noticing; RTDD enforces it.

`--expect green` runs the full T0 rather than stopping at first failure, because the
green phase's question is "do *all* affected tests pass", and rewrites the map rows for
every test it ran.

### Agent loop

```
1. rtdd status
2. write ONE test                    ← one behaviour, vertical slice
3. rtdd cycle --expect red           ← must fail, for the right reason
4. implement the minimum
5. rtdd cycle --expect green
6. refactor
7. rtdd cycle --expect green
8. repeat from 2
9. rtdd verify                       ← before commit / PR
```

## 7. Adapters

A language is a **declarative YAML file**. This is what makes "any programming language"
a real property rather than an aspiration — adding Elixir is a data file and a benchmark
row, not a pull request against the engine.

```yaml
name: python
detect: ["pytest.ini", "pyproject.toml", "setup.cfg"]
list:   "pytest --collect-only -q"
seed:   "pytest --cov --cov-context=test --cov-report=json:{out}"
subset: "pytest {tests} --cov --cov-context=test --cov-report=json:{out}"
parse:  coverage-json
failfast_flag: "-x"
opaque: ["**/*.yaml", "**/*.yml", "**/*.sql", "**/fixtures/**", "**/*.json"]
full_escalate: ["requirements.txt", "pyproject.toml", "**/conftest.py"]
```

| Key | Purpose |
|---|---|
| `detect` | Files whose presence selects this adapter |
| `list` | Enumerate test ids |
| `seed` / `subset` | Run all / run `{tests}`, with coverage, writing to `{out}` |
| `parse` | Coverage format: `coverage-json`, `lcov`, `cobertura`, `gocover`, `jacoco-xml` |
| `failfast_flag` | Appended in the red phase |
| `opaque` | Globs that escalate to T1 |
| `full_escalate` | Globs that escalate to T2 |

Where a mature TIA tool already exists, the adapter **delegates** to it rather than
reimplementing it. RTDD is not competing with `pytest-testmon`; it is giving an agent a
uniform protocol over it.

**v1 ships three first-class adapters:** Python (pytest + coverage.py), TypeScript/JS
(vitest/jest + v8/c8), Go (`go test -coverprofile`). Rust (`cargo-llvm-cov`) and Java
(JUnit + JaCoCo) follow in v1.1 through the same spec. Shipping five half-tested adapters
would poison the benchmark table, which is the repository's entire credibility.

## 8. `rtdd doctor`

Ranks source files by **fan-out** — the number of tests whose `f` contains them.

That number is a coupling metric, computed in milliseconds from data already on disk. A
file covered by 80% of the suite is a hub, and a hub is exactly where RTDD's selection
ratio collapses. `doctor` names them.

This is where the original "relational project structure" idea lands: not as a rule the
developer must follow, but as a **measurement** that makes coupling visibly, numerically
expensive. The pressure toward one-test-per-unit structure comes from the number, not
from a style guide.

```
$ rtdd doctor
  fan-out hotspots — files covered by the most tests

  src/db.py            8,142 tests   96.3%   ← selection ratio ~1.0, refactor candidate
  src/types.ts         5,004 tests   59.2%
  src/utils/index.ts   3,880 tests   45.9%

  p50 selection ratio: 1.8%   p90: 12.4%   worst: 96.3%
```

## 9. Benchmark

The benchmark is the repository's credibility. "20x faster" is worthless in isolation —
anything is fast if it runs fewer tests. Speed and safety are published **in the same
table**, or the claim is marketing.

### Method

Mutation-based fault injection over real open-source repositories, fixed seed,
CI-enforced on every release so the numbers cannot rot.

```
for each seeded mutant m in M:
    apply m
    F_full  = failing tests under a full suite run      # ground truth
    F_rtdd  = failing tests under rtdd selection
    detected(m) = |F_rtdd| > 0  whenever  |F_full| > 0
    record: selection ratio, wall-clock, detected
```

### Metrics

- **Selection ratio** (primary) — tests selected / tests total. Deterministic, machine-
  independent, portable across repos. This is the headline speed number.
- **Detection recall** (safety) — fraction of mutants that a full suite detects and RTDD
  also detects. The number that decides whether RTDD is a tool or a footgun.
- **Wall-clock delta** (secondary) — reported with disclosed hardware, never as the
  primary claim.

### Reporting rules

- Published as **p50 / p90 / worst**, never a bare mean. Hub-file changes are the worst
  case and they get printed.
- **Recall below 100% is printed and the misses are categorized** by cause (opaque file,
  stale row, dynamic dispatch, flaky test). Hiding a miss is the one thing that would
  make this repository worthless.
- The corpus covers **every adapter shipped in the release** — three at v1 (Python,
  TypeScript, Go), five at v1.1. An adapter without a benchmark row does not ship.
- At least one benchmark repository must have a **genuinely slow suite**. Benchmarking a
  30-second suite proves nothing about the 40-minute case RTDD exists to solve.

### Headline demo

A worked scenario, reproducible from a clone, where a full suite prints `10000 passed`
on uncovered code while RTDD prints `RED: uncovered change`. This sells the safety claim
in a way a table cannot.

## 10. Deferred to v2

- **Adaptive symbol-level granularity.** When a file's fan-out exceeds a threshold,
  upgrade it to per-symbol tracking with content checksums (checksums survive line
  shifts; line numbers do not). Huge win exactly where file-level fails. Requires
  tree-sitter parsing, which v1 deliberately avoids.
- **Time-budgeted selection.** `--budget 60s` runs ranked tests until exhausted, reporting
  partial verification. Deferred because probabilistic green sits badly beside v1's
  hard-RED semantics.
- **Rust and Java adapters.**
- **`rtdd explain --graph`** — optional graphify export for human-readable blast radius.
  Explicitly out of the hot path: LLM-extracted semantic edges are good for explanation
  and unfit as an execution gate.

## 11. Milestones

v1 is one coherent effort but four shippable slices. Each ends at a demonstrable state.

| # | Slice | Done when |
|---|---|---|
| **M1** | Engine + Python adapter | `seed`, `cycle --expect red/green`, `status`, hard-RED, map read/write/compact all work end to end on a real Python repo |
| **M2** | Node + Go adapters, `doctor`, `explain` | Same loop passes on a TS repo and a Go repo; fan-out report lands |
| **M3** | Benchmark harness | Mutation runner, corpus, result tables, `bench.yml` green in CI |
| **M4** | Protocol + front-ends + release | `PROTOCOL.md` generating `dist/`, drift check in CI, GoReleaser, README with the published table |

M3 is the gate on publishing. Nothing goes to the VocanicZ GitHub with an unbenchmarked
recall claim.

## 12. Repository layout

```
rtdd/
  cmd/rtdd/                 CLI entrypoint
  internal/
    mapstore/               JSONL read/write, union-merge resolution, compaction
    selector/               tier rules, ranking
    adapter/                YAML loading, detection, command templating
    coverage/               format parsers: coverage-json, lcov, cobertura, gocover, jacoco
    doctor/                 fan-out analysis
  adapters/
    python.yaml  node.yaml  go.yaml
  protocol/
    PROTOCOL.md             single source of the agent-facing instructions
  dist/                     GENERATED — never hand-edited
    SKILL.md                Claude Code
    AGENTS.md               Codex / Cursor / Copilot
    cursor/rules/rtdd.mdc   Cursor
  bench/
    harness/                mutation runner
    repos.yaml              benchmark corpus
    results/                committed result tables
  .github/workflows/
    ci.yml  bench.yml
```

`dist/` is generated from `protocol/PROTOCOL.md` by `make protocol`, and CI fails if the
generated files are out of date. Front-ends cannot drift.

## 13. Decisions of record

| # | Decision | Rationale |
|---|---|---|
| D1 | Coverage-derived edges, not static analysis | Ground truth: sees dynamic dispatch, DI, reflection. Zero LLM tokens in the hot path. Already transitive — no depth parameter, no leak. |
| D2 | File-level granularity | Stable under refactor; line numbers are not. Over-selection is safe. |
| D3 | Hard-RED on empty T0 | Makes RTDD stricter than full-suite TDD; the map's emptiness is the signal. |
| D4 | Go static binary | Zero runtime deps forced into the host repo; ~5ms startup against a per-cycle tax. |
| D5 | Committed sorted JSONL + union merge | Fleet worktrees inherit the map; conflicts widen, never narrow. |
| D6 | Accept hub files; rank + fail-fast; publish the distribution | Honest about the weak case; ranking preserves time-to-red where it matters most. |
| D7 | Mutation-based benchmark, recall published with speed | The only way the speed claim means anything. |
| D8 | Adapters as declarative YAML | Makes language-agnosticism structural rather than aspirational. |
| D9 | Three adapters at v1, not five | A half-tested adapter poisons the benchmark table. |
| D10 | `rtdd verify` advisory, not enforced | Mandating it would fight every adopting repo's existing CI. |

## 14. Open questions

- **Seed cost on very large suites.** A 40-minute instrumented seed may be 2–3× slower
  than an uninstrumented run. Needs measurement before recommending a default workflow;
  may justify a `rtdd seed --shard` mode for CI parallelism.
- **Per-test coverage collection cost per language.** `--cov-context=test` in Python is
  well-supported; per-test attribution in the JS and Go ecosystems is less uniform and
  may require running tests in smaller batches. This is an adapter-level measurement
  task and is the highest-risk unknown in v1.
- **Flaky tests.** A flaky failure inside T0 is indistinguishable from a real red. v1
  reports it as a red; whether RTDD should track flakiness in `s` is unresolved.
