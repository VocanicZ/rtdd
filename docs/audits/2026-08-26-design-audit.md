# Design audit — 2026-08-26

Three independent adversarial audits of the v1 design (soundness, feasibility, positioning).
This record keeps the findings that changed the design. Measurements marked **[M]** were
run on the audit machine (coverage.py 7.15.4, pytest 9.0.3, vitest 4.1.11, Node 22.23.0,
Go 1.24.0); **[V]** verified against a cited source.

## Findings that killed a contribution

### A1. Import-time code is attributed to no test **[M — reproduced independently]**

```
src/constants.py  (dataclass + module constant, imported and asserted by 2 passing tests)
  → contexts: ['<EMPTY>']          ← zero test attribution
src/logic.py
  → contexts: ['<EMPTY>', 'tests/test_it.py::test_logic|run']
```

pytest imports every test module during collection, before any dynamic context is set, so
all import-time execution lands in the empty context `''`.

Under v1's rule ("a changed instrumentable file whose T0 is empty → RED"), a correctly
tested `constants.py` hard-REDs. The affected class is very large in Python: dataclasses,
enums, config modules, Pydantic/attrs models, Django models, SQLAlchemy declarative bases,
route decorators, Protocol/ABC definitions, `__init__.py` re-exports. Same shape in JS
(module-level `export const`, decorator evaluation) and Go (`init()`, package vars).

**Consequence:** hard-RED as a gate is unusable. See D-N1, D-N2.

### A2. Hard-RED was file-granular, so it detected new *files*, not new *code*

Adding a function to an already-covered file leaves T0 non-empty → no RED → green on
uncovered code. That is the exact pathology the v1 spec opened by describing. The headline
demo only reproduced when the agent happened to create a new file.

**Consequence:** the uncovered signal must be line-granular. See D-N2.

### A3. A newly written test is in no tier and never runs

T0 is defined as "tests whose `f` intersects the changed file set". A new test has no row,
so `rtdd cycle --expect red` — step 3 of v1's own loop — never executed the test the agent
had just written. Contribution #2 was unreachable.

**Consequence:** changed and new test files are always executed directly, ahead of tiering.
See D-N5.

### A4. Rows narrowed silently, with no merge involved

v1 §4 said each cycle "rewrites the rows of the tests it just executed". A subset run
legitimately yields *less* coverage than the seed run, because import-time and
first-caller-wins lines migrate to whichever test runs first in that subset. A failing test
records a truncated prefix of its real coverage. Both narrow `f`, on one branch, with no
merge conflict.

v1's "can only widen, never narrow" was true of the union merge driver and was
over-generalised into a property of the whole design.

**Consequence:** `f` is unioned, never replaced, outside a full re-seed. See D-N6.

### A5. Per-test attribution does not exist in JS or Go **[M + V]**

- **JS:** Istanbul/v8 coverage entries are `["path","statementMap","fnMap","branchMap","s","f","b","meta"]` — aggregate counters, no test dimension. lcov's `TN:` field exists in the format and is emitted empty. [vitest#6735](https://github.com/vitest-dev/vitest/issues/6735) requests exactly this; open since Oct 2024, no maintainer response. Per-test-*file* isolation measured at **27.5×** (2.57 s → 77.8 s for 30 files). Batched attribution instead produces a widening ratchet that drives selection ratio to 1.0.
- **Go:** default `-coverprofile` records **count 0** for every package except the one under test **[M]**; `-coverpkg=./...` is mandatory for a correct file set and still gives no test dimension. Per-test needs a prebuilt `-c` binary driven once per test (**7 ms floor on a trivial package, re-running `TestMain` each time**), or `runtime/coverage.ClearCounters()` with an injected `TestMain` — codegen in the user's repo. Subtests are invisible to `go test -list` and their names are not round-trippable (spaces mangled, `/` collides with the separator).

**Consequence:** v1 is Python-only. See D-N3.

### A6. Three of five `parse:` formats structurally cannot carry a test identifier **[M]**

`coverage lcov` and `coverage xml` have no `--show-contexts` option (`grep -c context out.xml` → 0). gocover's four fields have no test identifier. JaCoCo's XML report is session-merged. Only `coverage json --show-contexts` works, and only under a non-default flag.

Also **[M]**: `--show-contexts` JSON is 12× larger than plain (45 KB → 537 KB on a 300-test toy; ~150 B per line-context pair). Extrapolates to ~0.5–1.2 GB at 10k tests. The `.coverage` SQLite file was 96 KB for the same data and *is* the bipartite relation:

```sql
SELECT DISTINCT f.path, c.context FROM line_bits lb
  JOIN file f ON f.id = lb.file_id JOIN context c ON c.id = lb.context_id
```

**Consequence:** read SQLite directly. See D-N4.

### A7. `COVERAGE_CORE=sysmon` silently drops contexts, and is the default on Python 3.14+ **[M + V]**

```
sysmon-context=test   0.86 s   → 31 contexts recorded, not 302
CoverageWarning: Dynamic contexts aren't supported with core=sysmon (no-sysmon-context)
```

