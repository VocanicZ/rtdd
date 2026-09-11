# Axis 2 — real-commit replay: `flask`

**Hardware:** Intel(R) Xeon(R) CPU E5-2697A v4 @ 2.60GHz, 32 cores, 32768 MiB, Linux-7.0.0-3-pve-x86_64-with-glibc2.41, Python 3.12.13 · fingerprint `65b8fad78747d6ef`
**Binary:** rtdd dev (commit none, built unknown) · **corpus digest:** `6145ea8f568d` · **config digest:** `561862e4e44d`
**Tools:** coverage 7.16.0, pytest 8.4.2, pytest-cov 7.1.0, pytest-reportlog 1.0.0, pytest-testmon 2.2.0, pytest-xdist 3.8.0
**Replayed commits:** 6 (skipped: 0) · shipped defaults only, no tuning flags

verdict: change-level recall rtdd=1.000 vs path heuristic=0.400; selected-duration fraction rtdd=0.757 vs path=0.027 — rtdd does NOT clearly beat the naive path heuristic — per the pre-registered criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this must be stated in the README

## Per-strategy — `probe`, all strata pooled within this repo

| strategy | cycles | detecting | change recall | test recall (micro) | test recall (macro) | selection ratio | selected duration | escalation |
|---|---|---|---|---|---|---|---|---|
| rtdd | 6 | 5 | 1.000 (5/5) | 0.900 (434/482) | 0.980 (4.89958/5) | 0.687 (2029/2955) | 0.757 (6413/8468) | 0.167 (1/6) |
| importgraph | 6 | 5 | 0.000 (0/5) | 0.000 (0/482) | 0.000 (0/5) | 0.000 (0/2955) | 0.000 (0/8468) | 0.000 (0/6) |
| lf | 6 | 5 | 0.600 (3/5) | 0.006 (3/482) | 0.600 (3/5) | 0.667 (1972/2955) | 0.699 (5920/8468) | 0.667 (4/6) |
| path | 6 | 5 | 0.400 (2/5) | 0.075 (36/482) | 0.215 (1.07322/5) | 0.031 (93/2955) | 0.027 (228/8468) | 0.000 (0/6) |
| testmon | 6 | 5 | 1.000 (5/5) | 0.900 (434/482) | 0.980 (4.89958/5) | 0.551 (1627/2955) | 0.603 (5106/8468) | 0.000 (0/6) |
| xdist | 6 | 5 | 1.000 (5/5) | 1.000 (482/482) | 1.000 (5/5) | 1.000 (2955/2955) | 1.000 (8468/8468) | 1.000 (6/6) |
| random | 6 | 5 | 0.800 (4/5) | 0.896 (432/482) | 0.779 (3.89749/5) | 0.687 (2029/2955) | 0.690 (5845/8468) | 0.000 (0/6) |
| full | 6 | 5 | 1.000 (5/5) | 1.000 (482/482) | 1.000 (5/5) | 1.000 (2955/2955) | 1.000 (8468/8468) | 1.000 (6/6) |

## Stratified by |F_full| — the `|F_full| == 1` stratum is where selection safety is genuinely under test

| strategy | stratum | n | change recall | test recall (micro) |
|---|---|---|---|---|
| rtdd | 1 | 4 | 1.000 (4/4) | 1.000 (4/4) |
| rtdd | 21+ | 1 | 1.000 (1/1) | 0.900 (430/478) |
| importgraph | 1 | 4 | 0.000 (0/4) | 0.000 (0/4) |
| importgraph | 21+ | 1 | 0.000 (0/1) | 0.000 (0/478) |
| lf | 1 | 4 | 0.750 (3/4) | 0.750 (3/4) |
| lf | 21+ | 1 | 0.000 (0/1) | 0.000 (0/478) |
| path | 1 | 4 | 0.250 (1/4) | 0.250 (1/4) |
| path | 21+ | 1 | 1.000 (1/1) | 0.073 (35/478) |
| testmon | 1 | 4 | 1.000 (4/4) | 1.000 (4/4) |
| testmon | 21+ | 1 | 1.000 (1/1) | 0.900 (430/478) |
| xdist | 1 | 4 | 1.000 (4/4) | 1.000 (4/4) |
| xdist | 21+ | 1 | 1.000 (1/1) | 1.000 (478/478) |
| random | 1 | 4 | 0.750 (3/4) | 0.750 (3/4) |
| random | 21+ | 1 | 1.000 (1/1) | 0.897 (429/478) |
| full | 1 | 4 | 1.000 (4/4) | 1.000 (4/4) |
| full | 21+ | 1 | 1.000 (1/1) | 1.000 (478/478) |

## Uncovered-report false signal

- fired on 4 of 6 cycles (0.667 (4/6))
- change-level false-signal rate: 0.000 (0/4)
- line-level false-signal rate: 0.000 (0/1034)
- `rtdd run` refused on 0 of 6 cycles — the shipped binary exits 2 rather than execute a map that names a test the tree no longer collects, and those cycles have no uncovered report

## Isolation

A subset run that does not reproduce `F_full ∩ selected` is a finding, not a timing number, so it is published here whether or not wall-clock was permitted.

- isolation violations: 0 of 0 really-executed subset runs

## Wall-clock

Suppressed: withheld by operator (--no-wallclock) — measured on a contended workstation, timings would be inflated and, once cached, reused forever; re-run on a quiet box to publish them.

## By variant

`probe` seeds every map-based strategy at the child commit and is therefore an **upper bound** on their selection quality; it gets its own table and is never pooled with `natural`.

### `probe`

| strategy | cycles | change recall | test recall (micro) | selection ratio | selected duration | bound |
|---|---|---|---|---|---|---|
| rtdd | 6 | 1.000 (5/5) | 0.900 (434/482) | 0.687 (2029/2955) | 0.757 (6413/8468) | upper bound (map seeded at the child commit) |
| importgraph | 6 | 0.000 (0/5) | 0.000 (0/482) | 0.000 (0/2955) | 0.000 (0/8468) | — |
| lf | 6 | 0.600 (3/5) | 0.006 (3/482) | 0.667 (1972/2955) | 0.699 (5920/8468) | — |
| path | 6 | 0.400 (2/5) | 0.075 (36/482) | 0.031 (93/2955) | 0.027 (228/8468) | — |
| testmon | 6 | 1.000 (5/5) | 0.900 (434/482) | 0.551 (1627/2955) | 0.603 (5106/8468) | upper bound (map seeded at the child commit) |
| xdist | 6 | 1.000 (5/5) | 1.000 (482/482) | 1.000 (2955/2955) | 1.000 (8468/8468) | — |
| random | 6 | 0.800 (4/5) | 0.896 (432/482) | 0.687 (2029/2955) | 0.690 (5845/8468) | — |
| full | 6 | 1.000 (5/5) | 1.000 (482/482) | 1.000 (2955/2955) | 1.000 (8468/8468) | — |

