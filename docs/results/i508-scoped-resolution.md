# Scoped call resolution (#508), before and after, per corpus row — 2026-10-26

Issue #508 replaced name-only `calls` linking with scoped resolution (receiver type, scope,
imports, package, and a capped fallback; spec §4.3). This page sets it against the v0.3.0
baseline published in [`n4-bench-rounds.md`](n4-bench-rounds.md) on the one corpus row it
could re-run, `flask`, and on the repository the issue was filed from, ORGE.

## Method

- **Before:** `bench/results/n4/flask/` as published:
  `rtdd 0.3.0-dev (commit 7c4721065202961a19cdd4e87dfa81ef65b72c48, built 2026-10-07T23:55:22Z)`,
  hardware fingerprint `65b8fad78747d6ef`. The only scanner change between that commit
  and #508's base is the `**/*.mdc` exclusion (#500), which `flask` has no file for.
- **After:** `bench/results/i508/flask/`, with
  `rtdd 0.3.0-dev (commit 2f510d4615843a5376747ffed554c70c1c4578b5, built 2026-10-25T01:53:47Z)`.
  It was built from #508's branch and was first on `PATH`. Corpus digest `6145ea8f568d`,
  hardware fingerprint `a22fadaee7dfe008`, shipped defaults:
  `uv run python -m replay.cli replay --repo flask --strategies rtdd,rtdd-r12,path,full --results-label i508`.
- **`httpie` was not re-run to completion.** Its replay ran for 20 hours on a CPU-capped
  host shared with other agents and was stopped. Its re-run is deferred to the #506
  release bench. There is no `bench/results/i508/httpie/`.
- **Time is not comparable across the two runs. Selection ratio is.** The after-run
  shared the host with other agents, at a load average of 60-100 on 32 cores. Its full
  suite summed to 584289 ms against the baseline's 36239 ms. "Time vs /tdd" is a ratio
  inside one run, so it is printed below, but contention inflates and skews it unevenly
  across tests. The `path` arm's own time ratio moved, 0.048 → 0.039, with an identical
  selection. **Selection ratio** — selected tests over collected tests — does not depend
  on load. The `path` arm selects exactly the same tests in both runs, 568/11274 and
  134/11274, which shows that the commits and the collected suites match.
- One run, nothing re-run to move a number. `flask` skipped the same 4 commit/variant
  pairs as the baseline did (`no-tests-collected` / `collect-error-or-empty`).

## `flask`

### `natural`

| arm | run | change recall | test recall (micro) | selection ratio | time vs /tdd |
|---|---|---|---|---|---|
| `rtdd` — Round 1 | before | n/a (0/0) | n/a (0/0) | 0.149 (1682/11274) | 0.178 (6443/36239) |
| `rtdd` — Round 1 | after | n/a (0/0) | n/a (0/0) | 0.053 (601/11274) | 0.027 (15749/584289) |
| `rtdd-r12` — Rounds 1+2 | before | n/a (0/0) | n/a (0/0) | 0.297 (3346/11274) | 0.328 (11896/36239) |
| `rtdd-r12` — Rounds 1+2 | after | n/a (0/0) | n/a (0/0) | 0.181 (2040/11274) | 0.118 (69111/584289) |
| `path` — path heuristic | before | n/a (0/0) | n/a (0/0) | 0.050 (568/11274) | 0.048 (1727/36239) |
| `path` — path heuristic | after | n/a (0/0) | n/a (0/0) | 0.050 (568/11274) | 0.039 (22508/584289) |

### `probe` — upper bound (map seeded at the child commit)

| arm | run | change recall | test recall (micro) | selection ratio | time vs /tdd |
|---|---|---|---|---|---|
| `rtdd` — Round 1 | before | 0.667 (2/3) | 0.667 (2/3) | 0.037 (422/11274) | 0.036 (1428/39557) |
| `rtdd` — Round 1 | after | 0.667 (2/3) | 0.667 (2/3) | 0.017 (192/11274) | 0.005 (3011/641659) |
| `rtdd-r12` — Rounds 1+2 | before | 0.667 (2/3) | 0.667 (2/3) | 0.159 (1790/11274) | 0.173 (6829/39557) |
| `rtdd-r12` — Rounds 1+2 | after | 0.667 (2/3) | 0.667 (2/3) | 0.101 (1135/11274) | 0.062 (40057/641659) |
| `path` — path heuristic | before | 0.333 (1/3) | 0.333 (1/3) | 0.012 (134/11274) | 0.009 (343/39557) |
| `path` — path heuristic | after | 0.333 (1/3) | 0.333 (1/3) | 0.012 (134/11274) | 0.013 (8062/641659) |

`/tdd` caught 3/3 in both runs.

**Reading.**
- Recall did not move. `natural` has no detecting cycle in either run, so there is no
  recall to measure. In `probe`, both arms catch the same 2 of 3 before and after.
- Selection fell. Round 1 runs about a third of the tests it did (`natural` 1682 → 601,
  `probe` 422 → 192), and Rounds 1+2 about 60% (3346 → 2040, 1790 → 1135).
- One isolation violation was recorded in the after-run, on `rtdd-r12`: 1 of 46
  subset runs. It is in `summary.json`.
- With three detecting cycles this is one measurement, not a trend.

## ORGE @ 52f902e — tests per main-code node

Built with `rtdd graph` and counted from `.rtdd/graph.json`. A node's tests are the test
nodes with an edge into it, using one path-based test classifier for both builds. That
classifier gives 138 where the issue's figure, from rtdd's own `test_files`, is 145.

| | before (#508's base) | after (#508) |
|---|---:|---:|
| `calls` edges | 8242 | 4271 |
| `implements` edges | 0 | 18 |
| `imports` / `ambiguous_calls` (`rtdd graph`) | — | 568 / 9 |
| max tests on one main node | 138 (`OrgeCommandLogic::get`, `MaterialRegistry::get`, `ActiveSet::get`, `MaterialPalette::get`) | 71 (`Material::builder`) |
| p50 / p90 / p99 tests per node (nonzero) | 5 / 24 / 138 | 4 / 19 / 63 |
| main nodes with ≥ 50 / ≥ 20 / ≥ 10 tests | 13 / 52 / 99 | 6 / 25 / 70 |
| main nodes with no test edge | 445 | 471 |

`captureBlockChange` (`BlockChangeCapture.java:101`) had 32 callees before and 15 after.
None of the 15 is another class's `get`, `levelFor` or `id`. The price is 26 more main
nodes with no test edge: some of those are true edges that the narrowing now misses.
Round 3 remains the net for them.
