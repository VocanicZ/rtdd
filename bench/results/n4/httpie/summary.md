# Axis 2 — real-commit replay: `httpie`

**Hardware:** Intel(R) Xeon(R) CPU E5-2697A v4 @ 2.60GHz, 32 cores, 32768 MiB, Linux-7.0.0-3-pve-x86_64-with-glibc2.41, Python 3.12.13 · fingerprint `65b8fad78747d6ef`
**Binary:** rtdd 0.3.0-dev (commit 7c4721065202961a19cdd4e87dfa81ef65b72c48, built 2026-10-07T23:55:22Z) · **corpus digest:** `6145ea8f568d` · **config digest:** `b3e214958252`
**Tools:** coverage 7.16.2, pytest 9.1.1, pytest-cov 7.1.0, pytest-reportlog 1.0.0, pytest-testmon 2.2.0, pytest-xdist 3.8.0
**Replayed commits:** 34 (skipped: 0) · shipped defaults only, no tuning flags

verdict: change-level recall rtdd=0.000 vs path heuristic=0.000; selected-duration fraction rtdd=0.022 vs path=0.023 — rtdd does NOT clearly beat the naive path heuristic — per the pre-registered criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this must be stated in the README

verdict (probe, upper bound — map seeded at the child commit, never pooled with `natural`): change-level recall rtdd=0.000 vs path heuristic=0.000; selected-duration fraction rtdd=0.000 vs path=0.000 — rtdd does NOT clearly beat the naive path heuristic — per the pre-registered criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this must be stated in the README

## Per-strategy — `natural`, all strata pooled within this repo

| strategy | cycles | detecting | change recall | test recall (micro) | test recall (macro) | selection ratio | selected duration | escalation |
|---|---|---|---|---|---|---|---|---|
| rtdd | 17 | 2 | 0.000 (0/2) | 0.000 (0/2) | 0.000 (0/2) | 0.053 (918/17476) | 0.022 (49875/2.24895e+06) | 0.000 (0/17) |
| rtdd-r12 | 17 | 2 | 0.000 (0/2) | 0.000 (0/2) | 0.000 (0/2) | 0.056 (978/17476) | 0.024 (54611/2.24895e+06) | 0.000 (0/17) |
| path | 17 | 2 | 0.000 (0/2) | 0.000 (0/2) | 0.000 (0/2) | 0.032 (567/17476) | 0.023 (52799/2.24895e+06) | 0.000 (0/17) |
| full | 17 | 2 | 1.000 (2/2) | 1.000 (2/2) | 1.000 (2/2) | 1.000 (17476/17476) | 1.000 (2.24895e+06/2.24895e+06) | 1.000 (17/17) |

## Rounds against /tdd

`rtdd` runs Round 1 alone, `rtdd-r12` runs Rounds 1+2, and `full` is `/tdd`, the full suite every cycle; time in tests is the selected tests' share of the same commit's full-suite durations.

| strategy | change recall | test recall (micro) | time in tests (ms) | time vs /tdd |
|---|---|---|---|---|
| `rtdd` — Round 1 | 0.000 (0/2) | 0.000 (0/2) | 49875 | 0.022 (49875/2.24895e+06) |
| `rtdd-r12` — Rounds 1+2 | 0.000 (0/2) | 0.000 (0/2) | 54611 | 0.024 (54611/2.24895e+06) |
| `full` — `/tdd` | 1.000 (2/2) | 1.000 (2/2) | 2248947 | 1.000 (2.24895e+06/2.24895e+06) |

## Stratified by |F_full| — the `|F_full| == 1` stratum is where selection safety is genuinely under test

| strategy | stratum | n | change recall | test recall (micro) |
|---|---|---|---|---|
| rtdd | 1 | 2 | 0.000 (0/2) | 0.000 (0/2) |
| rtdd-r12 | 1 | 2 | 0.000 (0/2) | 0.000 (0/2) |
| path | 1 | 2 | 0.000 (0/2) | 0.000 (0/2) |
| full | 1 | 2 | 1.000 (2/2) | 1.000 (2/2) |

## Uncovered-report false signal

- fired on 0 of 17 cycles (0.000 (0/17))
- change-level false-signal rate: n/a (0/0)
- line-level false-signal rate: n/a (0/0)
- `rtdd run` refused on 0 of 17 cycles — the shipped binary exits 2 rather than execute a map that names a test the tree no longer collects, and those cycles have no uncovered report

