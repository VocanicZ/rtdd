# Axis 2 — real-commit replay: `sqlfluff`

**Hardware:** Intel(R) Core(TM) i9-10900 CPU @ 2.80GHz, 20 cores, 128632 MiB, Linux-7.0.0-30-generic-x86_64-with-glibc2.39, Python 3.12.3 · fingerprint `bb7b40167f9a4ef7`
**Binary:** sha256:14b033b011306c6f3b6ed7cd6f9dded9d828a0213e80e36330e5084ac4478c0d · **corpus digest:** `95cd1505766b` · **config digest:** `1216aaead852`
**Tools:** coverage 7.16.0, pytest 9.1.1, pytest-cov 7.1.0, pytest-reportlog 1.0.0, pytest-testmon 2.2.0, pytest-xdist 3.8.0
**Replayed commits:** 4 (skipped: 0) · shipped defaults only, no tuning flags

verdict: not computable (no detecting commits)

## Per-strategy — `natural`, all strata pooled within this repo

| strategy | cycles | detecting | change recall | test recall (micro) | test recall (macro) | selection ratio | selected duration | escalation |
|---|---|---|---|---|---|---|---|---|
| rtdd | 2 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/3.00242e+06) | 0.000 (0/2) |
| importgraph | 2 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/3.00242e+06) | 0.000 (0/2) |
| lf | 2 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.000 (6/26926) | 0.000 (55/3.00242e+06) | 0.000 (0/2) |
| path | 2 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/3.00242e+06) | 0.000 (0/2) |
| testmon | 2 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.036 (969/26926) | 0.024 (72129/3.00242e+06) | 0.000 (0/2) |
| xdist | 2 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (26926/26926) | 1.000 (3.00242e+06/3.00242e+06) | 1.000 (2/2) |
| random | 2 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/3.00242e+06) | 0.000 (0/2) |
| full | 2 | 0 | n/a (0/0) | n/a (0/0) | n/a (0/0) | 1.000 (26926/26926) | 1.000 (3.00242e+06/3.00242e+06) | 1.000 (2/2) |

## Stratified by |F_full| — the `|F_full| == 1` stratum is where selection safety is genuinely under test

| strategy | stratum | n | change recall | test recall (micro) |
|---|---|---|---|---|

## Uncovered-report false signal

- fired on 0 of 2 cycles (0.000 (0/2))
- change-level false-signal rate: n/a (0/0)
- line-level false-signal rate: n/a (0/0)
- `rtdd run` refused on 0 of 2 cycles — the shipped binary exits 2 rather than execute a map that names a test the tree no longer collects, and those cycles have no uncovered report

## Isolation

A subset run that does not reproduce `F_full ∩ selected` is a finding, not a timing number, so it is published here whether or not wall-clock was permitted.

- isolation violations: 0 of 0 really-executed subset runs

## Wall-clock

Suppressed: withheld by operator (--no-wallclock) — measured on a contended workstation, timings would be inflated and, once cached, reused forever; re-run on a quiet box to publish them.

## By variant

`probe` seeds every map-based strategy at the child commit and is therefore an **upper bound** on their selection quality; it gets its own table and is never pooled with `natural`.

### `natural`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 2 | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/3.00242e+06) | — |
| importgraph | 2 | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/3.00242e+06) | — |
| lf | 2 | n/a (0/0) | n/a (0/0) | 0.000 (6/26926) | 0.000 (55/3.00242e+06) | — |
| path | 2 | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/3.00242e+06) | — |
| testmon | 2 | n/a (0/0) | n/a (0/0) | 0.036 (969/26926) | 0.024 (72129/3.00242e+06) | — |
| xdist | 2 | n/a (0/0) | n/a (0/0) | 1.000 (26926/26926) | 1.000 (3.00242e+06/3.00242e+06) | — |
| random | 2 | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/3.00242e+06) | — |
| full | 2 | n/a (0/0) | n/a (0/0) | 1.000 (26926/26926) | 1.000 (3.00242e+06/3.00242e+06) | — |

### `probe`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 2 | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/2.54727e+06) | upper bound (map seeded at the child commit) |
| importgraph | 2 | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/2.54727e+06) | — |
| lf | 2 | n/a (0/0) | n/a (0/0) | 0.000 (6/26926) | 0.000 (55/2.54727e+06) | — |
| path | 2 | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/2.54727e+06) | — |
| testmon | 2 | n/a (0/0) | n/a (0/0) | 0.000 (6/26926) | 0.000 (55/2.54727e+06) | upper bound (map seeded at the child commit) |
| xdist | 2 | n/a (0/0) | n/a (0/0) | 1.000 (26926/26926) | 1.000 (2.54727e+06/2.54727e+06) | — |
| random | 2 | n/a (0/0) | n/a (0/0) | 0.000 (0/26926) | 0.000 (0/2.54727e+06) | — |
| full | 2 | n/a (0/0) | n/a (0/0) | 1.000 (26926/26926) | 1.000 (2.54727e+06/2.54727e+06) | — |

