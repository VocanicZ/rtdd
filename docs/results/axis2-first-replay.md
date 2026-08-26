# Axis 2, first published replay: what the pre-registered criterion says

The first real-commit replay is committed under `bench/results/flask/`. This document
states its outcome, because the criterion it is measured against was pre-registered
before any number existed and the plan requires the verdict to be recorded rather than
re-run until it improves.

**The criterion**, from `docs/plans/04-m3-replay-benchmark.md`:

> if RTDD does not clearly beat the naive `tests/test_<module>.py` path heuristic on
> change-level recall at equal or better selected-duration fraction, the map is
> unjustified and the honest outcome is to say so in the README rather than ship it.

## What was run

25 consecutive commits of `flask` at the frozen pin `d318b683`, both variants
(`natural` and `probe`), all eight strategies, on disclosed hardware off CI. The full
config — corpus digest, tool versions, the identity of the binary under test, the
commit count — is in `bench/results/flask/config.json` and reprinted in `summary.md`.
Four commit/variant pairs were skipped because their tree would not collect; they are
listed in `summary.json` under `skipped`.

## The outcome, both ways it falls

**On `natural`, the criterion is not yet decided.** Zero of the 23 replayed natural
cycles had a non-empty `F_full`: flask's commits are pushed green, so replaying a
commit over its parent almost never produces a failing test. Recall has no
denominator, and `summary.md` says `verdict: not computable (no detecting commits)`
rather than inventing one. Deciding the criterion on `natural` needs a longer walk —
the corpus's own `replay_commits: 200` — or a repo whose history lands red more often.

**On `probe`, the criterion fires.** The probe population — the same commits' tests
kept and their source half reverted — produced three detecting cycles, and there:

| strategy | change-level recall | selected-duration fraction |
|---|---|---|
| rtdd | 1.000 (3/3) | 0.253 |
| path heuristic | 0.333 (1/3) | 0.013 |
| testmon | 1.000 (3/3) | 0.117 |
| import graph | 0.000 (0/3) | 0.000 |

RTDD catches every detecting change and the path heuristic catches one of three — but
it spends roughly twenty times the test-duration doing it, and the criterion asks for
better recall **at equal or better selected-duration fraction**. It is not met. The
published line says so verbatim, and `probe` is an upper bound for RTDD anyway: its
map is seeded at the child commit.

Two further numbers belong beside that, from the same run:

- **testmon dominates RTDD on this sample** — the same recall at less than half the
  selected duration. Three detecting cycles cannot settle anything, but it is the
  comparison to beat, and it is not the path heuristic.
- **RTDD escalated on 7 of 23 natural cycles** (`T2`, `full-escalate file changed:
  pyproject.toml`), which is most of the gap in selected duration.

## What this does not say

Three detecting cycles is a sample, not a result, and `probe` is explicitly an upper
bound rather than the population the criterion names. The honest reading is: *the
first real table does not justify the map, and the `natural` population large enough
to decide it has not been run yet.* The next run to make is the corpus's full
`replay_commits: 200` across all three repos; nothing about the config needs to change
to get it, and changing the config would produce a new digest and a new set of results
rather than a correction to this one.

The table that produced every number here is in git: `bench/results/flask/summary.md`.