## Isolation

A subset run that does not reproduce `F_full ∩ selected` is a finding, not a timing number, so it is published here whether or not wall-clock was permitted.

- isolation violations: 6 of 136 really-executed subset runs
  - `full`: 6 of 34
  - `path`: 0 of 34
  - `rtdd`: 0 of 34
  - `rtdd-r12`: 0 of 34

## Wall-clock

| strategy | measurement | n | mean | p50 | p90 | worst |
|---|---|---|---|---|---|---|
| full | full uninstrumented | 34 | 137208 ms | 137052 ms | 145852 ms | 155221 ms |
| full | subset instrumented | 34 | 151772 ms | 150270 ms | 163784 ms | 172571 ms |
| full | subset uninstrumented | 34 | 137580 ms | 137747 ms | 147601 ms | 163895 ms |
| path | full uninstrumented | 34 | 137208 ms | 137052 ms | 145852 ms | 155221 ms |
| path | subset instrumented | 34 | 2110 ms | 0 ms | 0 ms | 55178 ms |
| path | subset uninstrumented | 34 | 1746 ms | 0 ms | 0 ms | 47494 ms |
| rtdd | full uninstrumented | 34 | 137208 ms | 137052 ms | 145852 ms | 155221 ms |
| rtdd | subset instrumented | 34 | 2143 ms | 0 ms | 0 ms | 56270 ms |
| rtdd | subset uninstrumented | 34 | 1789 ms | 0 ms | 0 ms | 49357 ms |
| rtdd-r12 | full uninstrumented | 34 | 137208 ms | 137052 ms | 145852 ms | 155221 ms |
| rtdd-r12 | subset instrumented | 34 | 2581 ms | 0 ms | 2796 ms | 60132 ms |
| rtdd-r12 | subset uninstrumented | 34 | 2035 ms | 0 ms | 2610 ms | 48658 ms |

One row per measurement rather than one cell: the population is bimodal — a cycle whose strategy selected nothing costs almost nothing, a cycle that selected the hub costs nearly a full run — so the mean sits between two modes and describes neither. `p50`, `p90` and `worst` are nearest-rank over the per-cycle samples in `commits.jsonl`, so each is a cycle that really ran. Isolation violations are per strategy and published above, under `## Isolation`.

A strategy that carries `Selection.exec_args` — `xdist` is the only one in the shipped set — runs **both** subset columns with those flags (`pytest -n auto`); per-test coverage contexts survive the parallel instrumented run, so that column is not silently serial either. The `full uninstrumented` column is always the serial full suite, which is what makes the two directly comparable.

## By variant

`probe` seeds every map-based strategy at the child commit and is therefore an **upper bound** on their selection quality; it gets its own table and is never pooled with `natural`.

### `natural`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 17 | 0.000 (0/2) | 0.000 (0/2) | 0.053 (918/17476) | 0.022 (49875/2.24895e+06) | — |
| rtdd-r12 | 17 | 0.000 (0/2) | 0.000 (0/2) | 0.056 (978/17476) | 0.024 (54611/2.24895e+06) | — |
| path | 17 | 0.000 (0/2) | 0.000 (0/2) | 0.032 (567/17476) | 0.023 (52799/2.24895e+06) | — |
| full | 17 | 1.000 (2/2) | 1.000 (2/2) | 1.000 (17476/17476) | 1.000 (2.24895e+06/2.24895e+06) | — |

### `probe`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 17 | 0.000 (0/3) | 0.000 (0/3) | 0.000 (0/17476) | 0.000 (0/2.25641e+06) | upper bound (map seeded at the child commit) |
| rtdd-r12 | 17 | 0.000 (0/3) | 0.000 (0/3) | 0.002 (34/17476) | 0.001 (2239/2.25641e+06) | upper bound (map seeded at the child commit) |
| path | 17 | 0.000 (0/3) | 0.000 (0/3) | 0.000 (0/17476) | 0.000 (0/2.25641e+06) | — |
| full | 17 | 1.000 (3/3) | 1.000 (3/3) | 1.000 (17476/17476) | 1.000 (2.25641e+06/2.25641e+06) | — |