A warning, not an error — exit 0 with a 90%-missing map. Per the [coverage.py changelog](https://coverage.readthedocs.io/en/latest/changes.html), sysmon became the default on Python 3.14+ where supported. The fast core is therefore unavailable with per-test attribution; Python is locked to ~2× tracing overhead **[M: 2.0–2.4×]**.

**Consequence:** force `COVERAGE_CORE=ctrace`; treat the `no-sysmon-context` warning as fatal.

### A8. The benchmark could not fail, and could not run

- **Degenerate:** for a single-file mutant on a freshly seeded map, T0 is by construction a superset of `F_full`. Recall ≈ 100% because the experiment measures whether coverage.py is deterministic.
- **Blind to the actual failure modes:** mutants are single-token, single-file, in-place edits to existing covered code. They never create files, rename, delete, or touch config — i.e. they cannot exercise the uncovered-change path at all.
- **Infeasible:** 500 mutants × 3 repos with a 40-minute suite in the corpus ≈ **455 CPU-hours, 76 parallel 6h jobs, ~$218 per run** against GitHub Actions' 6h/job cap.
- **Wrong metric:** `detected(m) = |F_rtdd| > 0` scores a detection when an unrelated flaky test fails. Meta publishes both mutant-level and test-level recall; v1 published only the flattering one.
- **No baselines:** RTDD vs full suite only. Missing: pytest-testmon, a naive `tests/test_<module>.py` heuristic, `pytest --lf`, static import graph, `pytest -n auto`, and random selection at equal ratio.

**Consequence:** benchmark replaced. See D-N7.

### A9. The novelty claim was false **[V — verified directly]**

[arXiv:2603.17973](https://arxiv.org/abs/2603.17973) — *TDAD: Test-Driven Agentic
Development*, Alonso / Yovine / Braberman, submitted 18 March 2026. Builds a source↔test
dependency map so an agent knows which tests to verify before committing; ships as an agent
skill file. SWE-bench Verified: regressions **6.08% → 1.82%**. A
[TypeScript port](https://github.com/fmguerreiro/tdad-ts) already exists.

v1 §3 claimed "none of them define how an agent should drive a TDD inner loop". False.

**TDAD's second result is the one that reshaped this project:** adding TDD *procedural*
instructions without targeted test context raised regressions to **9.94% — worse than no
intervention**. Their conclusion: surfacing contextual information outperforms prescribing
procedural workflows.

v1 was overwhelmingly procedural — gates, expected phases, enforcing exit codes. That is
the arm TDAD measured as harmful.

Also unacknowledged in v1: [Wallaby.js's MCP server](https://wallabyjs.com/docs/features/ai/)
exposes `wallaby_coveredLinesForTest` / `wallaby_allTestsForFileAndLine` to Claude Code and
Cursor today; [Infinitest](https://github.com/infinitest/infinitest) (2007) marketed
classpath impact analysis for "tight TDD cycles"; SonarQube's new-code coverage gate and
Codecov's patch status are prior art for "uncovered change is a failure";
[pytest-testmon](https://www.testmon.org/blog/determining-affected-tests/) has shipped
method-level AST checksums since ~2016 — finer-grained than v1's deferred v2 plan.

**Consequence:** §3 rewritten to cite prior art; the contribution restated as the
dynamic-vs-static question. See D-N8.

## Findings absorbed without changing direction

- **Changed-file set was undefined.** `git diff HEAD` excludes untracked files, so a newly written file was invisible and the flagship demo could not fire. Now defined explicitly (D-N9).
- **Empty selection returned exit 0**, the drain every under-selection bug flowed into. Now an explicit distinct outcome.
- **Ranking + `-x` defeated `--expect red`**: last-failed-first put a quarantined test at the head of T0, satisfying "a red exists" before the new test ran. Moot under the reshaped design (no gate), but the pre-tier in D-N5 fixes it regardless.
- **Refactors false-REDed.** Extract-module and `git mv` both produced an unsatisfiable RED on a fully green suite.
- **Memoization/session fixtures give hubs a fan-out of 1.** `@lru_cache`, singletons, `sync.Once`, session-scoped fixtures execute once under whichever test ran first. `doctor` would report the most-coupled file as the cleanest. Documented as a known limitation (§9).
- **Union merge is textually safe but semantically stale.** It cannot narrow relative to its two parents, but the merged map remains stale relative to the merged code. Merge commits now escalate.
- **`s` and `d` had no data source.** Neither is in any coverage report; both come from the test report. Adapter now specifies one.
- **Adapters-as-data was false.** `parse:` is a closed enum implemented in engine code, and the schema could not express `show_contexts`, env vars, path normalisation, exit-code maps, or polyglot detection. Claim dropped.
- **Milestone sizing was inverted** — M1 was the entire engine. Re-sliced.
- **argv limits:** 8,000 test ids ≈ 613 KB — fits Linux `ARG_MAX` (2 MB) but is 74× over the Windows `CMD` limit; pytest has no argfile option. Chunking policy now required.
- **Map size:** file-level rows ≈ 200 B, so 10k tests ≈ 2 MB — acceptable to commit. Line-level would have been 50–100 MB+ and is therefore never persisted (D-N4).

## Tried and could not break

- Union merge genuinely cannot narrow relative to its parents. Every narrowing path found was elsewhere (A4).
- Over-selection is safe *for selection*. The error was transferring that argument to the RED rule, where file granularity under-detects (A2).
- Ranking cannot cause a miss on its own; only `-x` makes it unsound.
- Committing the map is sound as a mechanism. Its danger was making machine-local narrowing persistent and shared.
