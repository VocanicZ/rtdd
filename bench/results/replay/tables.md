Condensed from `bench/results/{flask,httpie,sqlfluff}/summary.md` and
`bench/results/aggregate.md`. Full per-cycle detail, drift curves, and wall-clock rows are in
[`docs/results/axis2-corpus-replay.md`](../../../docs/results/axis2-corpus-replay.md).
Per spec §10, recall is never pooled across repos, and `probe` is never pooled with `natural`.

> **Superseded in part by #184.** `sqlfluff` breaches the corpus's own admission criterion
> (~24 min uninstrumented suite against a 10-minute budget) and was removed at
> `corpus_version: 2`; it is kept here for the record and stays reproducible with
> `--corpus-version 1`.

## Depth reached, and the pre-registered criterion

> if RTDD does not clearly beat the naive `tests/test_<module>.py` path heuristic on
> change-level recall at equal or better selected-duration fraction, the map is
> unjustified and the honest outcome is to say so in the README rather than ship it.
> — `docs/plans/04-m3-replay-benchmark.md`

| repo | natural cycles | detecting commits (natural) | probe cycles | detecting commits (probe) |
|---|---|---|---|---|
| flask | 23 | 0 | 23 | 3 |
| httpie | 17 | 0 | 18 | 0 |
| sqlfluff | 2 | 0 | 2 | 0 |

**`natural` is not decided for any of the three repos.** All three real projects are
committed green; replaying a commit over its parent almost never reproduces a test the
parent already failed. Recall has no denominator in any of the three tables.

**`probe`** (source half of each commit reverted, tests kept, map seeded at the child
commit — an explicit upper bound, never pooled with `natural`) decides once and is silent
twice:

| repo | change recall (rtdd) | change recall (path heuristic) | selected-duration fraction (rtdd) | selected-duration fraction (path) |
|---|---|---|---|---|
| flask | 1.000 (3/3) | 0.333 (1/3) | 0.243 | 0.010 |
| httpie | n/a (0/0) | n/a (0/0) | — | — |
| sqlfluff | n/a (0/0) | n/a (0/0) | — | — |

On flask's probe population RTDD catches every detecting change but spends roughly 24×
the path heuristic's selected-duration fraction doing it. The criterion asks for better
recall **at equal or better duration** — **not met**. `testmon` matches RTDD's recall at
under half the duration on this sample.

**Verdict: the pre-registered criterion is not met, and on `natural` it remains undecided.**
No stratified `|F_full| == 1` table is shown: zero detecting commits occurred at any
`|F_full|` in `natural`, in any repo, at the depth reached.

## Duration-weighted aggregate (never pooled for recall, ordering only)

Source: `bench/results/aggregate.md`.

| strategy | duration-weighted selected fraction | repos |
|---|---|---|
| full | 1.000 | 2 |
| xdist | 1.000 | 2 |
| importgraph | 0.417 | 2 |
| testmon | 0.238 | 2 |
| random | 0.199 | 2 |
| rtdd | 0.190 | 2 |
| path | 0.033 | 2 |
| lf | 0.019 | 2 |
