# The measurements: RTDD against the other selectors

> **This is not the RTDD-vs-full-suite comparison.** That one is in the
> [README](../../README.md#results): it asks whether an agent running RTDD spends less
> time than an agent running the whole suite, and whether it still catches what the whole
> suite catches.
>
> This page asks the *pre-registered* question instead — **against the cheap baselines a
> reviewer will name, is building a coverage map justified at all?** It is deliberately
> unflattering and RTDD does not currently win it. It lives here, in full and unedited,
> because deleting a pre-registered result after seeing it go against you would make
> every other number this project publishes worth less.

Everything above compares RTDD to running the whole suite, because that is the baseline
RTDD exists to replace. This section is the other question, and it is deliberately
unflattering: **against the cheap baselines a reviewer will name, is building a coverage
map justified at all?** It is pre-registered, it is not the project's own claim, and RTDD
does not currently win it. Both results are published because dropping the second one
would make the first one worth less.

### Axis 1 — agent regression rate on SWE-bench Verified

**Pending.** The pre-registered SWE-bench Verified run (git tag `prereg-m4`, 100-instance
sample, five arms — vanilla, TDD prose, TDAD static graph, RTDD dynamic coverage, RTDD +
prose — one harness, one model) has not been executed yet. `bench/results/swebench/` holds
only the run's pre-committed `config.json`; `tables.md` does not exist. No number is printed
here in its place. This section will be filled, unedited, from
[`bench/swebench/report.py`](../../bench/swebench/report.py)'s output once the run completes —
see [`bench/PREREGISTRATION.md`](../../bench/PREREGISTRATION.md) for the frozen sample, arms,
model, equivalence band, and kill criterion.

A regression is defined mechanically, in
[`bench/swebench/metrics.py`](../../bench/swebench/metrics.py): a test id in the SWE-bench
evaluation report's `tests_status.PASS_TO_PASS.failure` list — a test that passed before the
patch and fails after it. The denominator comes from the dataset rather than the report, and
instances whose patch is empty or fails to apply stay in both denominators, so no arm can
lower its regression rate by producing nothing. Resolution rate will be printed beside the
regression rate in every row for the same reason.

The RTDD arm's integration is a `rtdd_which` tool and a declarative description of what it
returns. Its prompt is byte-for-byte the vanilla arm's prompt plus one `<test-context>`
block — [asserted in a test](../../bench/swebench/tests/test_prompts.py), along with an
imperative-phrase lint on the block itself — because TDAD measured procedural prose as
actively harmful and an arm that smuggled it in would be uninterpretable.

### Axis 2 — selection quality on real commits

Replay of real commits per repo (`flask`, `httpie`, `sqlfluff`) against every baseline a
reviewer will ask for: pytest-testmon, a naive `tests/test_<module>.py` path heuristic,
`pytest --lf`, a static import graph, `pytest -n auto`, and random selection at equal
selection ratio. Never pooled across repos.

Condensed from `bench/results/{flask,httpie,sqlfluff}/summary.md` and
`bench/results/aggregate.md`. Full per-cycle detail and wall-clock rows are in
[`docs/results/axis2-corpus-replay.md`](axis2-corpus-replay.md).

Everything in this section replays **committed** commits, one per fresh checkout with the
map seeded — so it is unaffected by the tier-rule fix, which only changes what happens
once a full run has already covered a config change within one session. The `drift.json`
curves that page also carries are the uncommitted-session measurement, they were taken
under the previous rule, and they are not summarised here or on the time axis above.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="figures/axis2-savings-dark.svg">
  <img alt="How much of the suite each strategy runs, per repo. On flask rtdd selects 0.235 of the tests and 0.282 of the suite's test time; on httpie 0.186 and 0.188. The naive path heuristic runs 0.050 and 0.056 on flask, 0.032 and 0.032 on httpie. lf runs 0.783 of flask and 0.002 of httpie. full and xdist run all of it." src="figures/axis2-savings-light.svg">
</picture>

Cheapest is not best: `lf` runs 0.2% of httpie and `importgraph` 4% of flask, and nothing
in this figure says what either would have caught. That is the safety figure below.

> **Superseded in part by #184.** `sqlfluff` breaches the corpus's own admission criterion
> (~24 min uninstrumented suite against a 10-minute budget) and was removed at
> `corpus_version: 2`; it is kept here for the record and stays reproducible with
> `--corpus-version 1`.

**The pre-registered criterion**, from `docs/plans/04-m3-replay-benchmark.md`:

> if RTDD does not clearly beat the naive `tests/test_<module>.py` path heuristic on
> change-level recall at equal or better selected-duration fraction, the map is
> unjustified and the honest outcome is to say so in the README rather than ship it.

| repo | natural cycles | detecting commits (natural) | probe cycles | detecting commits (probe) |
|---|---|---|---|---|
| flask | 23 | 0 | 23 | 3 |
| httpie | 17 | 0 | 18 | 0 |
| sqlfluff | 2 | 0 | 2 | 0 |

**`natural` is not decided for any of the three repos.** All three real projects are
committed green; replaying a commit over its parent almost never reproduces a test the
parent already failed. Recall has no denominator in any of the three tables — the same
property that makes these repos worth measuring makes their recent history a poor source of
naturally red commits.

**`probe`** (source half of each commit reverted, tests kept, map seeded at the child
commit — an explicit upper bound, never pooled with `natural`) decides once and is silent
twice:

| repo | change recall (rtdd) | change recall (path heuristic) | selected-duration fraction (rtdd) | selected-duration fraction (path) |
|---|---|---|---|---|
| flask | 1.000 (3/3) | 0.333 (1/3) | 0.243 | 0.010 |
| httpie | n/a (0/0) | n/a (0/0) | — | — |
| sqlfluff | n/a (0/0) | n/a (0/0) | — | — |

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="figures/axis2-safety-dark.svg">
  <img alt="Change-level recall against selected-duration fraction on flask's probe population, an upper bound. rtdd catches 3 of 3 at 0.243 of the suite's time; testmon catches 3 of 3 at 0.131; lf 3 of 3 at 0.768; full and xdist 3 of 3 at 1.000; random 2 of 3 at 0.220; path and static 1 of 3 at 0.010; importgraph 0 of 3 at 0.000. The pre-registered criterion is not met." src="figures/axis2-safety-light.svg">
</picture>

The trade RTDD exists to make is the vertical axis bought with the horizontal one: keep what
a full run catches, on a fraction of a full run's time. Nine points, one repository and an
upper-bound population decide everything visible here — the picture is the shape of the
question, not an answer to it.

On flask's probe population RTDD catches every detecting change but spends roughly 24× the
path heuristic's selected-duration fraction doing it. The criterion asks for better recall
**at equal or better duration** — **not met**. `testmon` matches RTDD's recall at under half
the duration on this sample.

**Verdict: the pre-registered criterion is not met on `probe`, and remains undecided on
`natural`.** No stratified `|F_full| == 1` table is shown: zero detecting commits occurred at
any `|F_full|` in `natural`, in any repo, at the depth reached.

#### The static tier against the same baseline — the pre-registered kill condition

The map is not the only thing that was pre-registered against `path`. So was the **static
tier** — the tier every non-Python repository gets — in
[`docs/specs/2026-09-05-multi-language.md`](../../docs/specs/2026-09-05-multi-language.md) §7:

> If the static tier does not beat the `path` baseline it is not worth shipping as a
> distinct tier, and the README says so.

"Beat", on the metrics that section publishes: higher change-level recall at a comparable or
better selected-duration fraction. A tie is not a beat, and recall bought by selecting more
of the suite is not a beat either — an arm that buys recall with time is on its way to being
`full`.

The `static` arm executes nothing: it is a function of each replayed commit's own committed
records — the changed set, the tests that commit collected, and its `importgraph` selection —
so it was derived offline and **no benchmark was re-run** to score it. Read from
[`bench/results/flask/summary.json`](../../bench/results/flask/summary.json) and
[`bench/results/httpie/summary.json`](../../bench/results/httpie/summary.json):

| repo | population | change recall (static) | change recall (path heuristic) | selected duration (static) | selected duration (path) |
|---|---|---|---|---|---|
| flask | natural | n/a (0/0) | n/a (0/0) | 0.056 | 0.056 |
| flask | probe (upper bound) | 0.333 (1/3) | 0.333 (1/3) | 0.010 | 0.010 |
| httpie | natural | n/a (0/0) | n/a (0/0) | 0.426 | 0.032 |
| httpie | probe (upper bound) | n/a (0/0) | n/a (0/0) | 0.286 | 0.000 |

**The kill condition fired.** On the one population that has any ground truth at all —
flask's `probe`, three detecting commits — `static` scores 0.333 change-level recall at a
0.010 selected-duration fraction and the path heuristic scores 0.333 at 0.010: the same
commits caught, at the same cost. A tie is not a beat, and the criterion asks for a beat.
Nowhere else is the comparison computable — neither repo's `natural` population and neither
httpie population contains a single detecting commit, so recall has no denominator — and
where only cost can be read, httpie's `natural` `static` arm spends 0.426 of the suite's
duration against the path heuristic's 0.032, roughly 13× more of the suite for recall nobody
could measure.

**So the static tier does not carry its weight as a distinct tier on this evidence.** It
selects what a naive `tests/test_<module>.py` heuristic already selects, and where it selects
more it has not been shown to catch more. It ships anyway because a repository RTDD cannot
instrument is otherwise offered nothing at all — not because it is measured to be better —
and two repositories would not be a general result in either direction.

#### Wall-clock: the distribution, never a bare mean

Wall-clock is from disclosed hardware in each repo's own `config.json`, never from a CI
runner; `sqlfluff`'s wall-clock rows are withheld (`--no-wallclock`, contended host) rather
than published inflated.

What one cycle costs to *execute* the selection, uninstrumented. The population is bimodal —
a cycle whose strategy selected nothing costs almost nothing, a cycle that selected the hub
costs nearly a full run — so the mean falls between the two modes and describes neither half.
`p50`, `p90` and `worst` are nearest-rank over the committed per-cycle samples in
`bench/results/<repo>/commits.jsonl`, so every figure below is a cycle that really ran.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="figures/axis2-wallclock-dark.svg">
  <img alt="Median, p90 and worst cycle cost per strategy against the full uninstrumented suite. On flask rtdd's median cycle costs 0 ms, p90 3109 ms and worst 4045 ms against a 3127 ms full suite. On httpie rtdd's median is 0 ms, p90 96666 ms and worst 171039 ms against an 88694 ms full suite — the worst cycle costs nearly twice a full run." src="figures/axis2-wallclock-light.svg">
</picture>

The distance between the bar and the dot is the finding. RTDD's median cycle is free on both
repos; its worst crosses the dashed full-suite line on httpie and comes close on flask.

**flask** — full suite, uninstrumented: mean 3127 ms · p50 3001 ms · p90 4294 ms · worst 4486 ms.

| strategy | n | mean | p50 | p90 | worst |
|---|---|---|---|---|---|
| rtdd | 24 | 1284 ms | 0 ms | 3109 ms | 4045 ms |
| testmon | 24 | 1103 ms | 959 ms | 2905 ms | 2970 ms |
| path | 24 | 255 ms | 0 ms | 1064 ms | 1680 ms |
| lf | 24 | 2496 ms | 2790 ms | 3523 ms | 4994 ms |
| importgraph | 24 | 171 ms | 0 ms | 1063 ms | 1150 ms |
| xdist | 24 | 6593 ms | 6655 ms | 7091 ms | 7357 ms |
| random | 24 | 1201 ms | 0 ms | 2857 ms | 2969 ms |
| full | 24 | 3133 ms | 3081 ms | 3558 ms | 4761 ms |

**httpie** — full suite, uninstrumented: mean 88694 ms · p50 84531 ms · p90 102994 ms · worst 108210 ms.

| strategy | n | mean | p50 | p90 | worst |
|---|---|---|---|---|---|
| rtdd | 18 | 25423 ms | 0 ms | 96666 ms | 171039 ms |
| testmon | 18 | 26377 ms | 946 ms | 86517 ms | 198967 ms |
| path | 18 | 443 ms | 0 ms | 0 ms | 7971 ms |
| lf | 18 | 932 ms | 895 ms | 1042 ms | 1210 ms |
| importgraph | 18 | 49603 ms | 7962 ms | 97522 ms | 173053 ms |
| xdist | 18 | 20964 ms | 19748 ms | 25135 ms | 32619 ms |
| random | 18 | 20880 ms | 0 ms | 97273 ms | 109040 ms |
| full | 18 | 89957 ms | 83994 ms | 106813 ms | 114537 ms |

Read the `rtdd` rows against their own means. On flask the mean of 1284 ms is 41% of the p90
and 32% of the worst, and the median cycle costs nothing at all; the worst cycle costs more
than the whole suite's mean run. On httpie the mean of 25.4 s hides a worst cycle of 171 s —
close to twice the full suite. A reader given only the mean would take RTDD for a strategy
that steadily costs a fraction of a run. It is a strategy that usually costs nothing and
occasionally costs more than running everything, and the two halves are the finding.

Duration-weighted aggregate (ordering only — recall is never pooled), from
`bench/results/aggregate.md`:

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
