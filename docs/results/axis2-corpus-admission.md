# The corpus enforced none of its criteria, and one admitted repo breached the one that excluded another

*Issue #184. Decided 2026-09-04, after #183 closed and with its `sqlfluff` result in hand.*

## What was wrong

`bench/corpus.yaml` v1 stated seven admission criteria and enforced none of them. They
were prose in a YAML header; no code read them. Criterion 4 reads:

> Full suite completes in under 10 minutes uninstrumented on the disclosed hardware.

and the same file excludes `pandas` on exactly that ground. `sqlfluff` was admitted
anyway. Measured uninstrumented on the disclosed hardware (fingerprint
`65b8fad78747d6ef`):

| repo | collected tests | full suite, uninstrumented | vs. the 10-minute budget |
|---|---|---|---|
| flask | 490 | ~3 s | 0.005x |
| httpie | 1,028 | 124 s | 0.21x |
| **sqlfluff** | **13,463** | **~1,450 s (24 min)** | **2.4x over** |

The cost was not incidental. A single `sqlfluff` replay at `--replay-commits 2` ran over
9 hours of wall-clock, against a declared `replay_commits: 200`, because each cycle runs
8 strategies x 2 variants and `full`, `xdist` and `random` each select the whole 13k-test
suite.

## The decision: `sqlfluff` is dropped

It moves to `excluded:` at `corpus_version: 2`, with its measured numbers as the reason.

The exclusion rests on a **pre-registered** criterion, not on how the replay came out.
That distinction is the whole defence against cherry-picking, so it is worth stating
plainly what the replay did produce, since the decision is being taken with it in hand:

> `verdict: not computable (no detecting commits)`

Every recall column in `results/sqlfluff/summary.md` reads `n/a (0/0)`. At the depth the
hardware allowed — 2 commit pairs — the population contained nothing to detect, so the
repo contributed no verdict-bearing row to the experiment at any price. Reaching a
population that could decide anything means going deeper at roughly 4.5 h per commit
pair. The criterion said no before that was known; the result says the cost bought
nothing. Both point the same way, and only the first is doing the work.

**The result is not withdrawn.** `bench/results/sqlfluff/` stays exactly as published,
byte for byte, reproducible against corpus v1 with `--corpus-version 1`. Deleting a
published number is worse than superseding it.

### What dropping it did to the aggregate

`results/aggregate.md` is duration-weighted, and `sqlfluff`'s 3.0M-unit suite dominated
the weights. Removing it moves every row, and the honest reading is that the previous
figures were mostly a statement about one over-budget repo:

| strategy | v1 (3 repos) | v2 (2 repos) |
|---|---|---|
| rtdd | 0.064 | 0.190 |
| testmon | 0.096 | 0.238 |
| importgraph | 0.141 | 0.417 |
| random | 0.067 | 0.199 |
| path | 0.011 | 0.033 |
| lf | 0.007 | 0.019 |
| full / xdist | 1.000 | 1.000 |

Per spec §10 recall is never pooled across repos; the aggregate exists only to give a
single ordering, and the per-repo tables remain the result. The ordering is unchanged.

## What now enforces this

`replay.cli audit` checks every admitted repo against the machine-readable half of the
criteria — the `admission:` block — using each repo's recorded `measured:` values. It
fails; it does not warn. An unmeasured repo is a finding, because unverifiable is not
the same as compliant, which is precisely the conflation v1 shipped.

```console
$ uv run python -m replay.cli audit
corpus version 2 · digest 6145ea8f568d · repos flask, httpie
clean — every admitted repo meets every criterion a threshold can decide

$ uv run python -m replay.cli audit --corpus-version 1
corpus version 1 · digest 95cd1505766b · repos flask, httpie, sqlfluff
BREACH (corpus): admission — no machine-readable `admission:` thresholds; no criterion is decidable
1 breach(es) — the corpus does not meet its own criteria
```

That second output is the defect stated in its own terms: v1 cannot be audited, because
it recorded no thresholds and no measurements. The `sqlfluff` breach itself is kept as an
executable record rather than only a paragraph —
`tests/test_audit.py::test_the_sqlfluff_breach_v1_never_caught` reconstructs the repo at
its measured v1 numbers and asserts the validator fails it, so the breach can fail a
build and not merely be read.

`doctor` reports the audit too, and refuses to say `publishable: yes` while a breach
stands.

## `replay_commits` now means something measurable

v1 declared `replay_commits: 200` for all three repos. No repo could reach it:

| repo | declared (v1) | measured ceiling | what stops it |
|---|---|---|---|
| flask | 200 | 25 | deepest walk published |
| httpie | 200 | 17 | trees older than `3524ccf` will not seed — the suite needs `pytest.lazy_fixture`, dropped in its pinned pytest |
| sqlfluff | 200 | 2 | ~4.5 h per commit pair |

v2 sets each repo's `replay_commits` to its measured ceiling and records the ceiling and
its cause in the repo's `measured:` block. `audit` reports a repo that declares more
depth than was measured as reachable, so the number cannot silently drift back into
aspiration.

## Follow-up

The `sqlfluff` wall-clock columns were published as `n/a` by #183 — measured on a
contended workstation, so the timings were withheld rather than published wrong, with a
follow-up to re-take them on a quiet box. That follow-up is now moot: the repo is out of
the corpus, and its published result stands as it is.
