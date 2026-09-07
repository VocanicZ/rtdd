# Axis 2 — real-commit replay: `flask`

**Hardware:** Intel(R) Xeon(R) CPU E5-2697A v4 @ 2.60GHz, 32 cores, 32768 MiB, Linux-7.0.0-3-pve-x86_64-with-glibc2.41, Python 3.12.13 · fingerprint `65b8fad78747d6ef`
**Binary:** sha256:e98120d465e10c204565bdc6c7026a667e418f6f5ea3663fe599396c4ec21f2a · **corpus digest:** `95cd1505766b` · **config digest:** `a91d0bf07807`
**Tools:** coverage 7.15.4, pytest 8.4.2, pytest-cov 7.1.0, pytest-reportlog 1.0.0, pytest-testmon 2.2.0, pytest-xdist 3.8.0
**Replayed commits:** 46 (skipped: 4) · shipped defaults only, no tuning flags

verdict: not computable (no detecting commits)

verdict (probe, upper bound — map seeded at the child commit, never pooled with `natural`): change-level recall rtdd=1.000 vs path heuristic=0.333; selected-duration fraction rtdd=0.243 vs path=0.010 — rtdd does NOT clearly beat the naive path heuristic — per the pre-registered criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this must be stated in the README

## Per-strategy — `natural`, all strata pooled within this repo

| strategy | cycles | detecting | change recall | test recall (micro) | test recall (macro) | selection ratio | selected duration | escalation |
|---|---|---|---|---|---|---|---|---|
| rtdd | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.235 (2650/11274) | 0.282 (10288/36469) | 0.304 (7/23) |
| importgraph | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.041 (460/11274) | 0.047 (1714/36469) | 0.000 (0/23) |
| lf | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.783 (8825/11274) | 0.790 (28810/36469) | 0.783 (18/23) |
| path | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.050 (568/11274) | 0.056 (2036/36469) | 0.000 (0/23) |
| static | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.050 (568/11274) | 0.056 (2036/36469) | 0.000 (0/23) |
| testmon | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.158 (1786/11274) | 0.190 (6939/36469) | 0.000 (0/23) |
| xdist | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (11274/11274) | 1.000 (36469/36469) | 1.000 (23/23) |
| random | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.235 (2650/11274) | 0.258 (9411/36469) | 0.000 (0/23) |
| full | 23 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (11274/11274) | 1.000 (36469/36469) | 1.000 (23/23) |

## The static arm

Spec §7 asks what the `TS` static tier is worth against the corpus where the coverage-derived answer is already known. `static` is scored here against `rtdd`, the naive `path` baseline and the `full` ceiling, on the three metrics the pre-registration names.

| arm | change recall | selection ratio | selected duration | mean subset uninstrumented | p50 | p90 | worst |
|---|---|---|---|---|---|---|---|
| `static` | n/a (0/0) | 0.050 (568/11274) | 0.056 (2036/36469) | not measured | not measured | not measured | not measured |
| `rtdd` | n/a (0/0) | 0.235 (2650/11274) | 0.282 (10288/36469) | 1284 ms | 0 ms | 3109 ms | 4045 ms |
| `path` | n/a (0/0) | 0.050 (568/11274) | 0.056 (2036/36469) | 255 ms | 0 ms | 1064 ms | 1680 ms |
| `full` | n/a (0/0) | 1.000 (11274/11274) | 1.000 (36469/36469) | 3133 ms | 3081 ms | 3558 ms | 4761 ms |

