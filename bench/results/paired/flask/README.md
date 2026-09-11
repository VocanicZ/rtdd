# `paired` commit selection — flask

**This is not the published Axis 2 result.** That is `bench/results/flask/`, produced
under the shipped `recent` rule over 46 commits, and it is what the README's tables,
`aggregate.md` and `outcomes_test.go`'s derived verdict read.

This directory holds a run of `--commit-selection=paired`: only commits touching **both**
a test file and a non-test file. It exists because the published corpus can measure cost
and cannot measure recall — `natural` writes the child's whole diff over the parent, which
reproduces the green child commit and detects nothing in any repo, ever, and under the
`recent` rule `probe` found 3 detecting commits in flask and none at all in httpie or
sqlfluff. Every published recall figure therefore reads `n/a (0/0)`.

| commit selection | replayed | detecting |
|---|---|---|
| `recent` (published) | 23 | 3 |
| `paired` (this run) | 6 | **5** |

## Read this beside the numbers

- **The population is biased, by construction.** These are commits that changed code and
  tests together. That is not the average commit, and no figure here should be read as
  describing one. The filter reads the diff's paths only and never asks what any strategy
  would select, so it cannot bias toward or against RTDD — but it does bias toward commits
  whose tests exercise the code that changed.
- **`probe` is an upper bound**, unchanged by this: every map-based strategy is seeded at
  the child commit, so `rtdd` and `testmon` both know about code the agent would not yet
  have recorded.
- **n = 5 detecting commits, one repository.** It is enough to make recall computable for
  the first time. It is not enough to be a general result.
- **Wall-clock is withheld** (`--no-wallclock`): the run shared a box with other work.
- The tool versions and config digest differ from the published run, so the two are not
  directly comparable cell by cell.

## What it shows

`rtdd` reaches full-suite recall — 5/5 change-level, and 4/4 in the `|F_full| == 1`
stratum where a single failing test is the only thing selection safety is under test
against. So does `testmon`, at a lower selected-duration fraction. The naive `path`
heuristic catches 2 of 5, and `importgraph` catches none.

The pre-registered criterion is still **not met**: it asks for better recall than `path`
*at equal or better selected duration*, and `rtdd` spends 0.757 of the suite's test time
against `path`'s 0.027. The verdict line in `summary.md` says so verbatim.
