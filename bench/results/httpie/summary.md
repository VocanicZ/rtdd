# Axis 2 — real-commit replay: `httpie`

**Hardware:** Intel(R) Xeon(R) CPU E5-2697A v4 @ 2.60GHz, 32 cores, 32768 MiB, Linux-7.0.0-3-pve-x86_64-with-glibc2.41, Python 3.12.13 · fingerprint `65b8fad78747d6ef`
**Binary:** sha256:a93bfac3e9762606352301fc7ec1d2dda2902f4d422590130dad27ddf2d40f25 · **corpus digest:** `95cd1505766b` · **config digest:** `57e35207ae50`
**Tools:** coverage 7.15.4, pytest 9.1.1, pytest-cov 7.1.0, pytest-reportlog 1.0.0, pytest-testmon 2.2.0, pytest-xdist 3.8.0
**Replayed commits:** 35 (skipped: 33) · shipped defaults only, no tuning flags

verdict: not computable (no detecting commits)

## Per-strategy — `natural`, all strata pooled within this repo

| strategy | cycles | detecting | change recall | test recall (micro) | test recall (macro) | selection ratio | selected duration | escalation |
|---|---|---|---|---|---|---|---|---|
| rtdd | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.186 (3250/17493) | 0.162 (333384/2.05988e+06) | 0.353 (6/17) |
| importgraph | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.416 (7274/17493) | 0.416 (856982/2.05988e+06) | 0.000 (0/17) |
| lf | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.003 (51/17493) | 0.001 (1113/2.05988e+06) | 0.000 (0/17) |
| path | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.032 (567/17493) | 0.027 (55227/2.05988e+06) | 0.000 (0/17) |
| testmon | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.258 (4512/17493) | 0.216 (443933/2.05988e+06) | 0.000 (0/17) |
| xdist | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (17493/17493) | 1.000 (2.05988e+06/2.05988e+06) | 1.000 (17/17) |
| random | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.186 (3250/17493) | 0.190 (390908/2.05988e+06) | 0.000 (0/17) |
| full | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (17493/17493) | 1.000 (2.05988e+06/2.05988e+06) | 1.000 (17/17) |

## Stratified by |F_full| — the `|F_full| == 1` stratum is where selection safety is genuinely under test

| strategy | stratum | n | change recall | test recall (micro) |
|---|---|---|---|---|

## Uncovered-report false signal

- fired on 5 of 17 cycles (0.294 (5/17))
- change-level false-signal rate: 0.000 (0/5)
- line-level false-signal rate: 0.000 (0/22)
- `rtdd run` refused on 0 of 17 cycles — the shipped binary exits 2 rather than execute a map that names a test the tree no longer collects, and those cycles have no uncovered report

## Isolation

A subset run that does not reproduce `F_full ∩ selected` is a finding, not a timing number, so it is published here whether or not wall-clock was permitted.

- isolation violations: 0 of 87 really-executed subset runs
  - `importgraph`: 0 of 18
  - `lf`: 0 of 18
  - `path`: 0 of 18
  - `rtdd`: 0 of 18
  - `testmon`: 0 of 15

## Wall-clock

| strategy | n | full uninstrumented | subset instrumented | subset uninstrumented | isolation violations |
|---|---|---|---|---|---|
| importgraph | 18 | 123365 ms | 70226 ms | 63712 ms | 0 |
| lf | 18 | 123365 ms | 2634 ms | 1434 ms | 0 |
| path | 18 | 123365 ms | 836 ms | 574 ms | 0 |
| rtdd | 18 | 123365 ms | 84893 ms | 27446 ms | 0 |
| testmon | 15 | 123140 ms | 10107 ms | 7429 ms | 0 |

A strategy that carries `Selection.exec_args` — `xdist` is the only one in the shipped set — runs **both** subset columns with those flags (`pytest -n auto`); per-test coverage contexts survive the parallel instrumented run, so that column is not silently serial either. The `full uninstrumented` column is always the serial full suite, which is what makes the two directly comparable.

## By variant

`probe` seeds every map-based strategy at the child commit and is therefore an **upper bound** on their selection quality; it gets its own table and is never pooled with `natural`.

### `natural`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 17 | n/a (0/0) | n/a (0/0) | 0.186 (3250/17493) | 0.162 (333384/2.05988e+06) | — |
| importgraph | 17 | n/a (0/0) | n/a (0/0) | 0.416 (7274/17493) | 0.416 (856982/2.05988e+06) | — |
| lf | 17 | n/a (0/0) | n/a (0/0) | 0.003 (51/17493) | 0.001 (1113/2.05988e+06) | — |
| path | 17 | n/a (0/0) | n/a (0/0) | 0.032 (567/17493) | 0.027 (55227/2.05988e+06) | — |
| testmon | 17 | n/a (0/0) | n/a (0/0) | 0.258 (4512/17493) | 0.216 (443933/2.05988e+06) | — |
| xdist | 17 | n/a (0/0) | n/a (0/0) | 1.000 (17493/17493) | 1.000 (2.05988e+06/2.05988e+06) | — |
| random | 17 | n/a (0/0) | n/a (0/0) | 0.186 (3250/17493) | 0.190 (390908/2.05988e+06) | — |
| full | 17 | n/a (0/0) | n/a (0/0) | 1.000 (17493/17493) | 1.000 (2.05988e+06/2.05988e+06) | — |

### `probe`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 18 | n/a (0/0) | n/a (0/0) | 0.145 (2683/18522) | 0.127 (273511/2.1591e+06) | upper bound (map seeded at the child commit) |
| importgraph | 18 | n/a (0/0) | n/a (0/0) | 0.277 (5130/18522) | 0.279 (602273/2.1591e+06) | — |
| lf | 18 | n/a (0/0) | n/a (0/0) | 0.004 (71/18522) | 0.001 (1236/2.1591e+06) | — |
| path | 18 | n/a (0/0) | n/a (0/0) | 0.000 (0/18522) | 0.000 (0/2.1591e+06) | — |
| testmon | 18 | n/a (0/0) | n/a (0/0) | 0.134 (2490/18522) | 0.092 (199565/2.1591e+06) | upper bound (map seeded at the child commit) |
| xdist | 18 | n/a (0/0) | n/a (0/0) | 1.000 (18522/18522) | 1.000 (2.1591e+06/2.1591e+06) | — |
| random | 18 | n/a (0/0) | n/a (0/0) | 0.145 (2683/18522) | 0.142 (307652/2.1591e+06) | — |
| full | 18 | n/a (0/0) | n/a (0/0) | 1.000 (18522/18522) | 1.000 (2.1591e+06/2.1591e+06) | — |