`static` carry no wall-clock record in this run — a derived arm **executed nothing** at all, and any arm can simply have gone unsampled. Nothing was invented to fill the gap: the cells read `not measured` rather than a blank that would read as zero, a figure synthesised from `durations_ms` (another execution's per-test time), or a row borrowed from an arm that really ran. The cost those arms do publish is the **selected duration** column, a ratio of the same commit's own recorded per-test durations and therefore independent of the machine.

## Stratified by |F_full| — the `|F_full| == 1` stratum is where selection safety is genuinely under test

| strategy | stratum | n | change recall | test recall (micro) |
|---|---|---|---|---|

## Uncovered-report false signal

- fired on 7 of 23 cycles (0.304 (7/23))
- change-level false-signal rate: 0.000 (0/7)
- line-level false-signal rate: 0.000 (0/30)
- `rtdd run` refused on 2 of 23 cycles — the shipped binary exits 2 rather than execute a map that names a test the tree no longer collects, and those cycles have no uncovered report

## Isolation

A subset run that does not reproduce `F_full ∩ selected` is a finding, not a timing number, so it is published here whether or not wall-clock was permitted.

- isolation violations: 0 of 192 really-executed subset runs
  - `full`: 0 of 24
  - `importgraph`: 0 of 24
  - `lf`: 0 of 24
  - `path`: 0 of 24
  - `random`: 0 of 24
  - `rtdd`: 0 of 24
  - `testmon`: 0 of 24
  - `xdist`: 0 of 24

## Wall-clock

| strategy | measurement | n | mean | p50 | p90 | worst |
|---|---|---|---|---|---|---|
| full | full uninstrumented | 24 | 3127 ms | 3001 ms | 4294 ms | 4486 ms |
| full | subset instrumented | 24 | 4860 ms | 4717 ms | 5537 ms | 5845 ms |
| full | subset uninstrumented | 24 | 3133 ms | 3081 ms | 3558 ms | 4761 ms |
| importgraph | full uninstrumented | 24 | 3127 ms | 3001 ms | 4294 ms | 4486 ms |
| importgraph | subset instrumented | 24 | 286 ms | 0 ms | 1356 ms | 2357 ms |
| importgraph | subset uninstrumented | 24 | 171 ms | 0 ms | 1063 ms | 1150 ms |
| lf | full uninstrumented | 24 | 3127 ms | 3001 ms | 4294 ms | 4486 ms |
| lf | subset instrumented | 24 | 4125 ms | 4663 ms | 6174 ms | 6621 ms |
| lf | subset uninstrumented | 24 | 2496 ms | 2790 ms | 3523 ms | 4994 ms |
| path | full uninstrumented | 24 | 3127 ms | 3001 ms | 4294 ms | 4486 ms |
| path | subset instrumented | 24 | 360 ms | 0 ms | 1577 ms | 2323 ms |
| path | subset uninstrumented | 24 | 255 ms | 0 ms | 1064 ms | 1680 ms |
| random | full uninstrumented | 24 | 3127 ms | 3001 ms | 4294 ms | 4486 ms |
| random | subset instrumented | 24 | 1958 ms | 0 ms | 4554 ms | 4936 ms |
| random | subset uninstrumented | 24 | 1201 ms | 0 ms | 2857 ms | 2969 ms |
| rtdd | full uninstrumented | 24 | 3127 ms | 3001 ms | 4294 ms | 4486 ms |
| rtdd | subset instrumented | 24 | 12357 ms | 14675 ms | 25102 ms | 26730 ms |
| rtdd | subset uninstrumented | 24 | 1284 ms | 0 ms | 3109 ms | 4045 ms |
| testmon | full uninstrumented | 24 | 3127 ms | 3001 ms | 4294 ms | 4486 ms |
| testmon | subset instrumented | 24 | 1778 ms | 1300 ms | 4419 ms | 6163 ms |
| testmon | subset uninstrumented | 24 | 1103 ms | 959 ms | 2905 ms | 2970 ms |
| xdist | full uninstrumented | 24 | 3127 ms | 3001 ms | 4294 ms | 4486 ms |
| xdist | subset instrumented | 24 | 8489 ms | 8460 ms | 9832 ms | 10751 ms |
| xdist | subset uninstrumented | 24 | 6593 ms | 6655 ms | 7091 ms | 7357 ms |

One row per measurement rather than one cell: the population is bimodal — a cycle whose strategy selected nothing costs almost nothing, a cycle that selected the hub costs nearly a full run — so the mean sits between two modes and describes neither. `p50`, `p90` and `worst` are nearest-rank over the per-cycle samples in `commits.jsonl`, so each is a cycle that really ran. Isolation violations are per strategy and published above, under `## Isolation`.

A strategy that carries `Selection.exec_args` — `xdist` is the only one in the shipped set — runs **both** subset columns with those flags (`pytest -n auto`); per-test coverage contexts survive the parallel instrumented run, so that column is not silently serial either. The `full uninstrumented` column is always the serial full suite, which is what makes the two directly comparable.

## By variant

`probe` seeds every map-based strategy at the child commit and is therefore an **upper bound** on their selection quality; it gets its own table and is never pooled with `natural`.

### `natural`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 23 | n/a (0/0) | n/a (0/0) | 0.235 (2650/11274) | 0.282 (10288/36469) | — |
| importgraph | 23 | n/a (0/0) | n/a (0/0) | 0.041 (460/11274) | 0.047 (1714/36469) | — |
| lf | 23 | n/a (0/0) | n/a (0/0) | 0.783 (8825/11274) | 0.790 (28810/36469) | — |
| path | 23 | n/a (0/0) | n/a (0/0) | 0.050 (568/11274) | 0.056 (2036/36469) | — |
| static | 23 | n/a (0/0) | n/a (0/0) | 0.050 (568/11274) | 0.056 (2036/36469) | — |
| testmon | 23 | n/a (0/0) | n/a (0/0) | 0.158 (1786/11274) | 0.190 (6939/36469) | — |
| xdist | 23 | n/a (0/0) | n/a (0/0) | 1.000 (11274/11274) | 1.000 (36469/36469) | — |
| random | 23 | n/a (0/0) | n/a (0/0) | 0.235 (2650/11274) | 0.258 (9411/36469) | — |
| full | 23 | n/a (0/0) | n/a (0/0) | 1.000 (11274/11274) | 1.000 (36469/36469) | — |

### `probe`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 23 | 1.000 (3/3) | 1.000 (3/3) | 0.220 (2481/11274) | 0.243 (8348/34336) | upper bound (map seeded at the child commit) |
| importgraph | 23 | 0.000 (0/3) | 0.000 (0/3) | 0.000 (0/11274) | 0.000 (0/34336) | — |
| lf | 23 | 1.000 (3/3) | 1.000 (3/3) | 0.783 (8825/11274) | 0.768 (26377/34336) | — |
| path | 23 | 0.333 (1/3) | 0.333 (1/3) | 0.012 (134/11274) | 0.010 (348/34336) | — |
| static | 23 | 0.333 (1/3) | 0.333 (1/3) | 0.012 (134/11274) | 0.010 (348/34336) | — |
| testmon | 23 | 1.000 (3/3) | 1.000 (3/3) | 0.104 (1171/11274) | 0.131 (4481/34336) | upper bound (map seeded at the child commit) |
| xdist | 23 | 1.000 (3/3) | 1.000 (3/3) | 1.000 (11274/11274) | 1.000 (34336/34336) | — |
| random | 23 | 0.667 (2/3) | 0.667 (2/3) | 0.220 (2481/11274) | 0.220 (7539/34336) | — |
| full | 23 | 1.000 (3/3) | 1.000 (3/3) | 1.000 (11274/11274) | 1.000 (34336/34336) | — |

