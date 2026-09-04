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

## Wall-clock, uninstrumented: the distribution, never a bare mean

Source: the `## Wall-clock` tables in `bench/results/{flask,httpie}/summary.md`, re-derived
from the per-cycle samples in each repo's `commits.jsonl` with
`uv run python -m replay.cli report --rebuild`.

The population is bimodal — a cycle whose strategy selected nothing costs almost nothing, a
cycle that selected the hub costs nearly a full run — so a mean falls between the two modes
and describes neither half. `p50`, `p90` and `worst` are nearest-rank, so each is a cycle
that really ran.

| repo | strategy | n | mean | p50 | p90 | worst |
|---|---|---|---|---|---|---|
| flask | rtdd | 24 | 1284 ms | 0 ms | 3109 ms | 4045 ms |
| flask | testmon | 24 | 1103 ms | 959 ms | 2905 ms | 2970 ms |
| flask | path | 24 | 255 ms | 0 ms | 1064 ms | 1680 ms |
| flask | full | 24 | 3133 ms | 3081 ms | 3558 ms | 4761 ms |
| httpie | rtdd | 18 | 25423 ms | 0 ms | 96666 ms | 171039 ms |
| httpie | testmon | 18 | 26377 ms | 946 ms | 86517 ms | 198967 ms |
| httpie | path | 18 | 443 ms | 0 ms | 0 ms | 7971 ms |
| httpie | full | 18 | 89957 ms | 83994 ms | 106813 ms | 114537 ms |

`sqlfluff` withheld its wall-clock rows (`--no-wallclock`, contended host) rather than
publish inflated timings, and was dropped at `corpus_version: 2`; its published files are
left as they were.

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
