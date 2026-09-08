# RTDD M6e — Evidence for the Static Tier, and Fidelity on Every Surface

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Measure what the `TS` static tier is actually worth against the one corpus where the coverage-derived answer is already known, and make every agent-facing surface state which fidelity it is reporting. Two halves. The **bench half** adds a `static` arm to `bench/results/{flask,httpie}/summary.{json,md}` **without re-running the benchmark** — it is derived offline from the records those runs already committed — and publishes the pre-registered comparison against `path`, win or lose. The **engine half** puts `selection_fidelity` on both `--json` surfaces, a static caveat in `warnings`, `(static)` on the human tier line, and removes the three surviving coverage claims `rtdd run` and `rtdd doctor` still make in a repository where nothing is ever instrumented. Nothing here runs a test, seeds a map, or touches `~/.cache/rtdd-bench`.

**Architecture:** The bench half rests on one fact: `bench/results/<repo>/commits.jsonl` already holds, per replayed commit, the `all_tests` the commit collected, the `changed` set, the per-test `durations_ms`, the newly-failing `f_full`, **and every strategy's `selected` list**. A selection that is a function of only those fields can therefore be computed after the fact and appended to the same file, and the `report --rebuild` path from issue #214 re-derives `summary.json` and `summary.md` from it. So `static` is a **derived arm**: `bench/replay/derive.py` computes it, `replay derive` writes it, `report --rebuild` publishes it, and nothing clones, provisions, materialises a worktree or executes pytest at any point. The engine half touches only the presentation layer: `internal/selector` is not opened, `Select` stays pure, and no tier, ranking or reason string changes — what changes is what `cmd/rtdd` and `internal/doctor` are willing to *say* about a selection.

**Tech Stack:** Go 1.24+ and stdlib `testing` for the engine half. Python 3.12 + pytest under `bench/` (a `uv` project, separate from `bench/swebench`) for the bench half. No new dependency on either side.

