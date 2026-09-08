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
| static | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.416 (7274/17476) | 0.426 (637080/1.49501e+06) | 0.000 (0/17) |
| testmon | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.257 (4495/17476) | 0.239 (358016/1.49501e+06) | 0.000 (0/17) |
| xdist | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (17476/17476) | 1.000 (1.49501e+06/1.49501e+06) | 1.000 (17/17) |
| random | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.186 (3250/17476) | 0.197 (295151/1.49501e+06) | 0.000 (0/17) |
| full | 17 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (17476/17476) | 1.000 (1.49501e+06/1.49501e+06) | 1.000 (17/17) |

## The static arm

Spec §7 asks what the `TS` static tier is worth against the corpus where the coverage-derived answer is already known. `static` is scored here against `rtdd`, the naive `path` baseline and the `full` ceiling, on the three metrics the pre-registration names.

static verdict: not computable (no detecting commits)

| arm | change recall | selection ratio | selected duration | mean subset uninstrumented | p50 | p90 | worst |
|---|---|---|---|---|---|---|---|
| `static` | n/a (0/0) | 0.416 (7274/17476) | 0.426 (637080/1.49501e+06) | not measured | not measured | not measured | not measured |
| `rtdd` | n/a (0/0) | 0.186 (3250/17476) | 0.188 (280956/1.49501e+06) | 25423 ms | 0 ms | 96666 ms | 171039 ms |
| `path` | n/a (0/0) | 0.032 (567/17476) | 0.032 (48226/1.49501e+06) | 443 ms | 0 ms | 0 ms | 7971 ms |
| `full` | n/a (0/0) | 1.000 (17476/17476) | 1.000 (1.49501e+06/1.49501e+06) | 89957 ms | 83994 ms | 106813 ms | 114537 ms |

`static` carry no wall-clock record in this run — a derived arm **executed nothing** at all, and any arm can simply have gone unsampled. Nothing was invented to fill the gap: the cells read `not measured` rather than a blank that would read as zero, a figure synthesised from `durations_ms` (another execution's per-test time), or a row borrowed from an arm that really ran. The cost those arms do publish is the **selected duration** column, a ratio of the same commit's own recorded per-test durations and therefore independent of the machine.

`static` models the `TS` tier (spec §4.1) over this corpus and is **derived** from the records above, never re-run. Level 1 is `test_for` correspondence, resolved against the test files each commit collected, first match wins:

- `{dir}/test_{name}.py`
- `{dir}/tests/test_{name}.py`
- `tests/{subdir}/test_{name}.py`
- `tests/test_{name}.py`

Level 2 is the committed `importgraph` selection — that baseline measures exactly the transitive-import question level 2 asks, and it ran on every replayed commit. Level 3 (path proximity) orders and never admits, so it cannot change the selected set and none of the metrics above depend on it. Two consequences follow: `static ⊇ importgraph` **by construction**, so beating that baseline is arithmetic rather than a finding, which is why the pre-registered comparison is against `path`; and `adapters/python.yaml` declares no `test_for`, so the adapter modelled here **does not ship** — the row answers what the static tier WOULD have selected on this corpus, which is the question §7 pre-registers.

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

| strategy | measurement | n | mean | p50 | p90 | worst |
|---|---|---|---|---|---|---|
| full | full uninstrumented | 18 | 88694 ms | 84531 ms | 102994 ms | 108210 ms |
| full | subset instrumented | 18 | 98283 ms | 94079 ms | 116582 ms | 123262 ms |
| full | subset uninstrumented | 18 | 89957 ms | 83994 ms | 106813 ms | 114537 ms |
| importgraph | full uninstrumented | 18 | 88694 ms | 84531 ms | 102994 ms | 108210 ms |
| importgraph | subset instrumented | 18 | 57059 ms | 10081 ms | 106859 ms | 236818 ms |
| importgraph | subset uninstrumented | 18 | 49603 ms | 7962 ms | 97522 ms | 173053 ms |
| lf | full uninstrumented | 18 | 88694 ms | 84531 ms | 102994 ms | 108210 ms |
| lf | subset instrumented | 18 | 1379 ms | 1312 ms | 1657 ms | 1680 ms |
| lf | subset uninstrumented | 18 | 932 ms | 895 ms | 1042 ms | 1210 ms |
| path | full uninstrumented | 18 | 88694 ms | 84531 ms | 102994 ms | 108210 ms |
| path | subset instrumented | 18 | 550 ms | 0 ms | 0 ms | 9899 ms |
| path | subset uninstrumented | 18 | 443 ms | 0 ms | 0 ms | 7971 ms |
| random | full uninstrumented | 18 | 88694 ms | 84531 ms | 102994 ms | 108210 ms |
| random | subset instrumented | 18 | 23026 ms | 0 ms | 105405 ms | 121104 ms |
| random | subset uninstrumented | 18 | 20880 ms | 0 ms | 97273 ms | 109040 ms |
| rtdd | full uninstrumented | 18 | 88694 ms | 84531 ms | 102994 ms | 108210 ms |
| rtdd | subset instrumented | 18 | 60603 ms | 91177 ms | 106358 ms | 107944 ms |
| rtdd | subset uninstrumented | 18 | 25423 ms | 0 ms | 96666 ms | 171039 ms |
| testmon | full uninstrumented | 18 | 88694 ms | 84531 ms | 102994 ms | 108210 ms |
| testmon | subset instrumented | 18 | 29278 ms | 1454 ms | 94921 ms | 212995 ms |
| testmon | subset uninstrumented | 18 | 26377 ms | 946 ms | 86517 ms | 198967 ms |
| xdist | full uninstrumented | 18 | 88694 ms | 84531 ms | 102994 ms | 108210 ms |
| xdist | subset instrumented | 18 | 26861 ms | 26692 ms | 30545 ms | 33393 ms |
| xdist | subset uninstrumented | 18 | 20964 ms | 19748 ms | 25135 ms | 32619 ms |

One row per measurement rather than one cell: the population is bimodal — a cycle whose strategy selected nothing costs almost nothing, a cycle that selected the hub costs nearly a full run — so the mean sits between two modes and describes neither. `p50`, `p90` and `worst` are nearest-rank over the per-cycle samples in `commits.jsonl`, so each is a cycle that really ran. Isolation violations are per strategy and published above, under `## Isolation`.

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
| static | 17 | n/a (0/0) | n/a (0/0) | 0.416 (7274/17476) | 0.426 (637080/1.49501e+06) | — |
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
| static | 18 | n/a (0/0) | n/a (0/0) | 0.277 (5130/18504) | 0.286 (439640/1.5388e+06) | — |
| testmon | 18 | n/a (0/0) | n/a (0/0) | 0.134 (2472/18504) | 0.113 (173606/1.5388e+06) | upper bound (map seeded at the child commit) |
| xdist | 18 | n/a (0/0) | n/a (0/0) | 1.000 (18504/18504) | 1.000 (1.5388e+06/1.5388e+06) | — |
| random | 18 | n/a (0/0) | n/a (0/0) | 0.145 (2683/18504) | 0.163 (250294/1.5388e+06) | — |
| full | 18 | n/a (0/0) | n/a (0/0) | 1.000 (18504/18504) | 1.000 (1.5388e+06/1.5388e+06) | — |

