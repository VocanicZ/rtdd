# Axis 2, corpus-wide: what the pre-registered criterion says once all three repos have run

> **Superseded in part by #184 (2026-09-04).** `sqlfluff` was found to breach the corpus's
> own criterion 4 — ~24 min uninstrumented against a 10-minute budget, the same ground that
> excluded `pandas` — and was removed at `corpus_version: 2`. `bench/results/aggregate.md`
> now reads `repos | 2` and its duration weights moved accordingly. Everything below is the
> record of the run as it was performed under corpus v1, including
> `bench/results/sqlfluff/`, which is kept and stays reproducible with `--corpus-version 1`.
> See [`axis2-corpus-admission.md`](axis2-corpus-admission.md).

`docs/results/axis2-first-replay.md` recorded the outcome for `flask` alone, at the depth
that first run reached, and flagged that deciding the criterion on `natural` needed either
the corpus's own `replay_commits: 200` or a repo whose history lands red more often. This
document is that longer walk (#183): `bench/results/httpie/` and `bench/results/sqlfluff/`
are now committed alongside `flask/`, `bench/results/aggregate.md` reads `repos | 3`, and
every repo has its own `drift.json`. The verdict below is the honest reading at the depths
actually reached — not a re-run with different settings to chase a different number.

**The criterion**, unchanged from `docs/plans/04-m3-replay-benchmark.md`:

> if RTDD does not clearly beat the naive `tests/test_<module>.py` path heuristic on
> change-level recall at equal or better selected-duration fraction, the map is
> unjustified and the honest outcome is to say so in the README rather than ship it.

## Depth reached, per repo, and why

None of the three repos reached the frozen `replay_commits: 200`. Each was stopped by a
different, measured wall — not by choice:

| repo | collected tests | uninstrumented full suite | usable depth | wall |
|---|---|---|---|---|
| flask | 490 | ~3 s | 23 natural cycles (25 commits, 4 skipped) | no history wall hit; this is simply the depth the first replay ran |
| httpie | 1,028 | 124 s | 17 natural cycles (35 commits, 15 skipped) | `pytest.lazy_fixture` dropped at `3524ccf`/`db16bbe`; every tree older than that will not `rtdd seed` |
| sqlfluff | 13,463 | ~1,228–1,450 s (20–24 min) | 2 natural cycles (4 commits, 0 skipped) | wall-clock budget: a `natural` cycle costs a full instrumented ground-truth run plus a re-execution per strategy, on the order of an hour or more per cycle at this suite size |

sqlfluff's uninstrumented full suite (1,228–1,450 s depending on hardware) is itself over
the corpus's own `< 10 minutes uninstrumented` admission criterion by roughly 2–2.4×, which
is why depth there is capped at 2 rather than a choice made to save time — see #184, which
may remove sqlfluff from the corpus on exactly this ground.

## The outcome: `natural` is not decided for any of the three repos

Zero of 23 flask cycles, 0 of 17 httpie cycles, and 0 of 2 sqlfluff cycles had a non-empty
`F_full`. All three real projects are commits pushed green — replaying a commit over its
parent essentially never reproduces a test that the parent already failed. Recall has no
denominator in any of the three tables, and every `summary.md` says
`verdict: not computable (no detecting commits)` rather than inventing one:

| repo | natural cycles | detecting commits |
|---|---|---|
| flask | 23 | 0 |
| httpie | 17 | 0 |
| sqlfluff | 2 | 0 |

**This is itself the finding the issue asked for.** Three real-world repos, at the deepest
walk each one's history and this hardware's wall-clock budget permits, produce no evidence
either way on `natural`. The corpus's own admission criteria select for healthy, green,
well-maintained projects — the same property that makes them worth measuring makes their
recent history a poor source of naturally red commits. Deciding this criterion on `natural`
needs either a repo whose real history lands red more often than these three, or a much
larger `replay_commits` than any of the three could reach here (flask's own history might
support 200; httpie is hard-capped at 25 by the `lazy_fixture` drop regardless of budget;
sqlfluff cannot reach double digits at this suite size on non-CI hardware).

## `probe`, the upper bound, decides for one repo and stays silent for two

`probe` reverts each commit's source half while keeping its tests, seeding the map at the
child commit. It is explicitly an upper bound on map-based strategies and is never pooled
with `natural`, but it is the only population that produced any detecting commits at all
— and only for one of the three repos:

| repo | probe cycles | detecting commits | rtdd change-level recall |
|---|---|---|---|
| flask | 23 | 3 | 1.000 (3/3) |
| httpie | 18 | 0 | n/a (0/0) |
| sqlfluff | 2 | 0 | n/a (0/0) |

flask's probe result is unchanged from the first-replay document: RTDD catches every
detecting change but spends roughly twenty times the path heuristic's selected-duration
fraction doing it, so the criterion — recall **at equal or better duration** — is not met
there either. httpie's and sqlfluff's probe populations produced zero detecting cycles even
under the artificial source-revert, at the depths reached; a population this small (2–18
cycles) genuinely may not contain a commit whose reverted half breaks anything the suite
notices, which is a property of the sample size rather than of RTDD.

## Drift (cycles-since-commit degradation, spec §5)

Each repo now has its own `drift.json`, applying consecutive commits to one uncommitted
worktree and recording RTDD's selection ratio as the accumulated changed set grows:

- **flask** (25 cycles): selection ratio starts at 0.000, rises to ~0.002 by cycle 3, then
  the tree stops collecting (`collect_failed`, ratio `None`) from cycles 8–24 — an
  intermediate uncommitted state this repo's real history never actually holds — and
  recovers to 0.012 at cycle 25.
- **httpie** (16 cycles, capped at `db16bbe` to stay clear of the `lazy_fixture` zone):
  selection ratio is flat at 0.0175 across all 16 cycles — the accumulated changed set does
  not grow into new modules across this window.
- **sqlfluff** (2 cycles): selection ratio is 0.000 at both cycles — with 3 and 10 files
  changed respectively against a 13,463-test suite, nothing in RTDD's map fires yet.

None of the three curves show the degradation-toward-1.0 shape spec §5 predicts for a long
uncommitted session, because none of the three walks is long enough to show it — flask's
25-cycle window is the longest reached and most of it fell in the non-collecting zone.
This is a depth limitation of this replay, not a refutation of the drift hypothesis.

## Wall-clock: two of three repos have it, one states why it doesn't

flask's and httpie's tables carry full wall-clock rows, measured on disclosed hardware off
CI. sqlfluff's do not: the replay that produced its committed results ran with
`--no-wallclock` because the host was a contended daily-driver machine at the time (a
40 GB-heap game, Discord, browsers), and a timing taken there would be inflated and, once
written into the durable cache, silently reused forever after. `bench/results/sqlfluff/summary.md`
states this in its own Wall-clock section rather than omitting the rows silently. Taking
that measurement on a quiet box is tracked in #185, blocked on #184.

Every wall-clock row is published as `mean`, `p50`, `p90` and `worst`, never as a mean
alone (#214). The population is bimodal — a cycle whose strategy selected nothing costs
almost nothing, a cycle that selected the hub costs nearly a full run — so a mean falls
between the two modes and describes neither. The percentiles are nearest-rank over the
per-cycle samples committed in each repo's `commits.jsonl`, and
`uv run python -m replay.cli report --rebuild` re-derives the tables from those samples
without re-running the benchmark.

## What this does not say

Three repos at these depths is not the `replay_commits: 200` the plan asks for, and the
scope item this satisfies is explicit that a criterion still not computable at the reached
depth is itself the recordable outcome — not a reason to retune settings and re-run.
The honest reading is: **on `natural`, the pre-registered criterion remains undecided
across all three corpus repos at the deepest walk each permits on this hardware; on
`probe`, it decides once (flask, against RTDD) and is silent twice (httpie, sqlfluff) for
want of any detecting commit in a small sample.** Nothing here justifies shipping the map,
and nothing here justifies pulling it either — the table this issue was asked to produce
is now complete, and it says "not yet decided" honestly rather than inventing a number.

The tables that produced every number here are in git: `bench/results/flask/summary.md`,
`bench/results/httpie/summary.md`, `bench/results/sqlfluff/summary.md`, and the duration-
weighted cross-repo ordering in `bench/results/aggregate.md`.