**Spec:** `docs/specs/2026-09-05-multi-language.md` **§6** (the honesty surfaces: the human tier line, `selection_fidelity`, the `warnings` caveat, `doctor`'s per-adapter fidelity, and the `PROTOCOL.md` → `SKILL.md`/`AGENTS.md`/`.mdc` regeneration) and **§7** (the evidence: the static arm over the existing flask and httpie replay corpus, recall / selection ratio / selected-duration fraction against `rtdd`, `path` and `full`, the `p50`/`p90`/`worst` rule, and the pre-registered kill condition). Each task below names its section and its PRD acceptance criteria on its `**Discharges:**` line, and the mapping table after the decisions lists all thirteen.

## Global Constraints

*(the first five are copied verbatim from `00-interfaces.md`)*

- **Go 1.24+**, module `github.com/VocanicZ/rtdd`.
- **Zero non-stdlib dependencies in the engine** except `modernc.org/sqlite` and
  `gopkg.in/yaml.v3`. This milestone adds none; `TestGoModRequiresExactlyYAMLAndSQLite`
  in `internal/contract` fails if it does.
- All paths inside the engine are **repo-relative, slash-separated, cleaned**. Conversion
  happens at the boundary (`internal/paths`), never ad hoc.
- Every exported function returns `error` rather than panicking. `cmd/` is the only place
  that prints to stdout/stderr.
- Table-driven tests, stdlib `testing`, golden files under `testdata/`.

M6e-specific. The first four are the PRD's own Global constraint and are constraints on
**every** task here, not concerns of the one task that would breach them:

- **`~/.cache/rtdd-bench` is never deleted, re-keyed or invalidated.** It holds 201 MB of
  ground truth, it is exported to every worktree by `.harness/worktree-hook.sh` as
  `RTDD_BENCH_CACHE`, and `bench/replay/cli.py`'s `CACHE` already reads that variable. No
  task in this plan reads it, writes it, changes its key, or runs anything that would. A
  task that finds itself needing the cache has taken the wrong route: everything the
  static arm needs is in `commits.jsonl`, which is in git.
- **No hardware fingerprint is added to `cache.key()`.** That is issue #218 — an open
  human decision — and doing it here orphans the cache above. `bench/replay/cache.py` is
  not in any task's file list.
- **No benchmark is re-run.** Not `replay`, not `session`, not a single pytest invocation
  against flask or httpie. Every number this milestone publishes is derived from records
  already committed under `bench/results/`. `replay_repo`, `clone_pinned`, `add_worktree`,
  `provision` and `runner.py` are not called by anything this plan adds.
- **`bench/results/sqlfluff/` is untouched.** It is a `corpus_version: 1` artifact that
  issue #184 dropped at `corpus_version: 2`. `cmd_rebuild` already skips a directory the
  corpus does not admit, and `test_rebuild_does_not_touch_a_repo_the_corpus_no_longer_admits`
  already pins that; Task 4 extends the same guard to `derive`, and Task 7 asserts the
  directory's bytes are unchanged after the regeneration.
- **`bench/results/` is committed and diffable.** `records.to_jsonl_lines` sorts and emits
  byte-stable lines, so a derivation that runs twice must produce a byte-identical
  `commits.jsonl`. Task 4 tests exactly that.
- **The engine's selection does not move.** `internal/selector` is not in any file list.
  No tier is added or renumbered, no ranking changes, no `Reason` string is reworded. A
  seeded Python repository's *selection* is byte-identical to before this milestone; what
  changes is the fidelity label printed beside it and the coverage claims removed from
  around it. If a selector test moves, the change is wrong.
- **`adapters/*.yaml` stay byte-frozen.** `internal/contract`'s sha256 guard covers them.
  No task here declares a `test_for` for Python, and the static arm's templates
  (decision 1) live in `bench/`, are named as a *model* of the tier, and never reach
  `adapters/python.yaml`.
- **A number that was never measured is never published.** The static arm executed no
  test, so it has no wall-clock record and none may be invented for it — decision 3 states
  what happens instead, and Task 6 tests it.
- **The pre-registered comparison is published win or lose.** §7's kill condition is not a
  contingency to skip if it looks unlikely; on the corpus as it stands today it is the
  branch that fires (see *What the corpus already says*). Task 14 writes it.

### The four under-determined decisions, resolved here

The PRD implies these and does not answer them. They are decided here so no implementer
has to guess, and each has a task that pins the decision with a test.

**1. The `static` arm is DERIVED from committed records, and this is the mechanism, in
full** (Tasks 2, 3, 4, 5).

The published tier, `TS`, admits a test on two signals and orders on three (spec §4.1,
resolved in `06-m6b-static-tier.md` decision 1):

| Level | Signal | Admits? |
|---|---|---|
| 1 | `test_for` correspondence resolving to an existing file | **yes** |
| 2 | transitive import distance, shortest first | **yes** |
| 3 | longest shared directory prefix | **no** — ordering only |

Level 3 never admits, so it cannot change the selected *set*, and every metric §7 asks
for — change-level recall, selection ratio, selected-duration fraction — is a function of
the set alone. **Ranking is therefore irrelevant to every number this milestone
publishes**, and the derivation does not reconstruct it. That is the whole reason an
offline derivation is faithful rather than approximate.

Level 1 is a pure function of `CommitRecord.changed` and `CommitRecord.all_tests`. The
engine resolves a `test_for` template against the filesystem (`Adapter.TestForCandidate`,
"the FIRST template naming a file the repository actually has"); offline, the set of test
files the commit *collected* stands in for the filesystem, which is strictly the more
conservative reading — a test file that exists but collects nothing cannot fail, so
admitting it would inflate the selection and never the recall.

Level 2 is **not** a function of those fields — it needs the import graph, which needs the
tree. But it does not need to be recomputed, because it was already measured: the
`importgraph` baseline is exactly "the test files that transitively import a changed
file", it ran on every replayed commit of both repos, and its answer is committed as a
`StrategyRecord`. So:

```
static(commit) = level1(commit.changed, commit.all_tests)  ∪  selected(importgraph, commit)
```

with `level1` expanding a fixed, literal template list against the collected test files.
Both halves come out of `commits.jsonl`. Nothing is cloned, provisioned, materialised or
executed. `replay derive` rewrites `commits.jsonl` with the new `StrategyRecord`s folded
in; `report --rebuild` re-derives `summary.json` and `summary.md` from it, exactly as
issue #214 built it to.

Three consequences that must be stated in `summary.md` rather than left for a reader to
infer, and Task 6 writes them:

- **`static ⊇ importgraph` by construction.** The static arm beating the import-graph
  baseline is arithmetic, not a finding, and must not be reported as one. The
  pre-registered comparison is against `path`, which it does **not** contain: `path`
  matches a test file by *basename anywhere in the tree*, level 1 matches only
  template-resolved paths, so neither set contains the other and the comparison is real.
- **The arm models a Python static adapter that does not ship.** `adapters/python.yaml`
  declares no `test_for` and no `importscan` and stays byte-frozen, so the shipped tool
  would never reach `TS` on flask or httpie. The `static` row answers "what would the
  static tier have selected here", which is precisely §7's question, and `summary.md`
  says so in those words.
- **The templates are a published input, not an implementation detail.** They live in one
  module constant, they are printed in `summary.md`, and changing them changes the number.

**2. `static` is a derived arm and is deliberately NOT in `strategies.REGISTRY`** (Task 3).

`bench/tests/test_registry_complete.py` asserts `set(all_ids()) == REQUIRED` — exact
equality, by design: "an eighth entry appearing here means the registry grew a strategy
the PRD does not describe". Registry membership means *the orchestrator may execute this
against a materialised worktree*, and this arm is never executed — executing it is the one
thing the PRD forbids. So `static` is not registered, that test is left exactly as it is,
and a new test asserts the absence with its reason attached. The arm reaches the published
tables through a second, explicit route: `derive.DERIVED_ARMS`, and a `_rebuild_repo` that
publishes every arm the *records* carry rather than only the arms `config.json` lists
(Task 5). That change is also what keeps `config.json` untouchable — it stamps the run
that produced the numbers, a rebuild has no standing to rewrite it, and
`test_rebuild_leaves_the_run_config_that_stamped_the_result_untouched` already says so.

**3. The static arm has no wall-clock row, and the absence is explained rather than
filled** (Task 6).

Nothing executed, so no `WallClockRecord` exists, so `wallclock_table` — which builds its
rows purely from `output.wallclocks` — emits no `static` row, and `_wallclock_rows`
iterates what is there. That is already the correct behaviour and no code makes it happen.
What is missing is that a reader sees eight arms in the headline table and seven under
`## Wall-clock` with nothing saying why. Three options were considered and two are
refusals:

- Fabricating a timing from `durations_ms` — refused. `durations_ms` is per-test time from
  a *different* execution; summing it produces a plausible number no run ever took, on
  hardware the row would then claim.
- Copying `importgraph`'s row — refused, and worse: it attributes another arm's
  measurement to this one.
- **Publishing the absence**: `summary.json` gains a `derived_arms` list, and
  `render_markdown` emits one sentence under `## Wall-clock` naming each derived arm and
  saying it executed nothing and therefore has no timing. The cost column that *is*
  published for it is `selected_duration_fraction`, which is a ratio of the same
  commit's own recorded per-test durations and is therefore machine-independent.

The same applies to `## Isolation`, which is also built from the wall-clock records: no
`static` row, and the sentence covers both. `StrategyRecord.select_ms` for a derived
record is `0` and is not a measurement either — hence the new `derived: bool` field
(Task 1), which is what a reader and a test can key on.

**4. `selection_fidelity` is a property of the SELECTION, not of the adapter** (Task 9).

`Adapter.Fidelity()` already answers "what is the best this adapter could ever produce",
and `rtdd doctor` already publishes it per adapter (M6a). §6's `selection_fidelity` is a
different question — "what was *this* answer derived from" — and the two genuinely differ:
a coverage adapter with an unseeded map produces a T2 full-suite escalation whose fidelity
is `none`, while `Fidelity()` says `execution-derived`. Derived from the tier that
answered:

| Tier | `selection_fidelity` | Why |
|---|---|---|
| `direct`, `T0`, `T1` | `execution-derived` | the map answered; those rows were recorded during real execution |
| `empty` | `execution-derived` | reachable only from a usable map — the map answered, and its answer was "nothing" |
| `TS` | `static` | declared correspondence and imports, by construction |
| `T2` | `none` | nothing was derived: the full suite is what you run when no relation could answer |

`empty` is the one that repays stating. `TierEmpty` is only ever reached with a seeded map
that matched nothing, so it is an execution-derived *answer*; labelling it `none` would
say "no evidence" about the one outcome the `warnings` array already has to defend as a
real result. The value is never null and never absent: it is a plain `string` field on a
struct, so a zero value cannot marshal to `null`, and Task 9 tests every tier.

### Which task covers which spec section and which PRD acceptance criterion

| Task | Spec | PRD #233 AC |
|---|---|---|
| 1 — `StrategyRecord.derived` | §7 | precondition for 2, 4 |
| 2 — level-1 correspondence, pure over `CommitRecord` | §7 | 1 |
| 3 — `derive_static` composes level 1 with the committed `importgraph` record | §7 | 1 |
| 4 — the `replay derive` verb | §7 | 1, 2, 5 |
| 5 — `_rebuild_repo` publishes every arm the records carry | §7 | 2 |
| 6 — the static verdict, the kill condition, the derived-arm wall-clock sentence | §7 | 3, 4, 12 |
| 7 — regenerate flask and httpie; sqlfluff untouched | §7 | 2, 3, 5 |
| 8 — the bench replay gate in CI | §7 | 13 |
| 9 — `selection_fidelity` on `which --json` and `run --json` | §6 | 6 |
| 10 — the static warning and `tier: TS (static)` | §6 | 7, 8 |
| 11 — `run` stops claiming coverage it does not have | §6 | 9a, 9b, 9 (the end-to-end test) |
| 12 — the fan-out caveat is suppressed where fan-out cannot be computed | §6 | 9c |
| 13 — `PROTOCOL.md`, the regenerated front-ends, and the drift check | §6 | 10 |
| 14 — `README.md`, `docs/outcomes/`, and the pre-registered verdict | §6, §7 | 11, 12 |

AC13 (`scripts/ci-local.sh` exits 0) is discharged by Task 8 and re-asserted by every
task's step 4 and by the Definition of Done.

## What is already true in the tree

Read these before writing code; several tasks are much smaller than they look.

- `bench/replay/records.py` — `CommitRecord` already carries `all_tests`, `durations_ms`,
  `f_full`, `pre_existing_failures` and `changed`; `StrategyRecord` already carries
  `selected`, `escalated`, `reason`, `select_ms` and `stale_dropped`. `stale_dropped` is
  the precedent Task 1 follows exactly: a field with a default, emitted by `to_dict`, read
  back with `d.get(...)` so an older line still parses. `to_jsonl_lines` sorts and emits
  byte-stable lines; `parse_jsonl_lines` reads them back.
- `bench/replay/cli.py:299` — `_rebuild_repo` already reads `commits.jsonl` and
  `config.json` back, rebuilds a `ReplayOutput`, carries `skipped`, `rtdd_run_errors` and
  a `--no-wallclock` refusal across from the prior summary, and re-renders. Its one
  hard-coded assumption is `strategy_order(cfg.strategies)` — the arm list comes from the
  run's config rather than from the records. Task 5 is that one line.
- `bench/replay/cli.py:349` — `cmd_rebuild` already skips a results directory the corpus
  does not admit and already refuses `commits.jsonl` with no `config.json`. Task 4 reuses
  both guards rather than inventing new ones.
- `bench/replay/report.py:186` — `wallclock_table` builds rows only from
  `output.wallclocks`, and `_wallclock_rows` iterates what it built. An arm with no
  records is already absent from both. Nothing needs to be suppressed; a sentence needs to
  be added.
- `bench/replay/report.py:488` — `render_markdown` audits its own output with
  `assert_distribution_beside_mean` before returning, so AC4's guard covers whatever the
  renderer grows into, including everything Task 6 adds. It does not need to be re-invoked.
- `bench/replay/report.py:273` — `verdict_line` and `secondary_verdict_lines` are the
  shape Task 6's static verdict copies: one primary-variant line, then one labelled line
  per non-primary variant, `probe` carrying its upper-bound label. `SUCCESS_WORDING` and
  `FAILURE_WORDING` already exist.
- `bench/replay/metrics.py:104` — `change_level_recall` divides by the *detecting*
  commits, so a population with no failing commit yields `Ratio(0, 0)` whose `value` is
  `None` and whose verdict is `not computable`. This is not a hypothetical: see below.
- `bench/tests/test_registry_complete.py` — asserts exact set equality on the registry.
  Decision 2 exists because of this file, and this file does not change.
- `bench/tests/test_cli.py:607` — `_published_repo` builds a throwaway results directory
  from records; the `bench` fixture at line 102 monkeypatches `cli.RESULTS` and friends at
  a `tmp_path`. Tasks 4 and 5 write their tests in that shape.
- `internal/adapter/fidelity.go` — `Fidelity()` and the three wire strings
  (`execution-derived`, `static`, `none`) already exist and are already the §6 vocabulary.
  Task 9 consumes them; it does not add a fourth or restate them.
- `internal/doctor/doctor.go:56` — `StaticCaveat` already exists, already states both
  halves (what a static selection can miss, and that passing it is weaker evidence), and
  `rtdd doctor` already prints it. Task 10 puts **that same string** into `warnings` —
  AC7 is a new *surface* for an existing sentence, not a new sentence.
- `cmd/rtdd/whichtext.go:64` — `unmappedNoticeApplies` is the precedent for every
  suppression in Tasks 11 and 12: gate on `ad.Selection == adapter.SelectionStatic`,
  suppress rather than reword, and keep a nil adapter on the pre-existing path.
- `cmd/rtdd/run.go:333` — the map-rows line is one `fmt.Printf`. `cmd/rtdd/run.go`'s JSON
  branch passes `UncoveredOK: true` unconditionally, which is the machine-surface half of
  the same AC9a defect.
- `cmd/rtdd/jsonout.go` — `Output`'s field order is the emitted key order, and
  `JSONUncovered.MarshalJSON` already distinguishes "available, nothing uncovered" from
  "not available" by nil-ness of the `Files` pointer, with `unavailableReason` as the
  existing precedent for saying why.
- `protocol/PROTOCOL.md` — nine sections at `order=10..90`; the `json` section still lists
  `tier` as "one of `empty`, `direct`, `T0`, `T1`, `T2`", which M6b left stale. Task 13
  fixes that in the same edit that adds fidelity. `internal/install/protocol.md` is an
  embedded copy `ci-local.sh` diffs against the source, and `dist/` is generated by
  `rtdd-gen render`.
- `scripts/ci-local.sh` — runs the Go suite, `rtdd-gen check`/`verify`, the protocol diff
  and `scripts/ci-prereg.sh` (which is `bench/swebench` only). **It does not run the
  `bench/` replay suite at all**, and neither does `.github/workflows/ci.yml`, which runs
  only `test_audit.py`, `test_corpus.py` and `test_results_corpus_coverage.py` there.
  Task 8 closes that, because from this milestone on the published numbers are produced by
  code that would otherwise be ungated.

## What the corpus already says (measured, not assumed)

The derivation in decision 1 was run against the committed records before this plan was
written, and its `path`, `importgraph` and `rtdd` columns reproduce
`bench/results/*/summary.json` exactly, which is what makes the `static` column credible.
The result:

| repo / variant | arm | detecting commits | change recall | selection ratio | selected duration |
|---|---|---|---|---|---|
| flask / `natural` | path | 0 | n/a | 0.050 | 0.056 |
| flask / `natural` | static | 0 | n/a | 0.050 | 0.056 |
| flask / `probe` | path | 3 | 0.333 | 0.012 | 0.010 |
| flask / `probe` | static | 3 | 0.333 | 0.012 | 0.010 |
| flask / `probe` | rtdd | 3 | 1.000 | 0.220 | 0.243 |
| httpie / `natural` | path | 0 | n/a | 0.032 | 0.032 |
| httpie / `natural` | static | 0 | n/a | 0.416 | 0.426 |
| httpie / `probe` | static | 0 | n/a | 0.277 | 0.286 |

Three things follow, and every one of them shapes a task:

1. **`natural` — the primary variant — has no detecting commits in either repo.** Real
   commits are pushed green. The pre-registered static-vs-`path` comparison is therefore
   `not computable` on the primary variant of both repos, exactly as the existing
   rtdd-vs-`path` verdict already is. Task 6's verdict must have the same
   primary-plus-labelled-secondary shape as `verdict_line`/`secondary_verdict_lines`, and
   an implementer who writes a single unconditional line will publish `not computable` and
   nothing else.
2. **On the only population with ground truth — flask / `probe`, three detecting
   commits — `static` ties `path` and does not beat it.** AC12's branch is the live one,
   not the unlikely one. Task 14 writes it.
3. **These numbers are the expected result, not the published one.** Every task
   re-derives from the records; nothing in the implementation may hard-code a figure from
   this table, and Task 7 publishes whatever the code actually produces. If the
   implementation disagrees with this table, the implementation is what is true — and the
   disagreement is worth understanding before committing, because the prototype matched
   three existing published arms to three decimal places.

## File Structure

| File | Single responsibility |
|---|---|
| `bench/replay/records.py` | **MODIFIED** — `StrategyRecord.derived`, defaulted and round-tripped |
| `bench/tests/test_records.py` | **MODIFIED** — the new field's default, wire shape and back-compat |
| `bench/replay/derive.py` | **NEW** — `STATIC_TEST_FOR`, `test_for_candidate`, `level1_tests`, `derive_static`, `DERIVED_ARMS` |
| `bench/tests/test_derive.py` | **NEW** — template resolution, declaration order, the union, idempotence, the registry absence |
| `bench/replay/cli.py` | **MODIFIED** — the `derive` subcommand; `_rebuild_repo` publishes the arms the records carry |
| `bench/tests/test_cli.py` | **MODIFIED** — `derive` guards, idempotence, sqlfluff untouched, rebuild picks up a derived arm |
| `bench/replay/report.py` | **MODIFIED** — `static_verdict_line`, `static_secondary_verdict_lines`, `derived_arms` in the summary, the wall-clock sentence, the model disclosure |
| `bench/tests/test_report.py` | **MODIFIED** — the verdict both ways, the kill condition, no fabricated wall-clock row |
| `bench/results/flask/{commits.jsonl,summary.json,summary.md}` | **REGENERATED** — from their own records |
| `bench/results/httpie/{commits.jsonl,summary.json,summary.md}` | **REGENERATED** — from their own records |
| `bench/results/aggregate.md` | **REGENERATED** — `report` re-renders it from the two summaries |
| `bench/tests/test_results_corpus_coverage.py` | **MODIFIED** — every admitted repo publishes the `static` arm and its verdict |
| `scripts/ci-local.sh` | **MODIFIED** — a `bench replay gate` step; nothing existing is removed or weakened |
| `.github/workflows/ci.yml` | **MODIFIED** — the same step, so the push gate and the local gate agree |
| `cmd/rtdd/fidelity.go` | **NEW** — `selectionFidelity(tier) adapter.Fidelity`, the decision-4 table |
| `cmd/rtdd/fidelity_test.go` | **NEW** — every tier, and the never-null guarantee |
| `cmd/rtdd/jsonout.go` | **MODIFIED** — `selection_fidelity` on `Output` and on `JSONAdapterSelection`; `UncoveredOK` honoured |
| `cmd/rtdd/jsonout_test.go` | **MODIFIED** — the field's presence, value and key order |
| `cmd/rtdd/whichtext.go` | **MODIFIED** — `tier: TS (static)` |
| `cmd/rtdd/whichtext_test.go` | **MODIFIED** — the suffix, and its absence on a coverage tier |
| `cmd/rtdd/which.go` | **MODIFIED** — `whichNotes` gains the static caveat |
| `cmd/rtdd/which_test.go` | **MODIFIED** — the caveat reaches both stderr and `warnings` |
| `cmd/rtdd/run.go` | **MODIFIED** — no uncovered report and no map-rows claim for a `coverage: none` repository; `runNotes` gains the same caveat |
| `cmd/rtdd/run_test.go` | **MODIFIED** — AC9a, AC9b and the end-to-end neither-string assertion |
| `cmd/rtdd/doctor.go` | **MODIFIED** — the fan-out caveat is printed only where fan-out can be computed |
| `cmd/rtdd/doctor_test.go` | **MODIFIED** — suppressed for a static-only repo, unchanged for a coverage repo |
| `protocol/PROTOCOL.md` | **MODIFIED** — the fidelity section, `TS` in the tier list, `selection_fidelity` in the JSON block |
| `internal/install/protocol.md` | **REGENERATED** — the embedded copy `ci-local.sh` diffs |
| `dist/SKILL.md`, `dist/AGENTS.md`, `dist/cursor/*.mdc` | **REGENERATED** — `rtdd-gen render` |
| `internal/contract/contract_test.go` | **MODIFIED** — the drift check covers the new text |
| `README.md`, `docs/outcomes/README.positive.md` | **MODIFIED** — byte-identical, same commit |
| `docs/outcomes/README.negative.md` | **MODIFIED** — the same change in its own prose |
| `docs/plans/00-interfaces.md` | **MODIFIED** — `selection_fidelity` recorded beside `warnings` |

---

### Task 1 — `bench/replay/records.py`: a derived record says so

**Discharges:** spec §7. Precondition for AC2 and AC4 — nothing else can distinguish a
measured arm from a computed one, and both the wall-clock sentence and the guard against
fabricated timings key on it.

**Files:**
- Modify: `bench/replay/records.py`
- Test: `bench/tests/test_records.py`

**Interfaces:**
- Consumes: nothing new.
- Produces: `StrategyRecord.derived: bool = False`, emitted by `to_dict`, read back with
  `.get` so every already-committed line still parses as `derived: false`.

- [ ] **Step 1: Write the failing test**

Append to `bench/tests/test_records.py`:

```python
def test_strategy_record_defaults_to_measured():
    """Every record written before M6e came from a strategy that really ran."""
    rec = StrategyRecord("flask", "c1", "natural", "path", ("a",), False, "sib", 3)
    assert rec.derived is False
    assert rec.to_dict()["derived"] is False


def test_a_derived_record_is_marked_on_the_wire():
    """`select_ms` on a derived record is 0 because nothing was timed, not because it was
    fast. `derived` is the field that says which of those two a 0 means, and it has to
    survive the round-trip or a reader cannot tell them apart at all."""
    rec = StrategyRecord(
        "flask", "c1", "natural", "static", ("a", "b"), False, "derived offline", 0,
        derived=True,
    )
    d = rec.to_dict()
    assert d["derived"] is True
    assert StrategyRecord.from_dict(d) == rec


def test_an_already_committed_line_reads_back_as_measured():
    """bench/results/*/commits.jsonl predates this field. A missing key is `false`, not a
    KeyError: the published records are the input to every derivation and re-writing them
    to add a default would be a schema migration of committed ground truth."""
    old = {
        "kind": "strategy", "repo_id": "flask", "commit": "c1", "variant": "natural",
        "strategy": "path", "selected": ["a"], "n_selected": 1, "escalated": False,
        "reason": "sib", "select_ms": 3, "stale_dropped": [], "n_stale_dropped": 0,
    }
    assert StrategyRecord.from_dict(old).derived is False
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bench && uv run pytest tests/test_records.py -k derived -q`
Expected: FAIL — `TypeError: StrategyRecord.__init__() got an unexpected keyword argument 'derived'` on the second test, and `KeyError: 'derived'` on the first.

- [ ] **Step 3: Write minimal implementation**

In `bench/replay/records.py`, on `StrategyRecord`, after `stale_dropped`:

```python
    #: True when this record was COMPUTED from other committed records rather than
    #: produced by a strategy that ran. A derived arm executed nothing, so its
    #: `select_ms` is 0 because nothing was timed and it has no `WallClockRecord` at
    #: all — see `replay.derive`. Defaulted and read back with `.get` so every line
    #: already committed under `bench/results/` parses unchanged.
    derived: bool = False
```

Add `"derived": self.derived` to `to_dict` (after `n_stale_dropped`, so the key order the
sorted JSON emits is unaffected — `json.dumps(sort_keys=True)` orders it regardless) and
`derived=bool(d.get("derived", False))` to `from_dict`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bench && uv run pytest tests/test_records.py -q`
Expected: PASS, every pre-existing record test included.

- [ ] **Step 5: Commit**

```bash
git add bench/replay/records.py bench/tests/test_records.py
git commit -m "M6e: StrategyRecord.derived distinguishes a computed arm from a measured one"
```

---

### Task 2 — `bench/replay/derive.py`: level-1 `test_for` correspondence, pure over one `CommitRecord`

**Discharges:** spec §7 (the static arm over the existing corpus). PRD AC1.

**Files:**
- Create: `bench/replay/derive.py`
- Test: `bench/tests/test_derive.py`

**Interfaces:**
- Consumes: `replay.records.CommitRecord`.
- Produces:
  - `STATIC_TEST_FOR: tuple[str, ...]` — the model's templates, in declaration order
  - `test_for_candidate(rel: str, test_files: Set[str]) -> str | None`
  - `level1_tests(rec: CommitRecord) -> tuple[str, ...]`

- [ ] **Step 1: Write the failing test**

Create `bench/tests/test_derive.py`:

```python
"""The offline model of the `TS` static tier, level 1 — declared correspondence.

Level 1 is the half that is a pure function of `CommitRecord`: the changed set and
the test ids the commit collected are all it reads. The engine resolves a `test_for`
template against the filesystem; here the collected test files stand in for it, which
is the more conservative reading — a test file that exists but collects nothing cannot
fail, so admitting it would inflate the selection and never the recall.
"""

from __future__ import annotations

from replay.derive import STATIC_TEST_FOR, level1_tests, test_for_candidate
from replay.records import CommitRecord


def commit(changed, all_tests) -> CommitRecord:
    return CommitRecord(
        repo_id="synth", commit="c1", parent="c0", variant="natural",
        all_tests=tuple(all_tests), durations_ms={t: 1 for t in all_tests},
        f_full=(), pre_existing_failures=(), changed=tuple(changed),
    )


def test_the_templates_are_declared_in_confidence_order():
    """The order IS the model, and it is published in summary.md. A co-located test is
    more specific evidence than a same-named file in a central tests/ directory, so it is
    tried first — the same reason `Adapter.TestForCandidate` tries templates in the order
    the adapter author declared them."""
    assert STATIC_TEST_FOR == (
        "{dir}/test_{name}.py",
        "{dir}/tests/test_{name}.py",
        "tests/{subdir}/test_{name}.py",
        "tests/test_{name}.py",
    )


def test_the_first_template_naming_a_collected_file_wins():
    files = {"src/flask/tests/test_app.py", "tests/test_app.py"}
    assert test_for_candidate("src/flask/app.py", files) == "src/flask/tests/test_app.py"


def test_a_template_that_names_nothing_collected_is_skipped():
    """Expansion alone is not evidence: every template always expands, so a candidate
    counts only when the commit actually collected that file."""
    assert test_for_candidate("src/flask/app.py", {"tests/test_other.py"}) is None


def test_subdir_mirrors_a_suffix_of_the_source_tree_longest_first():
    """`{subdir}` takes the changed file's directory, then that directory with one leading
    segment dropped, and so on — `internal/adapter/testfor.go`'s `trailingDirs`. A test
    tree usually mirrors a suffix of the source tree, not the whole of it."""
    files = {"tests/flask/test_app.py"}
    assert test_for_candidate("src/flask/app.py", files) == "tests/flask/test_app.py"


def test_a_changed_test_file_contributes_its_own_collected_ids():
    """The engine's direct tier always runs a changed test file. The model does the same,
    and only for a file the commit collected — a deleted test cannot run."""
    rec = commit(["tests/test_app.py"], ["tests/test_app.py::a", "tests/test_other.py::b"])
    assert level1_tests(rec) == ("tests/test_app.py::a",)


def test_a_non_python_change_contributes_nothing():
    """A changed CHANGES.rst has no module and no correspondence. The tier under-selects
    there, and that is the property being measured, not a bug to paper over."""
    rec = commit(["CHANGES.rst"], ["tests/test_app.py::a"])
    assert level1_tests(rec) == ()


def test_level1_expands_a_resolved_file_to_every_id_it_collected():
    rec = commit(
        ["src/flask/app.py"],
        ["tests/test_app.py::a", "tests/test_app.py::b", "tests/test_cli.py::c"],
    )
    assert level1_tests(rec) == ("tests/test_app.py::a", "tests/test_app.py::b")
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bench && uv run pytest tests/test_derive.py -q`
Expected: FAIL at collection — `ModuleNotFoundError: No module named 'replay.derive'`, reported as an error for all seven tests.

- [ ] **Step 3: Write minimal implementation**

Create `bench/replay/derive.py` with the module docstring stating decision 1 in full, and:

```python
STATIC_TEST_FOR: tuple[str, ...] = (
    "{dir}/test_{name}.py",
    "{dir}/tests/test_{name}.py",
    "tests/{subdir}/test_{name}.py",
    "tests/test_{name}.py",
)
"""The `test_for` templates this model gives a Python static adapter.

`adapters/python.yaml` declares none and stays byte-frozen — it is the coverage
adapter, and the shipped tool would never reach `TS` on flask or httpie. These four
are the model, they are a PUBLISHED input rather than an implementation detail
(`summary.md` prints them), and changing them changes the number.
"""


def _trailing_dirs(d: str) -> list[str]:
    """The values `{subdir}` takes, longest first — `trailingDirs` in testfor.go."""
    if not d or d == ".":
        return [d]
    segs = d.split("/")
    return ["/".join(segs[i:]) for i in range(len(segs))]


def test_for_candidate(rel: str, test_files: AbstractSet[str]) -> str | None:
    """The first template naming a file this commit COLLECTED, or None."""
    d = posixpath.dirname(rel)
    name = posixpath.splitext(posixpath.basename(rel))[0]
    subs = _trailing_dirs(d)
    for tmpl in STATIC_TEST_FOR:
        tries = subs if "{subdir}" in tmpl else subs[:1]
        for sub in tries:
            cand = posixpath.normpath(
                tmpl.replace("{dir}", d).replace("{subdir}", sub).replace("{name}", name)
            )
            if cand in ("", "."):
                continue
            if cand in test_files:
                return cand
    return None


def level1_tests(rec: CommitRecord) -> tuple[str, ...]:
    """Every collected id in the test files this commit's changed set corresponds to."""
    test_files = {t.split("::", 1)[0] for t in rec.all_tests}
    hit: set[str] = set()
    for rel in rec.changed:
        if not rel.endswith(".py"):
            continue
        if rel in test_files:
            hit.add(rel)
            continue
        cand = test_for_candidate(rel, test_files)
        if cand is not None:
            hit.add(cand)
    return tuple(t for t in rec.all_tests if t.split("::", 1)[0] in hit)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bench && uv run pytest tests/test_derive.py -q`
Expected: PASS, 7 passed.

- [ ] **Step 5: Commit**

```bash
git add bench/replay/derive.py bench/tests/test_derive.py
git commit -m "M6e: level-1 test_for correspondence, derived from a CommitRecord alone"
```

---

### Task 3 — `bench/replay/derive.py`: `derive_static` composes level 1 with the committed `importgraph` record

**Discharges:** spec §7. PRD AC1. Decisions 1 and 2.

**Files:**
- Modify: `bench/replay/derive.py`
- Test: `bench/tests/test_derive.py`

**Interfaces:**
- Consumes: `CommitRecord`, `StrategyRecord`.
- Produces:
  - `DERIVED_ARMS: tuple[str, ...] = ("static",)`
  - `LEVEL2_SOURCE: str = "importgraph"`
  - `derive_static(records: Sequence[object]) -> list[StrategyRecord]`
  - `class DerivationError(ValueError)`

- [ ] **Step 1: Write the failing test**

Append to `bench/tests/test_derive.py`:

```python
import pytest

from replay.derive import DERIVED_ARMS, DerivationError, derive_static
from replay.records import StrategyRecord
from replay.strategies.base import all_ids


def ig(selected, commit_id="c1") -> StrategyRecord:
    return StrategyRecord("synth", commit_id, "natural", "importgraph", tuple(selected),
                          False, "static import closure", 7)


def test_static_is_level1_united_with_the_committed_import_graph():
    """Level 2 — transitive imports — is not a function of CommitRecord's fields, and it
    does not have to be: the importgraph baseline already measured exactly that question
    on every replayed commit, and its answer is committed."""
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a", "tests/test_cli.py::c"])
    out = derive_static([rec, ig(["tests/test_cli.py::c"])])
    assert len(out) == 1
    assert out[0].strategy == "static"
    assert set(out[0].selected) == {"tests/test_app.py::a", "tests/test_cli.py::c"}


def test_a_derived_record_carries_no_measurement_and_says_so():
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a"])
    out = derive_static([rec, ig([])])
    assert out[0].derived is True
    assert out[0].select_ms == 0
    assert out[0].escalated is False
    assert "derived" in out[0].reason and "importgraph" in out[0].reason


def test_the_selection_is_ordered_so_the_record_is_byte_stable():
    """bench/results/ is committed and diffable; a set iterated in hash order would make
    the same derivation produce a different file on a different run."""
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::b", "tests/test_app.py::a"])
    first = derive_static([rec, ig(["tests/test_app.py::a"])])
    second = derive_static([rec, ig(["tests/test_app.py::a"])])
    assert first[0].selected == second[0].selected
    assert list(first[0].selected) == sorted(first[0].selected)


def test_an_id_the_commit_never_collected_is_refused():
    """Scoring an id that cannot match ground truth would silently inflate the
    denominator — `UnknownTestError`'s argument in strategies/base.py, applied to a
    derivation that reads a peer's record rather than a strategy's answer."""
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a"])
    with pytest.raises(DerivationError, match="never collected"):
        derive_static([rec, ig(["tests/test_ghost.py::z"])])


def test_a_commit_with_no_import_graph_record_is_refused_not_guessed():
    """A missing level-2 half is missing DATA. Treating it as an empty selection would
    publish a level-1-only arm under a name that claims both levels."""
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a"])
    with pytest.raises(DerivationError, match="importgraph"):
        derive_static([rec])


def test_re_deriving_replaces_rather_than_duplicates():
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a"])
    once = derive_static([rec, ig([])])
    twice = derive_static([rec, ig([]), *once])
    assert twice == once


def test_static_is_not_in_the_strategy_registry():
    """REGISTRY membership means the orchestrator may EXECUTE this arm against a
    materialised worktree, and executing it is the one thing PRD #233 forbids. The arm
    reaches the published tables through DERIVED_ARMS instead, and
    test_registry_complete.py's exact-equality assertion stays exactly as it is."""
    assert "static" in DERIVED_ARMS
    assert "static" not in all_ids()
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bench && uv run pytest tests/test_derive.py -q`
Expected: FAIL at collection — `ImportError: cannot import name 'DERIVED_ARMS' from 'replay.derive'`.

- [ ] **Step 3: Write minimal implementation**

Add to `bench/replay/derive.py`:

```python
DERIVED_ARMS: tuple[str, ...] = ("static",)
LEVEL2_SOURCE = "importgraph"

DERIVED_REASON = (
    "derived offline from committed records: level 1 test_for correspondence over "
    "all_tests, united with the committed importgraph selection for level 2; "
    "nothing was executed"
)


class DerivationError(ValueError):
    """The records cannot support a derivation, and guessing would publish a number."""


def derive_static(records: Sequence[object]) -> list[StrategyRecord]:
    commits = [r for r in records if isinstance(r, CommitRecord)]
    level2 = {
        (r.commit, r.variant): r
        for r in records
        if isinstance(r, StrategyRecord) and r.strategy == LEVEL2_SOURCE
    }
    out: list[StrategyRecord] = []
    for rec in commits:
        src = level2.get((rec.commit, rec.variant))
        if src is None:
            raise DerivationError(
                f"{rec.repo_id} {rec.commit} ({rec.variant}): no {LEVEL2_SOURCE} record; "
                "the static arm's level 2 is that record and cannot be guessed"
            )
        known = set(rec.all_tests)
        unknown = sorted(set(src.selected) - known)
        if unknown:
            raise DerivationError(
                f"{rec.repo_id} {rec.commit} ({rec.variant}): {LEVEL2_SOURCE} selected "
                f"{unknown[0]!r}, which this commit never collected"
            )
        selected = tuple(sorted(set(level1_tests(rec)) | set(src.selected)))
        out.append(
            StrategyRecord(
                repo_id=rec.repo_id, commit=rec.commit, variant=rec.variant,
                strategy="static", selected=selected, escalated=False,
                reason=DERIVED_REASON, select_ms=0, derived=True,
            )
        )
    return out
```

`test_re_deriving_replaces_rather_than_duplicates` passes without a special case: the
function reads only `CommitRecord`s and `importgraph` records, so a previously derived
`static` record in the input is ignored, and Task 4 is what drops the stale ones from the
file.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bench && uv run pytest tests/test_derive.py tests/test_registry_complete.py -q`
Expected: PASS — 14 passed. `test_registry_complete.py` is included because decision 2's whole point is that it must still pass unmodified.

- [ ] **Step 5: Commit**

```bash
git add bench/replay/derive.py bench/tests/test_derive.py
git commit -m "M6e: derive_static composes level 1 with the committed importgraph record"
```

---

### Task 4 — `bench/replay/cli.py`: the `replay derive` verb

**Discharges:** spec §7. PRD AC1, AC2, AC5.

**Files:**
- Modify: `bench/replay/cli.py`
- Test: `bench/tests/test_cli.py`

**Interfaces:**
- Consumes: `replay.derive.derive_static`, `DERIVED_ARMS`, `DerivationError`.
- Produces: `python -m replay.cli derive [--corpus-version N]` → `cmd_derive(args) -> int`,
  rewriting each admitted repo's `commits.jsonl` in place.

- [ ] **Step 1: Write the failing test**

Append to `bench/tests/test_cli.py`:

```python
# --- deriving the static arm from the committed records (#340) ------------


def _published_with_importgraph(results: pathlib.Path, repo_id: str) -> pathlib.Path:
    d = _published_repo(results, repo_id)
    lines = (d / "commits.jsonl").read_text(encoding="utf-8").splitlines()
    records = parse_jsonl_lines(lines)
    records.append(
        StrategyRecord(repo_id, "c1", "natural", "importgraph", ("b",), False, "closure", 4)
    )
    (d / "commits.jsonl").write_text("".join(to_jsonl_lines(records)), encoding="utf-8")
    return d


def test_derive_appends_a_static_record_per_commit(bench, capsys):
    results = pathlib.Path(cli.RESULTS)
    d = _published_with_importgraph(results, "synth")
    assert cli.main(["derive"]) == cli.EXIT_OK
    recs = parse_jsonl_lines((d / "commits.jsonl").read_text(encoding="utf-8").splitlines())
    static = [r for r in recs if isinstance(r, StrategyRecord) and r.strategy == "static"]
    assert len(static) == 1
    assert static[0].derived is True
    assert "derived static for synth" in capsys.readouterr().out


def test_derive_is_idempotent_to_the_byte(bench):
    """`bench/results/` is committed and diffable — `git diff --stat` is the review. A
    verb that appended on every invocation would make the file grow without the numbers
    changing, and the diff would stop being the check."""
    results = pathlib.Path(cli.RESULTS)
    d = _published_with_importgraph(results, "synth")
    assert cli.main(["derive"]) == cli.EXIT_OK
    once = (d / "commits.jsonl").read_bytes()
    assert cli.main(["derive"]) == cli.EXIT_OK
    assert (d / "commits.jsonl").read_bytes() == once


def test_derive_never_touches_a_repo_the_corpus_no_longer_admits(bench):
    """`sqlfluff` is a corpus_version: 1 artifact that #184 dropped. Its published bytes
    are frozen, and a derivation that walked every directory would rewrite them."""
    results = pathlib.Path(cli.RESULTS)
    dropped = results / "sqlfluff"
    dropped.mkdir(parents=True)
    (dropped / "commits.jsonl").write_text("", encoding="utf-8")
    before = (dropped / "commits.jsonl").read_bytes()
    _published_with_importgraph(results, "synth")
    assert cli.main(["derive"]) == cli.EXIT_OK
    assert (dropped / "commits.jsonl").read_bytes() == before


def test_derive_refuses_a_repo_whose_records_cannot_support_it(bench, capsys):
    """A repo with no importgraph records has no level 2. Refusing is the whole point:
    the alternative publishes a level-1-only arm under a name claiming both levels."""
    results = pathlib.Path(cli.RESULTS)
    _published_repo(results, "synth")  # rtdd and path only
    assert cli.main(["derive"]) == cli.EXIT_GUARD
    assert "importgraph" in capsys.readouterr().err


def test_derive_leaves_config_json_untouched(bench):
    """config.json stamps the run that produced the numbers. A derivation is not a run."""
    results = pathlib.Path(cli.RESULTS)
    d = _published_with_importgraph(results, "synth")
    before = (d / "config.json").read_text(encoding="utf-8")
    assert cli.main(["derive"]) == cli.EXIT_OK
    assert (d / "config.json").read_text(encoding="utf-8") == before
```

Add `to_jsonl_lines` and `parse_jsonl_lines` to the existing `replay.records` import at
the top of `bench/tests/test_cli.py` if they are not already there.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bench && uv run pytest tests/test_cli.py -k derive -q`
Expected: FAIL — `SystemExit: 2` from argparse, with `invalid choice: 'derive'` on stderr, for all five tests.

- [ ] **Step 3: Write minimal implementation**

In `bench/replay/cli.py`, import `DERIVED_ARMS`, `DerivationError` and `derive_static`
from `replay.derive`, add `to_jsonl_lines` to the `replay.records` import, then:

```python
def cmd_derive(args) -> int:
    """`derive`: compute the derived arms from each admitted repo's own records.

    This is the whole of how the `static` arm reaches a published summary without a
    benchmark re-run (PRD #233, spec §7). It reads `commits.jsonl`, computes the arm,
    drops any stale copy of it, and rewrites the file through the same sorted,
    byte-stable serialiser the run used. It clones nothing, provisions nothing,
    materialises no worktree, executes no test, and never reads the cache.
    """
    ids = set(_corpus(getattr(args, "corpus_version", None)).ids())
    done = []
    for d in sorted(RESULTS.iterdir()) if RESULTS.exists() else []:
        if d.name not in ids or not (d / "commits.jsonl").exists():
            continue
        records = parse_jsonl_lines((d / "commits.jsonl").read_text(encoding="utf-8").splitlines())
        try:
            derived = derive_static(records)
        except DerivationError as exc:
            print(f"{d.name}: {exc}", file=sys.stderr)
            return EXIT_GUARD
        kept = [
            r
            for r in records
            if not (isinstance(r, StrategyRecord) and r.strategy in DERIVED_ARMS)
        ]
        (d / "commits.jsonl").write_text(
            "".join(to_jsonl_lines([*kept, *derived])), encoding="utf-8"
        )
        done.append(d.name)
        print(f"derived static for {d.name} from {d / 'commits.jsonl'}")
    if not done:
        print("no per-repo records found; run `replay` first", file=sys.stderr)
        return EXIT_GUARD
    return EXIT_OK
```

and in `build_parser`, beside `report`:

```python
    dv = sub.add_parser(
        "derive",
        help="compute the derived arms from committed records; runs no benchmark",
    )
    dv.add_argument("--corpus-version", type=int, default=None, dest="corpus_version")
    dv.set_defaults(func=cmd_derive)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bench && uv run pytest tests/test_cli.py -q`
Expected: PASS, including every pre-existing `report --rebuild` test unchanged.

- [ ] **Step 5: Commit**

```bash
git add bench/replay/cli.py bench/tests/test_cli.py
git commit -m "M6e: replay derive computes the static arm from committed records"
```

---

### Task 5 — `bench/replay/cli.py`: a rebuild publishes every arm the records carry

**Discharges:** spec §7. PRD AC2 — the summaries are regenerated "from committed
`commits.jsonl` records via the rebuild path added for issue #214".

**Files:**
- Modify: `bench/replay/cli.py`
- Test: `bench/tests/test_cli.py`

**Interfaces:**
- Consumes: `strategy_order`.
- Produces: no new name. `_rebuild_repo` derives its arm list from
  `cfg.strategies` **plus** the strategy ids present in the records.

- [ ] **Step 1: Write the failing test**

Append to `bench/tests/test_cli.py`:

```python
def test_rebuild_publishes_an_arm_the_records_carry_and_the_config_does_not(bench):
    """`config.json` lists the arms the RUN executed and is never rewritten by a rebuild
    — it stamps that run. A derived arm therefore only ever appears in the records, and a
    rebuild that read its arm list from the config alone would compute the derivation,
    write it to commits.jsonl, and then publish a summary that does not mention it."""
    results = pathlib.Path(cli.RESULTS)
    d = _published_with_importgraph(results, "synth")
    assert cli.main(["derive"]) == cli.EXIT_OK
    assert cli.main(["report", "--rebuild"]) == cli.EXIT_OK

    summary = json.loads((d / "summary.json").read_text(encoding="utf-8"))
    assert "static" in summary["strategies"]
    assert "static" not in json.loads((d / "config.json").read_text(encoding="utf-8"))["config"]["strategies"]
    assert summary["strategies"]["static"]["cycles"] == 1


def test_rebuild_orders_a_derived_arm_the_way_strategy_order_does(bench):
    """`rtdd` first, `random` then `full` last — the published table's reading order, and
    a derived arm is no exception to it."""
    results = pathlib.Path(cli.RESULTS)
    d = _published_with_importgraph(results, "synth")
    assert cli.main(["derive"]) == cli.EXIT_OK
    assert cli.main(["report", "--rebuild"]) == cli.EXIT_OK
    order = list(json.loads((d / "summary.json").read_text(encoding="utf-8"))["strategies"])
    assert order[0] == "rtdd"
    assert order.index("path") < order.index("static")
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bench && uv run pytest tests/test_cli.py -k "records_carry or derived_arm_the_way" -q`
Expected: FAIL — `AssertionError: assert 'static' in dict_keys(['rtdd', 'path'])`; the derived record is in `commits.jsonl` and absent from the summary.

- [ ] **Step 3: Write minimal implementation**

In `_rebuild_repo`, replace the `build_summary` call's arm list:

```python
    # The arms the RECORDS carry, not only the arms the run's config lists. A derived
    # arm (replay.derive) is computed after the run and appended to commits.jsonl;
    # config.json stamps the run that produced the numbers and a rebuild has no
    # standing to rewrite it, so the config can never mention one. Reading the ids out
    # of the records is also the more honest rule for a measured arm: a summary should
    # publish what was recorded.
    recorded = {s.strategy for s in out.strategies}
    ids = strategy_order([*cfg.strategies, *sorted(recorded - set(cfg.strategies))])

    summary = build_summary(out, ids, hw, wallclock_enabled=wallclock_enabled)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bench && uv run pytest tests/test_cli.py tests/test_report.py -q`
Expected: PASS. `test_rebuild_publishes_strategies_in_the_order_the_run_did` still passes — a run with no derived arm has `recorded ⊆ cfg.strategies`, so `ids` is exactly `strategy_order(cfg.strategies)`.

- [ ] **Step 5: Commit**

```bash
git add bench/replay/cli.py bench/tests/test_cli.py
git commit -m "M6e: a rebuild publishes every arm the committed records carry"
```

---

### Task 6 — `bench/replay/report.py`: the static verdict, the kill condition, and the derived-arm disclosure

**Discharges:** spec §7. PRD AC3 (recall, selection ratio and selected-duration fraction
for `static` against `rtdd`, `path` and `full`), AC4 (`p50`/`p90`/`worst`, never a bare
mean), AC12 (the pre-registered kill condition, implemented rather than skipped).

AC3's three columns are already emitted for every arm in `summary["strategies"]` by
`_headline_rows`, so Task 5 discharged the table half. What is missing is the *verdict* —
the pre-registered comparison stated in prose, win or lose — and the three disclosures
decision 1 and decision 3 require.

**Files:**
- Modify: `bench/replay/report.py`
- Test: `bench/tests/test_report.py`

**Interfaces** — reconciled with what shipped (#366). The block below described an API
that was never built, and a plan that describes a tree it does not have is the same defect
this milestone exists to correct elsewhere. What the task actually produced, and why:

- Consumes: `derive.STATIC_TEST_FOR`, `derive.LEVEL2_SOURCE`. **Not** `derive.DERIVED_ARMS`
  — `report.py` never imported it. The derived arm is recognised at the point it matters,
  by the absence of a `WallClockRecord`: `_comparison_wallclock_cells` prints
  `NOT_MEASURED` for any arm with no row, which covers a derived arm, a CI runner and a
  withheld run alike, and cannot go stale against a list.
- Produces:
  - `COMPARISON_ARMS`, `COMPARISON_METRICS`, `COMPARISON_WALLCLOCK_COLUMN`,
    `COMPARISON_HEADER_KEY`, `NOT_MEASURED`
  - `comparison_table(summary: dict) -> list[str]` — the §7 evidence table (AC3), rendered
    into `summary.md` under `## The static arm`, with the not-measured disclosure beneath it
  - `static_model_disclosure() -> list[str]` (#366) — the `test_for` templates, read from
    `derive.STATIC_TEST_FOR` and never re-typed, plus the level-2 source and both
    construction facts. `STATIC_TEST_FOR`'s docstring calls itself a published input that
    `summary.md` prints; until #366 it was not printed anywhere, and the number's most
    load-bearing input was invisible to the reader of the number.
  - `STATIC_SUCCESS_WORDING`, `STATIC_FAILURE_WORDING`,
    `static_verdict_line(summary: dict) -> str`,
    `static_secondary_verdict_lines(summary: dict) -> list[str]` (#366) — the verdict this
    task set out to state in prose. It was missing from `summary.md` for three milestones:
    a reader of `bench/results/flask/summary.md` alone saw `static` and `path` tie at 0.333
    recall / 0.010 duration in the by-variant table and was told nothing about it. These
    share one body, `_compare`, with `verdict_line` and `secondary_verdict_lines`, so the
    two pre-registered comparisons cannot drift apart.
- **Not** produced: `summary["derived_arms"]`. The key was never added and nothing needs
  it — `summary.json` is committed, so a key that no consumer reads is a re-keying of a
  published artifact for nothing.

**Where AC12 was actually discharged.** Not in a `summary.md` line, as planned, but in
`README.md`, guarded by `outcomes_test.go`'s
`TestREADMEStaticVerdictMatchesTheCommittedSummaries` (#351). That check parses both
committed `summary.json` files, applies the §7 rule to every population, and asserts the
README states the claim the numbers imply and never the opposite one — so the kill
condition is derived from the evidence rather than trusted as typed prose, which the
planned `summary.md` line alone would not have achieved. The README is the artifact the
pre-registration names ("and the README says so"), and it is the only place a reader is
given both repos at once, which is what a verdict over both populations needs. #366 adds
the per-repo line as well, by the same rule, so each artifact also states its own result.

- [ ] **Step 1: Write the failing test**

Append to `bench/tests/test_report.py`:

```python
from replay.derive import STATIC_TEST_FOR
from replay.report import (
    STATIC_FAILURE_WORDING,
    STATIC_SUCCESS_WORDING,
    static_secondary_verdict_lines,
    static_verdict_line,
)


def _summary(rows: dict, *, primary="natural", by_variant=None) -> dict:
    def arm(recall, dur):
        return {
            "change_level_recall": {"num": 0, "den": 0, "value": recall},
            "selected_duration_fraction": {"num": 0, "den": 0, "value": dur},
        }

    strategies = {k: arm(*v) for k, v in rows.items()}
    return {
        "primary_variant": primary,
        "strategies": strategies,
        "by_variant": by_variant or {primary: strategies},
        "derived_arms": ["static"],
    }


def test_the_static_verdict_fires_the_kill_condition_on_a_tie():
    """Spec §7: "if the static tier does not BEAT the path baseline it is not worth
    shipping as a distinct tier". A tie is not a beat. This is the branch the corpus
    actually produces today (flask/probe: 0.333 vs 0.333), so it is the branch that has
    to be right."""
    s = _summary({"static": (0.333, 0.010), "path": (0.333, 0.010)})
    assert STATIC_FAILURE_WORDING in static_verdict_line(s)


def test_the_static_verdict_reports_a_win_when_there_is_one():
    s = _summary({"static": (0.800, 0.100), "path": (0.400, 0.120)})
    assert STATIC_SUCCESS_WORDING in static_verdict_line(s)


def test_better_recall_bought_with_more_time_is_not_a_win():
    """"at comparable or better selected-duration fraction" is half the criterion. An arm
    that buys recall by selecting more of the suite is on its way to being `full`."""
    s = _summary({"static": (0.800, 0.900), "path": (0.400, 0.100)})
    assert STATIC_FAILURE_WORDING in static_verdict_line(s)


def test_a_population_with_no_ground_truth_is_not_computable_not_a_loss():
    """`natural` has no detecting commits in either published repo. Scoring that as a
    failure would publish a verdict about a comparison nothing was measured for."""
    s = _summary({"static": (None, 0.05), "path": (None, 0.05)})
    assert "not computable" in static_verdict_line(s)
    assert STATIC_FAILURE_WORDING not in static_verdict_line(s)


def test_the_secondary_verdict_labels_probe_as_an_upper_bound():
    rows = {"static": (0.333, 0.010), "path": (0.333, 0.010)}
    s = _summary(rows, by_variant={"natural": _summary(rows)["strategies"],
                                   "probe": _summary(rows)["strategies"]})
    lines = static_secondary_verdict_lines(s)
    assert len(lines) == 1
    assert "probe" in lines[0]


def test_a_derived_arm_gets_no_wall_clock_row_and_the_absence_is_explained():
    """The static arm executed nothing, so there is no timing to publish and none may be
    invented — not from durations_ms, which is another execution's per-test time, and not
    by borrowing importgraph's row. The renderer says so instead."""
    summary = build_summary(_output_with_static(), ("rtdd", "static"), _hw())
    text = render_markdown(summary, _cfg(), _hw())
    wall = text.split("## Wall-clock", 1)[1]
    assert "| static |" not in wall
    assert "executed nothing" in wall
    assert "selected-duration fraction" in wall


def test_the_model_the_static_arm_publishes_is_disclosed_in_the_table():
    """The templates are a published input: change them and the number changes. And the
    arm models a Python static adapter that does not ship, which a reader comparing it to
    `rtdd` has to be told."""
    summary = build_summary(_output_with_static(), ("rtdd", "static"), _hw())
    text = render_markdown(summary, _cfg(), _hw())
    for needle in (STATIC_TEST_FOR[0], "importgraph", "does not ship", "by construction"):
        assert needle in text


def test_the_distribution_guard_still_covers_the_regenerated_table():
    """AC4: any wall-clock figure in a NEW or regenerated table carries p50/p90/worst.
    render_markdown audits its own output, so the guarantee covers what Task 6 added."""
    summary = build_summary(_output_with_static(), ("rtdd", "static"), _hw())
    assert_distribution_beside_mean(render_markdown(summary, _cfg(), _hw()))
```

`_output_with_static`, `_cfg` and `_hw` are local helpers in the shape
`bench/tests/test_report.py` already uses for its existing `build_summary` tests; reuse
them rather than adding new fixtures, and give the `ReplayOutput` one commit, one `rtdd`
`WallClockRecord` and one derived `static` `StrategyRecord`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bench && uv run pytest tests/test_report.py -k "static or derived_arm or model" -q`
Expected: FAIL at collection — `ImportError: cannot import name 'STATIC_FAILURE_WORDING' from 'replay.report'`.

- [ ] **Step 3: Write minimal implementation**

In `bench/replay/report.py`:

```python
STATIC_SUCCESS_WORDING = (
    "the static tier beats the naive path heuristic — per the pre-registration in "
    "docs/specs/2026-09-05-multi-language.md §7 it carries its weight as a distinct tier"
)
STATIC_FAILURE_WORDING = (
    "the static tier does NOT beat the naive path heuristic — per the pre-registration "
    "in docs/specs/2026-09-05-multi-language.md §7 it does not carry its weight as a "
    "distinct tier and the README must say so"
)
```

`static_verdict_line` and `static_secondary_verdict_lines` are `verdict_line` and
`secondary_verdict_lines` with `"rtdd"` replaced by `"static"` and the two wordings
swapped; factor the shared body into one helper taking `(summary, arm, success, failure)`
and have both pairs call it, so the two verdicts cannot drift apart. Keep
`verdict_line`'s exact output — `test_every_summary_carries_a_verdict_line` matches
`^verdict: ` and `bench/results/*/summary.md` is regenerated against it in Task 7.

Emit the static verdict from `render_markdown` immediately after the existing
`rtdd`-vs-`path` block, add `"derived_arms": sorted(set(DERIVED_ARMS) & recorded)` to
`build_summary`'s returned dict, and append to the `## Wall-clock` prose:

```python
        if summary.get("derived_arms"):
            lines.append("")
            lines.append(
                "`" + "`, `".join(summary["derived_arms"]) + "` executed nothing — "
                "each is derived from records this benchmark already committed, so it has "
                "no wall-clock and no isolation row, and none was synthesised from "
                "`durations_ms` or borrowed from another arm. Its published cost figure is "
                "the **selected-duration fraction** above, a ratio of the same commit's own "
                "recorded per-test durations and therefore independent of the machine."
            )
```

and a `## The static arm` block after the headline table, disclosing the templates, the
level-2 source, and both construction facts:

```python
    lines.append("")
    lines.append("## The static arm")
    lines.append("")
    lines.append(
        "`static` models the `TS` tier (spec §4.1) over this corpus. It is **derived** "
        "from the records below, never re-run. Level 1 is `test_for` correspondence, "
        "resolved against the test files each commit collected, first match wins:"
    )
    lines.append("")
    for tmpl in STATIC_TEST_FOR:
        lines.append(f"- `{tmpl}`")
    lines.append("")
    lines.append(
        "Level 2 is the committed `importgraph` selection — that baseline measures "
        "exactly the transitive-import question level 2 asks. Level 3 (path proximity) "
        "orders and never admits, so it cannot change the selected set and no metric "
        "here depends on it. Two consequences: `static ⊇ importgraph` **by "
        "construction**, so beating that baseline is arithmetic and not a finding; and "
        "`adapters/python.yaml` declares no `test_for` and no `importscan`, so this "
        "adapter **does not ship** — the row answers what the static tier WOULD have "
        "selected here, which is the question §7 pre-registers."
    )
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bench && uv run pytest -q`
Expected: PASS, the whole bench replay suite.

- [ ] **Step 5: Commit**

```bash
git add bench/replay/report.py bench/tests/test_report.py
git commit -m "M6e: the pre-registered static-vs-path verdict and the derived-arm disclosure"
```

---

### Task 7 — regenerate flask and httpie; leave sqlfluff alone

**Discharges:** spec §7. PRD AC2, AC3, AC5.

This is the task that publishes the numbers. It writes no new code — it runs the code
Tasks 1–6 built and commits its output — and its test is a guard that the published tree
now says what the milestone promised.

**Files:**
- Regenerate: `bench/results/{flask,httpie}/{commits.jsonl,summary.json,summary.md}`,
  `bench/results/aggregate.md`
- Test: `bench/tests/test_results_corpus_coverage.py`

**Interfaces:** none. Data only.

- [ ] **Step 1: Write the failing test**

Append to `bench/tests/test_results_corpus_coverage.py`:

```python
@pytest.mark.parametrize("repo_id", corpus_ids())
def test_every_corpus_repo_publishes_the_static_arm(repo_id: str) -> None:
    """PRD #233 AC2: both admitted repos publish the static arm, derived from their own
    records. A repo that silently lacks it is the missing-number failure this file exists
    to catch."""
    summary = json.loads((RESULTS / repo_id / "summary.json").read_text(encoding="utf-8"))
    assert "static" in summary["strategies"]
    assert summary.get("derived_arms") == ["static"]


@pytest.mark.parametrize("repo_id", corpus_ids())
def test_every_summary_carries_the_static_verdict(repo_id: str) -> None:
    """AC12's branch is published win or lose, exactly as the rtdd-vs-path one is."""
    text = (RESULTS / repo_id / "summary.md").read_text(encoding="utf-8")
    assert re.search(r"^verdict \(static.*$", text, re.MULTILINE) or re.search(
        r"^verdict: static .+$", text, re.MULTILINE
    ), f"{repo_id}/summary.md has no static verdict line"


@pytest.mark.parametrize("repo_id", corpus_ids())
def test_the_static_arm_publishes_no_wall_clock_row(repo_id: str) -> None:
    """It executed nothing. AC4 governs the figures a table DOES carry; this governs the
    one it must not invent."""
    summary = json.loads((RESULTS / repo_id / "summary.json").read_text(encoding="utf-8"))
    assert "static" not in summary["wallclock"].get("rows", {})
    assert "static" not in summary["isolation"].get("rows", {})


def test_the_dropped_corpus_version_1_result_is_untouched() -> None:
    """`bench/results/sqlfluff/` is a corpus_version: 1 artifact #184 dropped. It carries
    no static arm, it is not regenerated, and this milestone does not open it."""
    summary = json.loads((RESULTS / "sqlfluff" / "summary.json").read_text(encoding="utf-8"))
    assert "static" not in summary["strategies"]
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bench && uv run pytest tests/test_results_corpus_coverage.py -q`
Expected: FAIL — `assert 'static' in dict_keys(['full', 'importgraph', 'lf', 'path', 'random', 'rtdd', 'testmon', 'xdist'])` for both `flask` and `httpie`. `test_the_dropped_corpus_version_1_result_is_untouched` passes from the start; it is the guard that must *stay* passing.

- [ ] **Step 3: Regenerate**

```bash
cd bench
uv run python -m replay.cli derive
uv run python -m replay.cli report --rebuild
```

Then read the diff before staging it — this is a published-number change and
`git diff --stat bench/results/` is the review:

```bash
git diff --stat bench/results/
git diff bench/results/flask/summary.md
```

Three things the diff must show, and any of them missing means stop and fix the cause
rather than the file:

- `commits.jsonl` grows by exactly one `static` line per commit record per variant
  (flask 46, httpie 35) and **no other line changes** — every measured record is
  byte-identical, because the derivation only appends.
- `summary.md` gains a `static` row in the headline, stratified and per-variant tables, a
  `## The static arm` block, a static verdict line, and the derived-arm sentence under
  `## Wall-clock` — and gains **no** `| static |` row under `## Wall-clock` or
  `## Isolation`.
- `bench/results/sqlfluff/` is absent from the diff entirely.

Compare the published `static` numbers against *What the corpus already says* above. They
should match. If they do not, the implementation is what is true — but understand the
difference before committing, because that table reproduced three already-published arms
exactly.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bench && uv run pytest -q && uv run python -m replay.cli audit`
Expected: PASS, and `clean — every admitted repo meets every criterion a threshold can decide`.

- [ ] **Step 5: Commit**

```bash
git add bench/results/flask bench/results/httpie bench/results/aggregate.md \
        bench/tests/test_results_corpus_coverage.py
git commit -m "M6e: publish the static arm for flask and httpie, derived from committed records"
```

---

### Task 8 — CI runs the bench replay suite

**Discharges:** PRD AC13. The published numbers are now produced by `bench/replay/`, and
neither gate runs its tests.

**Files:**
- Modify: `scripts/ci-local.sh`, `.github/workflows/ci.yml`

**Interfaces:** none.

- [ ] **Step 1: Write the failing test**

The gate is a shell script, so its test is a shell assertion. Append to
`bench/tests/test_results_corpus_coverage.py`:

```python
def test_the_local_ci_gate_runs_the_bench_replay_suite() -> None:
    """From #340 on, `bench/replay/` is what produces the published numbers. ci-local.sh
    is the authoritative gate for this repo, and it ran the swebench project and three
    bench test files — a change to derive.py, report.py or metrics.py could go green
    through both gates while breaking every table."""
    gate = (BENCH.parent / "scripts" / "ci-local.sh").read_text(encoding="utf-8")
    assert "bench replay gate" in gate
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bench && uv run pytest tests/test_results_corpus_coverage.py -k local_ci_gate -q`
Expected: FAIL — `AssertionError: assert 'bench replay gate' in '#!/usr/bin/env bash\n...'`.

- [ ] **Step 3: Write minimal implementation**

In `scripts/ci-local.sh`, immediately after the `prereg gate` step and before
`static binary`:

```bash
# Issue #340: bench/ (Axis 2 replay) is a separate uv project from bench/swebench, and
# until this milestone neither gate ran it. It now holds the derivation that produces
# bench/results/*/summary.{json,md}, so a change to derive.py, report.py or metrics.py
# could otherwise go green through both gates while breaking every published table.
echo "==> bench replay gate"
(cd bench && uv sync && uv run pytest -q && uv run python -m replay.cli audit)
```

In `.github/workflows/ci.yml`, replace the `corpus and audit tests` step's narrow file
list with the whole suite, keeping the `corpus admission gate` step as it is:

```yaml
      # #340: the whole replay suite, not three files of it. bench/replay/ now derives
      # the published static arm, so its tests are a push gate.
      - name: bench replay suite
        run: uv run pytest -q
        working-directory: bench
```

- [ ] **Step 4: Run test to verify it passes**

Run: `scripts/ci-local.sh`
Expected: exit 0, with `==> bench replay gate` in the output followed by the bench suite passing.

- [ ] **Step 5: Commit**

```bash
git add scripts/ci-local.sh .github/workflows/ci.yml bench/tests/test_results_corpus_coverage.py
git commit -m "M6e: the CI gate runs the bench replay suite that produces the published numbers"
```

---

### Task 9 — `cmd/rtdd`: `selection_fidelity` on `which --json` and `run --json`

**Discharges:** spec §6. PRD AC6.

**Files:**
- Create: `cmd/rtdd/fidelity.go`, `cmd/rtdd/fidelity_test.go`
- Modify: `cmd/rtdd/jsonout.go`, `cmd/rtdd/jsonout_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**
- Consumes: `selector.Tier`, `adapter.Fidelity`.
- Produces: `selectionFidelity(t selector.Tier) adapter.Fidelity`; `Output.SelectionFidelity`
  and `JSONAdapterSelection.SelectionFidelity`, both `json:"selection_fidelity"`.

- [ ] **Step 1: Write the failing test**

Create `cmd/rtdd/fidelity_test.go`:

```go
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// Spec §6: selection_fidelity answers "what was THIS answer derived from", which is not
// the same question Adapter.Fidelity() answers ("what is the best this adapter could ever
// produce"). A coverage adapter with an unseeded map escalates to T2, and that answer was
// derived from nothing at all.
func TestSelectionFidelityIsDerivedFromTheTierThatAnswered(t *testing.T) {
	for _, tc := range []struct {
		tier selector.Tier
		want adapter.Fidelity
	}{
		{selector.TierDirect, adapter.FidelityExecution},
		{selector.TierT0, adapter.FidelityExecution},
		{selector.TierT1, adapter.FidelityExecution},
		// TierEmpty is reachable only from a usable map, so the map DID answer and its
		// answer was "nothing". Labelling it `none` would say "no evidence" about the one
		// outcome `warnings` already has to defend as a real result.
		{selector.TierEmpty, adapter.FidelityExecution},
		{selector.TierTS, adapter.FidelityStatic},
		// T2 is the full suite: nothing was derived, which is why you run everything.
		{selector.TierT2, adapter.FidelityNone},
	} {
		if got := selectionFidelity(tc.tier); got != tc.want {
			t.Errorf("selectionFidelity(%s) = %q, want %q", tc.tier, got, tc.want)
		}
	}
}

// "Never null" is not a convention here, it is the contract §6 states. A consumer that
// has to branch on null cannot calibrate on the field at all.
func TestSelectionFidelityIsNeverEmpty(t *testing.T) {
	for tier := selector.TierEmpty; tier <= selector.TierT2; tier++ {
		if selectionFidelity(tier) == "" {
			t.Errorf("selectionFidelity(%s) is empty", tier)
		}
	}
	if got := selectionFidelity(selector.Tier(99)); got != adapter.FidelityNone {
		t.Errorf("an unknown tier must fall back to %q, got %q", adapter.FidelityNone, got)
	}
}

func TestJSONCarriesSelectionFidelityForEveryCommand(t *testing.T) {
	for _, cmd := range []string{"which", "run"} {
		out := BuildOutput(OutputInput{
			Command: cmd,
			Sel:     selector.Selection{Tier: selector.TierTS, Tests: []string{"a"}},
		})
		b, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"selection_fidelity":"static"`) {
			t.Errorf("%s --json has no selection_fidelity: %s", cmd, b)
		}
	}
}

// A polyglot document splits per adapter, and two adapters can answer at two fidelities —
// a seeded Python block beside a Go block that can only ever be static. A single top-level
// value would label one of them wrongly.
func TestEachPerAdapterBlockCarriesItsOwnFidelity(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "which",
		Sel:     selector.Selection{Tier: selector.TierT2},
		Blocks: []AdapterSelection{
			{Adapter: "python", Selection: selector.Selection{Tier: selector.TierT0}},
			{Adapter: "go", Selection: selector.Selection{Tier: selector.TierTS}},
		},
	})
	if len(out.Selections) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(out.Selections))
	}
	if out.Selections[0].SelectionFidelity != adapter.FidelityExecution {
		t.Errorf("python block = %q", out.Selections[0].SelectionFidelity)
	}
	if out.Selections[1].SelectionFidelity != adapter.FidelityStatic {
		t.Errorf("go block = %q", out.Selections[1].SelectionFidelity)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rtdd/ -run 'SelectionFidelity|SelectionFidelityForEvery|PerAdapterBlock' -v`
Expected: FAIL — `undefined: selectionFidelity` and `unknown field SelectionFidelity in struct literal`, i.e. `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

Create `cmd/rtdd/fidelity.go` with `selectionFidelity` as the decision-4 table, its doc
comment carrying the `TierEmpty` and `TierT2` reasoning, and a `default:` arm returning
`adapter.FidelityNone` — the conservative fallback, because an unrecognised tier is not
evidence of anything.

In `jsonout.go`, add `SelectionFidelity adapter.Fidelity \`json:"selection_fidelity"\`` to
`Output` immediately after `Reason` (field order is emitted key order, and fidelity
belongs beside the tier and reason it qualifies), and to `JSONAdapterSelection` after its
`Reason`. Set both in `BuildOutput` and `buildSelections` from `selectionFidelity`.
`adapter.Fidelity` is a `string` type, so it marshals as a JSON string and a zero value is
`""`, never `null` — and `selectionFidelity` never returns `""`.

Record the field in `docs/plans/00-interfaces.md` beside `warnings`, with the tier table.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/rtdd/... -count=1`
Expected: PASS. `cmd/rtdd/whichgolden_test.go` will need its golden regenerated — the document gained a key. Regenerate it (`go test ./cmd/rtdd/ -run Golden -update`) and read the diff: exactly one new key per document, nothing else moved.

- [ ] **Step 5: Commit**

```bash
git add cmd/rtdd/fidelity.go cmd/rtdd/fidelity_test.go cmd/rtdd/jsonout.go \
        cmd/rtdd/jsonout_test.go cmd/rtdd/testdata docs/plans/00-interfaces.md
git commit -m "M6e: selection_fidelity on which --json and run --json"
```

---

### Task 10 — `cmd/rtdd`: the static caveat in `warnings`, and `tier: TS (static)`

**Discharges:** spec §6. PRD AC7 and AC8.

**Files:**
- Modify: `cmd/rtdd/which.go`, `cmd/rtdd/run.go`, `cmd/rtdd/whichtext.go`
- Test: `cmd/rtdd/which_test.go`, `cmd/rtdd/run_test.go`, `cmd/rtdd/whichtext_test.go`

**Interfaces:**
- Consumes: `doctor.StaticCaveat` — the sentence already exists and already states both
  halves; AC7 is a new surface for it, not a new sentence.
- Produces: no new exported name. `whichNotes` and `runNotes` emit the caveat on a `TS`
  selection; `RenderWhich` suffixes the tier line.

- [ ] **Step 1: Write the failing test**

Append to `cmd/rtdd/whichtext_test.go`:

```go
// Spec §6: "the tier line reads `tier: TS (static)`". The bare tier name is a label an
// agent has to already know the meaning of; the suffix is the calibration.
func TestTierLineNamesTheFidelityOfAStaticSelection(t *testing.T) {
	got := RenderWhich(selector.Selection{
		Tier:   selector.TierTS,
		Tests:  []string{"pkg/a_test.go"},
		Reason: "declared correspondence to pkg/a.go",
	}, nil, nil)
	if !strings.Contains(got, "tier: TS (static)") {
		t.Errorf("want `tier: TS (static)`, got:\n%s", got)
	}
}

// A coverage tier's line does not move by one byte: every existing Python repository
// prints what it printed before, and the selector's own output is unchanged.
func TestACoverageTierLineIsUnchanged(t *testing.T) {
	for _, tier := range []selector.Tier{
		selector.TierDirect, selector.TierT0, selector.TierT1, selector.TierT2,
	} {
		got := RenderWhich(selector.Selection{Tier: tier}, nil, nil)
		if strings.Contains(got, "(static)") || strings.Contains(got, "(none)") {
			t.Errorf("tier %s gained a fidelity suffix:\n%s", tier, got)
		}
	}
}
```

Append to `cmd/rtdd/which_test.go`:

```go
// Spec §6 and PRD AC7. Under --json the document is the whole of stdout and a consumer
// normally discards stderr, so a caveat that lives only on stderr is a caveat the agent
// front-end never sees — the same argument `complete` and `warnings` already exist for.
func TestAStaticSelectionWarnsThatItIsWeakerEvidence(t *testing.T) {
	notes := whichNotes(&env{}, AdapterSelection{
		Selection: selector.Selection{Tier: selector.TierTS, Tests: []string{"a"}},
	}, false)
	if !containsString(notes, doctor.StaticCaveat) {
		t.Fatalf("a TS selection carries no static caveat: %#v", notes)
	}
}

func TestACoverageSelectionCarriesNoStaticCaveat(t *testing.T) {
	notes := whichNotes(&env{}, AdapterSelection{
		Selection: selector.Selection{Tier: selector.TierT0, Tests: []string{"a"}},
	}, false)
	if containsString(notes, doctor.StaticCaveat) {
		t.Fatalf("a T0 selection must not claim to be static: %#v", notes)
	}
}
```

and the same pair against `runNotes` in `cmd/rtdd/run_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rtdd/ -run 'TierLineNamesTheFidelity|StaticSelectionWarns' -v`
Expected: FAIL — `want \`tier: TS (static)\`, got: tier: TS  (1 test selected, ranked)`, and `a TS selection carries no static caveat: []string{}`.

- [ ] **Step 3: Write minimal implementation**

In `whichtext.go`, suffix the tier line from `selectionFidelity` — and only when it says
`static`, so every coverage tier's line is byte-identical:

```go
	tier := sel.Tier.String()
	if selectionFidelity(sel.Tier) == adapter.FidelityStatic {
		tier += " (static)"
	}
	fmt.Fprintf(&b, "  tier: %s  (%d %s selected, ranked)\n", tier, ...)
```

In `whichNotes` and `runNotes`, append `doctor.StaticCaveat` when
`selectionFidelity(sel.Tier) == adapter.FidelityStatic`. In `whichNotes` use the existing
`note(...)` closure so a polyglot repository prefixes it with the adapter name, which is
the whole reason that closure exists: two adapters at two fidelities produce two caveats,
and an unattributed one sends the reader to the wrong half of the repository.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/rtdd/... -count=1`
Expected: PASS. The `which` golden may move again — a `TS` fixture gains one warning and one suffix. Regenerate and read the diff.

- [ ] **Step 5: Commit**

```bash
git add cmd/rtdd/whichtext.go cmd/rtdd/which.go cmd/rtdd/run.go \
        cmd/rtdd/whichtext_test.go cmd/rtdd/which_test.go cmd/rtdd/run_test.go cmd/rtdd/testdata
git commit -m "M6e: tier: TS (static) and the static caveat in warnings"
```

---

### Task 11 — `cmd/rtdd/run.go`: stop claiming coverage that was never recorded

**Discharges:** spec §6. PRD AC9a, AC9b, and AC9's end-to-end assertion.

Two instances, one surface, one task. Both are `rtdd run` describing a `coverage: none`
adapter in the vocabulary of an adapter that records. AC9a is the harmful one:
`UNCOVERED` is a claim an agent acts on, and with `coverage: none` **every** changed line
reports that way, because nothing was instrumented — the report is not wrong about some
lines, it is meaningless for all of them.

**Files:**
- Modify: `cmd/rtdd/run.go`
- Test: `cmd/rtdd/run_test.go`

**Interfaces:**
- Consumes: `adapter.CoverageNone`.
- Produces: `coverageWasRecorded(ads []*adapter.Adapter) bool` — true when at least one
  detected adapter records coverage. It gates the uncovered report, the map-rows clause
  and `OutputInput.UncoveredOK`.

- [ ] **Step 1: Write the failing test**

Append to `cmd/rtdd/run_test.go`:

```go
// PRD #233 AC9a. `UNCOVERED: src/calc.js:2  (1 changed line, no executing test)` is a
// claim an agent acts on, and with `coverage: none` nothing was instrumented — so every
// changed line reports that way and the report means nothing for any of them. It is
// suppressed rather than reworded because there is no honest version of it: the tool has
// no line-level information about this repository at all.
func TestAStaticRunEmitsNoUncoveredReportAndSaysWhy(t *testing.T) {
	repo := staticRepo(t) // a coverage: none adapter with one changed source file
	out, _ := runMain(t, repo, "run")
	if strings.Contains(out, "UNCOVERED") {
		t.Fatalf("a coverage: none run claimed an uncovered line:\n%s", out)
	}
	if !strings.Contains(out, "no line-level report") {
		t.Fatalf("the absence of the report must be stated in one line:\n%s", out)
	}
}

// AC9b. The count is of map rows, and this adapter writes none.
func TestAStaticRunDoesNotClaimMapRows(t *testing.T) {
	repo := staticRepo(t)
	out, _ := runMain(t, repo, "run")
	if strings.Contains(out, "rows in the map") {
		t.Fatalf("a coverage: none run claimed map rows:\n%s", out)
	}
	if !strings.Contains(out, "ran,") || !strings.Contains(out, "failed") {
		t.Fatalf("the ran/failed counts are real and must survive:\n%s", out)
	}
}

// AC9's own acceptance test, named as the PRD names it.
func TestACoverageNoneRunMentionsNeitherUncoveredNorMapRows(t *testing.T) {
	repo := staticRepo(t)
	out, errOut := runMain(t, repo, "run")
	both := out + errOut
	for _, forbidden := range []string{"UNCOVERED", "rows in the map"} {
		if strings.Contains(both, forbidden) {
			t.Errorf("output contains %q:\n%s", forbidden, both)
		}
	}
}

// The machine surface tells the same story. `uncovered.available` already distinguishes
// "available, nothing uncovered" from "not available", and run.go passed UncoveredOK:true
// unconditionally — so --json published an available report whose every changed line was
// uncovered, which is the same defect one layer down.
func TestAStaticRunJSONMarksTheUncoveredReportUnavailable(t *testing.T) {
	repo := staticRepo(t)
	out, _ := runMain(t, repo, "run", "--json")
	var doc Output
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Uncovered.Available {
		t.Error("uncovered.available is true for an adapter that records nothing")
	}
	if doc.Uncovered.Reason == "" {
		t.Error("an unavailable report must say why")
	}
}

// A Python repository's run is byte-identical to before. This is the regression the task
// had to avoid, and it is asserted rather than assumed.
func TestACoverageRunStillReportsMapRowsAndUncoveredLines(t *testing.T) {
	repo := seededPythonRepo(t)
	out, _ := runMain(t, repo, "run")
	if !strings.Contains(out, "rows in the map") {
		t.Fatalf("a coverage repository lost its map-rows line:\n%s", out)
	}
}
```

`staticRepo` and `seededPythonRepo` are the existing `cmd/rtdd` fixture helpers — reuse
`fixtures_test.go`'s builders rather than adding new ones; `seed_static_test.go` already
constructs a `coverage: none` repository.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rtdd/ -run 'StaticRun|CoverageNoneRun' -v`
Expected: FAIL — `a coverage: none run claimed an uncovered line:` followed by the `UNCOVERED:` line, `a coverage: none run claimed map rows: 1 ran, 0 failed, 0 rows in the map`, and `uncovered.available is true for an adapter that records nothing`.

- [ ] **Step 3: Write minimal implementation**

Add to `run.go`:

```go
// coverageWasRecorded reports whether ANY detected adapter records coverage.
//
// It gates every claim `run` makes that only an instrumented run can support: the
// uncovered report, the map-rows count, and the --json report's availability. The
// quantifier is "any", not "all": in a mixed repository the Python half really did
// record, its rows really are in the map, and its uncovered report is real — suppressing
// it because a Go adapter sits beside it would lose a signal that was measured.
//
// A nil or empty set keeps the coverage reading, for the same reason unmappedNoticeApplies
// does: nothing declared otherwise, and that is what every existing repository is.
func coverageWasRecorded(ads []*adapter.Adapter) bool {
	if len(ads) == 0 {
		return true
	}
	for _, a := range ads {
		if a == nil || a.Coverage != adapter.CoverageNone {
			return true
		}
	}
	return false
}
```

Then in `cmdRun`:

- pass `UncoveredOK: coverageWasRecorded(ads)` instead of `true`;
- gate the text report:

```go
	if coverageWasRecorded(ads) {
		fmt.Printf("%d ran, %d failed, %d rows in the map\n", ran, len(failed), m.Len())
	} else {
		fmt.Printf("%d ran, %d failed\n", ran, len(failed))
	}
```

- and replace the `RenderUncovered` block:

```go
	if coverageWasRecorded(ads) {
		if s := RenderUncovered(reports); s != "" {
			fmt.Fprint(os.Stdout, "\n"+s)
		}
	} else {
		fmt.Println("no line-level report: this repository's adapter records no coverage, " +
			"so no changed line can be known to be covered or uncovered")
	}
```

`unavailableReason` in `jsonout.go` names `rtdd run` as the fix, which is wrong for this
case — extend `buildUncovered` to carry the adapter's reason when one is supplied, or add
a second constant; either way the reason a consumer reads must not tell them to run the
command they just ran.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/rtdd/... ./internal/... -count=1`
Expected: PASS, including `cmd/rtdd/acceptance_test.go` and `uncoveredtext_test.go` unchanged — `RenderUncovered` itself is not touched, only whether `run` calls it.

- [ ] **Step 5: Commit**

```bash
git add cmd/rtdd/run.go cmd/rtdd/jsonout.go cmd/rtdd/run_test.go
git commit -m "M6e: rtdd run makes no coverage claim for an adapter that records nothing"
```

---

### Task 12 — `cmd/rtdd/doctor.go`: no fan-out caveat where fan-out cannot be computed

**Discharges:** spec §6. PRD AC9c.

`doctor.Caveat` explains why a fan-out *number* can mislead: anything executed once per
process is attributed to whichever test ran first. In a repository where no fan-out is
computed at all, it is four Python concepts printed at a reader whose repository is not
Python, immediately after `doctor` has correctly said that fan-out is never computed
there. The caveat is not wrong; it is about a number that does not exist.

**Files:**
- Modify: `cmd/rtdd/doctor.go`
- Test: `cmd/rtdd/doctor_test.go`

**Interfaces:**
- Consumes: `selectionSplit` (already in `doctor.go`).
- Produces: `RenderDoctor` prints `doctor.Caveat` only when a fan-out table exists or a
  coverage adapter could ever produce one.

- [ ] **Step 1: Write the failing test**

Append to `cmd/rtdd/doctor_test.go`:

```go
// PRD #233 AC9c. The caveat qualifies a fan-out NUMBER. In a static-only repository
// `emptyFanOutLine` has just said the map "is empty, and stays empty" — following that
// with @lru_cache, module singletons, DI container wiring and session-scoped fixtures
// describes a Python attribution hazard to a reader whose repository has no Python in it,
// about a number that was never computed.
func TestTheFanOutCaveatIsSuppressedWhereFanOutIsNeverComputed(t *testing.T) {
	got := RenderDoctor(nil, 0, 0, []*adapter.Adapter{staticAdapter(t, "go")})
	for _, needle := range []string{"lru_cache", "session-scoped fixtures", "CAVEAT"} {
		if strings.Contains(got, needle) {
			t.Errorf("a static-only repository was shown %q:\n%s", needle, got)
		}
	}
	if !strings.Contains(got, "stays empty") {
		t.Errorf("the empty-map line itself must survive:\n%s", got)
	}
}

// An unseeded COVERAGE repository keeps it: fan-out is empty today and real after
// `rtdd seed`, so the reader is about to have exactly the number the caveat qualifies.
func TestAnUnseededCoverageRepositoryKeepsTheCaveat(t *testing.T) {
	got := RenderDoctor(nil, 0, 0, []*adapter.Adapter{coverageAdapter(t, "python")})
	if !strings.Contains(got, doctor.Caveat) {
		t.Errorf("an unseeded coverage repository lost the caveat:\n%s", got)
	}
}

// A populated table always keeps it — that is the case it was written for, and a mixed
// repository with any rows at all is showing a real number.
func TestAPopulatedFanOutTableKeepsTheCaveat(t *testing.T) {
	hubs := []doctor.Hub{{Path: "src/a.py", TestCount: 3, Fraction: 0.5}}
	got := RenderDoctor(hubs, 6, 0, []*adapter.Adapter{staticAdapter(t, "go")})
	if !strings.Contains(got, doctor.Caveat) {
		t.Errorf("a real fan-out table lost its caveat:\n%s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rtdd/ -run 'FanOutCaveat|UnseededCoverageRepositoryKeeps|PopulatedFanOut' -v`
Expected: FAIL on the first — `a static-only repository was shown "lru_cache"`. The other two pass already and are the regression guard.

- [ ] **Step 3: Write minimal implementation**

In `RenderDoctor`'s empty-hub branch, print the caveat only when fan-out could ever be
computed here — reusing `selectionSplit`, the same helper `emptyFanOutLine` uses, so the
two lines cannot disagree about what kind of repository this is:

```go
	if len(hubs) == 0 {
		b.WriteString(emptyFanOutLine(detected))
		// The caveat qualifies a fan-out NUMBER. With no table and no adapter that could
		// ever produce one, there is no number for it to qualify — and its four examples
		// are Python mechanisms, in a repository whose adapter is not Python, one line
		// after doctor has said fan-out is never computed here (PRD #233 AC9c). A
		// coverage adapter keeps it: its table is empty today and real after `rtdd seed`.
		if _, coverage := selectionSplit(detected); len(coverage) > 0 || len(detected) == 0 {
			b.WriteString("\n")
			b.WriteString(doctor.Caveat + "\n")
		}
		return b.String()
	}
```

The populated branch is untouched: a repository with rows is showing a real number,
whatever produced it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/rtdd/... ./internal/doctor/... -count=1`
Expected: PASS. `internal/doctor`'s `TestCaveatNamesEveryOncePerProcessMechanism` is untouched — the string is unchanged, only whether `doctor` prints it.

- [ ] **Step 5: Commit**

```bash
git add cmd/rtdd/doctor.go cmd/rtdd/doctor_test.go
git commit -m "M6e: doctor prints the fan-out caveat only where fan-out can be computed"
```

---

### Task 13 — `protocol/PROTOCOL.md`, the regenerated front-ends, and the drift check

**Discharges:** spec §6. PRD AC10.

**Files:**
- Modify: `protocol/PROTOCOL.md`, `internal/contract/contract_test.go`
- Regenerate: `internal/install/protocol.md`, `dist/SKILL.md`, `dist/AGENTS.md`, `dist/cursor/*.mdc`

**Interfaces:** none in Go. One new protocol section, `id=fidelity`.

- [ ] **Step 1: Write the failing test**

Append to `internal/contract/contract_test.go`:

```go
// Spec §6: the agent reads the fidelity statement BEFORE it reads a selection, so it has
// to be in the generated front-ends rather than only in the tool's output. The existing
// drift check (`rtdd-gen check` and the protocol.md diff in ci-local.sh) then keeps every
// target in step; this test is what fails if the source text is missing altogether.
func TestProtocolStatesSelectionFidelity(t *testing.T) {
	src := readRepoFile(t, "protocol/PROTOCOL.md")
	for _, needle := range []string{
		"selection_fidelity",
		"execution-derived",
		"static",
		"weaker evidence",
		"`TS`",
	} {
		if !strings.Contains(src, needle) {
			t.Errorf("protocol/PROTOCOL.md does not state %q", needle)
		}
	}
}

// M6b added the TS tier and left this list naming five tiers. An agent told the tier is
// one of five values, handed a sixth, has been given a contract its input violates.
func TestProtocolTierListNamesEveryTier(t *testing.T) {
	src := readRepoFile(t, "protocol/PROTOCOL.md")
	for _, tier := range []string{"`empty`", "`direct`", "`T0`", "`T1`", "`TS`", "`T2`"} {
		if !strings.Contains(src, tier) {
			t.Errorf("the protocol's tier list omits %s", tier)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/contract/ -run 'ProtocolStatesSelectionFidelity|ProtocolTierList' -v`
Expected: FAIL — `protocol/PROTOCOL.md does not state "selection_fidelity"` and `the protocol's tier list omits \`TS\``.

- [ ] **Step 3: Write minimal implementation**

In `protocol/PROTOCOL.md`, add a section between `empty` (order 50) and `json` (order 60):

```
<!-- rtdd:section id=fidelity title="Which fidelity you are reading" targets=skill,agents,mdc order=55 -->
Every selection says what it was derived from, and the two are not equivalent evidence.

- **execution-derived** — the tests come from coverage recorded while this repository's
  suite really ran. This is the tier RTDD exists for.
- **static** — the tests come from declared file correspondence and from imports. Nothing
  was instrumented. A static selection can miss a test an execution-derived selection would
  have caught, so passing it is weaker evidence than passing an execution-derived one.
- **none** — nothing narrower than the full suite could be derived.

`--json` carries this as `selection_fidelity`, which is always one of those three strings
and is never null. The human tier line carries it as a suffix: `tier: TS (static)`.

In a repository whose adapter records no coverage there is no uncovered report at all —
no line can be known to be covered or uncovered when nothing was instrumented — and
`rtdd run` says so in one line rather than reporting every changed line as uncovered.
<!-- rtdd:variant target=agents -->
Every selection states its fidelity: `execution-derived` (from recorded coverage),
`static` (from declared correspondence and imports — weaker evidence, it can miss a test
coverage would have caught), or `none` (the full suite). `--json` carries it as
`selection_fidelity`; the tier line carries it as `tier: TS (static)`. An adapter that
records no coverage produces no uncovered report.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->
```

In the `json` section, add `"selection_fidelity": "static"` to the example object and
replace the tier sentence: `` `tier` is one of `empty`, `direct`, `T0`, `T1`, `TS`,
`T2`. `TS` is the static tier … ``.

Then regenerate:

```bash
go run ./cmd/rtdd-gen render
cp protocol/PROTOCOL.md internal/install/protocol.md
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/... -count=1 && go run ./cmd/rtdd-gen check && go run ./cmd/rtdd-gen verify && diff -u protocol/PROTOCOL.md internal/install/protocol.md`
Expected: PASS, `check` and `verify` exit 0, `diff` silent.

- [ ] **Step 5: Commit**

```bash
git add protocol/PROTOCOL.md internal/install/protocol.md dist internal/contract/contract_test.go
git commit -m "M6e: the protocol and every generated front-end state the selection fidelity"
```

---

### Task 14 — `README.md`, `docs/outcomes/`, and the pre-registered verdict

**Discharges:** spec §6 and §7. PRD AC11 and AC12.

**Files:**
- Modify: `README.md`, `docs/outcomes/README.positive.md`, `docs/outcomes/README.negative.md`

**Interfaces:** none. Prose, and `outcomes_test.go`'s byte-identical check is the gate.

- [ ] **Step 1: Write the failing test**

Append to `outcomes_test.go`:

```go
// PRD #233 AC11: "It is Python only." was true before the static tier and is not true
// after it. The replacement is the two-tier statement, not a deletion — the Python-only
// LIMIT is still real for execution-derived selection, and dropping the bullet would
// quietly upgrade every non-Python repository's evidence.
func TestREADMEStatesTheTwoTiersRatherThanPythonOnly(t *testing.T) {
	for _, name := range []string{
		"README.md",
		filepath.Join("docs", "outcomes", "README.positive.md"),
		filepath.Join("docs", "outcomes", "README.negative.md"),
	} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if strings.Contains(text, "**It is Python only.**") {
			t.Errorf("%s still claims Python only", name)
		}
		for _, needle := range []string{"execution-derived", "static", "weaker evidence"} {
			if !strings.Contains(text, needle) {
				t.Errorf("%s does not state %q", name, needle)
			}
		}
	}
}

// AC12: the pre-registered kill condition is reported in the README, win or lose, and it
// names the measurement rather than asserting a conclusion.
func TestREADMEReportsThePreRegisteredStaticVerdict(t *testing.T) {
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{
		"bench/results/flask/summary.md",
		"path heuristic",
		"pre-registered",
	} {
		if !strings.Contains(text, needle) {
			t.Errorf("README.md does not cite %q for the static-tier verdict", needle)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run 'READMEStatesTheTwoTiers|READMEReportsThePreRegistered' -v`
Expected: FAIL — `README.md still claims Python only` for all three files.

- [ ] **Step 3: Write minimal implementation**

Read `bench/results/{flask,httpie}/summary.md` first — the verdict written here is the
one Task 7 published, not the one this plan predicted. On the corpus as it stands that is
the kill branch (see *What the corpus already says*), so the bullet reads in that
direction; if Task 7 published a win instead, write the win with the same citation
discipline.

Replace the `**It is Python only.**` bullet in `README.md` with:

```markdown
- **Two tiers, and only one of them is measured coverage.** *Execution-derived* selection —
  the tier RTDD exists for — is Python only. Per-test attribution does not exist in the
  JavaScript or Go ecosystems: Istanbul and v8 coverage carry aggregate counters with no
  test dimension ([vitest#6735](https://github.com/vitest-dev/vitest/issues/6735) has
  requested it since October 2024), and Go's `-coverprofile` has no test dimension either
  while per-test isolation costs a prebuilt binary driven once per test. Every other
  language gets *static* selection instead: declared file correspondence and imports, with
  nothing instrumented. Every surface says which one you are reading, because a static
  selection can miss a test an execution-derived one would have caught and passing it is
  weaker evidence. **The static tier was pre-registered against the naive path heuristic
  and did not beat it** on the replay corpus — see
  [`bench/results/flask/summary.md`](bench/results/flask/summary.md) and
  [`bench/results/httpie/summary.md`](bench/results/httpie/summary.md), where the primary
  `natural` population has no detecting commits at all and the only population with ground
  truth scores it level with `path`. It ships because a repository RTDD cannot instrument
  is otherwise offered nothing, not because it is measured to be better.
```

Copy the same bytes into `docs/outcomes/README.positive.md` — `outcomes_test.go` requires
byte identity with the selected outcome, and `docs/outcomes/SELECTED` is `positive`. Make
the same change in `docs/outcomes/README.negative.md` in that file's own prose; it is not
byte-compared, and a negative-outcome README that still says "Python only" would be stale
the moment it were selected.

Also update the `Python only, today.` line near the top of `README.md` and both outcome
files to the two-tier form.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test . -count=1`
Expected: PASS, `TestRootREADMEMatchesSelectedOutcome` included.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/outcomes outcomes_test.go
git commit -m "M6e: the README states both tiers and the pre-registered static verdict"
```

---

## Definition of Done

**The evidence (§7)**

- [ ] `bench/results/{flask,httpie}/summary.json` each carry a `static` entry under
      `strategies`, and `derived_arms == ["static"]`.
- [ ] Each `summary.md` publishes, for `static` beside `rtdd`, `path` and `full`,
      change-level recall, selection ratio and selected-duration fraction.
- [ ] Each `summary.md` carries the static verdict, in the same primary-plus-labelled-
      secondary shape as the rtdd-vs-`path` one, printed win or lose.
- [ ] Each `summary.md` discloses the model: the four `test_for` templates, the
      `importgraph` level-2 source, `static ⊇ importgraph` by construction, and that the
      adapter it models does not ship.
- [ ] No wall-clock or isolation row exists for `static`, and one sentence says why and
      names the selected-duration fraction as its published cost figure.
- [ ] Every wall-clock figure in every regenerated table carries `p50`, `p90` and `worst`
      beside its mean; `assert_distribution_beside_mean` runs inside `render_markdown` and
      passes.
- [ ] `commits.jsonl` grew by exactly one derived record per commit record per variant,
      and no pre-existing line changed.
- [ ] `replay derive` is idempotent to the byte, refuses a repo whose records cannot
      support the derivation, and leaves `config.json` untouched.

**The honesty surfaces (§6)**

- [ ] `rtdd which --json` and `rtdd run --json` both carry `selection_fidelity`, valued
      `execution-derived`, `static` or `none`, never null, for every tier.
- [ ] Each per-adapter block in a polyglot document carries its own fidelity.
- [ ] A `TS` selection's human tier line reads `tier: TS (static)`; a coverage tier's line
      is byte-identical to before.
- [ ] A `TS` selection carries `doctor.StaticCaveat` in `warnings` and on stderr; a
      coverage selection carries neither.
- [ ] `rtdd run` against a `coverage: none` adapter prints no `UNCOVERED` line, says in
      one line why there is no line-level report, and prints no `rows in the map` clause.
- [ ] `rtdd run --json` against the same adapter reports `uncovered.available: false` with
      a reason that does not tell the caller to run the command they just ran.
- [ ] A test drives a `coverage: none` fixture through `run` and asserts the output
      contains neither `UNCOVERED` nor `rows in the map`.
- [ ] `rtdd doctor` prints no fan-out CAVEAT in a repository where fan-out can never be
      computed, and still prints it for an unseeded coverage repository and for any
      populated table.
- [ ] `PROTOCOL.md` states the three fidelities and the `TS` tier; `rtdd-gen check` and
      `rtdd-gen verify` exit 0; `diff -u protocol/PROTOCOL.md internal/install/protocol.md`
      is silent.
- [ ] `README.md` states both tiers, cites the published verdict, and is byte-identical to
      `docs/outcomes/README.positive.md`; `README.negative.md` carries the same change.

**The constraints this milestone had to hold**

- [ ] `~/.cache/rtdd-bench` was never deleted, re-keyed or invalidated, and no task read it.
- [ ] `cache.key()` is unchanged; `bench/replay/cache.py` is not in the diff.
- [ ] No benchmark was re-run: `git log -p` shows no invocation of `replay`, `session`,
      `replay_repo` or `clone_pinned`, and `bench/results/*/config.json` is unchanged.
- [ ] `bench/results/sqlfluff/` is absent from the entire diff.
- [ ] `internal/selector` is absent from the entire diff; a seeded Python repository's
      selection, tier and reason are byte-identical.
- [ ] `adapters/*.yaml` are byte-frozen; `internal/contract`'s sha256 guard passes.
- [ ] `bench/tests/test_registry_complete.py` is unmodified and passes.
- [ ] No existing test was deleted, skipped or weakened, and no CI step was removed.

**Hygiene**

- [ ] `scripts/ci-local.sh` exits 0, and its output contains `==> bench replay gate`.
- [ ] `go.mod` still lists exactly `gopkg.in/yaml.v3` and `modernc.org/sqlite`; `bench`'s
      dependency set is unchanged.
- [ ] `docs/plans/00-interfaces.md` records `selection_fidelity` beside `warnings`; no name
      in the implementation diverges from it.
- [ ] Every task's commit is separate and its test was seen to fail before its
      implementation was written.

## What this plan deliberately leaves undone

Everything below is real work; none of it belongs to PRD #233, and no task above may start
it.

- **Re-running any benchmark, and anything that would require one.** No new corpus repo,
  no new variant, no re-measured wall-clock, no second static model. The static arm's
  level 2 is the committed `importgraph` selection precisely because recomputing an import
  graph means materialising a tree, and materialising a tree means the cache, and the
  cache is the thing this PRD is built around not disturbing. If a future milestone wants
  level 2 recomputed under different templates, it re-derives — it does not re-run.
- **A hardware fingerprint in `cache.key()` — issue #218, an OPEN HUMAN DECISION.** It
  would orphan 201 MB of ground truth. Nothing here touches the key, and the wall-clock
  numbers this milestone publishes are the ones already committed.
- **Shipping a `test_for` for Python.** `adapters/python.yaml` stays byte-frozen and stays
  the coverage adapter. The four templates in `bench/replay/derive.py` are a *model* of
  what a static Python adapter would declare, live in `bench/`, and never move into
  `adapters/`. Doing so would give every Python repository a second, weaker selection path
  competing with the map, which is the one thing spec §4.1's byte-identical promise
  forbids.
- **Whether the static tier changes RTDD's publication positioning.** The PRD names this a
  `wayfinder` decision. Task 14 reports the measured verdict and cites it; it does not
  restructure the README's argument, retitle the project, or decide whether a tier that
  did not beat `path` should keep shipping. It ships, the README says what it measured,
  and the decision is left where it belongs.
- **SWE-bench, Axis 1.** `bench/swebench/` is untouched. `scripts/ci-prereg.sh` keeps its
  existing steps unchanged; Task 8 adds a step beside it and removes none.
- **Recomputing the static tier's RANKING offline.** Level 3 orders and never admits
  (`06-m6b-static-tier.md` decision 1), so no metric §7 asks for depends on it, and the
  derivation reconstructs the selected set only. A future question about *order* — does a
  static selection put the failing test near the front — needs a per-position record the
  harness does not currently write, and that is a new record type, not a derivation.
- **`selection_fidelity` anywhere but `which` and `run`.** `rtdd status`, `rtdd explain`
  and `rtdd map` gain no field. `rtdd doctor` already publishes per-adapter fidelity from
  M6a and is edited here only to stop printing a caveat about a number it does not have.
- **Per-test coverage outside Python.** Audit A5 stands; spec §3 keeps it a non-goal. The
  static tier exists because that route is closed, this milestone measures what that costs,
  and nothing here claims the two are equivalent.
