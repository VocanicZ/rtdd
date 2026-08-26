# `bench/results/` — committed, diffable benchmark output

This directory is **committed on purpose** and is not gitignored. A benchmark
number that lives only on the machine that produced it cannot be reviewed, so the
results are in git and a re-run at an identical config must produce identical
bytes: `git diff --stat bench/results/<repo_id>/` is the check.

`bench/replay/report.py` writes it. One directory per repo:

```
bench/results/<repo_id>/
  commits.jsonl   one JSON line per record, sorted by (repo_id, commit, variant, kind, strategy)
  summary.json    every published metric, sorted keys, newline-terminated
  summary.md      the published per-repo table, including the `verdict:` line
  config.json     the RunConfig, its digest and the Hardware that produced the other three
  drift.json      the cycles-since-commit curve, only when a drift session was run
bench/results/aggregate.md   the sole cross-repo file — duration-weighted and labelled as such
```

Three rules the layout enforces:

- **No result without its config.** `config.json` is written on every emission and
  carries the corpus digest, the tool versions, the strategy set, the identity of
  the `rtdd` binary under test and the hardware fingerprint.
- **Never pooled.** Recall is never pooled across repos, and `natural` is never
  pooled with `probe` — `probe` seeds map-based strategies at the child commit, so
  its rows are an upper bound and are labelled as one on the same line as the
  numbers.
- **The verdict is unconditional.** Every `summary.md` carries a `verdict:` line
  comparing RTDD against the naive path heuristic, printed win or lose.

`bench/results/.cache/` is the one thing here that *is* gitignored.

## The first published table

`bench/results/flask/` is the first real replay: 25 consecutive commits of `flask` at
the frozen pin, both variants, all eight strategies, on disclosed hardware off CI.
Reproduce it with

```bash
cd bench && uv run python -m replay.cli replay --repo flask --replay-commits 25 --wallclock-sample 10
```

and `git diff --stat bench/results/flask/` must come back empty — a cached, identical
config reproduces the same bytes, selection timings included.

Its verdict fell against RTDD, and
[`docs/results/axis2-first-replay.md`](../../docs/results/axis2-first-replay.md) records
that rather than the run being repeated with different settings: the `natural`
population had no detecting commits at this length, so the pre-registered criterion is
undecided there, and on the `probe` upper bound RTDD wins recall but not at equal or
better selected duration, which is what the criterion asks for.
