# Axis 2 — real-commit replay: `flask`

**Hardware:** Intel(R) Xeon(R) CPU E5-2697A v4 @ 2.60GHz, 32 cores, 32768 MiB, Linux-7.0.0-3-pve-x86_64-with-glibc2.41, Python 3.12.13 · fingerprint `65b8fad78747d6ef`
**Binary:** rtdd 0.3.0-dev (commit 7c4721065202961a19cdd4e87dfa81ef65b72c48, built 2026-10-07T23:55:22Z) · **corpus digest:** `6145ea8f568d` · **config digest:** `175e7590e4e5`
**Tools:** coverage 7.16.2, pytest 8.4.2, pytest-cov 7.1.0, pytest-reportlog 1.0.0, pytest-testmon 2.2.0, pytest-xdist 3.8.0
**Replayed commits:** 46 (skipped: 4) · shipped defaults only, no tuning flags

verdict: not computable (no detecting commits)

verdict (probe, upper bound — map seeded at the child commit, never pooled with `natural`): change-level recall rtdd=0.667 vs path heuristic=0.333; selected-duration fraction rtdd=0.036 vs path=0.009 — rtdd does NOT clearly beat the naive path heuristic — per the pre-registered criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this must be stated in the README

## Per-strategy — `natural`, all strata pooled within this repo

| strategy | cycles | detecting | change recall | test recall (micro) | test recall (macro) | selection ratio | selected duration | escalation |
|---|---|---|---|---|---|---|---|---|
| rtdd | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.149 (1682/11274) | 0.178 (6443/36239) | 0.000 (0/23) |
| rtdd-r12 | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.297 (3346/11274) | 0.328 (11896/36239) | 0.000 (0/23) |
| path | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.050 (568/11274) | 0.048 (1727/36239) | 0.000 (0/23) |
| full | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (11274/11274) | 1.000 (36239/36239) | 1.000 (23/23) |

## Rounds against /tdd

`rtdd` runs Round 1 alone, `rtdd-r12` runs Rounds 1+2, and `full` is `/tdd`, the full suite every cycle; time in tests is the selected tests' share of the same commit's full-suite durations.

| strategy | change recall | test recall (micro) | time in tests (ms) | time vs /tdd |
|---|---|---|---|---|
| `rtdd` — Round 1 | n/a (0/0) | n/a (0/0) | 6443 | 0.178 (6443/36239) |
| `rtdd-r12` — Rounds 1+2 | n/a (0/0) | n/a (0/0) | 11896 | 0.328 (11896/36239) |
| `full` — `/tdd` | n/a (0/0) | n/a (0/0) | 36239 | 1.000 (36239/36239) |

## Stratified by |F_full| — the `|F_full| == 1` stratum is where selection safety is genuinely under test

| strategy | stratum | n | change recall | test recall (micro) |
|---|---|---|---|---|

## Uncovered-report false signal

- fired on 0 of 23 cycles (0.000 (0/23))
- change-level false-signal rate: n/a (0/0)
- line-level false-signal rate: n/a (0/0)
- `rtdd run` refused on 0 of 23 cycles — the shipped binary exits 2 rather than execute a map that names a test the tree no longer collects, and those cycles have no uncovered report

## Isolation

A subset run that does not reproduce `F_full ∩ selected` is a finding, not a timing number, so it is published here whether or not wall-clock was permitted.

- isolation violations: 0 of 184 really-executed subset runs
  - `full`: 0 of 46
  - `path`: 0 of 46
  - `rtdd`: 0 of 46
  - `rtdd-r12`: 0 of 46

## Wall-clock

| strategy | measurement | n | mean | p50 | p90 | worst |
|---|---|---|---|---|---|---|
| full | full uninstrumented | 46 | 4219 ms | 4182 ms | 5439 ms | 5853 ms |
| full | subset instrumented | 46 | 7330 ms | 7208 ms | 8472 ms | 9409 ms |
| full | subset uninstrumented | 46 | 4544 ms | 4596 ms | 5339 ms | 6231 ms |
| path | full uninstrumented | 46 | 4219 ms | 4182 ms | 5439 ms | 5853 ms |
| path | subset instrumented | 46 | 725 ms | 0 ms | 2613 ms | 3527 ms |
| path | subset uninstrumented | 46 | 520 ms | 0 ms | 1793 ms | 2613 ms |
| rtdd | full uninstrumented | 46 | 4219 ms | 4182 ms | 5439 ms | 5853 ms |
| rtdd | subset instrumented | 46 | 1318 ms | 0 ms | 5157 ms | 7118 ms |
| rtdd | subset uninstrumented | 46 | 1010 ms | 0 ms | 3767 ms | 3969 ms |
| rtdd-r12 | full uninstrumented | 46 | 4219 ms | 4182 ms | 5439 ms | 5853 ms |
| rtdd-r12 | subset instrumented | 46 | 2462 ms | 0 ms | 7012 ms | 8166 ms |
| rtdd-r12 | subset uninstrumented | 46 | 1646 ms | 0 ms | 4010 ms | 4897 ms |

One row per measurement rather than one cell: the population is bimodal — a cycle whose strategy selected nothing costs almost nothing, a cycle that selected the hub costs nearly a full run — so the mean sits between two modes and describes neither. `p50`, `p90` and `worst` are nearest-rank over the per-cycle samples in `commits.jsonl`, so each is a cycle that really ran. Isolation violations are per strategy and published above, under `## Isolation`.

A strategy that carries `Selection.exec_args` — `xdist` is the only one in the shipped set — runs **both** subset columns with those flags (`pytest -n auto`); per-test coverage contexts survive the parallel instrumented run, so that column is not silently serial either. The `full uninstrumented` column is always the serial full suite, which is what makes the two directly comparable.

## By variant

`probe` seeds every map-based strategy at the child commit and is therefore an **upper bound** on their selection quality; it gets its own table and is never pooled with `natural`.

### `natural`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 23 | n/a (0/0) | n/a (0/0) | 0.149 (1682/11274) | 0.178 (6443/36239) | — |
| rtdd-r12 | 23 | n/a (0/0) | n/a (0/0) | 0.297 (3346/11274) | 0.328 (11896/36239) | — |
| path | 23 | n/a (0/0) | n/a (0/0) | 0.050 (568/11274) | 0.048 (1727/36239) | — |
| full | 23 | n/a (0/0) | n/a (0/0) | 1.000 (11274/11274) | 1.000 (36239/36239) | — |

### `probe`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 23 | 0.667 (2/3) | 0.667 (2/3) | 0.037 (422/11274) | 0.036 (1428/39557) | upper bound (map seeded at the child commit) |
| rtdd-r12 | 23 | 0.667 (2/3) | 0.667 (2/3) | 0.159 (1790/11274) | 0.173 (6829/39557) | upper bound (map seeded at the child commit) |
| path | 23 | 0.333 (1/3) | 0.333 (1/3) | 0.012 (134/11274) | 0.009 (343/39557) | — |
| full | 23 | 1.000 (3/3) | 1.000 (3/3) | 1.000 (11274/11274) | 1.000 (39557/39557) | — |

