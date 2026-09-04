# Axis 2 — real-commit replay: `httpie`

**Hardware:** Intel(R) Core(TM) i9-10900 CPU @ 2.80GHz, 20 cores, 128632 MiB, Linux-7.0.0-30-generic-x86_64-with-glibc2.39, Python 3.12.3 · fingerprint `c200eeab1f38d1f9`
**Binary:** sha256:14b033b011306c6f3b6ed7cd6f9dded9d828a0213e80e36330e5084ac4478c0d · **corpus digest:** `95cd1505766b` · **config digest:** `b087cceff4e3`
**Tools:** coverage 7.16.0, pytest 9.1.1, pytest-cov 7.1.0, pytest-reportlog 1.0.0, pytest-testmon 2.2.0, pytest-xdist 3.8.0
**Replayed commits:** 35 (skipped: 15) · shipped defaults only, no tuning flags

verdict: not computable (no detecting commits)

## Per-strategy — `natural`, all strata pooled within this repo

| strategy | cycles | detecting | change recall | test recall (micro) | test recall (macro) | selection ratio | selected duration | escalation |
|---|---|---|---|---|---|---|---|---|
| rtdd | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.186 (3250/17476) | 0.188 (280956/1.49501e+06) | 0.353 (6/17) |
| importgraph | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.416 (7274/17476) | 0.426 (637080/1.49501e+06) | 0.000 (0/17) |
| lf | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.002 (34/17476) | 0.000 (721/1.49501e+06) | 0.000 (0/17) |
| path | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.032 (567/17476) | 0.032 (48226/1.49501e+06) | 0.000 (0/17) |
| testmon | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.257 (4495/17476) | 0.239 (358016/1.49501e+06) | 0.000 (0/17) |
| xdist | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (17476/17476) | 1.000 (1.49501e+06/1.49501e+06) | 1.000 (17/17) |
| random | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.186 (3250/17476) | 0.197 (295151/1.49501e+06) | 0.000 (0/17) |
| full | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (17476/17476) | 1.000 (1.49501e+06/1.49501e+06) | 1.000 (17/17) |

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

- isolation violations: 0 of 144 really-executed subset runs
  - `full`: 0 of 18
  - `importgraph`: 0 of 18
  - `lf`: 0 of 18
  - `path`: 0 of 18
  - `random`: 0 of 18
  - `rtdd`: 0 of 18
  - `testmon`: 0 of 18
  - `xdist`: 0 of 18

## Wall-clock

| strategy | n | full uninstrumented | subset instrumented | subset uninstrumented | isolation violations |
|---|---|---|---|---|---|
| full | 18 | 88694 ms | 98283 ms | 89957 ms | 0 |
| importgraph | 18 | 88694 ms | 57059 ms | 49603 ms | 0 |
| lf | 18 | 88694 ms | 1379 ms | 932 ms | 0 |
| path | 18 | 88694 ms | 550 ms | 443 ms | 0 |
| random | 18 | 88694 ms | 23026 ms | 20880 ms | 0 |
| rtdd | 18 | 88694 ms | 60603 ms | 25423 ms | 0 |
| testmon | 18 | 88694 ms | 29278 ms | 26377 ms | 0 |
| xdist | 18 | 88694 ms | 26861 ms | 20964 ms | 0 |

A strategy that carries `Selection.exec_args` — `xdist` is the only one in the shipped set — runs **both** subset columns with those flags (`pytest -n auto`); per-test coverage contexts survive the parallel instrumented run, so that column is not silently serial either. The `full uninstrumented` column is always the serial full suite, which is what makes the two directly comparable.

## By variant

`probe` seeds every map-based strategy at the child commit and is therefore an **upper bound** on their selection quality; it gets its own table and is never pooled with `natural`.

### `natural`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 17 | n/a (0/0) | n/a (0/0) | 0.186 (3250/17476) | 0.188 (280956/1.49501e+06) | — |
| importgraph | 17 | n/a (0/0) | n/a (0/0) | 0.416 (7274/17476) | 0.426 (637080/1.49501e+06) | — |
| lf | 17 | n/a (0/0) | n/a (0/0) | 0.002 (34/17476) | 0.000 (721/1.49501e+06) | — |
| path | 17 | n/a (0/0) | n/a (0/0) | 0.032 (567/17476) | 0.032 (48226/1.49501e+06) | — |
| testmon | 17 | n/a (0/0) | n/a (0/0) | 0.257 (4495/17476) | 0.239 (358016/1.49501e+06) | — |
| xdist | 17 | n/a (0/0) | n/a (0/0) | 1.000 (17476/17476) | 1.000 (1.49501e+06/1.49501e+06) | — |
| random | 17 | n/a (0/0) | n/a (0/0) | 0.186 (3250/17476) | 0.197 (295151/1.49501e+06) | — |
| full | 17 | n/a (0/0) | n/a (0/0) | 1.000 (17476/17476) | 1.000 (1.49501e+06/1.49501e+06) | — |

### `probe`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 18 | n/a (0/0) | n/a (0/0) | 0.145 (2683/18504) | 0.151 (232029/1.5388e+06) | upper bound (map seeded at the child commit) |
| importgraph | 18 | n/a (0/0) | n/a (0/0) | 0.277 (5130/18504) | 0.286 (439640/1.5388e+06) | — |
| lf | 18 | n/a (0/0) | n/a (0/0) | 0.003 (53/18504) | 0.000 (763/1.5388e+06) | — |
| path | 18 | n/a (0/0) | n/a (0/0) | 0.000 (0/18504) | 0.000 (0/1.5388e+06) | — |
| testmon | 18 | n/a (0/0) | n/a (0/0) | 0.134 (2472/18504) | 0.113 (173606/1.5388e+06) | upper bound (map seeded at the child commit) |
| xdist | 18 | n/a (0/0) | n/a (0/0) | 1.000 (18504/18504) | 1.000 (1.5388e+06/1.5388e+06) | — |
| random | 18 | n/a (0/0) | n/a (0/0) | 0.145 (2683/18504) | 0.163 (250294/1.5388e+06) | — |
| full | 18 | n/a (0/0) | n/a (0/0) | 1.000 (18504/18504) | 1.000 (1.5388e+06/1.5388e+06) | — |

